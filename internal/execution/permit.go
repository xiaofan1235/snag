package execution

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// permit.go —— 两层 Permit（§9.5 / §4.3 #4 / §31 C1）。红线三闸门第二道（授权凭证）。
//
//   - Action Permit：一次性，「启动一次该动作」。消费 = 单条条件更新（CAS）+ 同事务把 Action
//     置 DISPATCHED。每个 Permit 只能消费一次（§4.3 #4）；并发消费只 1 成功（单写者串行 + CAS）。
//   - Request Capability：逐请求（浏览器多子请求/重定向），声明 max_subrequests，每线上请求原子消耗一个，
//     超限即中断。
//
// 消费 CAS 条件（§9.5）：Permit=ISSUED ∧ 未过期 ∧ Job 当前 generation==permit 的 attempt generation
// ∧ Attempt 活跃（非 EXPIRED）∧ Run 可执行（RUNNING/EXITING）∧ PolicyRevision 未被取代。
// 任一不满足即拒（fail-closed）。Proxy（Day 18）逐请求回查 Permit。

// Permits 管理 Action Permit 与 Request Capability 的签发/消费。写走单写者 durable 连接。
type Permits struct {
	w *store.Writer
}

// NewPermits 构造。
func NewPermits(w *store.Writer) *Permits { return &Permits{w: w} }

func permitNow(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// ActionPermitGrant 是签发一个 Action Permit 的绑定参数（§9.5）。
type ActionPermitGrant struct {
	RunID          domain.ID
	ActionID       domain.ID
	AttemptID      domain.ID
	FencingToken   int64
	PolicyRevision int64
	IdentityID     string
	ScopeJSON      string
	TTL            time.Duration
}

// IssueActionPermit 在 PDP allow 后签发一次性 Action Permit（状态 ISSUED）。
func (pm *Permits) IssueActionPermit(ctx context.Context, g ActionPermitGrant) (domain.ID, error) {
	id := domain.NewID()
	now := time.Now().UTC()
	issued := permitNow(now)
	var expires any
	if g.TTL > 0 {
		expires = permitNow(now.Add(g.TTL))
	}
	err := pm.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`INSERT INTO action_permits
			 (id, run_id, action_id, attempt_id, fencing_token, policy_revision, identity_id, scope_json, state, issued_at, expires_at)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			string(id), string(g.RunID), string(g.ActionID), string(g.AttemptID), g.FencingToken,
			g.PolicyRevision, g.IdentityID, nullStr(g.ScopeJSON), string(domain.PermitIssued), issued, expires)
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// ConsumeActionPermit 消费 Permit：单条 CAS + 同事务把 Action AUTHORIZED→DISPATCHED（§4.3 #4）。
// 成功返回 nil；各类不满足条件返回 ErrConflict（并发/重放/越代/过期/Run 不可执行/策略被取代）。
func (pm *Permits) ConsumeActionPermit(ctx context.Context, permitID domain.ID, now time.Time) error {
	nowStr := permitNow(now)
	return pm.w.Durable(ctx, func(tx *sql.Tx) error {
		var state, actionID, attemptID, expiresAt string
		var policyRev int64
		e := tx.QueryRowContext(ctx,
			`SELECT state, action_id, COALESCE(attempt_id,''), COALESCE(expires_at,''), policy_revision
			 FROM action_permits WHERE id=?`, string(permitID)).
			Scan(&state, &actionID, &attemptID, &expiresAt, &policyRev)
		if e == sql.ErrNoRows {
			return domain.ErrNotFound
		}
		if e != nil {
			return e
		}
		// §4.3 #4 一次性：非 ISSUED 一律拒（重放/已撤销/已过期）。
		if state != string(domain.PermitIssued) {
			return domain.Errorf(domain.ErrConflict, fmt.Sprintf("permit 非 ISSUED（state=%s）：不可重复消费", state))
		}
		// 过期。
		if expiresAt != "" && expiresAt <= nowStr {
			_, _ = tx.ExecContext(ctx, `UPDATE action_permits SET state=? WHERE id=? AND state=?`,
				string(domain.PermitExpired), string(permitID), string(domain.PermitIssued))
			return domain.Errorf(domain.ErrConflict, "permit 已过期")
		}
		// 越代 / Attempt 活跃 / Run 可执行。
		if attemptID == "" {
			return domain.Errorf(domain.ErrConflict, "permit 未绑定 attempt")
		}
		var attGen, jobGen int64
		var attState, runID, runState string
		e = tx.QueryRowContext(ctx,
			`SELECT a.generation, a.state, j.generation, j.run_id, r.state
			 FROM attempts a JOIN jobs j ON a.job_id=j.id JOIN runs r ON j.run_id=r.id
			 WHERE a.id=?`, attemptID).Scan(&attGen, &attState, &jobGen, &runID, &runState)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrConflict, "permit 的 attempt/job 不存在")
		}
		if e != nil {
			return e
		}
		if attGen != jobGen {
			return domain.Errorf(domain.ErrConflict,
				fmt.Sprintf("越代消费：permit 的 attempt generation=%d 非当前 %d", attGen, jobGen))
		}
		if attState == string(domain.AttemptExpired) {
			return domain.Errorf(domain.ErrConflict, "Attempt 租约已过期，permit 失效")
		}
		if runState != string(domain.RunRunning) && runState != string(domain.RunExiting) {
			return domain.Errorf(domain.ErrConflict, fmt.Sprintf("Run 非可执行态（%s）", runState))
		}
		// PolicyRevision 未被取代。
		var maxRev int64
		if e := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(revision),0) FROM policy_revisions WHERE run_id=?`, runID).Scan(&maxRev); e != nil {
			return e
		}
		if maxRev > policyRev {
			return domain.Errorf(domain.ErrConflict,
				fmt.Sprintf("PolicyRevision 已被取代（permit=%d, current=%d）", policyRev, maxRev))
		}
		// 原子消费（并发守卫：WHERE state='ISSUED'）。
		res, e := tx.ExecContext(ctx,
			`UPDATE action_permits SET state=?, consumed_at=? WHERE id=? AND state=?`,
			string(domain.PermitConsumed), nowStr, string(permitID), string(domain.PermitIssued))
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return domain.ErrConflict // 并发已被消费
		}
		// 同事务：Action AUTHORIZED→DISPATCHED（§4.3 #4）。
		res2, e := tx.ExecContext(ctx,
			`UPDATE actions SET state=?, dispatched_at=? WHERE id=? AND state=?`,
			string(domain.ActionDispatched), nowStr, actionID, string(domain.ActionAuthorized))
		if e != nil {
			return e
		}
		if n, _ := res2.RowsAffected(); n != 1 {
			return domain.Errorf(domain.ErrConflict, "Action 非 AUTHORIZED，无法随 permit 派发")
		}
		return nil
	})
}

