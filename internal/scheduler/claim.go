// Package scheduler 实现 Job 的认领（Claim）、租约（Lease）与单调 fencing（§4.1/§4.3 #2/§31）。
//
// 核心保证（"杀进程后能安全恢复、不重复执行"）：
//   - 并发认领同一 Job 只有一个成功——SM 守卫 READY→CLAIMED + 单写者串行 + revision 条件更新，
//     UNIQUE(job_id, generation) 作末端兜底；
//   - 每次认领增 Job generation（per-job，MAX+1）并发放**全局单调** fencing_token（MAX+1）；
//   - 旧 Attempt 即使进程复活，持旧 fencing_token 的写入一律被拒（CheckFencing / 受护写入）。
//
// fencing_token 选全局单调而非每 Job 计数：既满足"随 Job generation 单调递增"（同一 Job 后续代
// 必拿到更大令牌），又可直接服务 Day 17 的 Permit/Proxy 等**共享资源**的陈旧写者拒绝，无需返工。
// 单调性由单写者串行保证：令牌 = 现存 MAX(fencing_token)+1，读自持久表故跨进程/重启存活
// （attempts 为追加式审计记录、从不删除——这是单调性前提）。
package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pentagent/internal/control"
	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// DefaultLeaseDuration —— 认领后默认租约时长。
const DefaultLeaseDuration = 60 * time.Second

// Scheduler 承载认领/租约/fencing。所有写走单写者 durable 连接（§31 C1）。
type Scheduler struct {
	w             *store.Writer
	leaseDuration time.Duration
}

// Option 配置 Scheduler。
type Option func(*Scheduler)

// WithLeaseDuration 覆盖租约时长（测试/调参）。
func WithLeaseDuration(d time.Duration) Option {
	return func(s *Scheduler) { s.leaseDuration = d }
}

// New 构造 Scheduler。
func New(w *store.Writer, opts ...Option) *Scheduler {
	s := &Scheduler{w: w, leaseDuration: DefaultLeaseDuration}
	for _, o := range opts {
		o(s)
	}
	return s
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// ClaimResult 是一次成功认领的产物。
type ClaimResult struct {
	AttemptID    domain.ID
	Generation   int64
	FencingToken int64
	LeaseUntil   time.Time
}

// Claim 原子认领一个 READY 的 Job（§4.1）。失败：非 READY → ErrConflict；不存在 → ErrNotFound。
// 并发认领同一 Job：单写者串行下，首个把 READY→CLAIMED，后到者读到 CLAIMED 被 SM 拒（ErrConflict）。
func (s *Scheduler) Claim(ctx context.Context, jobID, agentSessionID domain.ID) (*ClaimResult, error) {
	now := time.Now().UTC()
	var res ClaimResult
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		var st string
		var rev int64
		if e := tx.QueryRowContext(ctx, `SELECT state, revision FROM jobs WHERE id=?`, string(jobID)).
			Scan(&st, &rev); e != nil {
			if e == sql.ErrNoRows {
				return domain.ErrNotFound
			}
			return e
		}
		to, ok := control.JobNext(domain.JobState(st), control.JobEventClaim)
		if !ok {
			return domain.Errorf(domain.ErrConflict,
				fmt.Sprintf("job %s: 状态 %s 不可认领（仅 READY 可领）", jobID, st))
		}

		// per-job generation = MAX+1；全局 fencing_token = MAX+1。
		var maxGen, maxFence int64
		if e := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(generation),0) FROM attempts WHERE job_id=?`, string(jobID)).
			Scan(&maxGen); e != nil {
			return e
		}
		if e := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(fencing_token),0) FROM attempts`).Scan(&maxFence); e != nil {
			return e
		}
		newGen := maxGen + 1
		newFence := maxFence + 1
		lease := now.Add(s.leaseDuration)

		r, e := tx.ExecContext(ctx,
			`UPDATE jobs SET state=?, generation=?, claimed_by=?, lease_until=?, revision=revision+1, updated_at=?
			 WHERE id=? AND revision=?`,
			string(to), newGen, string(agentSessionID), rfc3339(lease), rfc3339(now), string(jobID), rev)
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			return domain.ErrConflict
		}

		aid := domain.NewID()
		if _, e := tx.ExecContext(ctx,
			`INSERT INTO attempts (id, job_id, generation, agent_session_id, fencing_token, state, started_at)
			 VALUES (?,?,?,?,?,?,?)`,
			string(aid), string(jobID), newGen, string(agentSessionID), newFence,
			string(domain.AttemptRunning), rfc3339(now)); e != nil {
			return e
		}
		res = ClaimResult{AttemptID: aid, Generation: newGen, FencingToken: newFence, LeaseUntil: lease}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// currentFencingTx 读 Job 当代（= jobs.generation）对应 Attempt 的 fencing_token。
// found=false 表示该代无 Attempt（尚未认领）。
func currentFencingTx(ctx context.Context, tx *sql.Tx, jobID domain.ID) (int64, bool, error) {
	var gen int64
	if e := tx.QueryRowContext(ctx, `SELECT generation FROM jobs WHERE id=?`, string(jobID)).Scan(&gen); e != nil {
		if e == sql.ErrNoRows {
			return 0, false, domain.ErrNotFound
		}
		return 0, false, e
	}
	var tok int64
	e := tx.QueryRowContext(ctx,
		`SELECT fencing_token FROM attempts WHERE job_id=? AND generation=?`, string(jobID), gen).Scan(&tok)
	if e == sql.ErrNoRows {
		return 0, false, nil
	}
	if e != nil {
		return 0, false, e
	}
	return tok, true, nil
}

// guardFencingTx 校验 fencingToken 是否为该 Job **当代有效令牌**；不匹配（陈旧/越界）返回 ErrConflict。
// 旧 Attempt 的 fencing_token 既小于当代、其 generation 也非当代，故一律被拒（§31 旧写者拒绝）。
func guardFencingTx(ctx context.Context, tx *sql.Tx, jobID domain.ID, fencingToken int64) error {
	cur, found, e := currentFencingTx(ctx, tx, jobID)
	if e != nil {
		return e
	}
	if !found {
		return domain.Errorf(domain.ErrConflict, fmt.Sprintf("job %s: 当代无活跃 Attempt", jobID))
	}
	if fencingToken != cur {
		return domain.Errorf(domain.ErrConflict,
			fmt.Sprintf("job %s: fencing_token %d 已失效（当代 %d）——陈旧 Attempt 写入被拒", jobID, fencingToken, cur))
	}
	return nil
}

// CheckFencing 独立校验某 fencing_token 当前是否有效（陈旧返回 ErrConflict）。
func (s *Scheduler) CheckFencing(ctx context.Context, jobID domain.ID, fencingToken int64) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		return guardFencingTx(ctx, tx, jobID, fencingToken)
	})
}

// WriteAttemptResult 把执行结果写回**当代** Attempt，先过 fencing 校验（陈旧 Attempt 写入被拒，§31）。
// 这是"旧 Attempt 即使进程复活也写不进"的落点示范：所有带副作用的回写都应先 guardFencingTx。
func (s *Scheduler) WriteAttemptResult(ctx context.Context, jobID domain.ID, fencingToken int64, resultJSON string) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		if e := guardFencingTx(ctx, tx, jobID, fencingToken); e != nil {
			return e
		}
		r, e := tx.ExecContext(ctx,
			`UPDATE attempts SET result_json=?, state=? WHERE job_id=? AND fencing_token=?`,
			resultJSON, string(domain.AttemptSubmitted), string(jobID), fencingToken)
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			return domain.ErrConflict
		}
		return nil
	})
}
