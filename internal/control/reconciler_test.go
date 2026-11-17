package control_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/control"
	"pentagent/internal/domain"
	"pentagent/internal/outbox"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// reconciler_test.go —— Day 12 §7：启动恢复器验收（§19 顺序 / §5.3 崩溃语义 / §5.0.5 超时）。
// 命令：go test -tags sqlite_modernc ./internal/control/...（无驱动自动 SKIP）。
//
// 覆盖 DoD（§6）+ §23 M1「杀掉 Core 后 Job 可恢复」：
//   - 「DISPATCHED 后杀进程」→ 重启 Reconcile 后动作 OUTCOME_UNKNOWN、Job RECONCILING、
//     Attempt EXPIRED、增代；不重复执行、不误判成功；
//   - CLAIMED 孤儿安全重新上架；Outbox 启动补发；干净启动无副作用（幂等）；
//   - RECONCILING 超时经 SweepTimeouts 终态化（无阻塞态永久卡死）。

func openRecDB(t *testing.T) (*store.Writer, *store.Reader, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "rec.db")
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

func newReconciler(w *store.Writer, rd *store.Reader) *control.Reconciler {
	jobs := control.NewJobController(w)
	pub := outbox.NewPublisher(w, rd, outbox.NewMemBus())
	return control.NewReconciler(w, rd, jobs, pub, control.WithReconcileDeadline(30*time.Second))
}

const recTS = "2026-11-17T00:00:00Z"

func recSeedBase(t *testing.T, w *store.Writer, ctx context.Context) {
	t.Helper()
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			"ws-1", "acme", "mpk-1", recTS); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT OR IGNORE INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
			VALUES (?,?,?,?,?,?)`, "run-1", "ws-1", "r", "RUNNING", "digest", recTS); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT OR IGNORE INTO agent_sessions (id, run_id, role, state, created_at, last_active_at)
			VALUES (?,?,?,?,?,?)`, "sess-1", "run-1", "executor", "ACTIVE", recTS, recTS)
		return e
	}); err != nil {
		t.Fatalf("seed base: %v", err)
	}
}

// seedOrphanJob 造一个「崩溃现场」：指定状态的 Job + 一个 RUNNING 孤儿 Attempt（gen/fencing），
// 可选一个 DISPATCHED 动作。
func seedOrphanJob(t *testing.T, w *store.Writer, ctx context.Context, jobID, jobState string, gen, fencing int64, withDispatchedAction bool) {
	t.Helper()
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT INTO jobs (id, run_id, kind, state, generation, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`,
			jobID, "run-1", "probe", jobState, gen, recTS, recTS); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO attempts (id, job_id, generation, agent_session_id, fencing_token, state, started_at)
			VALUES (?,?,?,?,?,?,?)`, "att-"+jobID, jobID, gen, "sess-1", fencing, string(domain.AttemptRunning), recTS); e != nil {
			return e
		}
		if withDispatchedAction {
			_, e := tx.Exec(`INSERT INTO actions (id, run_id, job_id, attempt_id, tool_name, state, identity_id, dispatched_at)
				VALUES (?,?,?,?,?,?,?,?)`,
				"act-"+jobID, "run-1", jobID, "att-"+jobID, "http", string(domain.ActionDispatched), "idn-1", recTS)
			return e
		}
		return nil
	}); err != nil {
		t.Fatalf("seed orphan %s: %v", jobID, err)
	}
}

// 「DISPATCHED 后杀进程」：Reconcile 后动作 OUTCOME_UNKNOWN、Job RECONCILING、Attempt EXPIRED、增代。
func TestReconcileDispatchedCrash(t *testing.T) {
	w, rd, ctx := openRecDB(t)
	recSeedBase(t, w, ctx)
	seedOrphanJob(t, w, ctx, "job-run", string(domain.JobRunning), 1, 1, true)
	rec := newReconciler(w, rd)

	rep, err := rec.Reconcile(ctx)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if rep.UnknownActions != 1 || rep.JobsReconciling != 1 || rep.OrphanedAttempts != 1 || rep.GenerationsBumped != 1 {
		t.Fatalf("恢复汇总不符: %+v", rep)
	}

	// 动作 OUTCOME_UNKNOWN（§5.3：不得 EXECUTED/RECORDED，不误判成功）。
	var actState string
	if e := rd.DB().QueryRow(`SELECT state FROM actions WHERE id=?`, "act-job-run").Scan(&actState); e != nil {
		t.Fatal(e)
	}
	if actState != string(domain.ActionOutcomeUnknown) {
		t.Errorf("动作应 OUTCOME_UNKNOWN，得 %s", actState)
	}

	// Job RECONCILING + 增代 + 对账截止已设。
	var jState string
	var jGen int64
	var deadline sql.NullString
	if e := rd.DB().QueryRow(`SELECT state, generation, reconcile_deadline FROM jobs WHERE id=?`, "job-run").
		Scan(&jState, &jGen, &deadline); e != nil {
		t.Fatal(e)
	}
	if jState != string(domain.JobReconciling) {
		t.Errorf("Job 应 RECONCILING，得 %s", jState)
	}
	if jGen != 2 {
		t.Errorf("Job 应增代到 2，得 %d", jGen)
	}
	if !deadline.Valid {
		t.Errorf("应设 reconcile_deadline")
	}

	// Attempt EXPIRED（孤儿作废）。
	var attState string
	if e := rd.DB().QueryRow(`SELECT state FROM attempts WHERE id=?`, "att-job-run").Scan(&attState); e != nil {
		t.Fatal(e)
	}
	if attState != string(domain.AttemptExpired) {
		t.Errorf("孤儿 Attempt 应 EXPIRED，得 %s", attState)
	}

	// 增代后旧 Attempt 不再是当代（job.generation=2，gen=1 处有 EXPIRED Attempt）——
	// 死进程持旧 fencing 写入会被 guardFencingTx 判 found=false 拒（当代无 Attempt）。
	var curGenAtt int
	if e := rd.DB().QueryRow(`SELECT COUNT(1) FROM attempts WHERE job_id=? AND generation=?`, "job-run", 2).Scan(&curGenAtt); e != nil {
		t.Fatal(e)
	}
	if curGenAtt != 0 {
		t.Errorf("增代后当代(gen2)应无 Attempt（fencing 失效窗口），得 %d", curGenAtt)
	}
}

