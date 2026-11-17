package coverage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

var t0 = time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

func openDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "cov.db"))
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

func seedRun(t *testing.T, w *store.Writer, ctx context.Context) {
	t.Helper()
	const tss = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-c", "a", "m", tss); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`, "run-1", "ws-c", "r", "RUNNING", "d", tss)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

const goodFalsifier = "访问他人对象 id=2 返回 403 或空响应，则 IDOR 越权假设被证伪"

func idorSpec(required ...string) ExperimentSpec {
	return ExperimentSpec{
		AttackType: "idor", IdentityClass: "test_user_a", ParameterName: "id",
		Variable:         Variable{Location: "path", Name: "id", From: "1", To: "2"},
		Falsifier:        goodFalsifier,
		RequiredEvidence: required,
	}
}

func idorCell() Cell {
	return Cell{ModuleID: "mod-1", EndpointID: "ep-1", ParameterName: "id", IdentityClass: "test_user_a", AttackType: "idor"}
}

// 空洞 Falsifier 被拒（§31 V2）。
func TestVacuousFalsifierRejected(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	bad := idorSpec("baseline")
	bad.Falsifier = "若无异常则证伪" // 空洞
	if _, _, err := s.CreateExperiment(ctx, "run-1", bad, t0); !errors.Is(err, ErrVacuousFalsifier) {
		t.Fatalf("空洞 Falsifier 应拒，实得 %v", err)
	}
}

// DoD：IDOR 实验 baseline/variant/ownership 证据齐 + Falsifier 评估 → 格子标 TESTED。
func TestMarkTestedFullIDOR(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	expID, _, err := s.CreateExperiment(ctx, "run-1", idorSpec("baseline", "variant", "ownership"), t0)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range []string{"baseline", "variant", "ownership"} {
		if err := s.RecordEvidence(ctx, expID, ev, t0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.EvaluateFalsifier(ctx, expID, FalsRefuted, t0); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(ctx, expID, t0); err != nil {
		t.Fatalf("证据齐+Falsifier 评估应可验证：%v", err)
	}
	c := idorCell()
	if err := s.EnsureCell(ctx, "run-1", c, 1, t0); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkTested(ctx, "run-1", c, expID, t0); err != nil {
		t.Fatalf("应标 TESTED：%v", err)
	}
	if st, _ := s.CellState(ctx, "run-1", c); st != CellTested {
		t.Fatalf("格子应 TESTED，实得 %s", st)
	}
}

// DoD / §23 M4：只有 Baseline（无 variant）的格子不能标 TESTED。
func TestBaselineOnlyCannotTest(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	// required 只要 baseline，故可验证通过；但无 variant 实证。
	expID, _, _ := s.CreateExperiment(ctx, "run-1", idorSpec("baseline"), t0)
	_ = s.RecordEvidence(ctx, expID, "baseline", t0)
	_ = s.EvaluateFalsifier(ctx, expID, FalsRefuted, t0)
	if err := s.Verify(ctx, expID, t0); err != nil {
		t.Fatalf("baseline 齐应可验证：%v", err)
	}
	c := idorCell()
	_ = s.EnsureCell(ctx, "run-1", c, 1, t0)
	if err := s.MarkTested(ctx, "run-1", c, expID, t0); !errors.Is(err, ErrBaselineOnly) {
		t.Fatalf("只有 baseline 应拒标 TESTED（ErrBaselineOnly），实得 %v", err)
	}
	if st, _ := s.CellState(ctx, "run-1", c); st != CellUntested {
		t.Fatalf("格子应仍 UNTESTED，实得 %s", st)
	}
}

// DoD：Falsifier 未评估 → 不能验证、不计 TESTED（§31 V2）。
func TestFalsifierNotEvaluatedNotCounted(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	expID, _, _ := s.CreateExperiment(ctx, "run-1", idorSpec("baseline", "variant"), t0)
	_ = s.RecordEvidence(ctx, expID, "baseline", t0)
	_ = s.RecordEvidence(ctx, expID, "variant", t0)
	// 不调 EvaluateFalsifier → falsifier_result 仍 not_evaluated。
	if err := s.Verify(ctx, expID, t0); !errors.Is(err, ErrVacuousFalsifier) {
		t.Fatalf("Falsifier 未评估不应验证，实得 %v", err)
	}
}

// 证据不全 → 不能验证。
func TestVerifyRequiresAllEvidence(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	expID, _, _ := s.CreateExperiment(ctx, "run-1", idorSpec("baseline", "variant", "ownership"), t0)
	_ = s.RecordEvidence(ctx, expID, "baseline", t0)
	_ = s.RecordEvidence(ctx, expID, "variant", t0) // 缺 ownership
	_ = s.EvaluateFalsifier(ctx, expID, FalsHeld, t0)
	if err := s.Verify(ctx, expID, t0); !errors.Is(err, ErrEvidenceIncomplete) {
		t.Fatalf("证据不全应拒验证，实得 %v", err)
	}
}

// 维度不一致 → 拒标 TESTED。
func TestCellMismatch(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	expID, _, _ := s.CreateExperiment(ctx, "run-1", idorSpec("variant"), t0)
	_ = s.RecordEvidence(ctx, expID, "variant", t0)
	_ = s.EvaluateFalsifier(ctx, expID, FalsRefuted, t0)
	_ = s.Verify(ctx, expID, t0)
	// 单元格 attack_type 不同（sqli ≠ idor）。
	c := idorCell()
	c.AttackType = "sqli"
	_ = s.EnsureCell(ctx, "run-1", c, 1, t0)
	if err := s.MarkTested(ctx, "run-1", c, expID, t0); !errors.Is(err, ErrCellMismatch) {
		t.Fatalf("attack_type 不一致应 ErrCellMismatch，实得 %v", err)
	}
}

// §6.7：Executor 不能自改锁定字段。
func TestLockedFieldRejected(t *testing.T) {
	w, ctx := openDB(t)
	s := NewStore(w)
	if err := s.ModifyLocked(ctx, "exp-x", "budget"); !errors.Is(err, ErrLockedField) {
		t.Fatalf("改锁定字段应 ErrLockedField，实得 %v", err)
	}
}

// SurfaceRevision 变化 → 受影响端点子矩阵置 STALE（§31 V5）。
func TestStaleOnSurfaceChange(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	c := idorCell()
	_ = s.EnsureCell(ctx, "run-1", c, 1, t0)
	n, err := s.StaleAffectedEndpoints(ctx, "run-1", []string{"ep-1"}, 2, t0)
	if err != nil || n != 1 {
		t.Fatalf("受影响端点应置 STALE 1 行，实得 n=%d err=%v", n, err)
	}
	if st, _ := s.CellState(ctx, "run-1", c); st != CellStale {
		t.Fatalf("格子应 STALE，实得 %s", st)
	}
}

// 风险采样：按 模块风险×攻击适用性 取预算内高权重（§31 V5），非穷举、确定性。
func TestRiskSample(t *testing.T) {
	cells := []CellRisk{
		{Key: "a", ModuleRisk: 0.2, AttackApplicability: 0.5}, // 0.10
		{Key: "b", ModuleRisk: 0.9, AttackApplicability: 0.9}, // 0.81
		{Key: "c", ModuleRisk: 0.5, AttackApplicability: 0.8}, // 0.40
	}
	got := RiskSample(cells, 2)
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Fatalf("风险采样应取 [b,c]，实得 %v", got)
	}
}

// 假设状态机。
func TestHypothesisStateMachine(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	id, err := s.CreateHypothesis(ctx, "run-1", Hypothesis{Statement: "用户对象可越权访问", Falsifier: goodFalsifier}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.TransitionHypothesis(ctx, id, HypConfirmed, t0); err != nil {
		t.Fatalf("OPEN→CONFIRMED 应允许：%v", err)
	}
	if err := s.TransitionHypothesis(ctx, id, HypOpen, t0); !errors.Is(err, ErrBadHypothesisTransition) {
		t.Fatalf("CONFIRMED→OPEN 应拒，实得 %v", err)
	}
}
