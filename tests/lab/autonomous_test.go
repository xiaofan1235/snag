// Package lab 跑端到端自主推进实验（§23 M3 验收："AI 能根据新 Evidence 动态创建后续 Job"）。
//
// 本测试用真实 Strategist（agent）+ PlanCompiler（agent）+ Scheduler（scheduler）+ 内容感知 Mock Provider，
// 验证**自主、非固定脚本**的推进：快照变（有了新证据）→ Strategist 产出**不同**的 PlanPatch → 落库建不同
// 类型的 Job；以及一个方向停滞 → Worker 转向。
//
// 范围说明（实事求是）：执行面→证据→快照的完整回路（FakeRunner→Proxy→Evidence，Day 19-21 已各自测过）
// 在此以「证据驱动的快照演进」模拟——把一次执行的产物直接体现为下一版快照的 high_value_discoveries，
// 聚焦于「指挥层据新证据自主再规划 + 调度转向」这一 M3 核心能力。组件接入单一 daemon 属 Day 39+（见
// PROGRESS 集成待接线清单）。
package lab

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"pentagent/internal/agent"
	"pentagent/internal/providers"
	"pentagent/internal/scheduler"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

var revRe = regexp.MustCompile(`"snapshot_revision":(\d+)`)

// evidenceDrivenProvider 按请求里的 snapshot_revision 返回对应 PlanPatch——模拟「模型读了证据后决策」。
// 这正是「非固定脚本」：输出取决于输入快照，证据变则计划变。
type evidenceDrivenProvider struct {
	byRev map[int]string
}

func (p *evidenceDrivenProvider) Generate(_ context.Context, req providers.Request) (*providers.Response, error) {
	rev := -1
	for _, m := range req.Messages {
		if mm := revRe.FindStringSubmatch(m.Content); mm != nil {
			rev, _ = strconv.Atoi(mm[1])
			break
		}
	}
	patch, ok := p.byRev[rev]
	if !ok {
		return &providers.Response{StopReason: providers.StopEndTurn}, nil
	}
	return &providers.Response{
		ToolName: "emit_plan_patch", ToolInput: json.RawMessage(patch), StopReason: providers.StopToolUse,
		Usage: providers.Usage{InputTokens: 100, OutputTokens: 20},
	}, nil
}

func (p *evidenceDrivenProvider) StreamGenerate(ctx context.Context, req providers.Request) (<-chan providers.StreamChunk, error) {
	ch := make(chan providers.StreamChunk, 1)
	close(ch)
	return ch, nil
}
func (p *evidenceDrivenProvider) CountTokens(context.Context, providers.Request) (int, error) {
	return 100, nil
}
func (p *evidenceDrivenProvider) GetCapabilities() providers.Capabilities {
	return providers.Capabilities{ToolCalling: true, StructuredOutput: true, MaxOutput: 4096}
}
func (p *evidenceDrivenProvider) Cancel(context.Context, string) error { return nil }

func openDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "lab.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	return w, context.Background()
}

func seedRaw(t *testing.T, w *store.Writer, ctx context.Context) {
	t.Helper()
	const tss = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-l", "a", "m", tss); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,current_snapshot_revision,created_at) VALUES (?,?,?,?,?,?,?)`,
			"run-1", "ws-l", "r", "RUNNING", "d", 5, tss); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO strategy_epochs (epoch_id,run_id,reason,opened_at) VALUES (?,?,?,?)`, "ep-1", "run-1", "init", tss)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func jobKind(t *testing.T, w *store.Writer, ctx context.Context, jobID string) string {
	t.Helper()
	var k string
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT kind FROM jobs WHERE id=?`, jobID).Scan(&k)
	})
	return k
}

