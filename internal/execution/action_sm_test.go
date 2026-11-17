package execution_test

import (
	"encoding/json"
	"errors"
	"testing"

	"pgregory.net/rapid"

	"pentagent/internal/domain"
	"pentagent/internal/execution"
	"pentagent/internal/faultinject"
)

// action_sm_test.go —— Day 14 §7：Action 状态机 + 崩溃恢复 + 幂等重发 + canonical_digest。
// 命令：go test ./internal/execution/...（纯状态机 + digest，无需 DB）。

var actStateSet = func() map[domain.ActionState]bool {
	m := map[domain.ActionState]bool{}
	for _, s := range execution.ActionAllStates {
		m[s] = true
	}
	return m
}()

// 状态集与 §5.3 一致（9 态，3 终态）。
func TestActionStatesMatchSpec(t *testing.T) {
	want := []domain.ActionState{
		domain.ActionProposed, domain.ActionPolicyDenied, domain.ActionWaitingApproval,
		domain.ActionAuthorized, domain.ActionDispatched, domain.ActionExecuted,
		domain.ActionRecorded, domain.ActionOutcomeUnknown, domain.ActionCancelled,
	}
	if len(execution.ActionAllStates) != len(want) {
		t.Fatalf("状态数 %d != §5.3 的 %d", len(execution.ActionAllStates), len(want))
	}
	for _, s := range want {
		if !actStateSet[s] {
			t.Errorf("§5.3 状态 %s 缺失", s)
		}
	}
}

// 正常链 + 审批链逐迁移。
func TestActionHappyPaths(t *testing.T) {
	cases := []struct {
		from domain.ActionState
		ev   execution.ActionEvent
		to   domain.ActionState
	}{
		{domain.ActionProposed, execution.ActionEventAuthorize, domain.ActionAuthorized},
		{domain.ActionAuthorized, execution.ActionEventDispatch, domain.ActionDispatched},
		{domain.ActionDispatched, execution.ActionEventExecute, domain.ActionExecuted},
		{domain.ActionExecuted, execution.ActionEventRecord, domain.ActionRecorded},
		{domain.ActionProposed, execution.ActionEventNeedApproval, domain.ActionWaitingApproval},
		{domain.ActionWaitingApproval, execution.ActionEventApprove, domain.ActionAuthorized},
		{domain.ActionWaitingApproval, execution.ActionEventDenyApproval, domain.ActionPolicyDenied},
		{domain.ActionProposed, execution.ActionEventDeny, domain.ActionPolicyDenied},
	}
	for _, c := range cases {
		if to, ok := execution.ActionNext(c.from, c.ev); !ok || to != c.to {
			t.Errorf("ActionNext(%s,%s)=(%s,%v)，期望 (%s,true)", c.from, c.ev, to, ok, c.to)
		}
	}
}

// §5.3 核心：DISPATCHED 后崩溃 → OUTCOME_UNKNOWN，且 DISPATCHED 不得直接成功/终态/被取消。
func TestActionDispatchedCrashSemantics(t *testing.T) {
	// 唯一允许的两条出边：execute（确定回执）或 unknown（崩溃/未确认）。
	if to, ok := execution.ActionNext(domain.ActionDispatched, execution.ActionEventUnknown); !ok || to != domain.ActionOutcomeUnknown {
		t.Fatalf("DISPATCHED--unknown--> 应为 OUTCOME_UNKNOWN，得 %s ok=%v", to, ok)
	}
	// 不得直接 record（跳过回执标成功）/ cancel（跳过对账取消）/ authorize。
	for _, ev := range []execution.ActionEvent{execution.ActionEventRecord, execution.ActionEventCancel, execution.ActionEventAuthorize} {
		if to, ok := execution.ActionNext(domain.ActionDispatched, ev); ok {
			t.Errorf("DISPATCHED 不应接受 %s（§5.3 不自动成功/取消），却 -> %s", ev, to)
		}
	}
	// OUTCOME_UNKNOWN 两条对账出边。
	if to, ok := execution.ActionNext(domain.ActionOutcomeUnknown, execution.ActionEventReconcileRecord); !ok || to != domain.ActionRecorded {
		t.Errorf("OUTCOME_UNKNOWN--reconcile_record--> 应 RECORDED，得 %s", to)
	}
	if to, ok := execution.ActionNext(domain.ActionOutcomeUnknown, execution.ActionEventReconcileCancel); !ok || to != domain.ActionCancelled {
		t.Errorf("OUTCOME_UNKNOWN--reconcile_cancel--> 应 CANCELLED，得 %s", to)
	}
}

// 用故障注入（Day 13）演示「派发后崩溃 → OUTCOME_UNKNOWN」，不自动标成功。
func TestActionCrashViaFaultInject(t *testing.T) {
	t.Cleanup(faultinject.Disable)

	// 一次「派发并等待回执」：派发点后挂 §22.3 after_network_send 注入点。
	dispatchAndSettle := func() domain.ActionState {
		st := domain.ActionAuthorized
		st, _ = execution.ActionNext(st, execution.ActionEventDispatch) // → DISPATCHED
		if e := faultinject.Hook(faultinject.AfterNetworkSend); e != nil {
			// 模拟崩溃：结果未确认 → OUTCOME_UNKNOWN（不自动失败/成功）。
			st, _ = execution.ActionNext(st, execution.ActionEventUnknown)
			return st
		}
		st, _ = execution.ActionNext(st, execution.ActionEventExecute) // 正常回执
		return st
	}

	// 未注入：正常到 EXECUTED。
	if got := dispatchAndSettle(); got != domain.ActionExecuted {
		t.Fatalf("无故障应 EXECUTED，得 %s", got)
	}
	// 注入崩溃：到 OUTCOME_UNKNOWN，绝不 EXECUTED/RECORDED。
	faultinject.Arm(faultinject.AfterNetworkSend, errors.New("crash after send"))
	if got := dispatchAndSettle(); got != domain.ActionOutcomeUnknown {
		t.Fatalf("注入崩溃应 OUTCOME_UNKNOWN（不自动成功），得 %s", got)
	}
}

