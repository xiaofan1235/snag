package network

import "sync"

// breaker.go —— 安全不变量熔断（§4.2 v2.4 / §9.7 Kill Switch）。
//
// 对「应恒为 0」的安全计数（scope 违例、egress 绕过、Permit 重放、Fencing 失败）做自动熔断：
// 这些事件是红线纵深防御的绊线——一旦触发就说明执法层本身出了问题，累计超阈即拉 Kill Switch
// （revoke_all_target_egress + SYSTEM_DEGRADED + 诊断快照），宁可停机不可带病放行。

// SafetyEvent 是「应恒为 0」的安全绊线事件名。
type SafetyEvent string

const (
	EvScopeViolation SafetyEvent = "scope_violation" // 本不该放行却放行了的越域
	EvEgressBypass   SafetyEvent = "egress_bypass"   // 绕过代理直连
	EvPermitReplay   SafetyEvent = "permit_replay"   // 已消费 Permit 被再次消费
	EvFencingFailure SafetyEvent = "fencing_failure" // 陈旧 fencing 写入竟然成功
)

// Breaker 计数安全绊线事件，超阈即熔断。线程安全。
type Breaker struct {
	mu        sync.Mutex
	counts    map[SafetyEvent]int
	threshold int
	tripped   bool
	snapshot  []string // 触发时的诊断快照（事件序）
	onTrip    func(reason string)
}

// NewBreaker 构造。threshold<=0 视为 1（任一绊线即熔断——最严）。onTrip 在熔断时调用一次
// （应执行 revoke_all_target_egress + 置 SYSTEM_DEGRADED）。
func NewBreaker(threshold int, onTrip func(reason string)) *Breaker {
	if threshold <= 0 {
		threshold = 1
	}
	return &Breaker{counts: map[SafetyEvent]int{}, threshold: threshold, onTrip: onTrip}
}

// NewProxyBreaker 构造一个超阈即自动拉下 Proxy Kill Switch（revoke_all_target_egress，§9.7）的熔断器。
func NewProxyBreaker(threshold int, p *Proxy) *Breaker {
	return NewBreaker(threshold, func(string) { p.RevokeAllTargetEgress() })
}

// Record 记一次安全绊线事件。累计（单类）达阈值即熔断，返回是否已处于熔断态。
func (b *Breaker) Record(ev SafetyEvent) bool {
	b.mu.Lock()
	b.counts[ev]++
	b.snapshot = append(b.snapshot, string(ev))
	trip := !b.tripped && b.counts[ev] >= b.threshold
	if trip {
		b.tripped = true
	}
	onTrip := b.onTrip
	reason := string(ev)
	b.mu.Unlock()

	if trip && onTrip != nil {
		onTrip(reason) // 在锁外调用，避免回调里再触碰 breaker 造成死锁
	}
	return b.Tripped()
}

// Tripped 报告是否已熔断。
func (b *Breaker) Tripped() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tripped
}

// Counts 返回各绊线计数快照。
func (b *Breaker) Counts() map[SafetyEvent]int {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := map[SafetyEvent]int{}
	for k, v := range b.counts {
		out[k] = v
	}
	return out
}

// Snapshot 返回触发序列的诊断快照（§4.2 诊断快照）。
func (b *Breaker) Snapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.snapshot))
	copy(out, b.snapshot)
	return out
}
