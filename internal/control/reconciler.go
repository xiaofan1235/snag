package control

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/outbox"
	"pentagent/internal/store"
)

// reconciler.go —— 启动恢复器 + 每阻塞态超时（§19 启动顺序 / §5.0.5 超时表 / §31 C5 / §5.3）。
//
// M1 可靠性收口：杀进程后重启能自动恢复，且不重复执行已派发动作、不误判成功、无阻塞态永久卡死。
//
// 【§19 启动顺序】严格固定：①查库迁移（调用方前置）②恢复全局 Pause/撤销 ③作废过期 Permit
// ④标过期 Session/Attempt ⑤增受影响 Job generation ⑥恢复 Action 对账 ⑦恢复 Outbox
// ⑧启动 Scheduler ⑨最后才允许新目标动作。Reconcile 落实 ④⑤⑥⑦（现有组件覆盖）；②③与部分
// 阻塞态超时的组件（全局 Pause、Permit=Day17、Session=Day13、审批=Day15、Proxy/Strategist）
// 尚未建，按顺序留显式占位（下见注释），不假装完成。⑧⑨由调用方（cmd/pentagentd）在 Reconcile
// 返回后进行——"先恢复、后开调度、最后才接新动作"的次序由此保证。
//
// 【崩溃语义 §5.3】本进程为崩溃后的新进程，故库里任何 RUNNING Attempt 均为死进程遗留的孤儿；
// 其 DISPATCHED 动作结果未知 → 置 OUTCOME_UNKNOWN（不重试、不标成功、不直接失败），所属 Job
// 进 RECONCILING，并增代使死进程复活后持旧 fencing_token 的写入被拒（job.generation 前移、
// 当代无 Attempt → guardFencingTx 判 found=false 拒写，§31）。

// RecoverReport 汇总一次启动恢复处理的数量。
type RecoverReport struct {
	UnknownActions    int // DISPATCHED → OUTCOME_UNKNOWN 的动作数
	OrphanedAttempts  int // 标 EXPIRED 的在飞 Attempt 数
	JobsReconciling   int // 进入 RECONCILING 的 Job 数（RUNNING 孤儿）
	JobsReoffered     int // CLAIMED→READY 重新上架的 Job 数（认领后未执行的孤儿）
	GenerationsBumped int // 增代的 Job 数
	OutboxDrained     int // 启动时补发的 outbox 消息数
}

// Reconciler 跑 §19 启动恢复流程，并提供 §5.0.5 阻塞态超时扫描。
type Reconciler struct {
	w                *store.Writer
	rd               *store.Reader
	jobs             *JobController
	pub              *outbox.Publisher
	reconcileTimeout time.Duration
}

// ReconcilerOption 配置 Reconciler。
type ReconcilerOption func(*Reconciler)

// WithReconcileDeadline 设置启动恢复时把 Job 推入 RECONCILING 所用的对账截止时长。
func WithReconcileDeadline(d time.Duration) ReconcilerOption {
	return func(r *Reconciler) { r.reconcileTimeout = d }
}

