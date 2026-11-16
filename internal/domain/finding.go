package domain

import (
	"encoding/json"
	"time"
)

// finding.go —— Finding 聚合与 FindingEvidence 链接表（§4.1 findings / finding_evidence）。
//
// Finding 是从 Hypothesis 晋级来的漏洞发现。证据通过 FindingEvidence 关联，
// 复合外键 (run_id, evidence_id) 从 schema 层禁止跨 Run 引用证据（§4.1）。
type Finding struct {
	ID                 ID              `json:"id"`
	RunID              ID              `json:"run_id"`
	HypothesisID       ID              `json:"hypothesis_id"`
	Title              string          `json:"title"`
	VulnerabilityType  string          `json:"vulnerability_type"`
	Severity           string          `json:"severity"`
	State              FindingState    `json:"state"`
	AffectedAssetsJSON json.RawMessage `json:"affected_assets_json"`
	ImpactJSON         json.RawMessage `json:"impact_json"`
	TriageResultJSON   json.RawMessage `json:"triage_result_json"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// FindingEvidence —— Finding 与 Evidence 的链接（§4.1 finding_evidence）。
// 权威引用表：复合外键 (run_id, evidence_id) -> evidence(run_id, id)，禁止跨 Run 引用。
type FindingEvidence struct {
	RunID      ID                  `json:"run_id"`
	FindingID  ID                  `json:"finding_id"`
	EvidenceID ID                  `json:"evidence_id"`
	Role       FindingEvidenceRole `json:"role"`
}
