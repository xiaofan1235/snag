// Package faultinjection_test —— 故障注入全套（§22.3）。在真实 Hook 点逐个模拟崩溃，验证恢复。
//
// 覆盖本批次已接线的 Hook 点的关键验收断言（§22.3）：
//   - after_model_response_commit 崩溃后，恢复续提交已持久化效果，不产生重复 Action/Observation；
//   - after_exit_gate_pass 崩溃后，不写终态、Run 留 EXITING（可恢复），绝不留「半完成」。
//
// 命令：go test -tags sqlite_modernc ./tests/faultinjection/...（无驱动自动 SKIP）。
// 其余 Hook 点（before/after_permit_consume、before/after_network_send、during_browser_subrequests 等）
// 随对应子系统接线后补测，见 PROGRESS「集成待接线清单」。
package faultinjection_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"pentagent/internal/agent"
	"pentagent/internal/control"
	"pentagent/internal/faultinject"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

var t0 = time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

func openDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "fi.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() {
		faultinject.Disable() // 每用例后清空武装点
		_ = w.Close()
	})
	return w, context.Background()
}

func seedRun(t *testing.T, w *store.Writer, ctx context.Context, runID, state string) {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-fi", "a", "m", ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`,
			runID, "ws-fi", "r", state, "d", ts)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func runState(t *testing.T, w *store.Writer, ctx context.Context, runID string) string {
	t.Helper()
	var st string
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT state FROM runs WHERE id=?`, runID).Scan(&st)
	})
	return st
}

// recordingApplier 记录每个 (model_step_id, ordinal) 被 Apply 的次数，用于断言幂等不重复。
type recordingApplier struct {
	mu      sync.Mutex
	applied []string
}

func (r *recordingApplier) Apply(_ context.Context, _ *sql.Tx, step agent.StepContext, eff agent.Effect) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.applied = append(r.applied, step.ModelStepID+"#"+strconv.Itoa(eff.Ordinal))
	return "ref", nil
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

type scriptGen struct {
	resp agent.ModelResponse
}

func (g *scriptGen) Generate(_ context.Context, _ agent.StepContext) (agent.ModelResponse, error) {
	return g.resp, nil
}

// §22.3：after_model_response_commit 崩溃 → 恢复续提交已持久化效果，不产生重复 Action/Observation。
func TestAfterModelResponseCommitCrashRecovery(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1", "RUNNING")
	ap := &recordingApplier{}
	ex := agent.NewExecutor(w, ap)
	gen := &scriptGen{resp: agent.ModelResponse{
		Kind: agent.KindActionProposal,
		Effects: []agent.Effect{
			{Ordinal: 0, Kind: agent.KindActionProposal},
			{Ordinal: 1, Kind: agent.KindObservation},
		},
	}}

	// 武装 after_model_response_commit：响应持久化后立刻「崩溃」（效果未提交）。
	faultinject.ArmOnce(faultinject.AfterModelResponseCommit, errors.New("crash"), 1)

	res, err := ex.Step(ctx, agent.StepSpec{RunID: "run-1", PolicyRevision: 0}, gen, t0)
	if err == nil {
		t.Fatal("武装点应使 Step 返回崩溃错误")
	}
	if ap.count() != 0 {
		t.Fatalf("崩溃点在效果提交前，applier 不应被调用，实得 %d", ap.count())
	}

	// 恢复：对同一 model_step 续提交效果——幂等、不重复。
	n, err := ex.CommitEffects(ctx, res.ModelStepID, t0)
	if err != nil {
		t.Fatalf("恢复续提交失败：%v", err)
	}
	if n != 2 {
		t.Fatalf("恢复应提交 2 个效果，实得 %d", n)
	}
	if ap.count() != 2 || ap.hasDup() {
		t.Fatalf("applier 应恰 2 次且无重复（幂等），实得 count=%d dup=%v", ap.count(), ap.hasDup())
	}

	// 再次恢复（重复恢复）→ 0 新提交，applier 不再被调用。
	n2, _ := ex.CommitEffects(ctx, res.ModelStepID, t0)
	if n2 != 0 || ap.count() != 2 {
		t.Fatalf("重复恢复应 0 新提交、applier 仍 2 次，实得 n2=%d count=%d", n2, ap.count())
	}
}

// §22.3：after_exit_gate_pass 崩溃 → 不写终态、Run 留 EXITING（可恢复），绝不留「半完成」。
func TestAfterExitGatePassCrashNoHalfComplete(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-2", "RUNNING")
	g := control.NewExitGate(w, control.GateConfig{})

	// 武装 after_exit_gate_pass：门禁通过、终态写入前「崩溃」。
	faultinject.ArmOnce(faultinject.AfterExitGatePass, errors.New("crash"), 1)

	_, err := g.TryComplete(ctx, "run-2", t0)
	if err == nil {
		t.Fatal("武装点应使 TryComplete 返回崩溃错误（事务回滚）")
	}
	// 关键：Run 未 COMPLETED。EXITING 冻结在第一段事务已提交，故崩溃后留 EXITING（可恢复重评）。
	st := runState(t, w, ctx, "run-2")
	if st == "COMPLETED" {
		t.Fatalf("崩溃后绝不能 COMPLETED（半完成），实得 %s", st)
	}
	if st != "EXITING" {
		t.Fatalf("崩溃后应留 EXITING 待恢复，实得 %s", st)
	}

	// 恢复：解除故障后再次评估，正常完成。
	faultinject.Disarm(faultinject.AfterExitGatePass)
	res, err := g.TryComplete(ctx, "run-2", t0)
	if err != nil {
		t.Fatalf("恢复后 TryComplete 失败：%v", err)
	}
	if !res.Completed || runState(t, w, ctx, "run-2") != "COMPLETED" {
		t.Fatalf("恢复后应正常 COMPLETED，实得 completed=%v state=%s", res.Completed, runState(t, w, ctx, "run-2"))
	}
}

// 生产默认禁用：未 Enable 时 Hook 零注入（Step/TryComplete 正常）。
func TestFaultInjectDisabledByDefault(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-3", "RUNNING")
	// 不 Arm 任何点；faultinject 保持禁用。
	if faultinject.IsEnabled() {
		t.Fatal("默认应禁用")
	}
	g := control.NewExitGate(w, control.GateConfig{})
	res, err := g.TryComplete(ctx, "run-3", t0)
	if err != nil || !res.Completed {
		t.Fatalf("禁用时应正常完成：%v completed=%v", err, res.Completed)
	}
}
