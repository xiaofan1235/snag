package control_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/control"
	"pentagent/internal/domain"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// job_controller_test.go —— Day 08 §7：Job 控制器把 RECONCILING/取消/对账超时落到 durable 库。
// 命令：go test -tags sqlite_modernc ./internal/control/...（无驱动自动 SKIP）。
//
// 覆盖 DoD（§6）：带未决 OUTCOME_UNKNOWN 的 Job 进 RECONCILING、冻结新 Claim；对账超时后自动
// 终态（CANCELLED + cancel_code=OUTCOME_UNRESOLVED）+ run_partial_reasons 有记录；不永久卡。

func openJobTestDB(t *testing.T) (*store.Writer, *store.Reader, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "job.db")
	w, err := store.OpenWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	rd, err := store.OpenReader(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rd.Close() })
	return w, rd, context.Background()
}

// seedJob 造 workspace+run+job（指定初态）；pendingUnknown>0 时再造相应条数的 OUTCOME_UNKNOWN 动作。
func seedJob(t *testing.T, w *store.Writer, ctx context.Context, jobID, state string, pendingUnknown int) {
	t.Helper()
	const ts = "2026-11-16T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			"ws-1", "acme", "mpk-1", ts); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT OR IGNORE INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
			VALUES (?,?,?,?,?,?)`, "run-1", "ws-1", "r", "RUNNING", "digest", ts); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO jobs (id, run_id, kind, state, created_at, updated_at) VALUES (?,?,?,?,?,?)`,
			jobID, "run-1", "recon", state, ts, ts); e != nil {
			return e
		}
		for i := 0; i < pendingUnknown; i++ {
			if _, e := tx.Exec(`INSERT INTO actions (id, run_id, job_id, tool_name, state, identity_id) VALUES (?,?,?,?,?,?)`,
				string(domain.NewID()), "run-1", jobID, "http", string(domain.ActionOutcomeUnknown), "idn-1"); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func readJobRow(t *testing.T, rd *store.Reader, jobID string) (state string, rev int64, deadline, cancelCode, cancelActor sql.NullString) {
	t.Helper()
	e := rd.DB().QueryRow(
		`SELECT state, revision, reconcile_deadline, cancel_code, cancel_actor FROM jobs WHERE id=?`, jobID).
		Scan(&state, &rev, &deadline, &cancelCode, &cancelActor)
	if e != nil {
		t.Fatal(e)
	}
	return
}

// 主路径：未决 OUTCOME_UNKNOWN → RequestCancel 进 RECONCILING（冻结 Claim）→ 超时自动终态 + run_partial_reasons。
func TestJobReconcileTimeoutTerminalizes(t *testing.T) {
	w, rd, ctx := openJobTestDB(t)
	seedJob(t, w, ctx, "job-1", "RUNNING", 1) // 1 个未决 OUTCOME_UNKNOWN 动作
	c := control.NewJobController(w, control.WithReconcileTimeout(2*time.Second))

	// 取消请求：因存在未决未知动作 → 先进 RECONCILING，不直接终态（§4.1）。
	st, err := c.RequestCancel(ctx, "job-1", domain.CancelUserStop, "user:alice", "手动取消")
	if err != nil {
		t.Fatalf("RequestCancel: %v", err)
	}
	if st != domain.JobReconciling {
		t.Fatalf("有未决动作时取消应进 RECONCILING，得 %s", st)
	}
	state, rev, deadline, cc, _ := readJobRow(t, rd, "job-1")
	if state != "RECONCILING" || !deadline.Valid || deadline.String == "" {
		t.Fatalf("应入 RECONCILING 且写截止时刻，得 state=%s deadline=%v", state, deadline)
	}
	if cc.Valid { // cancel_code 不应提前落定（对账完成才终态）
		t.Errorf("RECONCILING 阶段不应写 cancel_code，得 %q", cc.String)
	}

	// 冻结新 Claim：对 RECONCILING 的 Job 施加 claim 必须被拒（ErrInvalid）。
	if _, e := c.Apply(ctx, "job-1", control.JobEventClaim); !errors.Is(e, domain.ErrInvalid) {
		t.Fatalf("RECONCILING 应冻结 claim（ErrInvalid），得 %v", e)
	}

	// 未到期：扫描不动它。
	nBefore, err := c.ExpireReconciliations(ctx, mustParse(t, deadline.String).Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if nBefore != 0 {
		t.Fatalf("未到期不应终态化，却处理了 %d 个", nBefore)
	}

	// 到期：自动终态化。
	n, err := c.ExpireReconciliations(ctx, mustParse(t, deadline.String).Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("到期应终态化 1 个，得 %d", n)
	}
	state2, rev2, _, cc2, actor2 := readJobRow(t, rd, "job-1")
	if state2 != "CANCELLED" {
		t.Errorf("对账超时应终态 CANCELLED，得 %s", state2)
	}
	if cc2.String != string(domain.CancelOutcomeUnresolved) {
		t.Errorf("cancel_code 应为 OUTCOME_UNRESOLVED，得 %q", cc2.String)
	}
	if actor2.String != "system:reconcile-timeout" {
		t.Errorf("cancel_actor 应标系统对账超时，得 %q", actor2.String)
	}
	if rev2 <= rev {
		t.Errorf("终态化后 revision 应单调递增：%d -> %d", rev, rev2)
	}

	// run_partial_reasons 有记录（丢失覆盖）。
	var prCount int
	var code, entity string
	if e := rd.DB().QueryRow(
		`SELECT COUNT(1) FROM run_partial_reasons WHERE run_id=?`, "run-1").Scan(&prCount); e != nil {
		t.Fatal(e)
	}
	if prCount != 1 {
		t.Fatalf("应记 1 条 run_partial_reasons，得 %d", prCount)
	}
	if e := rd.DB().QueryRow(
		`SELECT code, entity_ref FROM run_partial_reasons WHERE run_id=?`, "run-1").Scan(&code, &entity); e != nil {
		t.Fatal(e)
	}
	if code != string(domain.CancelOutcomeUnresolved) || entity != "job-1" {
		t.Errorf("run_partial_reasons 应 code=OUTCOME_UNRESOLVED entity=job-1，得 code=%s entity=%s", code, entity)
	}

	// 幂等：再次扫描已无 RECONCILING，返回 0（不重复记 reason）。
	n2, err := c.ExpireReconciliations(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Errorf("重复扫描应为 0，得 %d", n2)
	}
}

// 无未决动作：RequestCancel 直接终态 CANCELLED，写具名 cancel_code（取消只影响未来动作）。
func TestJobRequestCancelNoPending(t *testing.T) {
	w, rd, ctx := openJobTestDB(t)
	seedJob(t, w, ctx, "job-2", "RUNNING", 0) // 无未决未知动作
	c := control.NewJobController(w)

	st, err := c.RequestCancel(ctx, "job-2", domain.CancelUserStop, "user:bob", "stop")
	if err != nil {
		t.Fatalf("RequestCancel: %v", err)
	}
	if st != domain.JobCancelled {
		t.Fatalf("无未决动作应直接 CANCELLED，得 %s", st)
	}
	state, _, deadline, cc, _ := readJobRow(t, rd, "job-2")
	if state != "CANCELLED" || cc.String != string(domain.CancelUserStop) {
		t.Errorf("应 CANCELLED + cancel_code=USER_STOP，得 state=%s code=%q", state, cc.String)
	}
	if deadline.Valid {
		t.Errorf("直接取消不应写 reconcile_deadline，得 %q", deadline.String)
	}
}

// Reconcile 对无未决动作的 Job 返回 ErrInvalid（无需对账，不应误入 RECONCILING）。
func TestJobReconcileRejectsWhenNoPending(t *testing.T) {
	w, _, ctx := openJobTestDB(t)
	seedJob(t, w, ctx, "job-3", "RUNNING", 0)
	c := control.NewJobController(w)
	if _, e := c.Reconcile(ctx, "job-3"); !errors.Is(e, domain.ErrInvalid) {
		t.Fatalf("无未决动作 Reconcile 应 ErrInvalid，得 %v", e)
	}
}

// Apply 普通生命周期：PENDING→READY→CLAIMED→RUNNING，revision 单调；非法事件被拒不改库。
func TestJobApplyLifecycle(t *testing.T) {
	w, rd, ctx := openJobTestDB(t)
	seedJob(t, w, ctx, "job-4", "PENDING", 0)
	c := control.NewJobController(w)

	seq := []struct {
		ev   control.JobEvent
		want domain.JobState
	}{
		{control.JobEventReady, domain.JobReady},
		{control.JobEventClaim, domain.JobClaimed},
		{control.JobEventRun, domain.JobRunning},
		{control.JobEventSubmit, domain.JobSubmitted},
		{control.JobEventVerify, domain.JobVerifying},
		{control.JobEventDone, domain.JobDone},
	}
	var prevRev int64
	_, prevRev, _, _, _ = readJobRow(t, rd, "job-4")
	for _, s := range seq {
		got, err := c.Apply(ctx, "job-4", s.ev)
		if err != nil {
			t.Fatalf("Apply %s: %v", s.ev, err)
		}
		if got != s.want {
			t.Fatalf("Apply %s -> %s，期望 %s", s.ev, got, s.want)
		}
		_, rev, _, _, _ := readJobRow(t, rd, "job-4")
		if rev != prevRev+1 {
			t.Fatalf("revision 应 +1：%d -> %d（事件 %s）", prevRev, rev, s.ev)
		}
		prevRev = rev
	}
	// 终态后非法事件被拒。
	if _, e := c.Apply(ctx, "job-4", control.JobEventRetry); !errors.Is(e, domain.ErrInvalid) {
		t.Fatalf("终态 DONE 应拒 retry，得 %v", e)
	}
}

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse deadline %q: %v", s, err)
	}
	return tm
}
