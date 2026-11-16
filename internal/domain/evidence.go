package domain

import (
	"encoding/json"
	"time"
)

// evidence.go —— Evidence 聚合（§4.1 evidence）。
//
// 证据内容不可修改（验证结果放另一张表）。血缘三元组
// (IdentityID, PolicyRevision, ProxyLeaseID) 让 Verifier 与报告导出据此校验
// 「证据确实来自本 Run 的合法执行路径」（§4.1）。
type Evidence struct {
	ID               ID              `json:"id"`
	RunID            ID              `json:"run_id"`
	ActionID         ID              `json:"action_id"`
	Kind             string          `json:"kind"` // request / response / screenshot / log 等
	IdentityID       ID              `json:"identity_id"`
	PolicyRevision   int64           `json:"policy_revision"`
	ProxyLeaseID     ID              `json:"proxy_lease_id"`
	PlainSHA256      string          `json:"plain_sha256"` // 原文哈希（内容寻址、防篡改）
	BlobPath         string          `json:"blob_path"`
	RedactedBlobPath string          `json:"redacted_blob_path,omitempty"`
	ContentType      string          `json:"content_type"`
	SizeBytes        int64           `json:"size_bytes"`
	CapturedAt       time.Time       `json:"captured_at"`
	CaptureSource    string          `json:"capture_source"`
	MetadataJSON     json.RawMessage `json:"metadata_json"`
}