// CLAIMED 孤儿（认领后未执行）→ 安全重新上架 READY、清认领、增代。
func TestReconcileClaimedOrphanReoffer(t *testing.T) {
	w, rd, ctx := openRecDB(t)
	recSeedBase(t, w, ctx)
	seedOrphanJob(t, w, ctx, "job-clm", string(domain.JobClaimed), 1, 1, false)
	// 给它设上认领者/租约，验证被清。
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`UPDATE jobs SET claimed_by=?, lease_until=? WHERE id=?`, "sess-1", recTS, "job-clm")
		return e
	}); err != nil {
		t.Fatal(err)
	}
	rec := newReconciler(w, rd)

	rep, err := rec.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.JobsReoffered != 1 || rep.JobsReconciling != 0 {
		t.Fatalf("CLAIMED 孤儿应重新上架 1、RECONCILING 0，得 %+v", rep)
	}
	var st string
	var gen int64
	var claimedBy, lease sql.NullString
	if e := rd.DB().QueryRow(`SELECT state, generation, claimed_by, lease_until FROM jobs WHERE id=?`, "job-clm").
		Scan(&st, &gen, &claimedBy, &lease); e != nil {
		t.Fatal(e)
	}
	if st != string(domain.JobReady) || gen != 2 || claimedBy.Valid || lease.Valid {
		t.Errorf("应 READY/gen2/清认领，得 state=%s gen=%d claimedBy=%v lease=%v", st, gen, claimedBy.Valid, lease.Valid)
	}
}

// 启动补发 Outbox：崩溃前入库未发布的消息，Reconcile 补发。
func TestReconcileDrainsOutbox(t *testing.T) {
	w, rd, ctx := openRecDB(t)
	recSeedBase(t, w, ctx)
	// 入一条未发布 outbox（模拟崩溃前已入库）。
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := outbox.Enqueue(ctx, tx, "run-1", "run.updated", `{"x":1}`)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	rec := newReconciler(w, rd)

	rep, err := rec.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.OutboxDrained != 1 {
		t.Errorf("应补发 1 条 outbox，得 %d", rep.OutboxDrained)
	}
	var published int
	if e := rd.DB().QueryRow(`SELECT published FROM outbox WHERE run_id=?`, "run-1").Scan(&published); e != nil {
		t.Fatal(e)
	}
	if published != 1 {
		t.Errorf("补发后应标 published=1，得 %d", published)
	}
}

// 干净启动（无孤儿）→ 全零汇总，幂等无副作用；二次 Reconcile 亦全零。
func TestReconcileCleanStartIdempotent(t *testing.T) {
	w, rd, ctx := openRecDB(t)
	recSeedBase(t, w, ctx)
	rec := newReconciler(w, rd)

	rep, err := rec.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep != (control.RecoverReport{}) {
		t.Errorf("干净启动应全零，得 %+v", rep)
	}
	// 跑一次恢复后再跑，仍全零（幂等）。
	seedOrphanJob(t, w, ctx, "job-run", string(domain.JobRunning), 1, 1, true)
	if _, e := rec.Reconcile(ctx); e != nil {
		t.Fatal(e)
	}
	rep2, err := rec.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep2 != (control.RecoverReport{}) {
		t.Errorf("二次 Reconcile 应全零（已无 RUNNING 孤儿/DISPATCHED 动作），得 %+v", rep2)
	}
}

// RECONCILING 超时经 SweepTimeouts 终态化（无阻塞态永久卡死，§5.0.5）。
func TestSweepTimeoutsReconcileExpiry(t *testing.T) {
	w, rd, ctx := openRecDB(t)
	recSeedBase(t, w, ctx)
	seedOrphanJob(t, w, ctx, "job-run", string(domain.JobRunning), 1, 1, true)
	rec := newReconciler(w, rd)

	rep, err := rec.Reconcile(ctx)
	if err != nil || rep.JobsReconciling != 1 {
		t.Fatalf("恢复应产生 1 个 RECONCILING，err=%v rep=%+v", err, rep)
	}
	// 未到对账截止：不动。
	s0, _ := rec.SweepTimeouts(ctx, time.Now())
	if s0.ReconcileTimedOut != 0 {
		t.Fatalf("未到期不应终态化，得 %d", s0.ReconcileTimedOut)
	}
	// 远超截止：终态化。
	s1, err := rec.SweepTimeouts(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if s1.ReconcileTimedOut != 1 {
		t.Fatalf("到期应终态化 1 个，得 %d", s1.ReconcileTimedOut)
	}
	var st string
	if e := rd.DB().QueryRow(`SELECT state FROM jobs WHERE id=?`, "job-run").Scan(&st); e != nil {
		t.Fatal(e)
	}
	if st != string(domain.JobCancelled) {
		t.Errorf("对账超时应终态 CANCELLED（OUTCOME_UNRESOLVED），得 %s", st)
	}
}
