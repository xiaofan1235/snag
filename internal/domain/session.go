package domain

import "time"

// session.go —— AgentSession 聚合（§4.1 agent_sessions）。
//
// Agent Session 是逻辑执行上下文，不等于系统进程。seat_id/occupant_generation 建立
// Session 与稳定席位（§27.6）的血缘，便于查询「哪个 Seat 的哪个 Occupant 干的」。
type AgentSession struct {
	ID                       ID           `json:"id"`
	RunID                    ID           `json:"run_id"`
	Role                     string       `json:"role"` // strategist / executor / verifier 等
	JobID                    ID           `json:"job_id,omitempty"`
	AttemptID                ID           `json:"attempt_id,omitempty"`
	SeatID                   ID           `json:"seat_id,omitempty"`
	OccupantGeneration       int64        `json:"occupant_generation"`
	ModelProfile             string       `json:"model_profile"`
	State                    SessionState `json:"state"`
	Generation               int64        `json:"generation"`
	ContextRevision          int64        `json:"context_revision"`
	ProviderSessionReference string       `json:"provider_session_reference,omitempty"`
	CreatedAt                time.Time    `json:"created_at"`
	LastActiveAt             time.Time    `json:"last_active_at"`
}
