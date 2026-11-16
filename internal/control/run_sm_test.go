package control_test

import (
	"testing"

	"pgregory.net/rapid"

	"pentagent/internal/control"
	"pentagent/internal/domain"
)

// run_sm_test.go —— Day 07 §7：纯状态机验收（无需 DB，默认模式即跑）。
//   - 任意 (状态,事件) 有确定结果（迁移或拒绝），无 panic、无卡死（§5.1 完整迁移表）；
//   - Stop/Fail 能从所有非终态进入；
//   - 生成的状态集与 §5.1 一致；
//   - 属性测试（rapid）：随机事件序列永不进入未知状态、拒绝时保持原态、终态无出边。

var stateSet = func() map[domain.RunState]bool {
	m := map[domain.RunState]bool{}
	for _, s := range control.AllStates {
		m[s] = true
	}
	return m
}()

// 生成的状态集必须与 §5.1 一致（不多不少）。
func TestStatesMatchSpec(t *testing.T) {
	want := []domain.RunState{
		domain.RunDraft, domain.RunPreflight, domain.RunQueued, domain.RunRunning,
		domain.RunPausing, domain.RunPaused, domain.RunResuming, domain.RunBlocked,
		domain.RunExiting, domain.RunStopping, domain.RunCompleted, domain.RunPartial,
		domain.RunStopped, domain.RunFailed,
	}
	if len(control.AllStates) != len(want) {
		t.Fatalf("状态数 %d != §5.1 的 %d", len(control.AllStates), len(want))
	}
	for _, s := range want {
		if !stateSet[s] {
			t.Errorf("§5.1 状态 %s 缺失", s)
		}
	}
}

// Stop/Fail 必须能从每个非终态进入（§5.1）。
func TestStopFailFromEveryNonTerminal(t *testing.T) {
	for _, s := range control.AllStates {
		if control.IsTerminal(s) {
			// 终态不接受 stop/fail。
			if _, ok := control.Next(s, control.EventStop); ok {
				t.Errorf("终态 %s 不应接受 stop", s)
			}
			continue
		}
		if to, ok := control.Next(s, control.EventStop); !ok || to != domain.RunStopping {
			t.Errorf("stop 应能从非终态 %s 进入 STOPPING，得到 %s ok=%v", s, to, ok)
		}
		if to, ok := control.Next(s, control.EventFail); !ok || to != domain.RunFailed {
			t.Errorf("fail 应能从非终态 %s 进入 FAILED，得到 %s ok=%v", s, to, ok)
		}
	}
}

// 终态无任何出边。
func TestTerminalNoOutgoing(t *testing.T) {
	for _, s := range control.AllStates {
		if !control.IsTerminal(s) {
			continue
		}
		for _, ev := range control.AllEvents {
			if to, ok := control.Next(s, ev); ok {
				t.Errorf("终态 %s 不应有出边，却 %s -> %s", s, ev, to)
			}
		}
	}
}

// 属性测试：随机事件序列下，状态机永不 panic、永不进入未知状态、拒绝时保持原态、终态吸收。
func TestRunSMProperties(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		state := domain.RunDraft
		n := rapid.IntRange(1, 80).Draw(rt, "steps")
		for i := 0; i < n; i++ {
			ev := rapid.SampledFrom(control.AllEvents).Draw(rt, "ev")
			prev := state
			to, ok := control.Next(prev, ev)
			switch {
			case control.IsTerminal(prev):
				if ok {
					rt.Fatalf("终态 %s 接受了事件 %s -> %s", prev, ev, to)
				}
			case ok:
				if !stateSet[to] {
					rt.Fatalf("迁移到未知状态 %s（from %s, ev %s）", to, prev, ev)
				}
				state = to
			default: // 拒绝
				if to != prev {
					rt.Fatalf("拒绝时应保持原态，from %s ev %s 得到 %s", prev, ev, to)
				}
			}
		}
		if !stateSet[state] {
			rt.Fatalf("终态非法: %s", state)
		}
	})
}

// 逐条核对几个关键迁移（§5.1 语义锚点）。
func TestKeyTransitions(t *testing.T) {
	cases := []struct {
		from domain.RunState
		ev   control.Event
		to   domain.RunState
		ok   bool
	}{
		{domain.RunDraft, control.EventPreflight, domain.RunPreflight, true},
		{domain.RunPreflight, control.EventStart, domain.RunRunning, true},
		{domain.RunRunning, control.EventPause, domain.RunPausing, true},
		{domain.RunPausing, control.EventPauseDrained, domain.RunPaused, true},
		{domain.RunPaused, control.EventResume, domain.RunResuming, true},
		{domain.RunResuming, control.EventResumeReady, domain.RunRunning, true},
		{domain.RunBlocked, control.EventUnblock, domain.RunResuming, true}, // 恢复经 RESUMING
		{domain.RunExiting, control.EventComplete, domain.RunCompleted, true},
		{domain.RunExiting, control.EventPartial, domain.RunPartial, true},
		{domain.RunStopping, control.EventStopDrained, domain.RunStopped, true},
		// 非法：DRAFT 收到 pause、RUNNING 收到 preflight。
		{domain.RunDraft, control.EventPause, domain.RunDraft, false},
		{domain.RunRunning, control.EventPreflight, domain.RunRunning, false},
	}
	for _, c := range cases {
		to, ok := control.Next(c.from, c.ev)
		if ok != c.ok || to != c.to {
			t.Errorf("Next(%s, %s) = (%s, %v); 期望 (%s, %v)", c.from, c.ev, to, ok, c.to, c.ok)
		}
	}
}
