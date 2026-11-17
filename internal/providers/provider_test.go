package providers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"pentagent/api/schemas"
)

func planPatchTool(t *testing.T) *Tool {
	t.Helper()
	sb, err := schemas.FS.ReadFile(schemas.PlanPatch)
	if err != nil {
		t.Fatalf("读取 PlanPatch schema: %v", err)
	}
	return &Tool{Name: "emit_plan_patch", Description: "输出一个 PlanPatch", InputSchema: sb}
}

func baseReq(tool *Tool) Request {
	return Request{
		RequestID: "r1", Role: "strategist", Model: "test-model",
		System:   "你是规划器，只输出 PlanPatch。",
		Messages: []Message{{Role: "user", Content: "基于当前快照给出下一步计划。"}},
		Tool:     tool, MaxOutput: 1024,
	}
}

func toolResp(name, input string, stop StopReason) *Response {
	return &Response{RequestID: "r1", ToolName: name, ToolInput: json.RawMessage(input), StopReason: stop,
		Usage: Usage{InputTokens: 100, OutputTokens: 20}}
}

var caps = Capabilities{ToolCalling: true, StructuredOutput: true, MaxContext: 200000, MaxOutput: 8192}

const validPlanPatch = `{"snapshot_revision":182,"epoch_reason":"new_high_value_discovery","operations":[{"op":"create_job"}]}`
const invalidPlanPatch = `{"operations":[{"op":"create_job"}]}` // 缺必填 snapshot_revision

// DoD①：Mock 按 schema 返回一个合法 PlanPatch，一次即过（无修复）。
func TestEmitStructuredValid(t *testing.T) {
	tool := planPatchTool(t)
	mp := NewMockProvider(caps, MockStep{Resp: toolResp("emit_plan_patch", validPlanPatch, StopToolUse)})

	out, usage, err := EmitStructured(context.Background(), mp, baseReq(tool), Retrier{})
	if err != nil {
		t.Fatalf("合法输出不应报错：%v", err)
	}
	if string(out) != validPlanPatch {
		t.Fatalf("返回入参不符：%s", out)
	}
	if mp.Calls() != 1 {
		t.Fatalf("合法输出应只调用 1 次，实际 %d", mp.Calls())
	}
	if usage.Total() != 120 {
		t.Fatalf("用量累计错误：%d", usage.Total())
	}
}

// DoD②：非法结构化输出触发「最多一次修复」，修复后合法即过；共 2 次调用，修复请求追加了修复消息。
func TestEmitStructuredRepairsOnce(t *testing.T) {
	tool := planPatchTool(t)
	mp := NewMockProvider(caps,
		MockStep{Resp: toolResp("emit_plan_patch", invalidPlanPatch, StopToolUse)}, // 第一次非法
		MockStep{Resp: toolResp("emit_plan_patch", validPlanPatch, StopToolUse)},   // 修复后合法
	)

	out, _, err := EmitStructured(context.Background(), mp, baseReq(tool), Retrier{})
	if err != nil {
		t.Fatalf("修复后应成功：%v", err)
	}
	if string(out) != validPlanPatch {
		t.Fatalf("修复后入参不符：%s", out)
	}
	if mp.Calls() != 2 {
		t.Fatalf("应恰好 2 次（原始 + 修复一次），实际 %d", mp.Calls())
	}
	seen := mp.Seen()
	if n := len(seen[1].Messages); n != len(baseReq(tool).Messages)+1 {
		t.Fatalf("修复请求应追加 1 条修复消息，实际消息数 %d", n)
	}
}

// 修复仍不合法 → fail-closed 返回 FaultInvalidSchema，且绝不第三次调用。
func TestEmitStructuredRepairFailsClosed(t *testing.T) {
	tool := planPatchTool(t)
	mp := NewMockProvider(caps,
		MockStep{Resp: toolResp("emit_plan_patch", invalidPlanPatch, StopToolUse)},
		MockStep{Resp: toolResp("emit_plan_patch", invalidPlanPatch, StopToolUse)},
	)

	_, _, err := EmitStructured(context.Background(), mp, baseReq(tool), Retrier{})
	if FaultOf(err) != FaultInvalidSchema {
		t.Fatalf("应为 FaultInvalidSchema，实际 %v", err)
	}
	if mp.Calls() != 2 {
		t.Fatalf("修复至多一次：应 2 次，实际 %d", mp.Calls())
	}
}

// 输出截断 → 不执行工具、不修复（§7.4）。
func TestEmitStructuredTruncated(t *testing.T) {
	tool := planPatchTool(t)
	mp := NewMockProvider(caps, MockStep{Resp: toolResp("emit_plan_patch", invalidPlanPatch, StopMaxTokens)})
	_, _, err := EmitStructured(context.Background(), mp, baseReq(tool), Retrier{})
	if FaultOf(err) != FaultTruncated {
		t.Fatalf("截断应为 FaultTruncated，实际 %v", err)
	}
	if mp.Calls() != 1 {
		t.Fatalf("截断不应修复：应 1 次，实际 %d", mp.Calls())
	}
}

