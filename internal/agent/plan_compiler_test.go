package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"pentagent/internal/store"
)

// seedRunEpoch 造 workspace + run（指定 current_snapshot_revision）+ strategy_epoch。
func seedRunEpoch(t *testing.T, w *store.Writer, ctx context.Context, runID, epochID string, snapRev int) {
	t.Helper()
	const tss = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-p", "a", "m", tss); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,current_snapshot_revision,created_at) VALUES (?,?,?,?,?,?,?)`,
			runID, "ws-p", "r", "RUNNING", "d", snapRev, tss); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO strategy_epochs (epoch_id,run_id,reason,opened_at) VALUES (?,?,?,?)`, epochID, runID, "init", tss)
		return e
	}); err != nil {
		t.Fatalf("seed run/epoch: %v", err)
	}
}

func pp(snapRev int, raw string) ParsedPlanPatch {
	return ParsedPlanPatch{SnapshotRevision: snapRev, Raw: json.RawMessage(raw)}
}

func runSnapRev(t *testing.T, w *store.Writer, ctx context.Context, runID string) int {
	t.Helper()
	var r int
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT current_snapshot_revision FROM runs WHERE id=?`, runID).Scan(&r)
	})
	return r
}

func jobCountFor(t *testing.T, w *store.Writer, ctx context.Context, runID string) int {
	t.Helper()
	var n int
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE run_id=?`, runID).Scan(&n)
	})
	return n
}

func dirtyOf(t *testing.T, w *store.Writer, ctx context.Context, runID string) int {
	t.Helper()
	var d int
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT strategy_dirty FROM runs WHERE id=?`, runID).Scan(&d)
	})
	return d
}

// DoD：合法 PlanPatch 落库建 Job；同事务 CAS 把 snapshot_revision +1。
func TestCompileAppliesValidPatch(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRunEpoch(t, w, ctx, "run-1", "ep-1", 5)
	pc, err := NewPlanCompiler(w)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"snapshot_revision":5,"operations":[{"op":"create_job","job":{"kind":"DISCOVER","spec":{"t":"a"}}},{"op":"create_job","job":{"kind":"MAP","spec":{"t":"b"}}}]}`
	res, err := pc.Compile(ctx, "run-1", "ep-1", "patch-1", pp(5, raw), t0)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied() || len(res.CreatedJobIDs) != 2 {
		t.Fatalf("合法 patch 应 APPLIED 建 2 Job，实得 %+v", res)
	}
	if runSnapRev(t, w, ctx, "run-1") != 6 {
		t.Fatalf("同事务 CAS 应把 snapshot_revision +1 到 6，实得 %d", runSnapRev(t, w, ctx, "run-1"))
	}
}

// DoD：过期 Snapshot 的 Patch 被拒（CAS 影响 0 行）并置 strategy_dirty。
func TestCompileStaleSnapshotRejectedAndDirty(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRunEpoch(t, w, ctx, "run-1", "ep-1", 5)
	pc, _ := NewPlanCompiler(w)
	raw := `{"snapshot_revision":4,"operations":[{"op":"create_job","job":{"kind":"DISCOVER"}}]}`
	res, err := pc.Compile(ctx, "run-1", "ep-1", "patch-stale", pp(4, raw), t0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != PatchRejectedStale {
		t.Fatalf("过期快照应 REJECTED_STALE，实得 %+v", res)
	}
	if runSnapRev(t, w, ctx, "run-1") != 5 {
		t.Fatal("过期拒绝不应推进 snapshot_revision")
	}
	if dirtyOf(t, w, ctx, "run-1") != 1 {
		t.Fatal("过期拒绝应置 strategy_dirty")
	}
	if jobCountFor(t, w, ctx, "run-1") != 0 {
		t.Fatal("过期拒绝不应建 Job")
	}
}

// DoD：重复 patch_id 幂等返回原结果，不重复建 Job。
func TestCompileIdempotentByPatchID(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRunEpoch(t, w, ctx, "run-1", "ep-1", 5)
	pc, _ := NewPlanCompiler(w)
	raw := `{"snapshot_revision":5,"operations":[{"op":"create_job","job":{"kind":"DISCOVER","spec":{"t":"a"}}}]}`

	r1, _ := pc.Compile(ctx, "run-1", "ep-1", "patch-1", pp(5, raw), t0)
	if !r1.Applied() {
		t.Fatalf("首次应 APPLIED：%+v", r1)
	}
	n := jobCountFor(t, w, ctx, "run-1")

	r2, err := pc.Compile(ctx, "run-1", "ep-1", "patch-1", pp(5, raw), t0)
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Idempotent || r2.Status != PatchApplied {
		t.Fatalf("重复 patch_id 应幂等返回 APPLIED，实得 %+v", r2)
	}
	if jobCountFor(t, w, ctx, "run-1") != n {
		t.Fatal("幂等重提不应重复建 Job")
	}
}

// 依赖成环 → REJECTED_INVALID。
func TestCompileRejectsDependencyCycle(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRunEpoch(t, w, ctx, "run-1", "ep-1", 5)
	pc, _ := NewPlanCompiler(w)
	raw := `{"snapshot_revision":5,"operations":[
		{"op":"create_job","job":{"kind":"DISCOVER","temp_id":"A","depends_on":["B"]}},
		{"op":"create_job","job":{"kind":"MAP","temp_id":"B","depends_on":["A"]}}]}`
	res, _ := pc.Compile(ctx, "run-1", "ep-1", "patch-cyc", pp(5, raw), t0)
	if res.Status != PatchRejectedInvalid || !strings.Contains(res.RejectReason, "cycle") {
		t.Fatalf("成环应 REJECTED_INVALID，实得 %+v", res)
	}
	if jobCountFor(t, w, ctx, "run-1") != 0 {
		t.Fatal("成环拒绝不应建 Job")
	}
}

// 超深度（>3）→ REJECTED_INVALID。
func TestCompileRejectsOverDepth(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRunEpoch(t, w, ctx, "run-1", "ep-1", 5)
	// 造深度 3 的父链 root(1)→j2(2)→j3(3)。
	const tss = "2026-11-17T00:00:00Z"
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		chain := []struct{ id, parent string }{{"root", ""}, {"j2", "root"}, {"j3", "j2"}}
		for _, c := range chain {
			var parent any
			if c.parent != "" {
				parent = c.parent
			}
			if _, e := tx.Exec(`INSERT INTO jobs (id,run_id,parent_job_id,kind,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
				c.id, "run-1", parent, "DISCOVER", "READY", tss, tss); e != nil {
				return e
			}
		}
		return nil
	})
	pc, _ := NewPlanCompiler(w)
	raw := `{"snapshot_revision":5,"operations":[{"op":"create_job","job":{"kind":"FOLLOWUP","parent_job_id":"j3"}}]}`
	res, _ := pc.Compile(ctx, "run-1", "ep-1", "patch-deep", pp(5, raw), t0)
	if res.Status != PatchRejectedInvalid || !strings.Contains(res.RejectReason, "depth") {
		t.Fatalf("超深度应 REJECTED_INVALID，实得 %+v", res)
	}
}

