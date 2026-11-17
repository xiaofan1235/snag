package findings

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"pentagent/internal/domain"
)

// fingerprint.go —— 技术栈指纹（§12.3）。核心不变量：**技术栈推断必须有 EvidenceRef，不能把经验猜测
// 升级成事实**。无证据的指纹在应用层直接拒（ErrNoEvidence），库层 evidence_ref 也设 NOT NULL 兜底。

// 指纹类别（§12.3 输出维度）。
const (
	FpFramework       = "framework"            // 框架及证据
	FpDataLayer       = "data_layer"           // 数据层可能性及证据
	FpAuthStructure   = "auth_structure"       // 鉴权结构
	FpParamBinding    = "param_binding"        // 参数绑定风格
	FpObjectOwnership = "object_ownership"     // 业务对象归属模式
	FpAttackSurface   = "attack_surface"       // 最相关攻击面
	FpAvoidDirection  = "avoid_direction"      // 不建议浪费预算的方向
	FpUnverified      = "unverified_inference" // 仍需验证的推断
)

// ErrNoEvidence —— 指纹缺 EvidenceRef（§12.3，不把猜测升级成事实）。
var ErrNoEvidence = &domain.DomainError{Code: "NO_EVIDENCE", Msg: "tech fingerprint requires EvidenceRef (§12.3)"}

// Fingerprint 是一条技术栈指纹。EvidenceRef 必填。
type Fingerprint struct {
	ModuleID    string
	Category    string
	Value       string
	EvidenceRef string // 必填：指纹来源证据（正常响应/错误格式/Cookie/鉴权流程/JS 框架…）
	Confidence  string // high/medium/low（不影响「必须有证据」这一硬约束）
}

// AddFingerprint 入库一条技术栈指纹。EvidenceRef 为空直接拒（ErrNoEvidence）——猜测不入库。
func (s *Store) AddFingerprint(ctx context.Context, runID string, fp Fingerprint, now time.Time) (string, error) {
	if strings.TrimSpace(fp.EvidenceRef) == "" {
		return "", ErrNoEvidence // §12.3 硬约束：无证据不生成指纹
	}
	if strings.TrimSpace(fp.Category) == "" || strings.TrimSpace(fp.Value) == "" {
		return "", domain.Errorf(domain.ErrInvalid, "fingerprint category/value required")
	}
	id := string(domain.NewID())
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO tech_fingerprints (id, run_id, module_id, category, value, evidence_ref, confidence, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			id, runID, nz(fp.ModuleID), fp.Category, fp.Value, fp.EvidenceRef, nz(fp.Confidence), ts(now))
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// Fingerprints 列出某 Run 的指纹（测试/报告用）。每条必带 evidence_ref。
func (s *Store) Fingerprints(ctx context.Context, runID string) ([]Fingerprint, error) {
	var out []Fingerprint
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx,
			`SELECT COALESCE(module_id,''), category, value, evidence_ref, COALESCE(confidence,'')
			 FROM tech_fingerprints WHERE run_id=? ORDER BY created_at, id`, runID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var fp Fingerprint
			if e = rows.Scan(&fp.ModuleID, &fp.Category, &fp.Value, &fp.EvidenceRef, &fp.Confidence); e != nil {
				return e
			}
			out = append(out, fp)
		}
		return rows.Err()
	})
	return out, err
}
