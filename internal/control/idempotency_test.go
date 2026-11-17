package control_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"pentagent/internal/control"
	"pentagent/internal/domain"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// idempotency_test.go —— Day 11 §7：幂等命令账本验收。
// 命令：go test -race -tags sqlite_modernc ./internal/control/...（无驱动自动 SKIP）。
//
// 覆盖 DoD（§6）+ §15.7 + §23 M1「同 Idempotency-Key 重试不产生第二份对象」：
//   - 同 key 重试返回同一对象、fn 不二次执行；并发同 key 只建一个；
//   - expected_revision 冲突 → 稳定错误码；digest 复用 → 稳定错误；
//   - 异步 operation ACCEPTED→RUNNING→SUCCEEDED 可查、终态不可再变；
//   - fn 失败整事务回滚（无命令行、无对象），重试重新执行。

func openCmdDB(t *testing.T) (*store.Writer, *store.Reader, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "cmd.db")
	w, err := store.OpenWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	rd, err := store.OpenReader(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rd.Close() })
	return w, rd, context.Background()
}

func seedCmdRun(t *testing.T, w *store.Writer, ctx context.Context) {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			"ws-1", "acme", "mpk-1", ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT OR IGNORE INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
			VALUES (?,?,?,?,?,?)`, "run-1", "ws-1", "r", "RUNNING", "digest", ts)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// createJobFn 返回一个「建 Job」的 fn，并用 calls 统计真正执行次数。
func createJobFn(calls *int32) func(tx *sql.Tx) ([]control.ResourceRef, error) {
	return func(tx *sql.Tx) ([]control.ResourceRef, error) {
		atomic.AddInt32(calls, 1)
		id := domain.NewID()
		const ts = "2026-11-17T00:00:00Z"
		if _, e := tx.Exec(`INSERT INTO jobs (id, run_id, kind, state, created_at, updated_at) VALUES (?,?,?,?,?,?)`,
			string(id), "run-1", "probe", string(domain.JobPending), ts, ts); e != nil {
			return nil, e
		}
		return []control.ResourceRef{{Kind: "job", ID: id}}, nil
	}
}

func countJobs(t *testing.T, rd *store.Reader) int {
	t.Helper()
	var n int
	if e := rd.DB().QueryRow(`SELECT COUNT(1) FROM jobs`).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

// 同 key 重试返回同一 Job、fn 只执行一次。
func TestReceiveIdempotentReplay(t *testing.T) {
	w, rd, ctx := openCmdDB(t)
	seedCmdRun(t, w, ctx)
	led := control.NewCommandLedger(w, rd)
	var calls int32

	r1, err := led.Receive(ctx, "actor-1", "key-1", "digestA", createJobFn(&calls))
	if err != nil {
		t.Fatalf("Receive#1: %v", err)
	}
	if r1.Replayed || len(r1.Refs) != 1 {
		t.Fatalf("首投不应 replay，应返回 1 ref，得 replayed=%v refs=%d", r1.Replayed, len(r1.Refs))
	}
	r2, err := led.Receive(ctx, "actor-1", "key-1", "digestA", createJobFn(&calls))
	if err != nil {
		t.Fatalf("Receive#2: %v", err)
	}
	if !r2.Replayed {
		t.Errorf("重试应 replay")
	}
	if r2.Refs[0].ID != r1.Refs[0].ID {
		t.Errorf("重试应返回同一 Job：%s vs %s", r1.Refs[0].ID, r2.Refs[0].ID)
	}
	if calls != 1 {
		t.Errorf("fn 应只执行 1 次，得 %d", calls)
	}
	if n := countJobs(t, rd); n != 1 {
		t.Errorf("应只建 1 个 Job，得 %d", n)
	}
}

// 并发同 key：只建一个对象，所有返回同一 ref。
func TestReceiveConcurrentSameKey(t *testing.T) {
	w, rd, ctx := openCmdDB(t)
	seedCmdRun(t, w, ctx)
	led := control.NewCommandLedger(w, rd)
	var calls int32

	const N = 16
	var wg sync.WaitGroup
	ids := make([]domain.ID, N)
	errs := make([]error, N)
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			r, e := led.Receive(ctx, "actor-1", "key-x", "digestA", createJobFn(&calls))
			if e == nil && len(r.Refs) == 1 {
				ids[i] = r.Refs[0].ID
			}
			errs[i] = e
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Fatalf("并发 Receive[%d] 出错: %v", i, e)
		}
	}
	first := ids[0]
	for i, id := range ids {
		if id != first {
			t.Fatalf("并发返回的 ref 应全相同：ids[%d]=%s != %s", i, id, first)
		}
	}
	if calls != 1 {
		t.Errorf("fn 应只执行 1 次，得 %d", calls)
	}
	if n := countJobs(t, rd); n != 1 {
		t.Errorf("并发应只建 1 个 Job，得 %d", n)
	}
}

// 同 key 不同 digest → ErrConflict（key 复用于不同请求体）。
func TestReceiveDigestMismatch(t *testing.T) {
	w, rd, ctx := openCmdDB(t)
	seedCmdRun(t, w, ctx)
	led := control.NewCommandLedger(w, rd)
	var calls int32

	if _, e := led.Receive(ctx, "actor-1", "key-1", "digestA", createJobFn(&calls)); e != nil {
		t.Fatal(e)
	}
	if _, e := led.Receive(ctx, "actor-1", "key-1", "digestB", createJobFn(&calls)); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("同 key 不同 digest 应 ErrConflict，得 %v", e)
	}
	if calls != 1 {
		t.Errorf("digest 不符不应执行 fn，calls 应为 1，得 %d", calls)
	}
}

// fn 失败 → 整事务回滚（无命令行、无对象），重试重新执行成功。
func TestReceiveFnFailureRollback(t *testing.T) {
	w, rd, ctx := openCmdDB(t)
	seedCmdRun(t, w, ctx)
	led := control.NewCommandLedger(w, rd)

	boom := errors.New("boom")
	if _, e := led.Receive(ctx, "actor-1", "key-1", "digestA",
		func(tx *sql.Tx) ([]control.ResourceRef, error) { return nil, boom }); !errors.Is(e, boom) {
		t.Fatalf("应透传 fn 错误，得 %v", e)
	}
	// 无命令行。
	var nCmd int
	if e := rd.DB().QueryRow(`SELECT COUNT(1) FROM commands`).Scan(&nCmd); e != nil {
		t.Fatal(e)
	}
	if nCmd != 0 {
		t.Fatalf("fn 失败后不应记命令行，得 %d", nCmd)
	}
	// 重试（同 key）应真正重新执行并成功。
	var calls int32
	r, e := led.Receive(ctx, "actor-1", "key-1", "digestA", createJobFn(&calls))
	if e != nil {
		t.Fatalf("重试应成功: %v", e)
	}
	if r.Replayed || calls != 1 || countJobs(t, rd) != 1 {
		t.Errorf("重试应重新执行建对象，得 replayed=%v calls=%d jobs=%d", r.Replayed, calls, countJobs(t, rd))
	}
}

// expected_revision 冲突 → 稳定错误码（ErrConflict）。
func TestGuardRevisionConflict(t *testing.T) {
	w, rd, ctx := openCmdDB(t)
	seedCmdRun(t, w, ctx)
	led := control.NewCommandLedger(w, rd)
	var calls int32

	// 先建一个 Job（revision 默认 1）。
	r, err := led.Receive(ctx, "actor-1", "key-1", "digestA", createJobFn(&calls))
	if err != nil {
		t.Fatal(err)
	}
	jobID := r.Refs[0].ID

	// expected_revision=1 命中 → fn 执行。
	okCmd := func(tx *sql.Tx) ([]control.ResourceRef, error) {
		if e := control.GuardRevision(ctx, tx, "jobs", jobID, 1); e != nil {
			return nil, e
		}
		return []control.ResourceRef{{Kind: "job", ID: jobID}}, nil
	}
	if _, e := led.Receive(ctx, "actor-1", "key-mut-ok", "digestM", okCmd); e != nil {
		t.Fatalf("expected_revision=1 应通过，得 %v", e)
	}

	// expected_revision=99 冲突 → 稳定 ErrConflict。
	badCmd := func(tx *sql.Tx) ([]control.ResourceRef, error) {
		if e := control.GuardRevision(ctx, tx, "jobs", jobID, 99); e != nil {
			return nil, e
		}
		return nil, nil
	}
	if _, e := led.Receive(ctx, "actor-1", "key-mut-bad", "digestM2", badCmd); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("expected_revision 不符应 ErrConflict，得 %v", e)
	}
}

// 异步 operation：ACCEPTED→RUNNING→SUCCEEDED 可查；终态不可再变；幂等受理返回同一 operation_id。
func TestOperationLifecycle(t *testing.T) {
	w, rd, ctx := openCmdDB(t)
	seedCmdRun(t, w, ctx)
	led := control.NewCommandLedger(w, rd)

	// 异步命令：fn 建 operation，返回其 ref。
	var opID domain.ID
	asyncFn := func(tx *sql.Tx) ([]control.ResourceRef, error) {
		id, e := control.CreateOperationTx(ctx, tx, "run-1", "scan")
		if e != nil {
			return nil, e
		}
		opID = id
		return []control.ResourceRef{{Kind: "operation", ID: id}}, nil
	}
	r, err := led.Receive(ctx, "actor-1", "op-key", "digestOp", asyncFn)
	if err != nil {
		t.Fatalf("async Receive: %v", err)
	}
	if r.Refs[0].Kind != "operation" {
		t.Fatalf("应返回 operation ref，得 %s", r.Refs[0].Kind)
	}

	op, err := led.GetOperation(ctx, opID)
	if err != nil {
		t.Fatal(err)
	}
	if op.State != control.OpAccepted {
		t.Errorf("初始应 ACCEPTED，得 %s", op.State)
	}

	// 推进 RUNNING → SUCCEEDED。
	if e := led.Transition(ctx, opID, control.OpRunning, nil); e != nil {
		t.Fatal(e)
	}
	if e := led.Transition(ctx, opID, control.OpSucceeded, []control.ResourceRef{{Kind: "finding", ID: "f-1"}}); e != nil {
		t.Fatal(e)
	}
	op, _ = led.GetOperation(ctx, opID)
	if op.State != control.OpSucceeded || len(op.Refs) != 1 || op.Refs[0].ID != "f-1" {
		t.Errorf("应 SUCCEEDED 且带 finding ref，得 state=%s refs=%+v", op.State, op.Refs)
	}

	// 终态不可再变。
	if e := led.Transition(ctx, opID, control.OpRunning, nil); !errors.Is(e, domain.ErrConflict) {
		t.Errorf("终态 operation 推进应 ErrConflict，得 %v", e)
	}

	// 幂等重试返回同一 operation_id、不新建。
	r2, err := led.Receive(ctx, "actor-1", "op-key", "digestOp", asyncFn)
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Replayed || r2.Refs[0].ID != opID {
		t.Errorf("异步命令重试应 replay 同一 operation_id，得 replayed=%v id=%s", r2.Replayed, r2.Refs[0].ID)
	}
	var nOp int
	if e := rd.DB().QueryRow(`SELECT COUNT(1) FROM operations`).Scan(&nOp); e != nil {
		t.Fatal(e)
	}
	if nOp != 1 {
		t.Errorf("应只建 1 个 operation，得 %d", nOp)
	}
}
