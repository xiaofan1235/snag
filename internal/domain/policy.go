package domain

import (
	"encoding/json"
	"time"
)

// policy.go —— PolicyRevision 聚合（§4.1 policy_revisions）。
//
// 策略不可原地修改：每次修改都产生新 Revision（§4.1）。任何策略变化都立即使未使用的
// Permit 失效；收窄类变更走两阶段切换以约束在途流量（§4.1 两阶段切换）。
type PolicyRevision struct {
	ID                   ID              `json:"id"`
	RunID                ID              `json:"run_id"`
	Revision             int64           `json:"revision"`
	CanonicalJSON        json.RawMessage `json:"canonical_json"`         // 规范化后的策略（JCS/RFC 8785）
	Digest               string          `json:"digest"`                 // CanonicalJSON 的哈希
	SourceDocumentDigest string          `json:"source_document_digest"` // 授权原文指纹
	ApprovedBy           string          `json:"approved_by"`
	CreatedAt            time.Time       `json:"created_at"`
	SupersedesRevision   int64           `json:"supersedes_revision"`
}
