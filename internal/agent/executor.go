package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// executor.go —— Executor 快循环的持久化核心（§7.1 Agent Loop / §7.2 响应后持久化）。
//
// 两条硬规则（§7.2，吸收 DeepSeek Harness）：
//  1. Model-visible ⟺ logged：rendered_request_ref/request_digest 保存实际请求，可重建且 digest 一致。
//  2. 效果幂等提交：CommitModelResponse 先持久化响应，各效果以 (model_step_id, ordinal) 为幂等键；
//     崩溃恢复后继续提交剩余效果，**不重新生成模型响应**。
//
// StepContext 冻结（§7.1，吸收 Codex）：Generate 前持久化一份请求级快照，请求期间不可变；流式返回
// 期间工具目录/Provider/Policy/Identity 任何变化 → 旧响应整体失效、重新规划。

// 步状态。
const (
	StepStarted           = "STEP_STARTED"       // 已冻结 StepContext，待 Generate/提交响应
	StepResponseCommitted = "RESPONSE_COMMITTED" // 响应已持久化，效果待提交
	StepEffectsCommitted  = "EFFECTS_COMMITTED"  // 全部效果已幂等提交
	StepStale             = "STALE"              // 冻结后配置变化，旧响应整体失效
)

// 响应 Kind（§7.1 loop 分发）。
const (
	KindActionProposal  = "ActionProposal"
	KindSpawnJob        = "SpawnJob"
	KindObservation     = "Observation"
	KindCompletionClaim = "CompletionClaim"
	KindYield           = "Yield"
)

// ErrStaleResponse —— 冻结后 Policy/工具目录/Attempt 代际变化，旧响应整体失效（§7.1）。
var ErrStaleResponse = &domain.DomainError{Code: "STALE_RESPONSE", Msg: "step context changed, replan"}

// StepContext 是请求级不可变快照（§7.1）。
type StepContext struct {
	ModelStepID            string
	RunID                  string
	AttemptID              string
	AgentSessionID         string
	AttemptGeneration      int64
	FencingToken           int64
	ContextRevision        int64
	PolicyRevision         int64
	ToolCatalogDigest      string
	ModelProfileDigest     string
	ProviderSessionVersion int64
	IdentityRefs           string
	SandboxProfile         string
	SampledThroughSequence int64
}

// StepSpec 是冻结一步所需的输入（由 Runtime 从 Attempt 快照组装）。
type StepSpec struct {
	RunID                  string
	AttemptID              string
	AgentSessionID         string
	AttemptGeneration      int64
	FencingToken           int64
	ContextRevision        int64
	PolicyRevision         int64
	ToolCatalogDigest      string
	ModelProfileDigest     string
	ProviderSessionVersion int64
	IdentityRefs           string
	SandboxProfile         string
	SampledThroughSequence int64
}

// Effect 是模型响应产生的一个待提交效果。
type Effect struct {
	Ordinal int
	Kind    string
	Payload json.RawMessage
}

// ModelResponse 是一次模型响应的持久化载荷（§7.2）。
type ModelResponse struct {
	StopReason            string
	StructuredOutput      json.RawMessage
	Usage                 TokenUsage
	ProviderRequestID     string
	RequestDigest         string
	RenderedRequestRef    string
	PublicProgressSummary string
	Kind                  string
	Effects               []Effect
}

// Generator 产生一步的结构化响应（Day 23 Provider 的适配；效果已解码）。
type Generator interface {
	Generate(ctx context.Context, step StepContext) (ModelResponse, error)
}

// EffectApplier 在给定 tx 内落地一个效果（与「标记已提交」同事务 → 原子、幂等）。
// 返回效果产生的实体 id（如 action id）。真实落地（execution.SubmitProposal、knowledge.RecordObservation
// 等）在后续天接线（见 PROGRESS「集成待接线清单」）。
type EffectApplier interface {
	Apply(ctx context.Context, tx *sql.Tx, step StepContext, eff Effect) (refID string, err error)
}

// Executor 是快循环的持久化执行体。
type Executor struct {
	w       *store.Writer
	applier EffectApplier
}

// NewExecutor 构造。applier 为效果落地器（测试可注入）。
func NewExecutor(w *store.Writer, applier EffectApplier) *Executor {
	return &Executor{w: w, applier: applier}
}

