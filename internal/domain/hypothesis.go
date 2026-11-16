package domain

import "time"

// hypothesis.go —— Hypothesis 聚合（§4.1 hypotheses）。
//
// 一条可证伪的假设：声明、信任边界、来源证据、期望影响、Falsifier（证伪条件）、
// 最小实验、停止条件。假设驱动实验，实验产出证据，证据晋级为 Finding。
//
// SourceEvidenceIDs 是展示冗余（JSON 数组）；权威引用一律经 finding_evidence 等
// Link 表校验，不以此数组为准（§4.1）。
type Hypothesis struct {
	ID                ID              `json:"id"`
	RunID             ID              `json:"run_id"`
	ModuleID          ID              `json:"module_id"`
	Statement         string          `json:"statement"`
	TrustBoundary     string          `json:"trust_boundary"`
	SourceType        string          `json:"source_type"`
	SourceEvidenceIDs []ID            `json:"source_evidence_ids"`
	State             HypothesisState `json:"state"`
	ExpectedImpact    string          `json:"expected_impact"`
	Falsifier         string          `json:"falsifier"`
	MinimumExperiment string          `json:"minimum_experiment"`
	StopCondition     string          `json:"stop_condition"`
	CreatedByAgent    ID              `json:"created_by_agent"`
	Revision          int64           `json:"revision"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}
