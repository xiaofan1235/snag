package coverage

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"pentagent/internal/domain"
)

// matrix.go —— 覆盖率矩阵 + TESTED 绑定校验（§14.1）+ SurfaceRevision 失效（§31 V5）+ 风险采样（§31 V5）。

// 单元格状态。
const (
	CellUntested = "UNTESTED"
	CellTested   = "TESTED"
	CellNA       = "NA"
	CellStale    = "STALE"
)

// ErrBaselineOnly —— 只有 Baseline（无 variant 实证）不能标 TESTED（§14.1）。
var ErrBaselineOnly = &domain.DomainError{Code: "BASELINE_ONLY", Msg: "only baseline, cannot mark TESTED (§14.1)"}

// ErrNotVerifiedExperiment —— 绑定的实验未验证，不能标 TESTED。
var ErrNotVerifiedExperiment = &domain.DomainError{Code: "EXPERIMENT_NOT_VERIFIED", Msg: "cell TESTED requires a VERIFIED experiment (§14.1)"}

// ErrCellMismatch —— 实验的 attack_type/identity/parameter 与单元格不一致。
var ErrCellMismatch = &domain.DomainError{Code: "CELL_MISMATCH", Msg: "experiment does not match cell dimensions"}

// Cell 是覆盖率矩阵的一个单元格坐标。
type Cell struct {
	ModuleID      string
	EndpointID    string
	ParameterName string
	IdentityClass string
	AttackType    string
}

// EnsureCell 确保单元格存在（初始 UNTESTED，记录 surface_revision）。
func (s *Store) EnsureCell(ctx context.Context, runID string, c Cell, surfaceRev int, now time.Time) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO coverage_cells
			  (run_id, module_id, endpoint_id, parameter_name, identity_class, attack_type, state, surface_revision, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			runID, c.ModuleID, c.EndpointID, c.ParameterName, c.IdentityClass, c.AttackType, CellUntested, surfaceRev, ts(now))
		return e
	})
}

// MarkTested 把单元格标 TESTED，严格按 §14.1 绑定校验：
//   - 绑定实验必须 VERIFIED（含 Falsifier 已真实评估，§31 V2，由 Verify 保证）；
//   - 实验维度（attack_type/identity/parameter）与单元格一致；
//   - 必须有 variant 实证——只有 baseline 不能标 TESTED（§14.1）。
//
// rationale 记 cell_id+spec_hash+attack_type+identity+parameter+falsifier 结果。
func (s *Store) MarkTested(ctx context.Context, runID string, c Cell, expID string, now time.Time) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		var state, specHash, at, idc, pn, fr, collected string
		e := tx.QueryRowContext(ctx, `
			SELECT state, spec_hash, attack_type, COALESCE(identity_class,''), COALESCE(parameter_name,''),
			       falsifier_result, COALESCE(collected_evidence,'')
			FROM experiments WHERE id=?`, expID).Scan(&state, &specHash, &at, &idc, &pn, &fr, &collected)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "experiment "+expID)
		}
		if e != nil {
			return e
		}
		// 1) 已验证（Verify 已保证 Falsifier 评估过 + 证据齐）。
		if state != ExpVerified {
			return ErrNotVerifiedExperiment
		}
		// 2) Falsifier 真实评估过（双重兜底，§31 V2）。
		if fr == FalsNotEvaluated {
			return domain.Errorf(ErrVacuousFalsifier, "falsifier not evaluated")
		}
		// 3) 维度一致。
		if at != c.AttackType || (idc != "" && idc != c.IdentityClass) || (pn != "" && pn != c.ParameterName) {
			return ErrCellMismatch
		}
		// 4) 必须有 variant 实证（只有 baseline 不能 TESTED，§14.1）。
		set := splitSet(collected)
		if !set["variant"] {
			return ErrBaselineOnly
		}

		rationale := "cell_tested spec_hash=" + specHash + " attack=" + at + " identity=" + c.IdentityClass + " param=" + c.ParameterName + " falsifier=" + fr
		r, e := tx.ExecContext(ctx, `
			UPDATE coverage_cells SET state=?, rationale=?, updated_at=?
			WHERE run_id=? AND module_id=? AND endpoint_id=? AND parameter_name=? AND identity_class=? AND attack_type=?`,
			CellTested, rationale, ts(now), runID, c.ModuleID, c.EndpointID, c.ParameterName, c.IdentityClass, c.AttackType)
		if e != nil {
			return e
		}
		if n, _ := r.RowsAffected(); n == 0 {
			return domain.Errorf(domain.ErrNotFound, "cell not found; EnsureCell first")
		}
		return nil
	})
}

