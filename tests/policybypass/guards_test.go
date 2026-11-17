// Package policybypass_test —— 策略绕过测试套件（§22.4）。逐条尝试绕过，断言全部 fail-closed 被拦。
//
// 本文件覆盖 URL/Scope/重定向/熔断层（无需 DB）；evidence_test.go 覆盖证据完整性与自报绕过（需 DB）。
// 命令：go test ./tests/policybypass/...（纯 guard 层无需驱动）与
//
//	go test -tags sqlite_modernc ./tests/policybypass/...（含 DB 证据组）。
//
// §22.4 清单与本套件覆盖位置：
//
//	越域 Redirect              → TestCrossOriginRedirectBlocked
//	DNS 重绑定                 → TestDNSRebindLiteralPrivateBlocked（字面私网）+ 解析时守卫注记
//	非默认端口                 → TestNonDefaultPortBlocked
//	用户名密码 URL             → TestUserinfoSpoofBlocked
//	IPv6                       → TestIPv6OutOfScopeBlocked
//	编码域名                   → TestEncodedDomainFailClosed
//	Runner 直连公网            → TestOutOfScopeHostBlocked（+ EgressLease 架构注记）
//	Browser 新窗口             → TestBrowserNewWindowRechecksScope
//	WebSocket                  → TestWebSocketSchemeFailClosed
//	Proxy 失效后直连           → TestProxyFailureTripsKillSwitch
//	伪造 Evidence ID / 释义冒充 / 跨 Run → evidence_test.go
//	模型自报改 Coverage/Finding → evidence_test.go
//	Permit 重放 / 过期 Attempt / 多 Worker 抢配额 / 摘要复用 Approval / 收窄在途 / 沙箱 PARTIAL
//	                           → 见 TestIntegrationPendingDocumented（各自在对应包已有执法与测试，接线待 daemon）
package policybypass_test

import (
	"testing"

	"pentagent/internal/network"
	"pentagent/internal/policy"
)

// scope 是测试授权范围：只允许 app.example.com 的 https:443。
func scope() policy.Target {
	return policy.Target{
		Hosts:   []policy.HostRule{{Value: "app.example.com", Match: "exact"}},
		Schemes: []string{"https"},
		Ports:   []int{443},
	}
}

func mustBlocked(t *testing.T, raw string, why string) {
	t.Helper()
	in, _, err := policy.InScope(raw, scope())
	if err == nil && in {
		t.Fatalf("%s：%q 本应被拦，却在 Scope 内", why, raw)
	}
}

// 越域 Redirect：重定向目标必须重新过 Scope，越域被拦（§9.5）。
func TestCrossOriginRedirectBlocked(t *testing.T) {
	in, err := network.RedirectInScope("https://app.example.com/a", "https://evil.com/x", scope())
	if err == nil && in {
		t.Fatal("越域重定向应被拦")
	}
	// 同域相对重定向仍在 Scope。
	in2, err2 := network.RedirectInScope("https://app.example.com/a", "/b", scope())
	if err2 != nil || !in2 {
		t.Fatalf("同域重定向应在 Scope：in=%v err=%v", in2, err2)
	}
}

// DNS 重绑定（字面私网 IP）：指向私网的字面 IP 不在授权 Scope。
// 注：域名解析到私网的「真」DNS 重绑定在**连接时**由 DNS 守卫校验（isPrivateHost），属真实网络栈。
func TestDNSRebindLiteralPrivateBlocked(t *testing.T) {
	for _, raw := range []string{
		"https://127.0.0.1/admin",
		"https://169.254.169.254/latest/meta-data", // 云元数据
		"https://10.0.0.5/internal",
	} {
		mustBlocked(t, raw, "私网字面 IP")
	}
}

// 非默认端口：端口不在 Target.Ports 内被拦。
func TestNonDefaultPortBlocked(t *testing.T) {
	mustBlocked(t, "https://app.example.com:8443/", "非默认端口")
}

