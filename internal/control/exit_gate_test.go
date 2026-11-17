package control

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

var tg0 = time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

func openGateDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "gate.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	return w, context.Background()
}

// seedRunningRun 造一个 RUNNING Run（门禁默认全过的干净底座）。
func seedRunningRun(t *testing.T, w *store.Writer, ctx context.Context, runID string) {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-g", "a", "m", ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`,
			runID, "ws-g", "r", string(domain.RunRunning), "scope-g", ts)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func runState(t *testing.T, w *store.Writer, ctx context.Context, runID string) string {
	t.Helper()
	var st string
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT state FROM runs WHERE id=?`, runID).Scan(&st)
	})
	return st
}

// DoD：干净 Run（无任何未决项）门禁全过并 COMPLETED。
func TestTryCompleteHappy(t *testing.T) {
	w, ctx := openGateDB(t)
	seedRunningRun(t, w, ctx, "run-1")
	g := NewExitGate(w, GateConfig{})
	res, err := g.TryComplete(ctx, "run-1", tg0)
	if err != nil {
		t.Fatalf("TryComplete: %v", err)
	}
	if !res.Completed || res.State != domain.RunCompleted {
		t.Fatalf("干净 Run 应 COMPLETED，实得 completed=%v state=%s failures=%+v", res.Completed, res.State, res.Gate.Failures)
	}
	if runState(t, w, ctx, "run-1") != string(domain.RunCompleted) {
		t.Fatalf("库状态应 COMPLETED")
	}
}

// DoD：有非终态 Job → 门禁不过，EXITING→RUNNING，不 COMPLETED。
func TestTryCompleteBlockedByJob(t *testing.T) {
	w, ctx := openGateDB(t)
	seedRunningRun(t, w, ctx, "run-2")
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO jobs (id,run_id,kind,state,created_at,updated_at) VALUES (?,?,?,?,?,?)`,
			"job-1", "run-2", "probe", string(domain.JobRunning), "2026-11-17T00:00:00Z", "2026-11-17T00:00:00Z")
		return e
	}); err != nil {
		t.Fatal(err)
	}
	g := NewExitGate(w, GateConfig{})
	res, err := g.TryComplete(ctx, "run-2", tg0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Completed {
		t.Fatalf("有非终态 Job 不应完成")
	}
	if res.State != domain.RunRunning {
		t.Fatalf("门禁不过应回 RUNNING，实得 %s", res.State)
	}
	found := false
	for _, f := range res.Gate.Failures {
		if f.Condition == "no_nonterminal_job" {
			found = true
		}
	}
	if !found {
		t.Fatalf("应报告 no_nonterminal_job 未过，实得 %+v", res.Gate.Failures)
	}
}

// DoD（门禁原子性）：评估期间 SnapshotRevision 前进（并发 PlanPatch）→ 放弃完成，回 RUNNING（§14.2）。
// 用同名条件注入一个「副作用」检查，在评估时推进 current_snapshot_revision，模拟评估窗口内的并发修订漂移。
func TestTryCompleteAbortsOnRevisionDrift(t *testing.T) {
	w, ctx := openGateDB(t)
	seedRunningRun(t, w, ctx, "run-3")
	drift := GateCondition{
		Name: "no_nonterminal_job", // 覆盖默认同名条件
		Check: func(ctx context.Context, tx *sql.Tx, runID string) (bool, string, error) {
			// 模拟并发 PlanPatch 在评估窗口内推进了 snapshot 修订。
			_, e := tx.ExecContext(ctx, `UPDATE runs SET current_snapshot_revision=current_snapshot_revision+1 WHERE id=?`, runID)
			return true, "drift-injected", e
		},
	}
	g := NewExitGate(w, GateConfig{}, drift)
	res, err := g.TryComplete(ctx, "run-3", tg0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Completed {
		t.Fatalf("修订漂移时不应完成")
	}
	if !res.Aborted || res.State != domain.RunRunning {
		t.Fatalf("修订漂移应放弃并回 RUNNING，实得 aborted=%v state=%s info=%s", res.Aborted, res.State, res.AbortInfo)
	}
}

// 预算耗尽 → PARTIAL（不绕门禁），且必须写类型化原因（§14.2）。
func TestPartialWritesTypedReasons(t *testing.T) {
	w, ctx := openGateDB(t)
	seedRunningRun(t, w, ctx, "run-4")
	g := NewExitGate(w, GateConfig{})
	// 空原因被拒。
	if _, err := g.Partial(ctx, "run-4", nil, tg0); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("空 PARTIAL 原因应拒：%v", err)
	}
	st, err := g.Partial(ctx, "run-4", []PartialReason{
		{Code: PartialBudgetExhausted, EntityRef: "budget", LostCoverage: "module:payment"},
	}, tg0)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if st != domain.RunPartial {
		t.Fatalf("应 PARTIAL，实得 %s", st)
	}
	var n int
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM run_partial_reasons WHERE run_id=? AND code=?`, "run-4", PartialBudgetExhausted).Scan(&n)
	})
	if n != 1 {
		t.Fatalf("应写 1 条 run_partial_reasons，实得 %d", n)
	}
}

// Evaluate 只读，不改 Run 状态。
func TestEvaluateReadOnly(t *testing.T) {
	w, ctx := openGateDB(t)
	seedRunningRun(t, w, ctx, "run-5")
	g := NewExitGate(w, GateConfig{})
	if _, err := g.Evaluate(ctx, "run-5"); err != nil {
		t.Fatal(err)
	}
	if runState(t, w, ctx, "run-5") != string(domain.RunRunning) {
		t.Fatalf("Evaluate 不应改状态")
	}
}
