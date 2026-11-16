package control_test

import (
	"testing"

	"pgregory.net/rapid"

	"pentagent/internal/control"
	"pentagent/internal/domain"
)

// job_sm_test.go —— Day 08 §7：Job 纯状态机验收（无需 DB，默认模式即跑）。
//   - 状态集与 §5.2 一致（13 态，3 终态）；
//   - 终态无出边；fail 能从所有非终态进入；cancel 从所有非终态进入**但排除 RECONCILING**；
//   - RECONCILING 是枢纽而非陷阱：有 reconcile_retry / reconcile_unresolved / fail 三条出边；
//   - 属性测试（rapid）：随机事件序列永不进入未知状态、拒绝时保持原态、终态吸收。

var jobStateSet = func() map[domain.JobState]bool {
	m := map[domain.JobState]bool{}
	for _, s := range control.JobAllStates {
		m[s] = true
	}
	return m
}()

// 生成的状态集必须与 §5.2 一致（不多不少）。
func TestJobStatesMatchSpec(t *testing.T) {
	want := []domain.JobState{
		domain.JobPending, domain.JobBlocked, domain.JobReady, domain.JobClaimed,
		domain.JobRunning, domain.JobReconciling, domain.JobWaitingApproval,
		domain.JobSubmitted, domain.JobVerifying, domain.JobDone, domain.JobRetrying,
		domain.JobCancelled, domain.JobFailed,
	}
	if len(control.JobAllStates) != len(want) {
		t.Fatalf("状态数 %d != §5.2 的 %d", len(control.JobAllStates), len(want))
	}
	for _, s := range want {
		if !jobStateSet[s] {
			t.Errorf("§5.2 状态 %s 缺失", s)
		}
	}
}

// fail 必须能从每个非终态进入 FAILED（含 RECONCILING，容灾）。
func TestJobFailFromEveryNonTerminal(t *testing.T) {
	for _, s := range control.JobAllStates {
		if control.JobIsTerminal(s) {
			if _, ok := control.JobNext(s, control.JobEventFail); ok {
				t.Errorf("终态 %s 不应接受 fail", s)
			}
			continue
		}
		if to, ok := control.JobNext(s, control.JobEventFail); !ok || to != domain.JobFailed {
			t.Errorf("fail 应能从非终态 %s 进 FAILED，得 %s ok=%v", s, to, ok)
		}
	}
}

// cancel 从所有非终态进 CANCELLED，但**有意排除 RECONCILING**（对账未完成不得被取消抢先终态，§5.2）。
func TestJobCancelExcludesReconciling(t *testing.T) {
	for _, s := range control.JobAllStates {
		to, ok := control.JobNext(s, control.JobEventCancel)
		switch {
		case s == domain.JobReconciling:
			if ok {
				t.Errorf("RECONCILING 不应接受 cancel（应先对账），却 -> %s", to)
			}
		case control.JobIsTerminal(s):
			if ok {
				t.Errorf("终态 %s 不应接受 cancel", s)
			}
		default:
			if !ok || to != domain.JobCancelled {
				t.Errorf("cancel 应能从非终态 %s 进 CANCELLED，得 %s ok=%v", s, to, ok)
			}
		}
	}
}

// 终态无任何出边。
func TestJobTerminalNoOutgoing(t *testing.T) {
	for _, s := range control.JobAllStates {
		if !control.JobIsTerminal(s) {
			continue
		}
		for _, ev := range control.JobAllEvents {
			if to, ok := control.JobNext(s, ev); ok {
				t.Errorf("终态 %s 不应有出边，却 %s -> %s", s, ev, to)
			}
		}
	}
}

