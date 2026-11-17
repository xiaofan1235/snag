// Package coverage 实现假设、单变量实验与覆盖率矩阵（§6.7/§14.1/§31 V2/V5）。
//
// 核心思想：把「自由发挥」变成「可证伪的实验」，并如实记覆盖——TESTED 绑定到已验证 Experiment +
// Falsifier 结果，只有 Baseline GET 的格子不能标 TESTED（§14.1）。
package coverage

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"pentagent/internal/canon"
	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// 实验状态。
const (
	ExpProposed = "PROPOSED"
	ExpRunning  = "RUNNING"
	ExpVerified = "VERIFIED"
	ExpFailed   = "FAILED"
)

// Falsifier 结果（§31 V2）。
const (
	FalsNotEvaluated = "not_evaluated"
	FalsHeld         = "held"    // 证伪条件成立（假设被证伪）
	FalsRefuted      = "refuted" // 证伪条件未触发（假设存活）
)

var (
	// ErrVacuousFalsifier —— Falsifier 空洞（§31 V2：不接受「若无异常则证伪」式文本）。
	ErrVacuousFalsifier = &domain.DomainError{Code: "VACUOUS_FALSIFIER", Msg: "falsifier must be concrete/checkable (§31 V2)"}
	// ErrLockedField —— 试图自改 §6.7 锁定字段（身份上限/对象归属/预算/范围/风险等级）。
	ErrLockedField = &domain.DomainError{Code: "LOCKED_FIELD", Msg: "executor cannot modify locked experiment field (§6.7)"}
	// ErrEvidenceIncomplete —— 必需证据未齐，不能验证。
	ErrEvidenceIncomplete = &domain.DomainError{Code: "EVIDENCE_INCOMPLETE", Msg: "required evidence not all collected"}
)

// Variable 是单变量（§6.7）。
type Variable struct {
	Location string `json:"location"`
	Name     string `json:"name"`
	From     string `json:"from"`
	To       string `json:"to"`
}

// ExperimentSpec 是一个单变量实验规格（§6.7）。
type ExperimentSpec struct {
	HypothesisID        string
	ModuleID            string
	EndpointID          string
	AttackType          string
	IdentityClass       string
	ParameterName       string
	Variable            Variable
	ExpectedObservation string
	Falsifier           string
	RequiredEvidence    []string // 如 baseline/variant/ownership
	// §6.7 锁定字段（创建后冻结，Executor 不可自改）。
	IdentityCap int
	BudgetReqs  int
	RiskLevel   string
}

// vacuousPatterns 是空洞 Falsifier 的启发式黑名单（§31 V2）。
var vacuousPatterns = []string{"若无异常", "如果没有异常", "没有报错", "no error", "if nothing", "无明显", "看起来正常", "seems fine"}

// validFalsifier 报告 Falsifier 是否具体可检（非空、够长、不含空洞模式）。
func validFalsifier(f string) bool {
	f = strings.TrimSpace(f)
	if len([]rune(f)) < 8 {
		return false
	}
	low := strings.ToLower(f)
	for _, p := range vacuousPatterns {
		if strings.Contains(f, p) || strings.Contains(low, strings.ToLower(p)) {
			return false
		}
	}
	return true
}

// Store 持久化实验/覆盖率。
type Store struct{ w *store.Writer }

// NewStore 构造。
func NewStore(w *store.Writer) *Store { return &Store{w: w} }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func nz(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// SpecHash 计算 ExperimentSpec 的 JCS 摘要（TESTED 绑定键之一）。
func SpecHash(spec ExperimentSpec) string {
	obj := map[string]any{
		"hypothesis_id": spec.HypothesisID, "module_id": spec.ModuleID, "endpoint_id": spec.EndpointID,
		"attack_type": spec.AttackType, "identity_class": spec.IdentityClass, "parameter_name": spec.ParameterName,
		"variable": map[string]string{"location": spec.Variable.Location, "name": spec.Variable.Name, "from": spec.Variable.From, "to": spec.Variable.To},
	}
	h, _ := canon.Digest(obj)
	return h
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CreateExperiment 创建实验（PROPOSED）。Falsifier 空洞直接拒（§31 V2）。返回 (id, spec_hash)。
func (s *Store) CreateExperiment(ctx context.Context, runID string, spec ExperimentSpec, now time.Time) (string, string, error) {
	if !validFalsifier(spec.Falsifier) {
		return "", "", ErrVacuousFalsifier
	}
	if strings.TrimSpace(spec.AttackType) == "" {
		return "", "", domain.Errorf(domain.ErrInvalid, "attack_type required")
	}
	id := string(domain.NewID())
	hash := SpecHash(spec)
	reqEv := strings.Join(spec.RequiredEvidence, ",")
	varJSON := `{"location":"` + spec.Variable.Location + `","name":"` + spec.Variable.Name + `","from":"` + spec.Variable.From + `","to":"` + spec.Variable.To + `"}`
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO experiments (id, run_id, hypothesis_id, module_id, endpoint_id, spec_hash, attack_type,
			  identity_class, parameter_name, variable_json, expected_observation, falsifier, required_evidence,
			  collected_evidence, state, falsifier_result, locked_identity_cap, locked_budget_reqs, locked_risk_level,
			  created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, runID, nz(spec.HypothesisID), nz(spec.ModuleID), nz(spec.EndpointID), hash, spec.AttackType,
			nz(spec.IdentityClass), nz(spec.ParameterName), varJSON, nz(spec.ExpectedObservation), spec.Falsifier,
			nz(reqEv), "", ExpProposed, FalsNotEvaluated, spec.IdentityCap, spec.BudgetReqs, nz(spec.RiskLevel),
			ts(now), ts(now))
		return e
	})
	if err != nil {
		return "", "", err
	}
	return id, hash, nil
}