// DoD：AI 据新证据动态创建后续 Job（非固定脚本）+ 方向停滞 → Worker 转向（§23 M3 / §32.5）。
func TestAutonomousProgressionAndPivot(t *testing.T) {
	w, ctx := openDB(t)
	now := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

	// 播种 run（snapshot_revision=5）+ epoch。
	seedRaw(t, w, ctx)

	prov := &evidenceDrivenProvider{byRev: map[int]string{
		// 快照 5（尚无发现）→ 先做发现 Job。
		5: `{"snapshot_revision":5,"operations":[{"op":"create_job","job":{"kind":"DISCOVER","spec":{"dir":"recon"}}}]}`,
		// 快照 6（已拿到高价值发现）→ 据此做实验 Job（与上一步不同，证据驱动）。
		6: `{"snapshot_revision":6,"operations":[{"op":"create_job","job":{"kind":"EXPERIMENT","spec":{"dir":"authz","target":"/api/admin"}}}]}`,
	}}

	strat, err := agent.NewStrategist(prov, "test-model")
	if err != nil {
		t.Fatal(err)
	}
	comp, err := agent.NewPlanCompiler(w)
	if err != nil {
		t.Fatal(err)
	}

	// ---- 轮次 1：快照 5 → 规划 → 落库建 DISCOVER Job ----
	snap1 := agent.StrategySnapshot{RunID: "run-1", SnapshotRevision: 5, PolicyDigest: "d"}
	pp1, err := strat.Plan(ctx, snap1)
	if err != nil {
		t.Fatalf("轮次1 规划失败：%v", err)
	}
	res1, err := comp.Compile(ctx, "run-1", "ep-1", "patch-1", pp1, now)
	if err != nil || !res1.Applied() || len(res1.CreatedJobIDs) != 1 {
		t.Fatalf("轮次1 应建 1 个 DISCOVER Job：%+v err=%v", res1, err)
	}

	// ---- 模拟执行：DISCOVER 产出高价值发现 → 快照演进到 6 ----
	snap2 := agent.StrategySnapshot{
		RunID: "run-1", SnapshotRevision: 6, PolicyDigest: "d",
		HighValueDiscoveries: json.RawMessage(`[{"endpoint":"/api/admin","why":"疑似越权"}]`),
	}

	// ---- 轮次 2：快照 6 → 规划 → 落库建 EXPERIMENT Job（证据驱动，非固定脚本）----
	pp2, err := strat.Plan(ctx, snap2)
	if err != nil {
		t.Fatalf("轮次2 规划失败：%v", err)
	}
	res2, err := comp.Compile(ctx, "run-1", "ep-1", "patch-2", pp2, now)
	if err != nil || !res2.Applied() || len(res2.CreatedJobIDs) != 1 {
		t.Fatalf("轮次2 应建 1 个 EXPERIMENT Job：%+v err=%v", res2, err)
	}

	// 关键断言：两轮计划不同（据新证据自主变化，而非固定脚本）。
	if pp1.Ops[0] == pp2.Ops[0] && string(pp1.Raw) == string(pp2.Raw) {
		t.Fatal("两轮计划相同——未体现证据驱动的自主再规划")
	}
	k1, k2 := jobKind(t, w, ctx, res1.CreatedJobIDs[0]), jobKind(t, w, ctx, res2.CreatedJobIDs[0])
	if k1 != "DISCOVER" || k2 != "EXPERIMENT" {
		t.Fatalf("应据证据建不同类型 Job：实得 %s, %s", k1, k2)
	}

	// ---- 方向停滞 → Worker 转向（§32.5）----
	pt := scheduler.NewProgressTracker(3)
	pt.SetScore("recon", 8)
	pt.SetScore("authz", 6)
	for i := 0; i < 3; i++ {
		pt.RecordAction("authz") // authz 方向连续无进展
	}
	if !pt.Stalled("authz") {
		t.Fatal("authz 方向应判停滞")
	}
	pt.Park("authz", scheduler.ParkNeedsPrecondition)

	sel := scheduler.NewSelector(1.0, 0, nil)
	if pick := sel.Pick(pt.ActiveScores()); pick != "recon" {
		t.Fatalf("authz 停滞泊车后 Worker 应转向 recon，实得 %q", pick)
	}

	t.Logf("自主推进通过：轮次1 建 %s，证据演进后轮次2 建 %s；authz 停滞→转向 recon", k1, k2)
}
