package kb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"pentagent/internal/domain"
)

// skill.go —— Skill 导入（§40）：本地受审包；**不声明越权**；按 applies_to 加载。
// 外部下载的 Skill 默认不可信（§40），导入时校验声明权限不含越权能力，命中 Canary 拒入库。

// 越权能力黑名单：Skill 不得声明这些（它只能用注册工具，红线执法在 Proxy/Permit，不在 Skill）。
var elevatedPerms = map[string]bool{
	"modify_scope": true, "modify_policy": true, "disable_proxy": true, "raw_socket": true,
	"bypass_permit": true, "elevate_identity": true, "disable_sandbox": true, "set_coverage_tested": true,
}

// ErrOverPrivilegedSkill —— Skill 声明了越权能力，拒绝导入（§40）。
var ErrOverPrivilegedSkill = &domain.DomainError{Code: "OVER_PRIVILEGED_SKILL", Msg: "skill declares elevated permission, rejected (§40)"}

// Skill 是一个导入的方法包。
type Skill struct {
	Name          string
	AppliesTo     []string
	DeclaredPerms []string
	Content       string
}

// ImportSkill 导入 Skill：声明越权能力拒（ErrOverPrivilegedSkill）、命中 Canary 拒、按 workspace 隔离。
func (s *Store) ImportSkill(ctx context.Context, workspaceID string, sk Skill, now time.Time) (string, error) {
	for _, p := range sk.DeclaredPerms {
		if elevatedPerms[strings.TrimSpace(strings.ToLower(p))] {
			return "", domain.Errorf(ErrOverPrivilegedSkill, p)
		}
	}
	if s.hitCanary(sk.Content) {
		return "", ErrSecretCanary
	}
	sum := sha256.Sum256([]byte(sk.Content))
	hash := hex.EncodeToString(sum[:])
	id := string(domain.NewID())
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO skills (id, workspace_id, name, applies_to, declared_perms, content, content_sha256, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			id, workspaceID, sk.Name, strings.Join(sk.AppliesTo, ","), strings.Join(sk.DeclaredPerms, ","),
			sk.Content, hash, ts(now))
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// LoadSkills 按 applies_to 加载某 workspace 的相关 Skill（§40）。Workspace 隔离。
func (s *Store) LoadSkills(ctx context.Context, workspaceID, appliesTo string) ([]Skill, error) {
	var out []Skill
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx,
			`SELECT name, COALESCE(applies_to,''), COALESCE(declared_perms,''), COALESCE(content,'') FROM skills WHERE workspace_id=?`, workspaceID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var name, at, perms, content string
			if e = rows.Scan(&name, &at, &perms, &content); e != nil {
				return e
			}
			if appliesTo != "" && !appliesToMatch(at, appliesTo) {
				continue
			}
			out = append(out, Skill{Name: name, AppliesTo: splitCSV(at), DeclaredPerms: splitCSV(perms), Content: content})
		}
		return rows.Err()
	})
	return out, err
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
