package domain

import (
	"encoding/json"
	"time"
)

// attempt.go —— Attempt 聚合（§4.1 attempts）。
//
// 一次对某 Job 的执行尝试。唯一约束 UNIQUE(job_id, generation)：同一代只能有一个 Attempt。
//
// FencingToken 在 Claim 事务中生成、随 Job.Generation 单调递增，写入 Attempt、Permit、
// ExecutionReceipt 与 Proxy 校验上下文——旧 Attempt 即使进程复活也无法覆盖新 Attempt，
// 其持有的未消费 Permit 在租约过期时全部撤销（§4.1）。
type Attempt struct {
	ID             ID              `json:"id"`
	JobID          ID              `json:"job_id"`
	Generation     int64           `json:"generation"` // 与 Job.Generation 对应；(job_id, generation) 唯一
	AgentSessionID ID              `json:"agent_session_id"`
	ClaimTokenHash string          `json:"claim_token_hash"`
	FencingToken   int64           `json:"fencing_token"`
	State          AttemptState    `json:"state"`
	ProgressJSON   json.RawMessage `json:"progress_json"`
	ResultJSON     json.RawMessage `json:"result_json"`
	StartedAt      time.Time       `json:"started_at"`
	EndedAt        *time.Time      `json:"ended_at,omitempty"`
	FailureClass   string          `json:"failure_class,omitempty"`
	FailureMessage string          `json:"failure_message,omitempty"`
}
