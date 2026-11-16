// Package canon 是全系统**唯一**的 JSON 规范化与摘要实现（§47.1 规范化总规）。
//
// 不变量（红线/审批/去重的立身之本，§47.5）：
//
//	任何两处要比较或引用同一对象，必须用同一 digest 的同一规范化。
//	相同内容的对象，无论键序/数字写法如何，Digest 必须相等。
//
// 规范化用 RFC 8785 (JCS)：键按 UTF-16 码元排序、数字按 ECMAScript 规则归一、去除无意义空白，
// 再 SHA-256。数字归一是最易出错处，故采用成熟库 github.com/gowebpki/jcs，不自行实现。
package canon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/gowebpki/jcs"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Digest 登记表（§47.1，单一规范）——每处要哈希的对象用哪种规范化，新增需哈希对象先登记再实现：
//
//	canonical_action_digest  动作规范化 JSON(JCS)   —— 红线/审批/去重的锚
//	policy_digest            PolicyRevision(JCS)
//	snapshot_digest          StrategySnapshot(JCS)
//	spec_hash                JobSpec(JCS)
//	request_digest           渲染给模型的请求(JCS, Redacted)
//	manifest_digest          工具/Skill Manifest(JCS)
//	content_hash             知识/资源文件原样 SHA-256（不走 JCS，用 RawSHA256）
//	plain_sha256             Evidence = CEBS 的 SHA-256（§13）
//	source_document_digest   授权材料原样 SHA-256（RawSHA256）
//
// 下列常量是 digest 用途标签，供调用处标注“这是哪种 digest”，避免混用：
const (
	DigestKindAction   = "canonical_action_digest"
	DigestKindPolicy   = "policy_digest"
	DigestKindSnapshot = "snapshot_digest"
	DigestKindSpec     = "spec_hash"
	DigestKindRequest  = "request_digest"
	DigestKindManifest = "manifest_digest"
)

// Canonicalize 把一段合法 JSON 转为 RFC 8785 (JCS) 规范形式。非法 JSON / 重复键会报错。
func Canonicalize(jsonBytes []byte) ([]byte, error) {
	out, err := jcs.Transform(jsonBytes)
	if err != nil {
		return nil, fmt.Errorf("canon: JCS transform: %w", err)
	}
	return out, nil
}

// DigestJSON 返回 SHA-256(JCS(jsonBytes)) 的小写 hex。
func DigestJSON(jsonBytes []byte) (string, error) {
	c, err := Canonicalize(jsonBytes)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(c)
	return hex.EncodeToString(sum[:]), nil
}

// Digest 对任意 Go 值返回其规范化摘要：json.Marshal → JCS → SHA-256 hex。
//
// 注意（§47.1）：传入对象**不应包含时间字段**——时间只用于比较，不入内容 digest，
// 以免时间扰动哈希使“同内容对时不对”。整数请用 Go 整型传入（json.Marshal 保精度），
// 避免先经 map[string]any 使大整数退化为 float64。
func Digest(obj any) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("canon: marshal: %w", err)
	}
	return DigestJSON(b)
}

// RawSHA256 对二进制或“原样”内容直接 SHA-256（content_hash / source_document_digest 等不走 JCS 的场景）。
func RawSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ValidateAgainst 用给定 JSON Schema 校验一段 JSON 实例；不合法返回非 nil error（含失败细节）。
func ValidateAgainst(schemaBytes, instanceBytes []byte) error {
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	if err != nil {
		return fmt.Errorf("canon: parse schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	const loc = "mem://schema.json"
	if err := c.AddResource(loc, schemaDoc); err != nil {
		return fmt.Errorf("canon: add schema: %w", err)
	}
	sch, err := c.Compile(loc)
	if err != nil {
		return fmt.Errorf("canon: compile schema: %w", err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(instanceBytes))
	if err != nil {
		return fmt.Errorf("canon: parse instance: %w", err)
	}
	if err := sch.Validate(inst); err != nil {
		return err // *jsonschema.ValidationError，含具体失败路径
	}
	return nil
}
