package faultinject_test

import (
	"errors"
	"testing"

	"pentagent/internal/faultinject"
)

// faultinject_test.go —— Day 13 §7：故障注入 Hook 能在测试里触发模拟崩溃。

func TestHookDisabledByDefault(t *testing.T) {
	faultinject.Disable()
	if faultinject.IsEnabled() {
		t.Fatal("默认应禁用")
	}
	if e := faultinject.Hook(faultinject.BeforeNetworkSend); e != nil {
		t.Errorf("未开启时 Hook 应返回 nil，得 %v", e)
	}
}

func TestArmAndHook(t *testing.T) {
	t.Cleanup(faultinject.Disable)
	boom := errors.New("injected crash")
	disarm := faultinject.Arm(faultinject.BeforeNetworkSend, boom)

	// 武装的点每次都触发。
	for i := 0; i < 3; i++ {
		if e := faultinject.Hook(faultinject.BeforeNetworkSend); !errors.Is(e, boom) {
			t.Fatalf("第 %d 次应触发注入故障，得 %v", i, e)
		}
	}
	// 其它点不受影响。
	if e := faultinject.Hook(faultinject.AfterNetworkSend); e != nil {
		t.Errorf("未武装的点应 nil，得 %v", e)
	}
	// 解除后不再触发。
	disarm()
	if e := faultinject.Hook(faultinject.BeforeNetworkSend); e != nil {
		t.Errorf("解除后应 nil，得 %v", e)
	}
}

func TestArmOnce(t *testing.T) {
	t.Cleanup(faultinject.Disable)
	boom := errors.New("once")
	faultinject.ArmOnce(faultinject.AfterExitGatePass, boom, 2)
	if e := faultinject.Hook(faultinject.AfterExitGatePass); !errors.Is(e, boom) {
		t.Fatalf("第 1 次应触发，得 %v", e)
	}
	if e := faultinject.Hook(faultinject.AfterExitGatePass); !errors.Is(e, boom) {
		t.Fatalf("第 2 次应触发，得 %v", e)
	}
	if e := faultinject.Hook(faultinject.AfterExitGatePass); e != nil {
		t.Errorf("第 3 次（超出 2 次）应 nil，得 %v", e)
	}
}

func TestDisableClears(t *testing.T) {
	faultinject.Arm(faultinject.BeforePermitConsume, errors.New("x"))
	faultinject.Disable()
	if e := faultinject.Hook(faultinject.BeforePermitConsume); e != nil {
		t.Errorf("Disable 应清空武装点，得 %v", e)
	}
}