// cancel 只从「尚未派发」的态进入；终态无出边。
func TestActionCancelScopeAndTerminal(t *testing.T) {
	canCancel := map[domain.ActionState]bool{
		domain.ActionProposed: true, domain.ActionWaitingApproval: true, domain.ActionAuthorized: true,
	}
	for _, s := range execution.ActionAllStates {
		to, ok := execution.ActionNext(s, execution.ActionEventCancel)
		if canCancel[s] {
			if !ok || to != domain.ActionCancelled {
				t.Errorf("%s 应可 cancel→CANCELLED，得 %s ok=%v", s, to, ok)
			}
		} else if ok {
			t.Errorf("%s 不应接受 cancel（尤其 DISPATCHED 须经对账），却 -> %s", s, to)
		}
	}
	// 终态无出边。
	for _, s := range execution.ActionAllStates {
		if !execution.ActionIsTerminal(s) {
			continue
		}
		for _, ev := range execution.ActionAllEvents {
			if to, ok := execution.ActionNext(s, ev); ok {
				t.Errorf("终态 %s 不应有出边，却 %s->%s", s, ev, to)
			}
		}
	}
}

// 幂等类驱动的「未对账能否重发」（§4.1）。
func TestCanRedispatchUnreconciled(t *testing.T) {
	cases := map[domain.IdempotencyClass]bool{
		domain.IdempotencyReadOnly:         true,
		domain.IdempotencyIdempotent:       true,
		domain.IdempotencyNonIdempotent:    false,
		domain.IdempotencyClass("UNKNOWN"): false, // 保守：未知按非幂等
		domain.IdempotencyClass(""):        false,
	}
	for class, want := range cases {
		if got := execution.CanRedispatchUnreconciled(class); got != want {
			t.Errorf("CanRedispatchUnreconciled(%q)=%v，期望 %v", class, got, want)
		}
	}
}

// canonical_digest：字段顺序无关（JCS），内容不同则摘要不同。
func TestActionProposalDigest(t *testing.T) {
	p1 := execution.ActionProposal{
		ToolName: "http", IdentityID: "idn-1", IdempotencyClass: domain.IdempotencyReadOnly,
		ActionJSON: json.RawMessage(`{"method":"GET","url":"https://t/x","headers":{"a":"1","b":"2"}}`),
	}
	p2 := execution.ActionProposal{ // 同语义、字段顺序不同
		ToolName: "http", IdentityID: "idn-1", IdempotencyClass: domain.IdempotencyReadOnly,
		ActionJSON: json.RawMessage(`{"url":"https://t/x","headers":{"b":"2","a":"1"},"method":"GET"}`),
	}
	d1, err := p1.CanonicalDigest()
	if err != nil {
		t.Fatal(err)
	}
	d2, err := p2.CanonicalDigest()
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Errorf("字段顺序不同的同语义动作应同摘要：%s vs %s", d1, d2)
	}
	// 内容不同 → 摘要不同。
	p3 := p1
	p3.ActionJSON = json.RawMessage(`{"method":"POST","url":"https://t/x","headers":{"a":"1","b":"2"}}`)
	d3, _ := p3.CanonicalDigest()
	if d3 == d1 {
		t.Errorf("不同动作不应同摘要")
	}
}

// NewProposedAction：构造 PROPOSED 态 + 填 digest；必填校验。
func TestNewProposedAction(t *testing.T) {
	p := execution.ActionProposal{
		ToolName: "http", IdentityID: "idn-1", IdempotencyClass: domain.IdempotencyNonIdempotent,
		ActionJSON: json.RawMessage(`{"method":"POST"}`),
	}
	a, err := execution.NewProposedAction("act-1", "run-1", "job-1", "att-1", p)
	if err != nil {
		t.Fatal(err)
	}
	if a.State != domain.ActionProposed || a.CanonicalDigest == "" || a.IdentityID != "idn-1" {
		t.Errorf("应 PROPOSED + 带 digest + identity，得 %+v", a)
	}
	// identity 缺失 → ErrInvalid。
	bad := p
	bad.IdentityID = ""
	if _, e := execution.NewProposedAction("act-2", "run-1", "job-1", "att-1", bad); !errors.Is(e, domain.ErrInvalid) {
		t.Errorf("缺 identity 应 ErrInvalid，得 %v", e)
	}
}

// 属性测试：随机事件序列永不进未知态、拒绝保持原态、终态吸收。
func TestActionSMProperties(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		state := domain.ActionProposed
		n := rapid.IntRange(1, 60).Draw(rt, "steps")
		for i := 0; i < n; i++ {
			ev := rapid.SampledFrom(execution.ActionAllEvents).Draw(rt, "ev")
			prev := state
			to, ok := execution.ActionNext(prev, ev)
			switch {
			case execution.ActionIsTerminal(prev):
				if ok {
					rt.Fatalf("终态 %s 接受了 %s->%s", prev, ev, to)
				}
			case ok:
				if !actStateSet[to] {
					rt.Fatalf("迁移到未知态 %s", to)
				}
				state = to
			default:
				if to != prev {
					rt.Fatalf("拒绝应保持原态，from %s ev %s 得 %s", prev, ev, to)
				}
			}
		}
	})
}