// FreezeStep 在 Generate 前持久化 StepContext（state=STEP_STARTED），返回冻结快照（§7.1）。
func (e *Executor) FreezeStep(ctx context.Context, spec StepSpec, now time.Time) (StepContext, error) {
	id := string(domain.NewID())
	sc := StepContext{
		ModelStepID: id, RunID: spec.RunID, AttemptID: spec.AttemptID, AgentSessionID: spec.AgentSessionID,
		AttemptGeneration: spec.AttemptGeneration, FencingToken: spec.FencingToken,
		ContextRevision: spec.ContextRevision, PolicyRevision: spec.PolicyRevision,
		ToolCatalogDigest: spec.ToolCatalogDigest, ModelProfileDigest: spec.ModelProfileDigest,
		ProviderSessionVersion: spec.ProviderSessionVersion, IdentityRefs: spec.IdentityRefs,
		SandboxProfile: spec.SandboxProfile, SampledThroughSequence: spec.SampledThroughSequence,
	}
	tstr := ts(now)
	err := e.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e2 := tx.ExecContext(ctx, `
			INSERT INTO model_steps
			  (id, run_id, attempt_id, agent_session_id, attempt_generation, fencing_token,
			   context_revision, policy_revision, tool_catalog_digest, model_profile_digest,
			   provider_session_version, identity_refs, sandbox_profile, sampled_through_sequence,
			   state, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, spec.RunID, nz(spec.AttemptID), nz(spec.AgentSessionID), spec.AttemptGeneration,
			spec.FencingToken, spec.ContextRevision, spec.PolicyRevision, spec.ToolCatalogDigest,
			spec.ModelProfileDigest, spec.ProviderSessionVersion, spec.IdentityRefs, spec.SandboxProfile,
			spec.SampledThroughSequence, StepStarted, tstr, tstr)
		return e2
	})
	if err != nil {
		return StepContext{}, err
	}
	return sc, nil
}

// CommitModelResponse 持久化响应本体 + 效果账本，同一事务（§7.2）。幂等：仅在 STEP_STARTED 时写入，
// 重复调用（恢复）直接返回成功，不重复插入效果（(model_step_id,ordinal) 主键 + INSERT OR IGNORE）。
func (e *Executor) CommitModelResponse(ctx context.Context, step StepContext, resp ModelResponse, now time.Time) error {
	usageJSON, _ := json.Marshal(resp.Usage)
	tstr := ts(now)
	return e.w.Durable(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE model_steps SET response_stop_reason=?, structured_output=?, usage_json=?,
			  request_digest=?, rendered_request_ref=?, public_progress_summary=?, provider_request_id=?,
			  response_kind=?, effects_total=?, state=?, updated_at=?
			WHERE id=? AND state=?`,
			resp.StopReason, string(resp.StructuredOutput), string(usageJSON), resp.RequestDigest,
			resp.RenderedRequestRef, resp.PublicProgressSummary, nz(resp.ProviderRequestID), resp.Kind,
			len(resp.Effects), StepResponseCommitted, tstr, step.ModelStepID, StepStarted)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			// 已非 STEP_STARTED：响应早已提交（恢复路径），幂等返回。
			return nil
		}
		for _, eff := range resp.Effects {
			if _, err = tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO model_step_effects (model_step_id, ordinal, kind, payload, committed, created_at)
				VALUES (?,?,?,?,0,?)`,
				step.ModelStepID, eff.Ordinal, eff.Kind, string(eff.Payload), tstr); err != nil {
				return err
			}
		}
		return nil
	})
}

// CheckFresh 校验冻结后配置未变（§7.1）：Policy 代际与（可选）工具目录摘要。变化即 ErrStaleResponse。
// 当前以 runs.current_policy_revision 为准；工具目录/Attempt 代际的实时比对随 Day 26/30 接线。
func (e *Executor) CheckFresh(ctx context.Context, step StepContext) error {
	var curPolicy int64
	err := e.w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT current_policy_revision FROM runs WHERE id=?`, step.RunID).Scan(&curPolicy)
	})
	if err == sql.ErrNoRows {
		return domain.Errorf(domain.ErrNotFound, "run "+step.RunID)
	}
	if err != nil {
		return err
	}
	if curPolicy != step.PolicyRevision {
		return domain.Errorf(ErrStaleResponse, "policy revision changed")
	}
	return nil
}

