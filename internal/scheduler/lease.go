package scheduler

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/control"
	"pentagent/internal/domain"
)

// Renew 续租：先过 fencing 校验（陈旧 Attempt 不能续租），再延长 lease_until 到 now+leaseDuration。
// 返回新的租约截止时刻。陈旧令牌 → ErrConflict。
func (s *Scheduler) Renew(ctx context.Context, jobID domain.ID, fencingToken int64) (time.Time, error) {
	now := time.Now().UTC()
	lease := now.Add(s.leaseDuration)
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		if e := guardFencingTx(ctx, tx, jobID, fencingToken); e != nil {
			return e
		}
		r, e := tx.ExecContext(ctx,
			`UPDATE jobs SET lease_until=?, updated_at=? WHERE id=?`, rfc3339(lease), rfc3339(now), string(jobID))
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return time.Time{}, err
	}
	return lease, nil
}

// ExpireLeases 扫描租约已过期的 **CLAIMED** Job（Day 09 范围）并安全重新上架：
//   - 当代 Attempt 标 EXPIRED（其 fencing_token 就此失效，后续写入被 guardFencingTx 拒）；
//   - SM lease_expired：CLAIMED→READY，清 claimed_by/lease_until，可被新 Claim 增代重领。
//
// 注：RUNNING 态租约过期牵涉已派发动作的对账（可能有副作用，不得直接重领），语义耦合
// Day 12 Reconciler，本方法有意不处理 RUNNING——只收 claimant 认领后未执行即失联的干净场景。
// now 由调用方注入（确定性测试 + 统一时钟源）。返回成功重新上架的 Job 数。
func (s *Scheduler) ExpireLeases(ctx context.Context, now time.Time) (int, error) {
	nowStr := rfc3339(now)
	count := 0
	to, ok := control.JobNext(domain.JobClaimed, control.JobEventLeaseExpired)
	if !ok { // 生成的状态机保证存在，防御性断言
		return 0, domain.Errorf(domain.ErrInvalid, "状态机缺 CLAIMED--lease_expired-->READY 边")
	}
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		type row struct {
			id  string
			gen int64
			rev int64
		}
		var rows []row
		rs, e := tx.QueryContext(ctx,
			`SELECT id, generation, revision FROM jobs
			 WHERE state=? AND lease_until IS NOT NULL AND lease_until<=?`,
			string(domain.JobClaimed), nowStr)
		if e != nil {
			return e
		}
		for rs.Next() {
			var x row
			if e := rs.Scan(&x.id, &x.gen, &x.rev); e != nil {
				rs.Close()
				return e
			}
			rows = append(rows, x)
		}
		if e := rs.Err(); e != nil {
			rs.Close()
			return e
		}
		rs.Close()

		for _, x := range rows {
			// 标当代 Attempt 过期（fencing 就此失效）。
			if _, e := tx.ExecContext(ctx,
				`UPDATE attempts SET state=?, ended_at=? WHERE job_id=? AND generation=?`,
				string(domain.AttemptExpired), nowStr, x.id, x.gen); e != nil {
				return e
			}
			// 回 READY，清认领；revision 条件更新。
			r, e := tx.ExecContext(ctx,
				`UPDATE jobs SET state=?, claimed_by=NULL, lease_until=NULL, revision=revision+1, updated_at=?
				 WHERE id=? AND revision=?`,
				string(to), nowStr, x.id, x.rev)
			if e != nil {
				return e
			}
			n, e := r.RowsAffected()
			if e != nil {
				return e
			}
			if n == 1 {
				count++
			}
		}
		return nil
	})
	return count, err
}