// RecordEvidence 记录某实验实际采集到的一类证据（baseline/variant/ownership…），幂等累加。
func (s *Store) RecordEvidence(ctx context.Context, expID, evidenceType string, now time.Time) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		var cur string
		if e := tx.QueryRowContext(ctx, `SELECT COALESCE(collected_evidence,'') FROM experiments WHERE id=?`, expID).Scan(&cur); e != nil {
			if e == sql.ErrNoRows {
				return domain.Errorf(domain.ErrNotFound, "experiment "+expID)
			}
			return e
		}
		set := splitSet(cur)
		if set[evidenceType] {
			return nil // 幂等
		}
		if cur != "" {
			cur += ","
		}
		cur += evidenceType
		_, e := tx.ExecContext(ctx, `UPDATE experiments SET collected_evidence=?, state=?, updated_at=? WHERE id=?`,
			cur, ExpRunning, ts(now), expID)
		return e
	})
}

// EvaluateFalsifier 记录 Falsifier 的真实评估结果（held/refuted）——确定性校验其被真实评估过（§31 V2）。
func (s *Store) EvaluateFalsifier(ctx context.Context, expID, result string, now time.Time) error {
	if result != FalsHeld && result != FalsRefuted {
		return domain.Errorf(domain.ErrInvalid, "falsifier result must be held/refuted")
	}
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		r, e := tx.ExecContext(ctx, `UPDATE experiments SET falsifier_result=?, updated_at=? WHERE id=?`, result, ts(now), expID)
		if e != nil {
			return e
		}
		if n, _ := r.RowsAffected(); n == 0 {
			return domain.Errorf(domain.ErrNotFound, "experiment "+expID)
		}
		return nil
	})
}

// Verify 把实验转 VERIFIED：要求必需证据全采齐 + Falsifier 已真实评估（§31 V2）。否则拒。
func (s *Store) Verify(ctx context.Context, expID string, now time.Time) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		var req, got, fr string
		e := tx.QueryRowContext(ctx,
			`SELECT COALESCE(required_evidence,''), COALESCE(collected_evidence,''), falsifier_result FROM experiments WHERE id=?`, expID).
			Scan(&req, &got, &fr)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "experiment "+expID)
		}
		if e != nil {
			return e
		}
		if fr == FalsNotEvaluated {
			return domain.Errorf(ErrVacuousFalsifier, "falsifier not evaluated, cannot verify (§31 V2)")
		}
		gotSet := splitSet(got)
		for _, r := range strings.Split(req, ",") {
			r = strings.TrimSpace(r)
			if r != "" && !gotSet[r] {
				return domain.Errorf(ErrEvidenceIncomplete, "missing evidence: "+r)
			}
		}
		_, e = tx.ExecContext(ctx, `UPDATE experiments SET state=?, updated_at=? WHERE id=?`, ExpVerified, ts(now), expID)
		return e
	})
}

// ModifyLocked 任何对 §6.7 锁定字段的修改一律拒绝（Executor 不能自改身份上限/对象归属/预算/范围/风险等级）。
func (s *Store) ModifyLocked(ctx context.Context, expID, field string) error {
	return domain.Errorf(ErrLockedField, field)
}

func splitSet(csv string) map[string]bool {
	m := map[string]bool{}
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			m[p] = true
		}
	}
	return m
}
