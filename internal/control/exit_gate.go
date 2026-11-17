package control

// exit_gate.go —— 退出门禁（§14.2）。只有全部条件满足才允许 COMPLETED；预算耗尽不绕门禁而进 PARTIAL。
//
// 门禁原子性（§14.2 审计修订）：进入评估**先置 EXITING 冻结**（新 Job/新 Permit/PlanPatch 应用/Surface
// 变更全部暂停——在单写者模型里由各控制器判 run.state==EXITING 时拒新工作来落实，见 PROGRESS 待接线清单）；
// 全部条件通过与 EXITING→COMPLETED 写入在**同一事务**，以 Run Revision + SnapshotRevision + SurfaceRevision
// 做 CAS——评估期间任何 Revision 前进都放弃本次完成，EXITING→RUNNING 回到 RUNNING 重新评估。
//
// 类型化 PARTIAL 原因（§14.2，吸收 Shannon）：PARTIAL/FAILED 必须写 run_partial_reasons，
// 空结果/超时/基础设施故障不能伪装成「零发现」。

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// GateConfig 是门禁阈值配置（来源：本机配置，不接受模型/目标指定）。
type GateConfig struct {
	CoverageThresholdPct int // 覆盖率阈值（TESTED+NA 占比，百分数）；默认 80
}

func (c GateConfig) withDefaults() GateConfig {
	if c.CoverageThresholdPct <= 0 {
		c.CoverageThresholdPct = 80
	}
	return c
}

// CheckFunc 是单条门禁条件的只读检查：ok=false 表示该条不通过，detail 给原因。
type CheckFunc func(ctx context.Context, tx *sql.Tx, runID string) (ok bool, detail string, err error)

// GateCondition 是一条具名门禁条件。
type GateCondition struct {
	Name  string
	Check CheckFunc
}

// GateFailure 是一条未通过的条件。
type GateFailure struct {
	Condition string
	Detail    string
}

// GateResult 是一次门禁评估结果。
type GateResult struct {
	Passed   bool
	Failures []GateFailure
}

// CompleteResult 是 TryComplete 的结果。
type CompleteResult struct {
	Completed bool // 是否成功 COMPLETED
	State     domain.RunState
	Gate      GateResult // 条件评估详情
	Aborted   bool       // 因 Revision 漂移放弃（回 RUNNING 重评）
	AbortInfo string
}

// ExitGate 评估并原子化完成 Run。
type ExitGate struct {
	w          *store.Writer
	cfg        GateConfig
	conditions []GateCondition
}

// NewExitGate 用默认条件集构造。extra 里同名条件覆盖默认（用于把「未接线」占位条件替换成真实实现）。
func NewExitGate(w *store.Writer, cfg GateConfig, extra ...GateCondition) *ExitGate {
	cfg = cfg.withDefaults()
	g := &ExitGate{w: w, cfg: cfg}
	g.conditions = defaultConditions(cfg)
	for _, c := range extra {
		g.replace(c)
	}
	return g
}

func (g *ExitGate) replace(c GateCondition) {
	for i := range g.conditions {
		if g.conditions[i].Name == c.Name {
			g.conditions[i] = c
			return
		}
	}
	g.conditions = append(g.conditions, c)
}

// Conditions 返回当前条件名（诊断/测试用）。
func (g *ExitGate) Conditions() []string {
	out := make([]string, len(g.conditions))
	for i, c := range g.conditions {
		out[i] = c.Name
	}
	return out
}

// Evaluate 只读评估全部门禁条件（不改 Run 状态）。
func (g *ExitGate) Evaluate(ctx context.Context, runID string) (GateResult, error) {
	var res GateResult
	err := g.w.Durable(ctx, func(tx *sql.Tx) error {
		r, e := g.evalTx(ctx, tx, runID)
		res = r
		return e
	})
	return res, err
}

func (g *ExitGate) evalTx(ctx context.Context, tx *sql.Tx, runID string) (GateResult, error) {
	res := GateResult{Passed: true}
	for _, c := range g.conditions {
		ok, detail, err := c.Check(ctx, tx, runID)
		if err != nil {
			return res, err
		}
		if !ok {
			res.Passed = false
			res.Failures = append(res.Failures, GateFailure{Condition: c.Name, Detail: detail})
		}
	}
	return res, nil
}

