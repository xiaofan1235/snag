package policy_test

import (
	"net/url"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"pentagent/internal/policy"
)

// scope_test.go —— Day 16 §7：Scope 规范化器验收（§9.3 全陷阱 + 硬属性 + Q-20 一致性）。
// 命令：go test ./internal/policy/...（属性/用例）；go test -fuzz=FuzzScope（差分模糊）。

// §9.3 逐陷阱：各变形规范化到预期 (scheme, host, port)。
func TestCanonicalizeTraps(t *testing.T) {
	cases := []struct {
		raw    string
		host   string
		scheme string
		port   int
	}{
		{"HTTPS://API.Target.TEST/", "api.target.test", "https", 443},     // 大小写
		{"https://api.target.test./", "api.target.test", "https", 443},    // 末尾点
		{"https://api.target.test:443/", "api.target.test", "https", 443}, // 显式默认端口
		{"http://api.target.test:80/", "api.target.test", "http", 80},
		{"https://evil-user@api.target.test/", "api.target.test", "https", 443}, // userinfo 欺骗
		{"https://real@api.target.test:8443/x", "api.target.test", "https", 8443},
		{"http://2130706433/", "127.0.0.1", "http", 80},        // 十进制 IPv4
		{"http://0x7f000001/", "127.0.0.1", "http", 80},        // 十六进制
		{"http://0177.0.0.1/", "127.0.0.1", "http", 80},        // 八进制
		{"http://127.1/", "127.0.0.1", "http", 80},             // 短式
		{"http://[::1]/", "::1", "http", 80},                   // IPv6
		{"http://[0:0:0:0:0:0:0:1]/", "::1", "http", 80},       // IPv6 压缩
		{"http://192.168.000.001/", "192.168.0.1", "http", 80}, // 前导零八进制段
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			got, err := policy.Canonicalize(c.raw)
			if err != nil {
				t.Fatalf("Canonicalize(%q) 出错: %v", c.raw, err)
			}
			if got.Host != c.host || got.Scheme != c.scheme || got.Port != c.port {
				t.Errorf("得 (%s,%s,%d)，期望 (%s,%s,%d)", got.Scheme, got.Host, got.Port, c.scheme, c.host, c.port)
			}
		})
	}
}

// 多重 URL 编码 path 消解（%2e%2e → ..，Clean 消解）。
func TestCanonicalizePathTraps(t *testing.T) {
	c, err := policy.Canonicalize("https://api.target.test/a/%2e%2e/b/./c")
	if err != nil {
		t.Fatal(err)
	}
	if c.Path != "/b/c" {
		t.Errorf("path 应消解为 /b/c，得 %s", c.Path)
	}
}

// 不支持/非法形态 fail-closed。
func TestCanonicalizeRejects(t *testing.T) {
	bad := []string{
		"ftp://api.target.test/",    // 非 http(s)
		"https://café.target.test/", // 非 ASCII host（无 IDN 库）
		"https:///path",             // 缺 host
		"",                          // 空
		"not a url at all ::://",    // 乱码
	}
	for _, b := range bad {
		if _, err := policy.Canonicalize(b); err == nil {
			t.Errorf("Canonicalize(%q) 应 fail-closed 报错", b)
		}
	}
}

