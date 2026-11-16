package control

//go:generate go run gen.go

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// DefaultReconcileTimeout —— RECONCILING 默认对账截止时长（§31 C3）。到点未完成对账即自动
// OUTCOME_UNRESOLVED 终态，防止某个 Job 永久卡在 RECONCILING 连坐冻结整条依赖链/整个 Run。
const DefaultReconcileTimeout = 30 * time.Second

// JobController 驱动 Job 状态迁移。与 Run 控制器同构：所有迁移走单写者 durable 连接的
// 条件更新（读 state+revision → 算 JobNext → UPDATE ... WHERE id=? AND revision=?），
// 单写者串行天然无并发写竞争（§31 C1/C2）。额外承载 §5.2 RECONCILING 与 §4.1 取消语义。
type JobController struct {
	w                *store.Writer
	reconcileTimeout time.Duration
}

// JobControllerOption 配置 JobController。
type JobControllerOption func(*JobController)

// WithReconcileTimeout 覆盖对账超时时长（测试/调参用）。
func WithReconcileTimeout(d time.Duration) JobControllerOption {
	return func(c *JobController) { c.reconcileTimeout = d }
}

// NewJobController 构造 Job 控制器。
func NewJobController(w *store.Writer, opts ...JobControllerOption) *JobController {
	c := &JobController{w: w, reconcileTimeout: DefaultReconcileTimeout}
	for _, o := range opts {
		o(c)
	}
	return c
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// readJob 读 Job 当前 (state, revision, run_id)。不存在返回 domain.ErrNotFound。
func readJob(ctx context.Context, tx *sql.Tx, jobID domain.ID) (domain.JobState, int64, string, error) {
	var st, runID string
	var rev int64
	row := tx.QueryRowContext(ctx, `SELECT state, revision, run_id FROM jobs WHERE id=?`, string(jobID))
	if e := row.Scan(&st, &rev, &runID); e != nil {
		if e == sql.ErrNoRows {
			return "", 0, "", domain.ErrNotFound
		}
		return "", 0, "", e
	}
	return domain.JobState(st), rev, runID, nil
}

// pendingUnknownCount 统计该 Job 名下仍处于 OUTCOME_UNKNOWN 的动作数（§5.2 对账触发条件）。
func pendingUnknownCount(ctx context.Context, tx *sql.Tx, jobID domain.ID) (int, error) {
	var n int
	e := tx.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM actions WHERE job_id=? AND state=?`,
		string(jobID), string(domain.ActionOutcomeUnknown)).Scan(&n)
	return n, e
}

// nextOrInvalid 算状态机下一态；非法迁移返回 ErrInvalid（不改库、不 panic）。
func nextOrInvalid(jobID domain.ID, from domain.JobState, ev JobEvent) (domain.JobState, error) {
	to, ok := JobNext(from, ev)
	if !ok {
		return "", domain.Errorf(domain.ErrInvalid,
			fmt.Sprintf("job %s: 状态 %s 不接受事件 %s", jobID, from, ev))
	}
	return to, nil
}

// condUpdateState 条件更新 state+revision（可附带额外列）。命中 0 行返回 ErrConflict。
// extra 为额外 SET 片段（如 ", cancel_code=?"）与其参数，拼在 state/revision 之后、WHERE 之前。
func condUpdateState(ctx context.Context, tx *sql.Tx, jobID domain.ID, rev int64,
	to domain.JobState, extraSet string, extraArgs ...any) error {
	args := append([]any{string(to)}, extraArgs...)
	args = append(args, string(jobID), rev)
	res, e := tx.ExecContext(ctx,
		`UPDATE jobs SET state=?, revision=revision+1`+extraSet+` WHERE id=? AND revision=?`, args...)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 {
		return domain.ErrConflict
	}
	return nil
}

// Apply 对 jobID 施加普通事件 ev（ready/claim/run/submit/verify/done/verify_retry/retry/
// reconcile_retry/approve/need_approval…）。取消与 RECONCILING 进入/超时走下面的专用方法
// （它们还要写 cancel_* / reconcile_deadline / run_partial_reasons）。
func (c *JobController) Apply(ctx context.Context, jobID domain.ID, ev JobEvent) (domain.JobState, error) {
	var next domain.JobState
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		from, rev, _, e := readJob(ctx, tx, jobID)
		if e != nil {
			return e
		}
		to, e := nextOrInvalid(jobID, from, ev)
		if e != nil {
			return e
		}
		if e := condUpdateState(ctx, tx, jobID, rev, to, ""); e != nil {
			return e
		}
		next = to
		return nil
	})
	return next, err
}

// enterReconcilingTx 把 Job 推入 RECONCILING 并写对账截止时刻（now+reconcileTimeout）。
// 复用于「检测到未决 OUTCOME_UNKNOWN」的两条路径：Reconcile（租约过期/提交触发）与
// RequestCancel 的未决分支（§4.1：有未决动作的 Job 取消时先对账，不得直接终态）。
func (c *JobController) enterReconcilingTx(ctx context.Context, tx *sql.Tx,
	jobID domain.ID, from domain.JobState, rev int64, now time.Time) (time.Time, error) {
	to, e := nextOrInvalid(jobID, from, JobEventReconcile)
	if e != nil {
		return time.Time{}, e
	}
	deadline := now.Add(c.reconcileTimeout)
	if e := condUpdateState(ctx, tx, jobID, rev, to, `, reconcile_deadline=?`, rfc3339(deadline)); e != nil {
		return time.Time{}, e
	}
	return deadline, nil
}

// Reconcile 显式把 Job 推入 RECONCILING（§5.2）：当某 attempt 租约过期或提交时，该 Job 存在
// 未决 OUTCOME_UNKNOWN 动作，则先入 RECONCILING（冻结新 Claim/Permit/依赖 Job），不直接重试/失败。
// 返回对账截止时刻。若该 Job 并无未决未知动作，返回 ErrInvalid（无需对账，调用方应走正常路径）。
func (c *JobController) Reconcile(ctx context.Context, jobID domain.ID) (time.Time, error) {
	now := time.Now()
	var deadline time.Time
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		from, rev, _, e := readJob(ctx, tx, jobID)
		if e != nil {
			return e
		}
		pending, e := pendingUnknownCount(ctx, tx, jobID)
		if e != nil {
			return e
		}
		if pending == 0 {
			return domain.Errorf(domain.ErrInvalid,
				fmt.Sprintf("job %s: 无未决 OUTCOME_UNKNOWN 动作，不进 RECONCILING", jobID))
		}
		d, e := c.enterReconcilingTx(ctx, tx, jobID, from, rev, now)
		if e != nil {
			return e
		}
		deadline = d
		return nil
	})
	return deadline, err
}

// RequestCancel 请求取消 Job（§4.1：取消只影响未来动作）。路由：
//   - 该 Job 存在未决 OUTCOME_UNKNOWN 动作 → 先进 RECONCILING（设对账截止），不直接终态；
//   - 否则 → cancel 迁移到 CANCELLED，写 cancel_code/cancel_actor/cancel_detail/取消时刻。
//
// 返回落定后的状态（CANCELLED 或 RECONCILING）。
func (c *JobController) RequestCancel(ctx context.Context, jobID domain.ID,
	code domain.CancelCode, actor, detail string) (domain.JobState, error) {
	now := time.Now()
	nowStr := rfc3339(now)
	var next domain.JobState
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		from, rev, _, e := readJob(ctx, tx, jobID)
		if e != nil {
			return e
		}
		pending, e := pendingUnknownCount(ctx, tx, jobID)
		if e != nil {
			return e
		}
		if pending > 0 {
			if _, e := c.enterReconcilingTx(ctx, tx, jobID, from, rev, now); e != nil {
				return e
			}
			next = domain.JobReconciling
			return nil
		}
		to, e := nextOrInvalid(jobID, from, JobEventCancel)
		if e != nil {
			return e
		}
		if e := condUpdateState(ctx, tx, jobID, rev, to,
			`, cancel_code=?, cancel_actor=?, cancel_detail=?, cancel_requested_at=?, cancel_settled_at=?`,
			string(code), actor, detail, nowStr, nowStr); e != nil {
			return e
		}
		next = to
		return nil
	})
	return next, err
}

// ExpireReconciliations 扫描所有已过对账截止时刻的 RECONCILING Job，自动终态化（§31 C3）：
//   - reconcile_unresolved 迁移到 CANCELLED，写 cancel_code=OUTCOME_UNRESOLVED；
//   - 记一条 run_partial_reasons（code=OUTCOME_UNRESOLVED, entity_ref=job_id），记录丢失的覆盖。
//
// now 由调用方注入（便于确定性测试与统一时钟源）。返回成功终态化的 Job 数。
// 整个扫描在一个 durable 事务内完成：单写者串行 + 快照一致，SELECT 与各 UPDATE 间无并发写。
func (c *JobController) ExpireReconciliations(ctx context.Context, now time.Time) (int, error) {
	nowStr := rfc3339(now)
	count := 0
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		type expired struct {
			id, runID string
			rev       int64
		}
		var rows []expired
		rs, e := tx.QueryContext(ctx,
			`SELECT id, run_id, revision FROM jobs
			 WHERE state=? AND reconcile_deadline IS NOT NULL AND reconcile_deadline<=?`,
			string(domain.JobReconciling), nowStr)
		if e != nil {
			return e
		}
		for rs.Next() {
			var x expired
			if e := rs.Scan(&x.id, &x.runID, &x.rev); e != nil {
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
			to, e := nextOrInvalid(domain.ID(x.id), domain.JobReconciling, JobEventReconcileUnresolved)
			if e != nil {
				return e
			}
			if e := condUpdateState(ctx, tx, domain.ID(x.id), x.rev, to,
				`, cancel_code=?, cancel_actor=?, cancel_settled_at=?`,
				string(domain.CancelOutcomeUnresolved), "system:reconcile-timeout", nowStr); e != nil {
				return e
			}
			if _, e := tx.ExecContext(ctx,
				`INSERT INTO run_partial_reasons (id, run_id, code, entity_ref, created_at)
				 VALUES (?,?,?,?,?)`,
				string(domain.NewID()), x.runID, string(domain.CancelOutcomeUnresolved), x.id, nowStr); e != nil {
				return e
			}
			count++
		}
		return nil
	})
	return count, err
}