// 超预算（>8 新 Job）→ REJECTED_INVALID。
func TestCompileRejectsOverBudget(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRunEpoch(t, w, ctx, "run-1", "ep-1", 5)
	pc, _ := NewPlanCompiler(w)
	var ops []string
	for i := 0; i < 9; i++ {
		ops = append(ops, fmt.Sprintf(`{"op":"create_job","job":{"kind":"DISCOVER","spec":{"i":%d}}}`, i))
	}
	raw := `{"snapshot_revision":5,"operations":[` + strings.Join(ops, ",") + `]}`
	res, _ := pc.Compile(ctx, "run-1", "ep-1", "patch-big", pp(5, raw), t0)
	if res.Status != PatchRejectedInvalid || !strings.Contains(res.RejectReason, "max new jobs") {
		t.Fatalf("超预算应 REJECTED_INVALID，实得 %+v", res)
	}
	if jobCountFor(t, w, ctx, "run-1") != 0 {
		t.Fatal("超预算拒绝不应建 Job")
	}
}

// 指纹去重：同 kind+spec 只建一个。
func TestCompileDedupBySpecHash(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRunEpoch(t, w, ctx, "run-1", "ep-1", 5)
	pc, _ := NewPlanCompiler(w)
	raw := `{"snapshot_revision":5,"operations":[
		{"op":"create_job","job":{"kind":"DISCOVER","spec":{"t":"same"}}},
		{"op":"create_job","job":{"kind":"DISCOVER","spec":{"t":"same"}}}]}`
	res, _ := pc.Compile(ctx, "run-1", "ep-1", "patch-dup", pp(5, raw), t0)
	if !res.Applied() || len(res.CreatedJobIDs) != 1 {
		t.Fatalf("同指纹应去重只建 1 个，实得 %+v", res)
	}
}

// 战役计划读写（§32.4）。
func TestCampaignPlanRoundTrip(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRunEpoch(t, w, ctx, "run-1", "ep-1", 5)
	pc, _ := NewPlanCompiler(w)
	plan := json.RawMessage(`{"directions":[{"name":"auth","status":"active"}]}`)
	dig, err := pc.UpsertCampaignPlan(ctx, "run-1", "ep-1", plan, t0)
	if err != nil || dig == "" {
		t.Fatalf("写战役计划失败：%v", err)
	}
	got, gotDig, err := pc.LatestCampaignPlan(ctx, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if gotDig != dig || !strings.Contains(string(got), "auth") {
		t.Fatalf("读回战役计划不符：%s", got)
	}
}
