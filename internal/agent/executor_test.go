package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"

	"pentagent/internal/store"
)

// recordingApplier 记录每个 (model_step_id, ordinal) 被 Apply 的次数，用于断言幂等。
type recordingApplier struct {
	mu      sync.Mutex
	applied []string
}

func (r *recordingApplier) Apply(_ context.Context, _ *sql.Tx, step StepContext, eff Effect) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.applied = append(r.applied, fmt.Sprintf("%s#%d", step.ModelStepID, eff.Ordinal))
	return "ref-" + eff.Kind, nil
}

func (r *recordingApplier) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.applied)
}

func (r *recordingApplier) hasDup() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[string]bool{}
	for _, a := range r.applied {
		if seen[a] {
			return true
		}
		seen[a] = true
	}
	return false
}

// scriptGen 按脚本逐步返回响应。
type scriptGen struct {
	steps []ModelResponse
	i     int
}

func (g *scriptGen) Generate(_ context.Context, _ StepContext) (ModelResponse, error) {
	r := g.steps[g.i]
	g.i++
	return r, nil
}

func stepState(t *testing.T, w *store.Writer, ctx context.Context, id string) (string, int, int) {
	t.Helper()
	var state string
	var total, committed int
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT state, effects_total, effects_committed FROM model_steps WHERE id=?`, id).
			Scan(&state, &total, &committed)
	})
	return state, total, committed
}

// DoD：CommitModelResponse 后崩溃 → 恢复续提交效果不重复（§7.2 幂等；§22.3 after_model_response_commit）。
func TestEffectIdempotentCrashRecovery(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRun(t, w, ctx, "run-1")
	ap := &recordingApplier{}
	ex := NewExecutor(w, ap)

	step, err := ex.FreezeStep(ctx, StepSpec{RunID: "run-1", PolicyRevision: 0}, t0)
	if err != nil {
		t.Fatal(err)
	}
	resp := ModelResponse{Kind: KindActionProposal, Effects: []Effect{
		{Ordinal: 0, Kind: KindActionProposal}, {Ordinal: 1, Kind: KindObservation},
	}}
	if err := ex.CommitModelResponse(ctx, step, resp, t0); err != nil {
		t.Fatal(err)
	}
	// 幂等：重复提交响应不重复插效果。
	if err := ex.CommitModelResponse(ctx, step, resp, t0); err != nil {
		t.Fatal(err)
	}

	// === 崩溃注入：响应已持久化、效果未提交。恢复 → 续提交。 ===
	n1, err := ex.CommitEffects(ctx, step.ModelStepID, t0)
	if err != nil || n1 != 2 {
		t.Fatalf("恢复应提交 2 个效果，实得 n=%d err=%v", n1, err)
	}
	// 再次恢复（重复）→ 0 个新提交（幂等）。
	n2, err := ex.CommitEffects(ctx, step.ModelStepID, t0)
	if err != nil || n2 != 0 {
		t.Fatalf("重复恢复应 0 新提交，实得 n=%d err=%v", n2, err)
	}
	if ap.count() != 2 {
		t.Fatalf("applier 应恰被调用 2 次（不重复），实得 %d", ap.count())
	}
	if ap.hasDup() {
		t.Fatal("同一 (step, ordinal) 被重复 Apply —— 幂等被破坏")
	}
	if state, total, committed := stepState(t, w, ctx, step.ModelStepID); state != StepEffectsCommitted || total != 2 || committed != 2 {
		t.Fatalf("步最终态异常：state=%s total=%d committed=%d", state, total, committed)
	}
}

// DoD：冻结后 Policy 变 → 旧响应整体失效，不提交任何效果（§7.1）。
func TestStaleResponseInvalidated(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRun(t, w, ctx, "run-1")
	// 冻结时 run policy=0；流式期间 Policy 跳到 6。
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `UPDATE runs SET current_policy_revision=6 WHERE id=?`, "run-1")
		return e
	})
	ap := &recordingApplier{}
	ex := NewExecutor(w, ap)

	res, err := ex.Step(ctx, StepSpec{RunID: "run-1", PolicyRevision: 5}, &scriptGen{steps: []ModelResponse{
		{Kind: KindActionProposal, Effects: []Effect{{Ordinal: 0, Kind: KindActionProposal}}},
	}}, t0)
	if !errors.Is(err, ErrStaleResponse) {
		t.Fatalf("Policy 变后应 ErrStaleResponse，实得 %v", err)
	}
	if !res.Stale {
		t.Fatal("结果应标记 Stale")
	}
	if ap.count() != 0 {
		t.Fatalf("失效响应不应提交任何效果，applier 被调用 %d 次", ap.count())
	}
	if state, _, committed := stepState(t, w, ctx, res.ModelStepID); state != StepStale || committed != 0 {
		t.Fatalf("步应为 STALE 且无效果提交，实得 state=%s committed=%d", state, committed)
	}
}

// DoD：Mock Provider 跑一个 Job：提案→观察→Yield（§7.1 循环从持久 Step 续至终态）。
func TestRunAttemptLoop(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRun(t, w, ctx, "run-1")
	const ts0 = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT INTO jobs (id,run_id,kind,state,generation,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, "job-1", "run-1", "k", "RUNNING", 1, ts0, ts0); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO attempts (id,job_id,generation,fencing_token,state,started_at) VALUES (?,?,?,?,?,?)`, "att-1", "job-1", 1, 7, "RUNNING", ts0)
		return e
	}); err != nil {
		t.Fatalf("seed job/attempt: %v", err)
	}

	ap := &recordingApplier{}
	rt := NewRuntime(w, ap)
	gen := &scriptGen{steps: []ModelResponse{
		{Kind: KindActionProposal, Effects: []Effect{{Ordinal: 0, Kind: KindActionProposal}}},
		{Kind: KindObservation, Effects: []Effect{{Ordinal: 0, Kind: KindObservation}}},
		{Kind: KindYield},
	}}

	results, err := rt.RunAttempt(ctx, "att-1", gen, t0, 10)
	if err != nil {
		t.Fatalf("循环出错：%v", err)
	}
	if len(results) != 3 {
		t.Fatalf("应跑 3 步（提案/观察/Yield），实得 %d", len(results))
	}
	if results[2].Kind != KindYield {
		t.Fatalf("末步应 Yield，实得 %s", results[2].Kind)
	}
	if ap.count() != 2 { // 提案 + 观察各 1 个效果；Yield 无效果
		t.Fatalf("应提交 2 个效果，实得 %d", ap.count())
	}
}
