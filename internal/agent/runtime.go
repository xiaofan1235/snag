package agent

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// runtime.go —— Agent Session 管理 + 快循环驱动（§7.1）。
//
// 循环每轮都从持久化快照重建 StepSpec（不保留「只能存在内存里的循环状态」，§7.1），
// 调 Executor.Step 走「冻结→Generate→提交响应→幂等提交效果」，直至 CompletionClaim/Yield/失效/步数上限。

// DefaultMaxSteps 防跑飞的步数上限（无进展/转向的精细判定在 Day 30）。
const DefaultMaxSteps = 256

// Runtime 驱动一个 Attempt 的快循环。
type Runtime struct {
	exec *Executor
	w    *store.Writer
}

// NewRuntime 构造。applier 为效果落地器。
func NewRuntime(w *store.Writer, applier EffectApplier) *Runtime {
	return &Runtime{exec: NewExecutor(w, applier), w: w}
}

// Exec 暴露底层 Executor（供崩溃恢复等细粒度操作/测试）。
func (rt *Runtime) Exec() *Executor { return rt.exec }

// loadSpec 从 Attempt 快照组装 StepSpec（§7.1 LoadAttemptSnapshot）。
func (rt *Runtime) loadSpec(ctx context.Context, attemptID string) (StepSpec, error) {
	var sp StepSpec
	sp.AttemptID = attemptID
	err := rt.w.Durable(ctx, func(tx *sql.Tx) error {
		var sessionID sql.NullString
		var ctxRev sql.NullInt64
		e := tx.QueryRowContext(ctx, `
			SELECT a.fencing_token, a.generation, r.id, r.current_policy_revision, a.agent_session_id
			FROM attempts a JOIN jobs j ON a.job_id=j.id JOIN runs r ON j.run_id=r.id
			WHERE a.id=?`, attemptID).
			Scan(&sp.FencingToken, &sp.AttemptGeneration, &sp.RunID, &sp.PolicyRevision, &sessionID)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "attempt "+attemptID)
		}
		if e != nil {
			return e
		}
		if sessionID.Valid {
			sp.AgentSessionID = sessionID.String
			// 带出 Session 的 context_revision / provider_session 版本（若有）。
			_ = tx.QueryRowContext(ctx,
				`SELECT context_revision FROM agent_sessions WHERE id=?`, sessionID.String).Scan(&ctxRev)
			if ctxRev.Valid {
				sp.ContextRevision = ctxRev.Int64
			}
		}
		return nil
	})
	return sp, err
}

// RunAttempt 跑一个 Attempt 的快循环，直至 CompletionClaim/Yield/失效/步数上限。返回每步结果。
// 每轮从持久快照重建 spec（§7.1）。失效（ErrStaleResponse）即停，交由上层重新规划。
func (rt *Runtime) RunAttempt(ctx context.Context, attemptID string, gen Generator, now time.Time, maxSteps int) ([]StepResult, error) {
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	var results []StepResult
	for i := 0; i < maxSteps; i++ {
		if err := ctx.Err(); err != nil { // ctx 贯穿取消（§46）
			return results, err
		}
		spec, err := rt.loadSpec(ctx, attemptID)
		if err != nil {
			return results, err
		}
		res, err := rt.exec.Step(ctx, spec, gen, now)
		results = append(results, res)
		if err != nil {
			return results, err // 含 ErrStaleResponse：旧响应失效，停循环重新规划
		}
		rt.touchSession(ctx, spec.AgentSessionID, now)
		if res.Kind == KindCompletionClaim || res.Kind == KindYield {
			return results, nil
		}
	}
	return results, nil
}

// touchSession 更新 Session 活跃时间（best-effort）。
func (rt *Runtime) touchSession(ctx context.Context, sessionID string, now time.Time) {
	if sessionID == "" {
		return
	}
	_ = rt.w.Durable(ctx, func(tx *sql.Tx) error {
		_, _ = tx.ExecContext(ctx,
			`UPDATE agent_sessions SET last_active_at=? WHERE id=?`, ts(now), sessionID)
		return nil
	})
}
