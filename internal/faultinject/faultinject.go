// Package faultinject 是故障注入 Hook 点注册表（§22.3）。M0/M1 先定义接口与 Hook 点清单，
// 各模块实现时在关键路径调用 Hook(name)；测试可武装某个点让其「在这里崩溃」，验证崩溃恢复。
//
// 生产默认全局禁用（Enabled=false）→ Hook 是一次布尔判断后立即返回 nil，零注入、近零开销。
// 仅测试显式 Enable() 后，被 Arm 的点才会返回其故障错误。线程安全。
package faultinject

import (
	"sync"
)

// §22.3 Hook 点清单（随各模块落地逐步在真实路径挂上；此处集中登记名字）。
const (
	BeforePermitConsume      = "before_permit_consume"
	AfterPermitConsume       = "after_permit_consume"
	BeforeNetworkSend        = "before_network_send"
	AfterNetworkSend         = "after_network_send"
	AfterModelResponseCommit = "after_model_response_commit"
	AfterExitGatePass        = "after_exit_gate_pass"
	BeforeRunStateApply      = "before_run_state_apply"
	AfterRunStateApply       = "after_run_state_apply"
)

var (
	mu      sync.RWMutex
	enabled bool
	armed   = map[string]*fault{}
)

type fault struct {
	err       error
	remaining int // <0 = 永久；>0 = 还剩几次；0 = 用尽
}

// Enable 打开故障注入（仅测试用）。
func Enable() {
	mu.Lock()
	enabled = true
	mu.Unlock()
}

// Disable 关闭故障注入并清空所有武装点（测试清理用）。
func Disable() {
	mu.Lock()
	enabled = false
	armed = map[string]*fault{}
	mu.Unlock()
}

// IsEnabled 报告是否已开启。
func IsEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return enabled
}

// Hook 在注入点被调用：若该点已武装则返回其故障错误（并消耗一次），否则返回 nil。
// 生产（未 Enable）下立即返回 nil。
func Hook(name string) error {
	mu.RLock()
	if !enabled {
		mu.RUnlock()
		return nil
	}
	f, ok := armed[name]
	if !ok || f.remaining == 0 {
		mu.RUnlock()
		return nil
	}
	mu.RUnlock()

	// 需改计数，升级为写锁复核。
	mu.Lock()
	defer mu.Unlock()
	f, ok = armed[name]
	if !enabled || !ok || f.remaining == 0 {
		return nil
	}
	if f.remaining > 0 {
		f.remaining--
	}
	return f.err
}

// Arm 武装一个注入点：之后对该点的每次 Hook 都返回 err（永久，直到 Disarm/Disable）。
// 返回解除函数。会自动 Enable。
func Arm(name string, err error) (disarm func()) {
	mu.Lock()
	enabled = true
	armed[name] = &fault{err: err, remaining: -1}
	mu.Unlock()
	return func() { Disarm(name) }
}

// ArmOnce 武装一个注入点，只触发 times 次（times<=0 视为 1）。返回解除函数。会自动 Enable。
func ArmOnce(name string, err error, times int) (disarm func()) {
	if times <= 0 {
		times = 1
	}
	mu.Lock()
	enabled = true
	armed[name] = &fault{err: err, remaining: times}
	mu.Unlock()
	return func() { Disarm(name) }
}

// Disarm 解除某个注入点。
func Disarm(name string) {
	mu.Lock()
	delete(armed, name)
	mu.Unlock()
}
