package findings

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
)

// finding.go —— Finding 晋级状态机（§13.3）。核心：**AI 不能自己宣布漏洞**——Agent 自报的 Finding 不能
// 直接成为 REPORTABLE（§4.3 #7），必须经独立 Verifier 逐级晋级。

// 唯一 FindingState 状态机（§13.3）。
const (
	FindHypothesis    = "HYPOTHESIS"
	FindCandidate     = "CANDIDATE"
	FindNeedsEvidence = "NEEDS_EVIDENCE"
	FindRejected      = "REJECTED"
	FindConfirmed     = "CONFIRMED"
	FindTriaged       = "TRIAGED"
	FindReportable    = "REPORTABLE"
)

// 合法迁移表（含反证自动降级边）。
var findTransitions = map[string]map[string]bool{
	FindHypothesis:    {FindCandidate: true},
	FindCandidate:     {FindNeedsEvidence: true, FindRejected: true, FindConfirmed: true},
	FindNeedsEvidence: {FindConfirmed: true, FindRejected: true, FindCandidate: true},
	FindConfirmed:     {FindTriaged: true, FindNeedsEvidence: true, FindRejected: true}, // 反证→降级
	FindTriaged:       {FindReportable: true, FindNeedsEvidence: true, FindRejected: true},
	FindReportable:    {FindNeedsEvidence: true, FindRejected: true}, // REPORTABLE 可被反证自动降级
}

// 需独立 Verifier（actor=verifier）才能进入的状态——Agent 自报不可直达。
var verifierOnly = map[string]bool{FindConfirmed: true, FindTriaged: true, FindReportable: true}

var (
	// ErrBadFindingTransition —— 非法 Finding 状态迁移。
	ErrBadFindingTransition = &domain.DomainError{Code: "BAD_FINDING_TRANSITION", Msg: "invalid finding state transition (§13.3)"}
	// ErrAgentSelfReport —— Agent 自报试图直达 Verifier-only 状态（§4.3 #7）。
	ErrAgentSelfReport = &domain.DomainError{Code: "AGENT_SELF_REPORT", Msg: "agent self-report cannot promote to verifier-only state (§4.3 #7)"}
)

// Actor 是迁移发起者。
type Actor string

const (
	ActorAgent    Actor = "agent"
	ActorVerifier Actor = "verifier"
)

// CreateCandidate 由 Agent 新建候选 Finding（state=CANDIDATE）。Agent 只能提到 CANDIDATE。
func (s *Store) CreateCandidate(ctx context.Context, runID, hypothesisID, title, vulnType string, now time.Time) (string, error) {
	id := string(domain.NewID())
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO findings (id, run_id, hypothesis_id, title, vulnerability_type, state, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			id, runID, nz(hypothesisID), nz(title), nz(vulnType), FindCandidate, ts(now), ts(now))
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// TransitionFinding 迁移 Finding 状态。actor=agent 不得进入 verifier-only 状态（CONFIRMED/TRIAGED/REPORTABLE）；
// 即便状态机允许该边，Agent 自报也被 ErrAgentSelfReport 拦下（§4.3 #7）。
func (s *Store) TransitionFinding(ctx context.Context, id, to string, actor Actor, now time.Time) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		var cur string
		e := tx.QueryRowContext(ctx, `SELECT state FROM findings WHERE id=?`, id).Scan(&cur)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "finding "+id)
		}
		if e != nil {
			return e
		}
		if !findTransitions[cur][to] {
			return domain.Errorf(ErrBadFindingTransition, cur+"→"+to)
		}
		if actor != ActorVerifier && verifierOnly[to] {
			return domain.Errorf(ErrAgentSelfReport, "to "+to)
		}
		_, e = tx.ExecContext(ctx, `UPDATE findings SET state=?, updated_at=? WHERE id=?`, to, ts(now), id)
		return e
	})
}

// FindingState 读 Finding 状态。
func (s *Store) FindingState(ctx context.Context, id string) (string, error) {
	var st string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT state FROM findings WHERE id=?`, id).Scan(&st)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "finding "+id)
		}
		return e
	})
	return st, err
}