// 硬属性①：幂等。Canonicalize(c.String()) == c。
func TestCanonicalizeIdempotent(t *testing.T) {
	inputs := []string{
		"HTTPS://API.Target.TEST:443/a/%2e%2e/b",
		"http://2130706433:8080/x",
		"http://[::1]/",
		"https://real@sub.target.test./p",
	}
	for _, in := range inputs {
		c1, err := policy.Canonicalize(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		c2, err := policy.Canonicalize(c1.String())
		if err != nil {
			t.Fatalf("再规范化 %q: %v", c1.String(), err)
		}
		if c1 != c2 {
			t.Errorf("非幂等：%q -> %+v -> %+v", in, c1, c2)
		}
	}
}

// 硬属性②（Q-20）：规范化输出经 PDP 的 net/url 再解析，host/scheme/port 三元组不变。
func TestCanonicalParserConsistency(t *testing.T) {
	inputs := []string{
		"HTTPS://API.Target.TEST/x", "http://0x7f000001/", "http://[::1]:8080/", "https://api.target.test:8443/p",
	}
	for _, in := range inputs {
		c, err := policy.Canonicalize(in)
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(c.String())
		if err != nil {
			t.Fatalf("PDP 再解析 %q 失败: %v", c.String(), err)
		}
		if strings.ToLower(u.Scheme) != c.Scheme {
			t.Errorf("scheme 分歧：%s vs %s", u.Scheme, c.Scheme)
		}
		h := strings.ToLower(strings.Trim(u.Hostname(), "[]"))
		if h != c.Host {
			t.Errorf("host 分歧：%q vs %q（输入 %q）", h, c.Host, in)
		}
	}
}

// 硬属性③：精确域名绝不匹配子域（§9.3）；泛域名匹配子域。
func TestExactVsWildcard(t *testing.T) {
	exact := policy.Target{Hosts: []policy.HostRule{{Value: "app.example.com", Match: "exact"}}}
	wild := policy.Target{Hosts: []policy.HostRule{{Value: "*.example.com", Match: "wildcard"}}}

	mustInScope := func(raw string, tg policy.Target, want bool) {
		in, _, err := policy.InScope(raw, tg)
		if err != nil {
			t.Fatalf("InScope(%q): %v", raw, err)
		}
		if in != want {
			t.Errorf("InScope(%q, %+v)=%v，期望 %v", raw, tg.Hosts, in, want)
		}
	}
	// 精确：自身 in；兄弟/子域 out。
	mustInScope("https://app.example.com/", exact, true)
	mustInScope("https://other.example.com/", exact, false)        // 兄弟域绝不匹配
	mustInScope("https://evil.app.example.com/", exact, false)     // 子域不匹配
	mustInScope("https://app.example.com.evil.com/", exact, false) // 后缀欺骗
	// 泛域名：base 及子域 in；别的域 out。
	mustInScope("https://a.example.com/", wild, true)
	mustInScope("https://deep.a.example.com/", wild, true)
	mustInScope("https://example.com/", wild, true)
	mustInScope("https://examplexcom.net/", wild, false)
	mustInScope("https://notexample.com/", wild, false)
}

// 等价变形在 scope 判定上一致（大小写/末尾点/默认端口/userinfo）。
func TestEquivalentVariantsConsistentDecision(t *testing.T) {
	tg := policy.Target{Hosts: []policy.HostRule{{Value: "api.target.test", Match: "exact"}}}
	variants := []string{
		"https://api.target.test/",
		"HTTPS://API.TARGET.TEST/",
		"https://api.target.test./",
		"https://api.target.test:443/",
		"https://someuser@api.target.test/",
	}
	for _, v := range variants {
		in, _, err := policy.InScope(v, tg)
		if err != nil || !in {
			t.Errorf("等价变形 %q 应 in-scope，得 in=%v err=%v", v, in, err)
		}
	}
}

// 属性：Canonicalize 对任意（可解析的）输入幂等、不 panic。
func TestScopeProperties(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		scheme := rapid.SampledFrom([]string{"http", "https", "HTTP", "HTTPS"}).Draw(rt, "scheme")
		host := rapid.SampledFrom([]string{"api.target.test", "API.Target.Test", "a.b.c", "127.0.0.1", "0x7f000001", "2130706433", "[::1]"}).Draw(rt, "host")
		port := rapid.SampledFrom([]string{"", ":80", ":443", ":8443"}).Draw(rt, "port")
		raw := scheme + "://" + host + port + "/x"
		c1, err := policy.Canonicalize(raw)
		if err != nil {
			return // 不可解析的组合跳过
		}
		c2, err := policy.Canonicalize(c1.String())
		if err != nil {
			rt.Fatalf("再规范化失败 %q: %v", c1.String(), err)
		}
		if c1 != c2 {
			rt.Fatalf("非幂等 %q: %+v vs %+v", raw, c1, c2)
		}
	})
}

// FuzzScope：差分模糊——任意输入不 panic；可规范化则幂等；且「越权绝不被判合规」
// （对一条精确 Scope，任何规范化 host 不等于授权 host 的输入都必须 out-of-scope）。
func FuzzScope(f *testing.F) {
	seeds := []string{
		"https://api.target.test/", "HTTPS://API.TARGET.TEST./", "https://evil@api.target.test/",
		"http://2130706433/", "http://0x7f000001/", "http://0177.0.0.1/", "http://127.1/",
		"http://[::1]/", "https://api.target.test.evil.com/", "https://sub.api.target.test/",
		"https://api.target.test:443/a/%2e%2e/b", "ftp://api.target.test/", "https://café.target.test/",
		"", "::://", "https://api.target.test@evil.test/",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	tg := policy.Target{Hosts: []policy.HostRule{{Value: "api.target.test", Match: "exact"}}}
	f.Fuzz(func(t *testing.T, raw string) {
		c, err := policy.Canonicalize(raw) // 不得 panic
		if err != nil {
			return
		}
		// 幂等。
		c2, err := policy.Canonicalize(c.String())
		if err != nil || c != c2 {
			t.Fatalf("非幂等或再规范化失败：%q -> %+v -> (%+v,%v)", raw, c, c2, err)
		}
		// 越权绝不合规：规范化 host != 授权 host ⇒ 必 out-of-scope。
		inScope := policy.HostInTarget(c, tg)
		if inScope && c.Host != "api.target.test" {
			t.Fatalf("越权被判合规：raw=%q 规范化 host=%q 却 in-scope", raw, c.Host)
		}
	})
}
