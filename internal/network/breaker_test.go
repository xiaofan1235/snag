package network

import (
	"context"
	"testing"
	"time"
)

// 阈值到点即熔断，onTrip 恰调一次（幂等），计数与诊断快照正确。
func TestBreakerTripsAtThresholdOnce(t *testing.T) {
	var trips int
	var reason string
	br := NewBreaker(2, func(r string) { trips++; reason = r })

	if br.Record(EvFencingFailure) {
		t.Fatal("阈值 2，第 1 次不应熔断")
	}
	if br.Tripped() {
		t.Fatal("第 1 次后不应处于熔断态")
	}
	if !br.Record(EvFencingFailure) {
		t.Fatal("第 2 次应触发熔断")
	}
	if trips != 1 {
		t.Fatalf("onTrip 调用次数=%d，应为 1", trips)
	}
	if reason != string(EvFencingFailure) {
		t.Fatalf("熔断原因=%q", reason)
	}
	// 熔断后继续记录不应二次触发回调。
	br.Record(EvFencingFailure)
	if trips != 1 {
		t.Fatalf("onTrip 被重复调用：%d", trips)
	}
	if c := br.Counts()[EvFencingFailure]; c != 3 {
		t.Fatalf("计数=%d，应为 3", c)
	}
	if n := len(br.Snapshot()); n != 3 {
		t.Fatalf("诊断快照长度=%d，应为 3", n)
	}
}

// threshold<=0 视为最严：任一绊线即熔断；onTrip 为 nil 不 panic。
func TestBreakerDefaultThresholdStrict(t *testing.T) {
	br := NewBreaker(0, nil)
	if !br.Record(EvEgressBypass) {
		t.Fatal("threshold<=0 应在首个绊线事件即熔断")
	}
}

// 计数按类独立：一类未到阈不应因另一类的累计而误熔断。
func TestBreakerCountsPerClass(t *testing.T) {
	br := NewBreaker(2, nil)
	if br.Record(EvScopeViolation) {
		t.Fatal("scope 第 1 次不应熔断")
	}
	if br.Record(EvPermitReplay) {
		t.Fatal("permit 第 1 次不应熔断（与 scope 分开计数）")
	}
	if br.Tripped() {
		t.Fatal("两类各 1 次、阈值 2，不应熔断")
	}
}

// DoD：注入上升的 scope 违例计数 → 自动拉下 Proxy Kill Switch → 全部出网被撤销（system_degraded）。
func TestBreakerRevokesProxyEgress(t *testing.T) {
	cache := NewEpochCache()
	cache.Rebuild()
	p := NewProxy(nil, nil, nil, cache)
	if p.Degraded() {
		t.Fatal("尚无违例时 Proxy 不应降级")
	}

	br := NewProxyBreaker(3, p)
	for i := 0; i < 2; i++ {
		if br.Record(EvScopeViolation) {
			t.Fatalf("i=%d 过早熔断", i)
		}
	}
	if !br.Record(EvScopeViolation) { // 第 3 次达阈
		t.Fatal("达阈值 3 应熔断")
	}
	if !p.Degraded() {
		t.Fatal("熔断后 Proxy egress 未被撤销")
	}

	res := p.CheckRequest(context.Background(), "cap", "http://target/", "GET", true, time.Now())
	if res.Allowed || res.Code != "system_degraded" {
		t.Fatalf("Kill Switch 后应 system_degraded，实得 allowed=%v code=%s", res.Allowed, res.Code)
	}
}