// RevokeAttemptPermits 撤销某 Attempt 的所有未消费 Permit（Attempt 租约过期时调用，接 Day 09）。
// 返回撤销条数。
func (pm *Permits) RevokeAttemptPermits(ctx context.Context, attemptID domain.ID, now time.Time) (int, error) {
	nowStr := permitNow(now)
	count := 0
	err := pm.w.Durable(ctx, func(tx *sql.Tx) error {
		res, e := tx.ExecContext(ctx,
			`UPDATE action_permits SET state=?, revoked_at=? WHERE attempt_id=? AND state=?`,
			string(domain.PermitRevoked), nowStr, string(attemptID), string(domain.PermitIssued))
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		count = int(n)
		return nil
	})
	return count, err
}

// RequestCapabilityGrant 是签发一个 Request Capability 的参数（§9.5 浏览器多子请求）。
type RequestCapabilityGrant struct {
	PermitID       domain.ID
	ActionID       domain.ID
	RunID          domain.ID
	AttemptID      domain.ID
	FencingToken   int64
	CanonicalURL   string
	IdentityID     string
	PolicyRevision int64
	RequestKind    string
	MaxSubrequests int
}

// IssueRequestCapability 签发逐请求能力（声明 max_subrequests）。
func (pm *Permits) IssueRequestCapability(ctx context.Context, g RequestCapabilityGrant) (domain.ID, error) {
	if g.MaxSubrequests < 1 {
		g.MaxSubrequests = 1
	}
	id := domain.NewID()
	err := pm.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`INSERT INTO request_capabilities
			 (id, permit_id, action_id, run_id, attempt_id, fencing_token, canonical_url, identity_id,
			  policy_revision, request_kind, max_subrequests, consumed_count)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,0)`,
			string(id), string(g.PermitID), string(g.ActionID), string(g.RunID), nullStr(string(g.AttemptID)),
			g.FencingToken, nullStr(g.CanonicalURL), nullStr(g.IdentityID), g.PolicyRevision,
			nullStr(g.RequestKind), g.MaxSubrequests)
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// ConsumeSubrequest 原子消耗一个子请求配额；超 max_subrequests 或已撤销 → ErrConflict（中断）。
func (pm *Permits) ConsumeSubrequest(ctx context.Context, capID domain.ID) error {
	return pm.w.Durable(ctx, func(tx *sql.Tx) error {
		res, e := tx.ExecContext(ctx,
			`UPDATE request_capabilities SET consumed_count=consumed_count+1
			 WHERE id=? AND consumed_count < max_subrequests AND revoked_at IS NULL`, string(capID))
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return domain.Errorf(domain.ErrConflict, "子请求超 max_subrequests 或 capability 已撤销/不存在")
		}
		return nil
	})
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