// TryComplete 原子尝试完成 Run（§14.2）：
//  1. RUNNING→EXITING 冻结，记录冻结时的 run/snapshot/surface 修订基线；
//  2. 同一事务内：重读修订号，任一前进→放弃（EXITING→RUNNING，Aborted=true）；
//  3. 否则评估全部条件：全过→EXITING→COMPLETED（CAS WHERE run.revision=基线）；
//     有不过→EXITING→RUNNING，返回 Failures。
//
// 冻结与「重读+评估+CAS」分两段事务：单写者串行保证两段之间其它写者若推进了修订，第 2 段能检出并放弃。
func (g *ExitGate) TryComplete(ctx context.Context, runID string, now time.Time) (CompleteResult, error) {
	// 第 1 段：冻结到 EXITING，拿冻结后基线。
	var baseRunRev, baseSnapRev, baseSurfRev int64
	err := g.w.Durable(ctx, func(tx *sql.Tx) error {
		cur, rev, e := readRunStateRev(ctx, tx, runID)
		if e != nil {
			return e
		}
		to, ok := Next(cur, EventExit)
		if !ok {
			return domain.Errorf(domain.ErrInvalid, fmt.Sprintf("run %s: 状态 %s 不接受 exit", runID, cur))
		}
		if e := casRunState(ctx, tx, runID, to, rev); e != nil {
			return e
		}
		baseRunRev = rev + 1 // EXITING 迁移后的 revision
		baseSnapRev, baseSurfRev, e = readSnapSurf(ctx, tx, runID)
		return e
	})
	if err != nil {
		return CompleteResult{}, err
	}

	// 第 2 段：重读修订 → 评估 → CAS 完成 / 放弃。
	var out CompleteResult
	err = g.w.Durable(ctx, func(tx *sql.Tx) error {
		cur, rev, e := readRunStateRev(ctx, tx, runID)
		if e != nil {
			return e
		}
		if cur != domain.RunExiting {
			// 评估窗口内被别的控制器移出 EXITING（pause/stop/fail）——交还控制，不强完成。
			out = CompleteResult{Completed: false, State: cur, Aborted: true, AbortInfo: "run left EXITING: " + string(cur)}
			return nil
		}
		// 1) 评估条件（只读/带副作用的注入条件都在此执行）。
		gr, e := g.evalTx(ctx, tx, runID)
		if e != nil {
			return e
		}
		out.Gate = gr
		if !gr.Passed {
			reopen, _ := Next(domain.RunExiting, EventReopen)
			if e := casRunState(ctx, tx, runID, reopen, rev); e != nil {
				return e
			}
			out.Completed = false
			out.State = reopen
			return nil
		}
		// 2) 评估后再验修订漂移：snapshot/surface 任一前进、或 run.revision 偏离冻结基线即放弃（§14.2「评估期间
		//    任何 Revision 前进都放弃」）。放在评估之后，才能捕获评估窗口内发生的并发 PlanPatch/Surface 变更。
		snap, surf, e := readSnapSurf(ctx, tx, runID)
		if e != nil {
			return e
		}
		if snap != baseSnapRev || surf != baseSurfRev || rev != baseRunRev {
			reopen, _ := Next(domain.RunExiting, EventReopen)
			if e := casRunState(ctx, tx, runID, reopen, rev); e != nil {
				return e
			}
			out = CompleteResult{Completed: false, State: reopen, Aborted: true, Gate: gr,
				AbortInfo: fmt.Sprintf("revision drift: snap %d→%d surf %d→%d run %d→%d",
					baseSnapRev, snap, baseSurfRev, surf, baseRunRev, rev)}
			return nil
		}
		// 3) 全过且无漂移：EXITING→COMPLETED，同一事务、CAS（WHERE run.revision=冻结基线）。
		done, _ := Next(domain.RunExiting, EventComplete)
		if e := casRunState(ctx, tx, runID, done, rev); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, `UPDATE runs SET ended_at=?, terminal_reason=? WHERE id=?`,
			ts(now), "exit_gate_passed", runID); e != nil {
			return e
		}
		out.Completed = true
		out.State = done
		return nil
	})
	return out, err
}

