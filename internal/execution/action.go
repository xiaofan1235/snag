// Package execution 承载 Action 的状态机与提案结构（M2 执行与证据核心的开端，§5.3）。
//
// Action 是「真正可能影响外部的动作」的载体；后面的 Permit（Day 17）、Policy Proxy（Day 18）、
// Evidence（Day 20）、Receipt 对账（Day 21）都绕它转。本包 Day 14 先立：
//   - Action 状态机（action_sm.gen.go，§5.3 完整迁移表）；
//   - ActionProposal 提案结构 + canonical_digest（红线/审批/去重的锚，§47.1）；
//   - idempotency_class 驱动的「未对账能否重发」判定（§4.1）。
package execution

import (
	"encoding/json"
	"time"

	"pentagent/internal/canon"
	"pentagent/internal/domain"
)

// ActionProposal 是规划器/模型提出的一个动作提案（进入状态机 PROPOSED 的载荷）。
// 不含 ID/时间等由控制面补全的字段——那些在落库时生成（Day 21）。
type ActionProposal struct {
	ToolName         string                  `json:"tool_name"`
	ActionJSON       json.RawMessage         `json:"action_json"`
	RiskClass        domain.RiskClass        `json:"risk_class"`
	IdentityID       domain.ID               `json:"identity_id"` // 必填，与 Evidence/Receipt 三方一致（§4.1）
	IdempotencyClass domain.IdempotencyClass `json:"idempotency_class"`
}

// CanonicalDigest 计算提案的规范化摘要：canon JCS（RFC 8785）规范化 action_json 后 SHA-256。
// 这是红线判定、审批绑定、去重与三方一致性的锚（§47.1）——同一语义动作无论字段顺序都得同一摘要。
func (p ActionProposal) CanonicalDigest() (string, error) {
	return canon.DigestJSON(p.ActionJSON)
}

// Validate 校验提案的必填约束（§4.1）：IdentityID 必填；action_json 必须是合法 JSON。
func (p ActionProposal) Validate() error {
	if p.IdentityID == "" {
		return domain.Errorf(domain.ErrInvalid, "ActionProposal: identity_id 必填（三方一致，§4.1）")
	}
	if p.ToolName == "" {
		return domain.Errorf(domain.ErrInvalid, "ActionProposal: tool_name 必填")
	}
	if !json.Valid(p.ActionJSON) {
		return domain.Errorf(domain.ErrInvalid, "ActionProposal: action_json 非合法 JSON")
	}
	return nil
}

// NewProposedAction 从提案构造一个 PROPOSED 态的 domain.Action，填入 canonical_digest。
// id/run/job/attempt 由调用方（控制面）给定；其余时间/回执字段在后续状态推进中补。
func NewProposedAction(id, runID, jobID, attemptID domain.ID, p ActionProposal) (domain.Action, error) {
	if err := p.Validate(); err != nil {
		return domain.Action{}, err
	}
	digest, err := p.CanonicalDigest()
	if err != nil {
		return domain.Action{}, err
	}
	return domain.Action{
		ID:               id,
		RunID:            runID,
		JobID:            jobID,
		AttemptID:        attemptID,
		ToolName:         p.ToolName,
		ActionJSON:       p.ActionJSON,
		CanonicalDigest:  digest,
		RiskClass:        p.RiskClass,
		IdempotencyClass: p.IdempotencyClass,
		State:            domain.ActionProposed,
		IdentityID:       p.IdentityID,
	}, nil
}

// CanRedispatchUnreconciled 报告一个处于 OUTCOME_UNKNOWN 的动作，在**对账未完成前**能否重发（§4.1/§967）：
//   - NON_IDEMPOTENT（非幂等写）→ false：未对账前禁止同 Job 增代重发（可能造成重复副作用）；
//   - READ_ONLY / IDEMPOTENT → true：可重试（IDEMPOTENT 的重试必须沿用目标侧 Idempotency Key，
//     不能仅生成新 Action——由重发方负责带上原 key）。
//
// 未知/空类按最保守处理（视同非幂等，禁止重发）。
func CanRedispatchUnreconciled(class domain.IdempotencyClass) bool {
	switch class {
	case domain.IdempotencyReadOnly, domain.IdempotencyIdempotent:
		return true
	default:
		return false
	}
}

// MarkDispatched 把 Action 置 DISPATCHED 并记派发时刻（状态推进的便捷封装；不碰库）。
func MarkDispatched(a *domain.Action, at time.Time) bool {
	to, ok := ActionNext(a.State, ActionEventDispatch)
	if !ok {
		return false
	}
	a.State = to
	t := at.UTC()
	a.DispatchedAt = &t
	return true
}
