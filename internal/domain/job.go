package domain

import (
	"encoding/json"
	"time"
)

// job.go —— Job 聚合（§4.1 jobs）。
//
// Job 是 Run 内的工作单元，由 Strategist 产出、Scheduler 派发、Executor 认领执行。
//
// 取消语义（§4.1，吸收 ARTEX 具名取消原因）：
//   - CancelCode 必须是稳定机器码（见 states.go CancelCode）；
//   - 取消只影响未来动作；已 DISPATCHED 的动作继续进入对账，
//     不得用 CANCELLED 掩盖已发生副作用；
//   - 存在未决 OUTCOME_UNKNOWN 动作的 Job 先进入 RECONCILING，对账完成后才能终态化（§5.2）。
type Job struct {
	ID                 ID              `json:"id"`
	RunID              ID              `json:"run_id"`
	ParentJobID        ID              `json:"parent_job_id,omitempty"`
	Kind               string          `json:"kind"`
	Title              string          `json:"title"`
	SpecJSON           json.RawMessage `json:"spec_json"`
	SpecHash           string          `json:"spec_hash"`
	PolicyRevision     int64           `json:"policy_revision"`
	StrategyEpoch      int64           `json:"strategy_epoch"`
	State              JobState        `json:"state"`
	Priority           int             `json:"priority"`
	Generation         int64           `json:"generation"` // 增代即旧 Attempt 失效（配合 fencing token）
	Revision           int64           `json:"revision"`   // 乐观锁版本
	RequiredCapability string          `json:"required_capability"`
	ClaimedBy          ID              `json:"claimed_by,omitempty"` // 认领它的 AgentSession
	LeaseUntil         *time.Time      `json:"lease_until,omitempty"`
	BudgetJSON         json.RawMessage `json:"budget_json"`
	CancelCode         CancelCode      `json:"cancel_code,omitempty"`
	CancelActor        string          `json:"cancel_actor,omitempty"`
	CancelDetail       string          `json:"cancel_detail,omitempty"`
	CancelRequestedAt  *time.Time      `json:"cancel_requested_at,omitempty"`
	CancelSettledAt    *time.Time      `json:"cancel_settled_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// NewJob 造一个新 Job，初态 PENDING、Generation=1、Revision=1。
func NewJob(runID ID, kind, title string) *Job {
	now := time.Now().UTC()
	return &Job{
		ID:         NewID(),
		RunID:      runID,
		Kind:       kind,
		Title:      title,
		State:      JobPending,
		Generation: 1,
		Revision:   1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
