package findings

// cross_run.go —— 跨 Run Finding 关联（§47.3，审计 B8）。同一 Workspace 内多次测试同一目标，关联
// Finding（上次那个/已修/重开/重复）。
//
// 红线（§47.3/§4.0）：
//   - 只连「同 Workspace 同 Scope Digest」，绝不跨目标——编译期由两 Finding 所属 Run 的 workspace_id
//     与 authorization_scope_digest 一致性校验拦下；
//   - 关联不复制证据（引用，不搬运）——只写 link 行，不拷 evidence。
//
// 关联依据：确定性指纹（端点+参数+攻击类型+对象归属）为主，语义相似为辅。
// 状态演进：上次 REPORTABLE 的 Finding，本次重测：复现→REOPENED；不复现→候选 FIXED（需验证「确已修复」
// 而非环境变化，故 FIXED 由调用方在确认确定性复现失败后给出）。

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
)

// 关联关系（§47.3）。
const (
	RelSameRootCause = "SAME_ROOT_CAUSE"
	RelReopened      = "REOPENED"
	RelFixed         = "FIXED"
	RelDuplicateOf   = "DUPLICATE_OF_PRIOR"
	RelSupersedes    = "SUPERSEDES"
)

var validRelation = map[string]bool{
	RelSameRootCause: true, RelReopened: true, RelFixed: true, RelDuplicateOf: true, RelSupersedes: true,
}

var (
	// ErrCrossWorkspaceLink —— 试图跨 Workspace 关联（红线）。
	ErrCrossWorkspaceLink = &domain.DomainError{Code: "CROSS_WORKSPACE_LINK", Msg: "cross-run link only within one workspace (§47.3)"}
	// ErrCrossScopeLink —— 试图跨 Scope Digest（跨目标）关联（红线）。
	ErrCrossScopeLink = &domain.DomainError{Code: "CROSS_SCOPE_LINK", Msg: "cross-run link only within one scope digest (§47.3/§4.0)"}
	// ErrBadRelation —— 非法关联关系。
	ErrBadRelation = &domain.DomainError{Code: "BAD_CROSSRUN_RELATION", Msg: "invalid cross-run relation (§47.3)"}
)

// CrossRunLink 是一条待写入的跨 Run 关联。
type CrossRunLink struct {
	FindingID       string // 本次 Run 的 Finding
	RunID           string
	LinkedFindingID string // 历史 Run 的 Finding
	LinkedRunID     string
	Relation        string
	Basis           string // 关联依据（端点+参数+攻击类型+对象归属 指纹）
	Confidence      string // high|medium|low
}

// runScope 读某 Run 的 workspace_id 与 scope digest（事务内）。
func runScope(ctx context.Context, tx *sql.Tx, runID string) (ws, scope string, err error) {
	e := tx.QueryRowContext(ctx,
		`SELECT workspace_id, authorization_scope_digest FROM runs WHERE id=?`, runID).Scan(&ws, &scope)
	if e == sql.ErrNoRows {
		return "", "", domain.Errorf(domain.ErrNotFound, "run "+runID)
	}
	return ws, scope, e
}

// LinkCrossRun 校验红线并写一条跨 Run 关联（§47.3）。校验顺序：关系合法 → 两 Run 同 Workspace →
// 同 Scope Digest → 写 link（带 workspace_id/scope_digest/basis/confidence）。关联不复制证据。
func (s *Store) LinkCrossRun(ctx context.Context, in CrossRunLink, now time.Time) (string, error) {
	if !validRelation[in.Relation] {
		return "", domain.Errorf(ErrBadRelation, in.Relation)
	}
	id := string(domain.NewID())
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		ws1, sc1, e := runScope(ctx, tx, in.RunID)
		if e != nil {
			return e
		}
		ws2, sc2, e := runScope(ctx, tx, in.LinkedRunID)
		if e != nil {
			return e
		}
		if ws1 != ws2 {
			return domain.Errorf(ErrCrossWorkspaceLink, ws1+" vs "+ws2)
		}
		if sc1 != sc2 {
			return domain.Errorf(ErrCrossScopeLink, "scope digests differ")
		}
		// 两 Finding 必须分别属于其声明的 Run（防伪造；复合 FK 也兜底）。
		if e := assertFindingInRun(ctx, tx, in.FindingID, in.RunID); e != nil {
			return e
		}
		if e := assertFindingInRun(ctx, tx, in.LinkedFindingID, in.LinkedRunID); e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `
			INSERT INTO cross_run_finding_links
			  (id, finding_id, run_id, linked_finding_id, linked_run_id, relation, basis, confidence, workspace_id, scope_digest, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			id, in.FindingID, in.RunID, in.LinkedFindingID, in.LinkedRunID, in.Relation,
			nz(in.Basis), nz(in.Confidence), ws1, sc1, ts(now))
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func assertFindingInRun(ctx context.Context, tx *sql.Tx, findingID, runID string) error {
	var got string
	e := tx.QueryRowContext(ctx, `SELECT run_id FROM findings WHERE id=?`, findingID).Scan(&got)
	if e == sql.ErrNoRows {
		return domain.Errorf(domain.ErrNotFound, "finding "+findingID)
	}
	if e != nil {
		return e
	}
	if got != runID {
		return domain.Errorf(ErrCrossRun, "finding "+findingID+" not in run "+runID)
	}
	return nil
}

// RelationForRetest 纯函数：给出历史 REPORTABLE Finding 在本次重测下的关联关系。
// reproduced=true → REOPENED（复现）；false → FIXED 候选（不复现，需调用方已确认是确定性复现失败而非环境变化）。
func RelationForRetest(reproduced bool) string {
	if reproduced {
		return RelReopened
	}
	return RelFixed
}

// CrossRunLinks 读某 Finding 的跨 Run 关联链（历史视图用）。
func (s *Store) CrossRunLinks(ctx context.Context, findingID string) ([]CrossRunLink, error) {
	var out []CrossRunLink
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `
			SELECT finding_id, run_id, linked_finding_id, linked_run_id, COALESCE(relation,''),
			       COALESCE(basis,''), COALESCE(confidence,'')
			FROM cross_run_finding_links WHERE finding_id=? OR linked_finding_id=? ORDER BY created_at`,
			findingID, findingID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var l CrossRunLink
			if e := rows.Scan(&l.FindingID, &l.RunID, &l.LinkedFindingID, &l.LinkedRunID, &l.Relation, &l.Basis, &l.Confidence); e != nil {
				return e
			}
			out = append(out, l)
		}
		return rows.Err()
	})
	return out, err
}
