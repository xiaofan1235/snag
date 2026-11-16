package domain

import "time"

// coverage.go —— CoverageCell 聚合（§4.1 coverage_cells、§14 覆盖率模型）。
//
// 覆盖率至少要做到「接口 × 参数 × 身份 × 攻击类型」四维，而不是只有「模块 × 漏洞类型」（§4.1）。
// 每个 Cell 记录某一组合当前的覆盖状态与依据证据。
type CoverageCell struct {
	RunID           ID        `json:"run_id"`
	ModuleID        ID        `json:"module_id"`
	EndpointID      ID        `json:"endpoint_id"`
	ParameterName   string    `json:"parameter_name"`
	IdentityClass   string    `json:"identity_class"`
	AttackType      string    `json:"attack_type"`
	State           string    `json:"state"`
	Rationale       string    `json:"rationale"`
	EvidenceIDs     []ID      `json:"evidence_ids"` // 展示冗余；权威引用经 Link 表（§4.1）
	SurfaceRevision int64     `json:"surface_revision"`
	UpdatedAt       time.Time `json:"updated_at"`
}