// MarkNA 把单元格标 N/A（须带确定性理由，不接受空理由自报，§14.1）。
func (s *Store) MarkNA(ctx context.Context, runID string, c Cell, rationale string, now time.Time) error {
	if strings.TrimSpace(rationale) == "" {
		return domain.Errorf(domain.ErrInvalid, "N/A requires deterministic rationale (§14.1)")
	}
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			UPDATE coverage_cells SET state=?, rationale=?, updated_at=?
			WHERE run_id=? AND module_id=? AND endpoint_id=? AND parameter_name=? AND identity_class=? AND attack_type=?`,
			CellNA, rationale, ts(now), runID, c.ModuleID, c.EndpointID, c.ParameterName, c.IdentityClass, c.AttackType)
		return e
	})
}

// CellState 读单元格状态。
func (s *Store) CellState(ctx context.Context, runID string, c Cell) (string, error) {
	var st string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `
			SELECT state FROM coverage_cells
			WHERE run_id=? AND module_id=? AND endpoint_id=? AND parameter_name=? AND identity_class=? AND attack_type=?`,
			runID, c.ModuleID, c.EndpointID, c.ParameterName, c.IdentityClass, c.AttackType).Scan(&st)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "cell")
		}
		return e
	})
	return st, err
}

// StaleAffectedEndpoints 在 SurfaceRevision 变化时，把受影响端点的已测/未测单元格置 STALE（§31 V5：只失效
// 受影响子矩阵，不整体清零）。返回置 STALE 的行数。
func (s *Store) StaleAffectedEndpoints(ctx context.Context, runID string, endpointIDs []string, newSurfaceRev int, now time.Time) (int, error) {
	if len(endpointIDs) == 0 {
		return 0, nil
	}
	total := 0
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		for _, ep := range endpointIDs {
			r, e := tx.ExecContext(ctx, `
				UPDATE coverage_cells SET state=?, surface_revision=?, updated_at=?
				WHERE run_id=? AND endpoint_id=? AND state IN (?,?)`,
				CellStale, newSurfaceRev, ts(now), runID, ep, CellTested, CellUntested)
			if e != nil {
				return e
			}
			n, _ := r.RowsAffected()
			total += int(n)
		}
		return nil
	})
	return total, err
}

// CellRisk 是风险采样的输入权重（§31 V5）。
type CellRisk struct {
	Key                 string
	ModuleRisk          float64 // 模块风险
	AttackApplicability float64 // 攻击类型适用性
}

// RiskSample 风险优先采样（§31 V5）：按 模块风险 × 攻击适用性 加权，取预算内的高权重单元格，非穷举。
// 确定性：同权重按 Key 稳定排序。
func RiskSample(cells []CellRisk, budget int) []string {
	cp := make([]CellRisk, len(cells))
	copy(cp, cells)
	sort.SliceStable(cp, func(i, j int) bool {
		wi, wj := cp[i].ModuleRisk*cp[i].AttackApplicability, cp[j].ModuleRisk*cp[j].AttackApplicability
		if wi != wj {
			return wi > wj
		}
		return cp[i].Key < cp[j].Key
	})
	if budget < 0 {
		budget = 0
	}
	out := make([]string, 0, budget)
	for i := 0; i < len(cp) && i < budget; i++ {
		out = append(out, cp[i].Key)
	}
	return out
}