// RECONCILING 是枢纽：进得来（RUNNING），且三条出边保证不永久卡死（§5.2 / §31 C3）。
func TestJobReconcilingHubNotTrap(t *testing.T) {
	// 进入：仅 RUNNING。
	if to, ok := control.JobNext(domain.JobRunning, control.JobEventReconcile); !ok || to != domain.JobReconciling {
		t.Fatalf("reconcile 应从 RUNNING 进 RECONCILING，得 %s ok=%v", to, ok)
	}
	// 出边三条。
	exits := map[control.JobEvent]domain.JobState{
		control.JobEventReconcileRetry:      domain.JobRetrying,
		control.JobEventReconcileUnresolved: domain.JobCancelled,
		control.JobEventFail:                domain.JobFailed,
	}
	for ev, want := range exits {
		if to, ok := control.JobNext(domain.JobReconciling, ev); !ok || to != want {
			t.Errorf("RECONCILING --%s--> 应为 %s，得 %s ok=%v", ev, want, to, ok)
		}
	}
	// 冻结新 Claim：RECONCILING 不接受 claim。
	if to, ok := control.JobNext(domain.JobReconciling, control.JobEventClaim); ok {
		t.Errorf("RECONCILING 应冻结 claim，却 -> %s", to)
	}
	// 至少有一条非 fail 出边（真枢纽，不是靠容灾 fail 兜底）。
	if _, ok := control.JobNext(domain.JobReconciling, control.JobEventReconcileRetry); !ok {
		t.Error("RECONCILING 缺正常出边 reconcile_retry")
	}
}

// 逐条核对关键迁移（§5.2 语义锚点）。
func TestJobKeyTransitions(t *testing.T) {
	cases := []struct {
		from domain.JobState
		ev   control.JobEvent
		to   domain.JobState
		ok   bool
	}{
		{domain.JobPending, control.JobEventReady, domain.JobReady, true},
		{domain.JobReady, control.JobEventClaim, domain.JobClaimed, true},
		{domain.JobClaimed, control.JobEventRun, domain.JobRunning, true},
		{domain.JobRunning, control.JobEventSubmit, domain.JobSubmitted, true},
		{domain.JobSubmitted, control.JobEventVerify, domain.JobVerifying, true},
		{domain.JobVerifying, control.JobEventDone, domain.JobDone, true},
		{domain.JobVerifying, control.JobEventVerifyRetry, domain.JobRetrying, true},
		{domain.JobRetrying, control.JobEventRetry, domain.JobReady, true},
		{domain.JobRunning, control.JobEventNeedApproval, domain.JobWaitingApproval, true},
		{domain.JobWaitingApproval, control.JobEventApprove, domain.JobRunning, true},
		{domain.JobRunning, control.JobEventReconcile, domain.JobReconciling, true},
		{domain.JobReconciling, control.JobEventReconcileRetry, domain.JobRetrying, true},
		{domain.JobReconciling, control.JobEventReconcileUnresolved, domain.JobCancelled, true},
		// 非法：reconcile 只从 RUNNING（PENDING 不行）；RECONCILING 拒 cancel；终态无出边。
		{domain.JobPending, control.JobEventReconcile, "", false},
		{domain.JobReconciling, control.JobEventCancel, "", false},
		{domain.JobDone, control.JobEventRetry, "", false},
	}
	for _, c := range cases {
		to, ok := control.JobNext(c.from, c.ev)
		wantTo := c.to
		if !c.ok {
			wantTo = c.from // 拒绝时保持原态
		}
		if ok != c.ok || to != wantTo {
			t.Errorf("JobNext(%s, %s) = (%s, %v); 期望 (%s, %v)", c.from, c.ev, to, ok, wantTo, c.ok)
		}
	}
}

// 属性测试：随机事件序列下，永不 panic、永不进未知态、拒绝保持原态、终态吸收。
func TestJobSMProperties(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		state := domain.JobPending
		n := rapid.IntRange(1, 80).Draw(rt, "steps")
		for i := 0; i < n; i++ {
			ev := rapid.SampledFrom(control.JobAllEvents).Draw(rt, "ev")
			prev := state
			to, ok := control.JobNext(prev, ev)
			switch {
			case control.JobIsTerminal(prev):
				if ok {
					rt.Fatalf("终态 %s 接受了事件 %s -> %s", prev, ev, to)
				}
			case ok:
				if !jobStateSet[to] {
					rt.Fatalf("迁移到未知状态 %s（from %s, ev %s）", to, prev, ev)
				}
				state = to
			default:
				if to != prev {
					rt.Fatalf("拒绝时应保持原态，from %s ev %s 得 %s", prev, ev, to)
				}
			}
		}
		if !jobStateSet[state] {
			rt.Fatalf("终态非法: %s", state)
		}
	})
}