// NewReconciler 构造恢复器。jobs 用于阻塞态超时（RECONCILING→终态）；pub 用于启动补发 Outbox。
func NewReconciler(w *store.Writer, rd *store.Reader, jobs *JobController, pub *outbox.Publisher, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{w: w, rd: rd, jobs: jobs, pub: pub, reconcileTimeout: DefaultReconcileTimeout}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Reconcile 执行 §19 启动恢复（步骤 ④⑤⑥⑦），返回处理汇总。幂等：重复运行无新孤儿时各计数为 0。
func (r *Reconciler) Reconcile(ctx context.Context) (RecoverReport, error) {
	var rep RecoverReport
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	// §19 ②：恢复全局 Pause/撤销 —— 全局 Pause 机制尚未建（Run 级 pause 见 Day 07），留待后续。
	// §19 ③：作废过期 Permit —— Permit = Day 17（permits 表未建），留占位。

	// §19 ④⑤⑥：孤儿 Attempt 的 Job 恢复（崩溃语义 §5.3），单事务原子完成。
	err := r.w.Durable(ctx, func(tx *sql.Tx) error {
		// ⑥-a 先把 DISPATCHED 动作置 OUTCOME_UNKNOWN（不重试/不标成功/不直接失败，§5.3）。
		res, e := tx.ExecContext(ctx, `UPDATE actions SET state=? WHERE state=?`,
			string(domain.ActionOutcomeUnknown), string(domain.ActionDispatched))
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n > 0 {
			rep.UnknownActions = int(n)
		}

		// ④ 收集孤儿 Attempt（本进程为新进程，RUNNING = 死进程遗留）所属 Job，再标 EXPIRED。
		rows, e := tx.QueryContext(ctx, `SELECT DISTINCT job_id FROM attempts WHERE state=?`, string(domain.AttemptRunning))
		if e != nil {
			return e
		}
		var jobIDs []string
		for rows.Next() {
			var jid string
			if e := rows.Scan(&jid); e != nil {
				rows.Close()
				return e
			}
			jobIDs = append(jobIDs, jid)
		}
		if e := rows.Err(); e != nil {
			rows.Close()
			return e
		}
		rows.Close()

		res, e = tx.ExecContext(ctx, `UPDATE attempts SET state=?, ended_at=? WHERE state=?`,
			string(domain.AttemptExpired), nowStr, string(domain.AttemptRunning))
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n > 0 {
			rep.OrphanedAttempts = int(n)
		}

		// ⑤+⑥-b 逐个孤儿 Job：增代 + 进 RECONCILING（RUNNING）/ 重新上架（CLAIMED）。
		for _, jid := range jobIDs {
			var st string
			var gen, rev int64
			if e := tx.QueryRowContext(ctx, `SELECT state, generation, revision FROM jobs WHERE id=?`, jid).
				Scan(&st, &gen, &rev); e != nil {
				if e == sql.ErrNoRows {
					continue
				}
				return e
			}
			newGen := gen + 1
			switch domain.JobState(st) {
			case domain.JobRunning:
				// RUNNING 孤儿 → RECONCILING（§5.3：崩溃后不得直接重试/成功/失败，先对账）。
				to, ok := JobNext(domain.JobRunning, JobEventReconcile)
				if !ok {
					return domain.Errorf(domain.ErrInvalid, "状态机缺 RUNNING--reconcile-->RECONCILING")
				}
				deadline := now.Add(r.reconcileTimeout).Format(time.RFC3339)
				if _, e := tx.ExecContext(ctx,
					`UPDATE jobs SET state=?, generation=?, reconcile_deadline=?, revision=revision+1, updated_at=?
					 WHERE id=? AND revision=?`,
					string(to), newGen, deadline, nowStr, jid, rev); e != nil {
					return e
				}
				rep.JobsReconciling++
				rep.GenerationsBumped++
			case domain.JobClaimed:
				// CLAIMED 孤儿（认领后未执行）→ READY 重新上架（无副作用，安全）。
				to, ok := JobNext(domain.JobClaimed, JobEventLeaseExpired)
				if !ok {
					return domain.Errorf(domain.ErrInvalid, "状态机缺 CLAIMED--lease_expired-->READY")
				}
				if _, e := tx.ExecContext(ctx,
					`UPDATE jobs SET state=?, generation=?, claimed_by=NULL, lease_until=NULL, revision=revision+1, updated_at=?
					 WHERE id=? AND revision=?`,
					string(to), newGen, nowStr, jid, rev); e != nil {
					return e
				}
				rep.JobsReoffered++
				rep.GenerationsBumped++
			default:
				// 其它状态（已终态或已自行流转）：Attempt 已标 EXPIRED，Job 不动。
			}
		}
		return nil
	})
	if err != nil {
		return RecoverReport{}, err
	}

	// §19 ⑦：恢复 Outbox —— 补发崩溃前已入库未发布的消息（Day 10）。
	if r.pub != nil {
		n, e := r.pub.Drain(ctx, now)
		if e != nil {
			return rep, e
		}
		rep.OutboxDrained = n
	}

	// §19 ⑧⑨：启动 Scheduler + 允许新目标动作 —— 由调用方在 Reconcile 返回后进行（次序保证）。
	return rep, nil
}

// SweepReport 汇总一次阻塞态超时扫描的升级数量。
type SweepReport struct {
	ReconcileTimedOut int // 对账超时被终态化的 Job 数
}

// SweepTimeouts 扫描各阻塞态、对到点者执行升级（§5.0.5）。应周期调用。now 由调用方注入。
//
// 现实现：RECONCILING 超时 → OUTCOME_UNRESOLVED 终态（Day 08 ExpireReconciliations）。
// 其余阻塞态的超时+升级随其组件落地时接入，均为「有上限、不永久卡」：
//   - WAITING_APPROVAL 超时 → 审批流 Day 15；
//   - Identity BLOCKED 超时 → 身份机 Day 13；
//   - Proxy 过期 / Permit 过期 → Day 17；
//   - EXITING / Strategist 超时 → 相应组件落地日。
//
// 注：Attempt 租约超时由 scheduler.ExpireLeases periodic 扫描（scheduler 反向依赖 control，
// 不在此处调用以免循环依赖；由 cmd 层在 Reconcile 后与 Scheduler 一并启动）。
func (r *Reconciler) SweepTimeouts(ctx context.Context, now time.Time) (SweepReport, error) {
	var rep SweepReport
	n, err := r.jobs.ExpireReconciliations(ctx, now)
	if err != nil {
		return rep, err
	}
	rep.ReconcileTimedOut = n
	return rep, nil
}