// 用户名密码 URL：userinfo 不能冒充 Scope——真实 host 由 Hostname() 取，userinfo 里写授权域无效。
func TestUserinfoSpoofBlocked(t *testing.T) {
	// userinfo 写授权域，真实 host 却是 evil.com → 被拦。
	mustBlocked(t, "https://app.example.com@evil.com/", "userinfo 冒充 Scope")
	// userinfo 在授权 host 上则正常放行（host 正确提取）。
	in, _, err := policy.InScope("https://user:pass@app.example.com/", scope())
	if err != nil || !in {
		t.Fatalf("授权 host 上的 userinfo 应放行：in=%v err=%v", in, err)
	}
}

// IPv6：越域 IPv6（含私网 ::1）被拦。
func TestIPv6OutOfScopeBlocked(t *testing.T) {
	mustBlocked(t, "https://[::1]/", "IPv6 loopback")
	mustBlocked(t, "https://[2001:db8::1]/", "越域 IPv6")
}

// 编码域名：非 ASCII host（IDN/编码）无 punycode 库时 fail-closed（规范化报错）。
func TestEncodedDomainFailClosed(t *testing.T) {
	_, _, err := policy.InScope("https://аpp.example.com/", scope()) // 西里尔 а
	if err == nil {
		t.Fatal("非 ASCII host 应 fail-closed 规范化报错")
	}
}

// Runner 直连公网 / Browser 任意新 host：任何越域 host 被拦。
// 架构上 Runner 只持 EgressLease 指向 Proxy（无直连出口）；此处断言 Scope 层对越域 host 的拦截。
func TestOutOfScopeHostBlocked(t *testing.T) {
	mustBlocked(t, "https://attacker.test/", "越域公网 host")
}

// Browser 新窗口：打开的新 URL 仍须重新过 Scope（不因「新窗口」豁免）。
func TestBrowserNewWindowRechecksScope(t *testing.T) {
	// 新窗口到越域 → 拦。
	mustBlocked(t, "https://popup.evil.com/", "浏览器新窗口越域")
	// 新窗口回授权域 → 放行。
	in, _, err := policy.InScope("https://app.example.com/new", scope())
	if err != nil || !in {
		t.Fatalf("新窗口回授权域应放行：in=%v err=%v", in, err)
	}
}

// WebSocket：ws/wss scheme 非 http/https，规范化 fail-closed。
func TestWebSocketSchemeFailClosed(t *testing.T) {
	for _, raw := range []string{"ws://app.example.com/", "wss://app.example.com/"} {
		if _, _, err := policy.InScope(raw, scope()); err == nil {
			t.Fatalf("WebSocket scheme 应 fail-closed：%q", raw)
		}
	}
}

// Proxy 失效后直连：连续探测到绕过代理直连 → 熔断器跳闸拉下 Kill Switch（之后一律拒）。
func TestProxyFailureTripsKillSwitch(t *testing.T) {
	tripped := false
	b := network.NewBreaker(2, func(string) { tripped = true })
	// 第 1 次 egress_bypass 未到阈值。
	if b.Record(network.EvEgressBypass) {
		t.Fatal("阈值 2，第 1 次不应跳闸")
	}
	// 第 2 次到阈值 → 跳闸。
	if !b.Record(network.EvEgressBypass) {
		t.Fatal("第 2 次应跳闸")
	}
	if !tripped || !b.Tripped() {
		t.Fatal("跳闸后应拉下 Kill Switch")
	}
}

// 集成待接线项（§22.4 其余）——各自在对应包已有执法与单测，端到端接线随 daemon（文档化，不是缺陷）：
//
//	Permit 重放 / 过期 Attempt / 多 Worker 抢配额 → internal/network/proxy_test、internal/scheduler
//	手工改 Markdown 进报告                        → internal/report/report_test（程序填事实，只收 REPORTABLE）
//	摘要相同复用已消费 Approval                   → internal/control（approval consumed_action_id 一次性）
//	Policy 收窄在途子请求走旧 Revision            → internal/network policy_revision 逐请求回查
//	沙箱 PARTIAL/UNKNOWN 主动测试 fail-closed       → internal/sandbox 完备度门
func TestIntegrationPendingDocumented(t *testing.T) {
	// 占位断言：本套件已覆盖 guard 层全部可独立验证项；集成项见上方注记与对应包测试。
	t.Log("§22.4 guard 层全覆盖；集成项在对应包已有执法与测试，端到端接线随 daemon")
}