// PartialReason 是类型化 PARTIAL 原因（§14.2）。
type PartialReason struct {
	Code          string // 如 BUDGET_EXHAUSTED/OUTCOME_UNRESOLVED/INFRA_FAILURE/TIMEOUT
	EntityRef     string
	LostCoverage  string
	RecoveredFrom string
}

// 常见 PARTIAL 原因码。
const (
	PartialBudgetExhausted = "BUDGET_EXHAUSTED"
	PartialInfraFailure    = "INFRA_FAILURE"
	PartialTimeout         = "TIMEOUT"
	PartialOutcomeUnknown  = "OUTCOME_UNRESOLVED"
)

// Partial 把 Run 收尾为 PARTIAL 并写类型化原因（§14.2）。预算耗尽走这里，**不绕门禁**。
// 从 RUNNING/BLOCKED/EXITING 均可进 PARTIAL（状态机已含这些边）。必须至少给一条原因，
// 否则空结果会被伪装成「零发现」。
func (g *ExitGate) Partial(ctx context.Context, runID string, reasons []PartialReason, now time.Time) (domain.RunState, error) {
	if len(reasons) == 0 {
		return "", domain.Errorf(domain.ErrInvalid, "PARTIAL 必须带至少一条 run_partial_reasons（§14.2）")
	}
	var next domain.RunState
	err := g.w.Durable(ctx, func(tx *sql.Tx) error {
		cur, rev, e := readRunStateRev(ctx, tx, runID)
		if e != nil {
			return e
		}
		to, ok := Next(cur, EventPartial)
		if !ok {
			return domain.Errorf(domain.ErrInvalid, fmt.Sprintf("run %s: 状态 %s 不接受 partial", runID, cur))
		}
		if e := casRunState(ctx, tx, runID, to, rev); e != nil {
			return e
		}
		for _, r := range reasons {
			if strings.TrimSpace(r.Code) == "" {
				return domain.Errorf(domain.ErrInvalid, "PARTIAL 原因 code 不能为空")
			}
			if _, e := tx.ExecContext(ctx, `
				INSERT INTO run_partial_reasons (id, run_id, code, entity_ref, lost_coverage, recovered_from_ref, created_at)
				VALUES (?,?,?,?,?,?,?)`,
				string(domain.NewID()), runID, r.Code, nzc(r.EntityRef), nzc(r.LostCoverage), nzc(r.RecoveredFrom), ts(now)); e != nil {
				return e
			}
		}
		if _, e := tx.ExecContext(ctx, `UPDATE runs SET ended_at=?, terminal_reason=? WHERE id=?`,
			ts(now), "partial", runID); e != nil {
			return e
		}
		next = to
		return nil
	})
	return next, err
}

// --- 事务内小工具 ---

func readRunStateRev(ctx context.Context, tx *sql.Tx, runID string) (domain.RunState, int64, error) {
	var st string
	var rev int64
	e := tx.QueryRowContext(ctx, `SELECT state, revision FROM runs WHERE id=?`, runID).Scan(&st, &rev)
	if e == sql.ErrNoRows {
		return "", 0, domain.Errorf(domain.ErrNotFound, "run "+runID)
	}
	return domain.RunState(st), rev, e
}

func readSnapSurf(ctx context.Context, tx *sql.Tx, runID string) (int64, int64, error) {
	var snap int64
	if e := tx.QueryRowContext(ctx, `SELECT current_snapshot_revision FROM runs WHERE id=?`, runID).Scan(&snap); e != nil {
		return 0, 0, e
	}
	var surf int64
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(surface_revision),0) FROM coverage_revisions WHERE run_id=?`, runID).Scan(&surf)
	return snap, surf, nil
}

func casRunState(ctx context.Context, tx *sql.Tx, runID string, to domain.RunState, rev int64) error {
	res, e := tx.ExecContext(ctx,
		`UPDATE runs SET state=?, revision=revision+1 WHERE id=? AND revision=?`, string(to), runID, rev)
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

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func nzc(s string) any {
	if s == "" {
		return nil
	}
	return s
}
