package coverage

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"pentagent/internal/domain"
)

// hypothesis.go —— 假设管理（§6.2/§6.7）。假设是可证伪的命题，由 Experiment 验证，结果回写覆盖率。

// 假设状态。
const (
	HypOpen      = "OPEN"
	HypConfirmed = "CONFIRMED"
	HypRefuted   = "REFUTED"
	HypBlocked   = "BLOCKED"
)

var hypTransitions = map[string]map[string]bool{
	HypOpen:    {HypConfirmed: true, HypRefuted: true, HypBlocked: true},
	HypBlocked: {HypOpen: true, HypRefuted: true},
}

// ErrBadHypothesisTransition —— 非法假设状态迁移。
var ErrBadHypothesisTransition = &domain.DomainError{Code: "BAD_HYP_TRANSITION", Msg: "invalid hypothesis state transition"}

// Hypothesis 是一个可证伪命题（§6.2）。
type Hypothesis struct {
	ModuleID       string
	Statement      string
	TrustBoundary  string
	ExpectedImpact string
	Falsifier      string
	StopCondition  string
}

// CreateHypothesis 新建假设（OPEN）。Falsifier 须具体可检（§31 V2）。
func (s *Store) CreateHypothesis(ctx context.Context, runID string, h Hypothesis, now time.Time) (string, error) {
	if strings.TrimSpace(h.Statement) == "" {
		return "", domain.Errorf(domain.ErrInvalid, "hypothesis statement required")
	}
	if h.Falsifier != "" && !validFalsifier(h.Falsifier) {
		return "", ErrVacuousFalsifier
	}
	id := string(domain.NewID())
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO hypotheses (id, run_id, module_id, statement, trust_boundary, state, expected_impact,
			  falsifier, stop_condition, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			id, runID, nz(h.ModuleID), h.Statement, nz(h.TrustBoundary), HypOpen, nz(h.ExpectedImpact),
			nz(h.Falsifier), nz(h.StopCondition), ts(now), ts(now))
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// TransitionHypothesis 迁移假设状态（fail-closed 非法迁移）。
func (s *Store) TransitionHypothesis(ctx context.Context, id, to string, now time.Time) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		var cur string
		e := tx.QueryRowContext(ctx, `SELECT state FROM hypotheses WHERE id=?`, id).Scan(&cur)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "hypothesis "+id)
		}
		if e != nil {
			return e
		}
		if !hypTransitions[cur][to] {
			return domain.Errorf(ErrBadHypothesisTransition, cur+"→"+to)
		}
		_, e = tx.ExecContext(ctx, `UPDATE hypotheses SET state=?, updated_at=? WHERE id=?`, to, ts(now), id)
		return e
	})
}

// HypothesisState 读假设状态。
func (s *Store) HypothesisState(ctx context.Context, id string) (string, error) {
	var st string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT state FROM hypotheses WHERE id=?`, id).Scan(&st)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "hypothesis "+id)
		}
		return e
	})
	return st, err
}
