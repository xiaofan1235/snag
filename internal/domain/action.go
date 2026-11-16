package domain

import (
	"encoding/json"
	"time"
)

// action.go —— Action 聚合（§4.1 actions）。
//
// 一次具体的、可能产生副作用的动作（一个请求 / 一次工具调用）。
//
// 约束（§4.1）：
//   - IdentityID 必填，与 Evidence、Receipt 三方一致，否则 Finding 晋级失败；
//   - IdempotencyClass 决定 OUTCOME_UNKNOWN 的对账方式：非幂等动作（写请求）未对账前
//     禁止同 Job 增代重发；幂等动作重试必须沿用目标侧 Idempotency Key，不能仅生成新 Action。
type Action struct {
	ID                 ID               `json:"id"`
	RunID              ID               `json:"run_id"`
	JobID              ID               `json:"job_id"`
	AttemptID          ID               `json:"attempt_id"`
	ToolName           string           `json:"tool_name"`
	ActionJSON         json.RawMessage  `json:"action_json"`
	CanonicalDigest    string           `json:"canonical_digest"` // 规范化动作的哈希（幂等/去重依据）
	RiskClass          RiskClass        `json:"risk_class"`
	IdempotencyClass   IdempotencyClass `json:"idempotency_class"`
	State              ActionState      `json:"state"`
	PolicyRevision     int64            `json:"policy_revision"`
	IdentityID         ID               `json:"identity_id"` // 必填，三方一致
	ApprovalID         ID               `json:"approval_id,omitempty"`
	PermitID           ID               `json:"permit_id,omitempty"`
	PermitExpiresAt    *time.Time       `json:"permit_expires_at,omitempty"`
	DispatchedAt       *time.Time       `json:"dispatched_at,omitempty"`
	CompletedAt        *time.Time       `json:"completed_at,omitempty"`
	ExecutionReceiptID ID               `json:"execution_receipt_id,omitempty"`
	FailureClass       string           `json:"failure_class,omitempty"`
}