// DoD③：429 → 按 Retry-After 退避并重试成功（注入 Sleep 断言退避时长）。
func TestCallRetries429HonorsRetryAfter(t *testing.T) {
	tool := planPatchTool(t)
	var slept []time.Duration
	rt := Retrier{
		Policy: RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Second},
		Sleep:  func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil },
	}
	mp := NewMockProvider(caps,
		MockStep{Err: &ProviderError{Class: FaultRateLimited, RetryAfter: 2 * time.Second}},
		MockStep{Resp: toolResp("emit_plan_patch", validPlanPatch, StopToolUse)},
	)

	resp, err := rt.Call(context.Background(), mp, baseReq(tool))
	if err != nil {
		t.Fatalf("429 后重试应成功：%v", err)
	}
	if resp.StopReason != StopToolUse {
		t.Fatalf("重试结果不符")
	}
	if len(slept) != 1 || slept[0] != 2*time.Second {
		t.Fatalf("应按 Retry-After 退避 2s 一次，实际 %v", slept)
	}
}

// 网络故障 → 指数退避重试；用尽后返回最后错误。
func TestCallRetriesNetworkExponential(t *testing.T) {
	tool := planPatchTool(t)
	var slept []time.Duration
	rt := Retrier{
		Policy: RetryPolicy{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: time.Second},
		Sleep:  func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil },
	}
	neterr := &ProviderError{Class: FaultNetwork}
	mp := NewMockProvider(caps, MockStep{Err: neterr}, MockStep{Err: neterr}, MockStep{Err: neterr})

	_, err := rt.Call(context.Background(), mp, baseReq(tool))
	if FaultOf(err) != FaultNetwork {
		t.Fatalf("应返回网络故障，实际 %v", err)
	}
	if mp.Calls() != 3 {
		t.Fatalf("应尝试满 3 次，实际 %d", mp.Calls())
	}
	if len(slept) != 2 || slept[0] != 10*time.Millisecond || slept[1] != 20*time.Millisecond {
		t.Fatalf("指数退避应为 [10ms,20ms]，实际 %v", slept)
	}
}

// DoD④：安全拒绝不盲重试、不换 Provider——无论拒绝以错误还是 StopRefusal 形式到来。
func TestSafetyRefusalNoRetryNoSwitch(t *testing.T) {
	tool := planPatchTool(t)
	var sleeps int
	rt := Retrier{
		Policy: RetryPolicy{MaxAttempts: 4, BaseDelay: time.Millisecond},
		Sleep:  func(_ context.Context, _ time.Duration) error { sleeps++; return nil },
	}

	// 形式 A：Provider 直接返回安全拒绝错误。
	refuser := NewMockProvider(caps, MockStep{Err: &ProviderError{Class: FaultSafetyRefusal}})
	other := NewMockProvider(caps, MockStep{Resp: toolResp("emit_plan_patch", validPlanPatch, StopToolUse)})
	_, err := rt.Call(context.Background(), refuser, baseReq(tool))
	if FaultOf(err) != FaultSafetyRefusal {
		t.Fatalf("应返回安全拒绝，实际 %v", err)
	}
	if refuser.Calls() != 1 {
		t.Fatalf("安全拒绝不应重试：应 1 次，实际 %d", refuser.Calls())
	}
	if sleeps != 0 {
		t.Fatalf("安全拒绝不应退避等待")
	}
	if other.Calls() != 0 {
		t.Fatalf("安全拒绝不应换 Provider：other 不应被调用")
	}

	// 形式 B：Provider 返回成功响应但 StopReason=refusal，EmitStructured 归类为安全拒绝、不修复。
	refuser2 := NewMockProvider(caps, MockStep{Resp: toolResp("", "", StopRefusal)})
	_, _, err = EmitStructured(context.Background(), refuser2, baseReq(tool), Retrier{})
	if FaultOf(err) != FaultSafetyRefusal {
		t.Fatalf("StopRefusal 应归类安全拒绝，实际 %v", err)
	}
	if refuser2.Calls() != 1 {
		t.Fatalf("安全拒绝不应修复：应 1 次，实际 %d", refuser2.Calls())
	}
}

// Registry 按角色解析 Provider/Profile。
func TestRegistryForRole(t *testing.T) {
	r := NewRegistry()
	mp := NewMockProvider(caps)
	r.Register("mock", mp)
	r.SetProfile("strategist", Profile{Provider: "mock", Model: "m1", Effort: "high"})

	p, pr, err := r.ForRole("strategist")
	if err != nil {
		t.Fatalf("解析角色失败：%v", err)
	}
	if p != Provider(mp) || pr.Model != "m1" {
		t.Fatalf("解析结果不符：%+v", pr)
	}
	if _, _, err := r.ForRole("verifier"); err != ErrNoProfile {
		t.Fatalf("未配置角色应 ErrNoProfile，实际 %v", err)
	}
	r.SetProfile("verifier", Profile{Provider: "missing", Model: "x"})
	if _, _, err := r.ForRole("verifier"); err != ErrNoProvider {
		t.Fatalf("Provider 未注册应 ErrNoProvider，实际 %v", err)
	}
}

// Cancel 被记录（最佳努力取消在途请求）。
func TestMockCancel(t *testing.T) {
	mp := NewMockProvider(caps)
	if err := mp.Cancel(context.Background(), "req-9"); err != nil {
		t.Fatalf("Cancel 不应报错：%v", err)
	}
	if !mp.Canceled("req-9") {
		t.Fatalf("Cancel 应被记录")
	}
}
