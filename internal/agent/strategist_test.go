package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"pentagent/internal/providers"
)

var stratCaps = providers.Capabilities{ToolCalling: true, StructuredOutput: true, MaxOutput: 4096}

func planResp(input string) *providers.Response {
	return &providers.Response{ToolName: "emit_plan_patch", ToolInput: json.RawMessage(input), StopReason: providers.StopToolUse}
}

func snap182() StrategySnapshot {
	return StrategySnapshot{RunID: "run-1", SnapshotRevision: 182, PolicyDigest: "d", AssetSummary: json.RawMessage(`{"hosts":1}`)}
}

// DoD：给一个 Snapshot，Strategist 产出合法 PlanPatch（创建若干 Job）。
func TestStrategistProducesValidPlanPatch(t *testing.T) {
	valid := `{"snapshot_revision":182,"epoch_reason":"new_high_value_discovery","operations":[{"op":"create_job","job":{"kind":"DISCOVER"}},{"op":"create_job","job":{"kind":"MAP"}},{"op":"upsert_hypothesis","hypothesis":{}}]}`
	mp := providers.NewMockProvider(stratCaps, providers.MockStep{Resp: planResp(valid)})
	s, err := NewStrategist(mp, "test-model")
	if err != nil {
		t.Fatal(err)
	}
	pp, err := s.Plan(context.Background(), snap182())
	if err != nil {
		t.Fatalf("合法快照应产出 PlanPatch：%v", err)
	}
	if pp.SnapshotRevision != 182 {
		t.Fatalf("snapshot_revision 应回填 182，实得 %d", pp.SnapshotRevision)
	}
	if len(pp.Ops) != 3 || pp.Ops[0] != "create_job" || pp.Ops[2] != "upsert_hypothesis" {
		t.Fatalf("ops 不符：%v", pp.Ops)
	}
}

// DoD：试图改 Scope 的 op 被拒——modify_scope 不在 op 枚举，schema 校验直接拒（修复一次仍拒）。
func TestStrategistRejectsScopeModViaSchema(t *testing.T) {
	bad := `{"snapshot_revision":182,"operations":[{"op":"modify_scope","scope":{"hosts":["evil.test"]}}]}`
	mp := providers.NewMockProvider(stratCaps,
		providers.MockStep{Resp: planResp(bad)}, // 首次
		providers.MockStep{Resp: planResp(bad)}, // 修复一次仍非法
	)
	s, _ := NewStrategist(mp, "test-model")
	_, err := s.Plan(context.Background(), snap182())
	if providers.FaultOf(err) != providers.FaultInvalidSchema {
		t.Fatalf("改 Scope 的 op 应被 schema 拒（FaultInvalidSchema），实得 %v", err)
	}
}

// 纵深防御：即便 schema 被放宽，解析层白名单仍拒非 §6.3 操作。
func TestParsePlanPatchForbiddenOpGuard(t *testing.T) {
	_, err := parsePlanPatch([]byte(`{"snapshot_revision":1,"operations":[{"op":"end_run"}]}`), 1)
	if !errors.Is(err, ErrForbiddenOp) {
		t.Fatalf("非白名单 op 应 ErrForbiddenOp，实得 %v", err)
	}
}

// 过期规划防护：patch 的 snapshot_revision 与规划所用快照不一致 → 拒。
func TestParsePlanPatchStaleSnapshot(t *testing.T) {
	_, err := parsePlanPatch([]byte(`{"snapshot_revision":5,"operations":[]}`), 7)
	if !errors.Is(err, ErrStaleSnapshot) {
		t.Fatalf("快照代际不一致应 ErrStaleSnapshot，实得 %v", err)
	}
}

// DoD：Debounce + 单飞 + 事件合并 + dirty 重排（§6.5/§6.4）。
func TestTriggerGateDebounceSingleFlight(t *testing.T) {
	g := NewTriggerGate(5 * time.Second)
	base := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

	g.Fire("run-1", "run.started", base)
	g.Fire("run-1", "surface.revision_changed", base)
	g.Fire("run-1", "surface.revision_changed", base.Add(1*time.Second)) // 合并重复

	// 静默期未到（距最后 fire base+1s 不足 5s）→ 不领取。
	if _, ok := g.Claim("run-1", base.Add(2*time.Second)); ok {
		t.Fatal("Debounce 期内不应领取规划")
	}
	// 静默 5s 后 → 领取，合并为 2 个不同事件。
	evs, ok := g.Claim("run-1", base.Add(6*time.Second))
	if !ok || len(evs) != 2 {
		t.Fatalf("静默期后应领取合并事件，实得 ok=%v evs=%v", ok, evs)
	}

	// 单飞：在途期间再 Claim → 拒。
	g.Fire("run-1", "job.failed", base.Add(7*time.Second))
	if _, ok := g.Claim("run-1", base.Add(13*time.Second)); ok {
		t.Fatal("单飞：在途时不应再领取")
	}
	// 释放后（规划期间有新事件 → dirty 重排），静默期后可重新领取 job.failed。
	g.Release("run-1", base.Add(14*time.Second))
	evs2, ok := g.Claim("run-1", base.Add(20*time.Second))
	if !ok || len(evs2) != 1 || evs2[0] != "job.failed" {
		t.Fatalf("dirty 重排应领取 job.failed，实得 ok=%v evs=%v", ok, evs2)
	}
}

// Snapshot 的 JCS 摘要确定且对内容敏感。
func TestSnapshotDigestStable(t *testing.T) {
	a, err := snap182().Digest()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := snap182().Digest()
	if a != b {
		t.Fatal("同一快照摘要应稳定")
	}
	other := snap182()
	other.SnapshotRevision = 183
	c, _ := other.Digest()
	if a == c {
		t.Fatal("不同快照摘要应不同")
	}
}