// MarkStale 置 STALE（旧响应整体失效，不提交任何效果）。
func (e *Executor) MarkStale(ctx context.Context, modelStepID string, now time.Time) error {
	return e.w.Durable(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE model_steps SET state=?, updated_at=? WHERE id=? AND state!=?`,
			StepStale, ts(now), modelStepID, StepEffectsCommitted)
		return err
	})
}

// CommitEffects 幂等提交全部未提交效果（§7.2）。每效果一个事务：applier.Apply 落地 + 标记 committed
// 同事务原子；仅处理 committed=0（崩溃恢复后续提交不重复）。全部提交后置 EFFECTS_COMMITTED。
// 返回本次新提交的效果数。
func (e *Executor) CommitEffects(ctx context.Context, modelStepID string, now time.Time) (int, error) {
	committed := 0
	for {
		done := false
		err := e.w.Durable(ctx, func(tx *sql.Tx) error {
			// 取下一个未提交效果（ordinal 升序）。
			var ord int
			var kind, payload string
			e2 := tx.QueryRowContext(ctx, `
				SELECT ordinal, kind, COALESCE(payload,'') FROM model_step_effects
				WHERE model_step_id=? AND committed=0 ORDER BY ordinal LIMIT 1`, modelStepID).
				Scan(&ord, &kind, &payload)
			if e2 == sql.ErrNoRows {
				// 无未提交效果：收束步。
				_, e3 := tx.ExecContext(ctx,
					`UPDATE model_steps SET state=?, updated_at=? WHERE id=? AND state=?`,
					StepEffectsCommitted, ts(now), modelStepID, StepResponseCommitted)
				done = true
				return e3
			}
			if e2 != nil {
				return e2
			}
			step, e3 := e.loadStep(ctx, tx, modelStepID)
			if e3 != nil {
				return e3
			}
			refID, e3 := e.applier.Apply(ctx, tx, step, Effect{Ordinal: ord, Kind: kind, Payload: json.RawMessage(payload)})
			if e3 != nil {
				return e3
			}
			_, e3 = tx.ExecContext(ctx, `
				UPDATE model_step_effects SET committed=1, ref_id=?, committed_at=?
				WHERE model_step_id=? AND ordinal=?`, nz(refID), ts(now), modelStepID, ord)
			if e3 != nil {
				return e3
			}
			_, e3 = tx.ExecContext(ctx,
				`UPDATE model_steps SET effects_committed=effects_committed+1, updated_at=? WHERE id=?`,
				ts(now), modelStepID)
			return e3
		})
		if err != nil {
			return committed, err
		}
		if done {
			return committed, nil
		}
		committed++
	}
}

// loadStep 在 tx 内读一步的 StepContext（供 applier 使用）。
func (e *Executor) loadStep(ctx context.Context, tx *sql.Tx, modelStepID string) (StepContext, error) {
	var sc StepContext
	sc.ModelStepID = modelStepID
	e2 := tx.QueryRowContext(ctx, `
		SELECT run_id, COALESCE(attempt_id,''), COALESCE(agent_session_id,''), attempt_generation,
		       fencing_token, context_revision, policy_revision, COALESCE(tool_catalog_digest,''),
		       COALESCE(sandbox_profile,'')
		FROM model_steps WHERE id=?`, modelStepID).
		Scan(&sc.RunID, &sc.AttemptID, &sc.AgentSessionID, &sc.AttemptGeneration, &sc.FencingToken,
			&sc.ContextRevision, &sc.PolicyRevision, &sc.ToolCatalogDigest, &sc.SandboxProfile)
	if e2 == sql.ErrNoRows {
		return sc, domain.Errorf(domain.ErrNotFound, "model_step "+modelStepID)
	}
	return sc, e2
}

// StepResult 是一步的结果。
type StepResult struct {
	ModelStepID      string
	Kind             string
	EffectsCommitted int
	Stale            bool
}

// Step 跑完整一步（§7.1）：冻结→Generate→提交响应→校验新鲜→幂等提交效果。
// 冻结后失效（Policy 等变化）→ 标 STALE 并返回 ErrStaleResponse（调用方重新规划）。
func (e *Executor) Step(ctx context.Context, spec StepSpec, gen Generator, now time.Time) (StepResult, error) {
	step, err := e.FreezeStep(ctx, spec, now)
	if err != nil {
		return StepResult{}, err
	}
	resp, err := gen.Generate(ctx, step)
	if err != nil {
		return StepResult{ModelStepID: step.ModelStepID}, err // 响应未提交，步留 STEP_STARTED 待重试
	}
	if err = e.CommitModelResponse(ctx, step, resp, now); err != nil {
		return StepResult{ModelStepID: step.ModelStepID}, err
	}
	if err = e.CheckFresh(ctx, step); err != nil {
		_ = e.MarkStale(ctx, step.ModelStepID, now)
		return StepResult{ModelStepID: step.ModelStepID, Kind: resp.Kind, Stale: true}, err
	}
	n, err := e.CommitEffects(ctx, step.ModelStepID, now)
	return StepResult{ModelStepID: step.ModelStepID, Kind: resp.Kind, EffectsCommitted: n}, err
}
