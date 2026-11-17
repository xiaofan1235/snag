package policy_test

import (
	"testing"
	"time"

	"pentagent/internal/policy"
)

// policy_engine_test.go —— Day 15 §7：PDP 判定验收（§9.2/§9.6）。纯确定性，无需 DB。
// 命令：go test ./internal/policy/...

func basePolicy() policy.Policy {
	return policy.Policy{
		Version: 1,
		Target: policy.Target{
			Hosts:           []policy.HostRule{{Value: "api.target.test", Match: "exact"}},
			AllowSubdomains: false,
		},
		Authorization: policy.AuthWindow{
			ValidFrom:  "2026-01-01T00:00:00Z",
			ValidUntil: "2026-12-31T23:59:59Z",
		},
		Methods:    policy.Methods{Default: []string{"GET", "HEAD", "POST"}, UnverifiedEndpoints: []string{"GET", "HEAD"}},
		Network:    policy.Network{RequireProxy: true, MaxRequests: 1000},
		Identities: policy.Identities{Allowed: []string{"idn-a"}, AllowedTenants: []string{"tenant-1"}},
		Limits:     policy.Limits{IdorOtherRecords: 5},
		Actions: map[string]string{
			"write_mode":        "require_approval",
			"financial_effects": "deny",
			"persistence":       "deny",
		},
	}
}

// 合规只读（已核验端点、GET、在 scope、身份合法）。
func baseInput() policy.PolicyInput {
	return policy.PolicyInput{
		Now:              time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		CanonicalURL:     "https://api.target.test/orders/1",
		Method:           "GET",
		IdentityID:       "idn-a",
		Tenant:           "tenant-1",
		EndpointVerified: true,
		ObjectOwnership:  "self",
	}
}

func TestCompliantReadAllowed(t *testing.T) {
	e := policy.NewEngine()
	d := e.Decide(basePolicy(), baseInput())
	if d.Effect != policy.EffectAllow {
		t.Fatalf("合规只读应 ALLOW，得 %s (%s: %s)", d.Effect, d.Code, d.Reason)
	}
}

func TestDenyCases(t *testing.T) {
	e := policy.NewEngine()
	cases := []struct {
		name    string
		mutate  func(*policy.Policy, *policy.PolicyInput)
		code    string
		wantEff policy.Effect
	}{
		{"越权Method(DELETE)", func(_ *policy.Policy, in *policy.PolicyInput) { in.Method = "DELETE" }, "method_denied", policy.EffectDeny},
		{"未核验端点非安全方法", func(_ *policy.Policy, in *policy.PolicyInput) { in.EndpointVerified = false; in.Method = "POST" }, "method_unverified_endpoint", policy.EffectDeny},
		{"越权身份", func(_ *policy.Policy, in *policy.PolicyInput) { in.IdentityID = "idn-x" }, "identity_not_allowed", policy.EffectDeny},
		{"空身份", func(_ *policy.Policy, in *policy.PolicyInput) { in.IdentityID = "" }, "identity_required", policy.EffectDeny},
		{"越权租户", func(_ *policy.Policy, in *policy.PolicyInput) { in.Tenant = "tenant-evil" }, "tenant_not_allowed", policy.EffectDeny},
		{"出界host", func(_ *policy.Policy, in *policy.PolicyInput) { in.CanonicalURL = "https://evil.test/x" }, "scope_violation", policy.EffectDeny},
		{"授权过期", func(_ *policy.Policy, in *policy.PolicyInput) { in.Now = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC) }, "auth_window_expired", policy.EffectDeny},
		{"授权未生效", func(_ *policy.Policy, in *policy.PolicyInput) { in.Now = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }, "auth_not_yet_valid", policy.EffectDeny},
		{"IDOR超额", func(_ *policy.Policy, in *policy.PolicyInput) { in.ObjectOwnership = "other"; in.IdorRecordsSoFar = 5 }, "idor_quota_exhausted", policy.EffectDeny},
		{"配额耗尽", func(_ *policy.Policy, in *policy.PolicyInput) { in.RequestsSoFar = 1000 }, "quota_exhausted", policy.EffectDeny},
		{"元操作(§6.3)", func(_ *policy.Policy, in *policy.PolicyInput) { in.IsMetaOperation = true }, "redline_meta_operation", policy.EffectDeny},
		{"红线写(deny)", func(p *policy.Policy, in *policy.PolicyInput) {
			in.ActionClasses = []string{"persistence"}
		}, "redline_persistence", policy.EffectDeny},
		{"红线未配置类(默认deny)", func(p *policy.Policy, in *policy.PolicyInput) {
			in.ActionClasses = []string{"real_user_data"} // 策略未列 → 默认 deny
		}, "redline_real_user_data", policy.EffectDeny},
		{"写动作需审批", func(_ *policy.Policy, in *policy.PolicyInput) {
			in.Method = "POST"
			in.ActionClasses = []string{"write_mode"}
		}, "approval_write_mode", policy.EffectRequireApproval},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, in := basePolicy(), baseInput()
			c.mutate(&p, &in)
			d := e.Decide(p, in)
			if d.Effect != c.wantEff {
				t.Fatalf("期望 %s，得 %s (%s: %s)", c.wantEff, d.Effect, d.Code, d.Reason)
			}
			if d.Code != c.code {
				t.Errorf("期望 code=%s，得 %s", c.code, d.Code)
			}
		})
	}
}

// IDOR 在额度内放行。
func TestIdorWithinLimit(t *testing.T) {
	e := policy.NewEngine()
	p, in := basePolicy(), baseInput()
	in.ObjectOwnership = "other"
	in.IdorRecordsSoFar = 3 // < 5
	if d := e.Decide(p, in); d.Effect != policy.EffectAllow {
		t.Errorf("额度内他方访问应 ALLOW，得 %s (%s)", d.Effect, d.Code)
	}
}

// 红线写配 allow → 放行。
func TestRedlineWriteAllowed(t *testing.T) {
	e := policy.NewEngine()
	p, in := basePolicy(), baseInput()
	p.Actions["write_mode"] = "allow"
	in.Method = "POST"
	in.ActionClasses = []string{"write_mode"}
	if d := e.Decide(p, in); d.Effect != policy.EffectAllow {
		t.Errorf("write_mode=allow 应 ALLOW，得 %s (%s)", d.Effect, d.Code)
	}
}

// wildcard host + 子域。
func TestWildcardAndSubdomain(t *testing.T) {
	e := policy.NewEngine()
	p, in := basePolicy(), baseInput()
	p.Target.Hosts = []policy.HostRule{{Value: "*.target.test", Match: "wildcard"}}
	in.CanonicalURL = "https://sub.target.test/x"
	if d := e.Decide(p, in); d.Effect != policy.EffectAllow {
		t.Errorf("wildcard 子域应 ALLOW，得 %s (%s)", d.Effect, d.Code)
	}
	// exact + allow_subdomains
	p2, in2 := basePolicy(), baseInput()
	p2.Target.AllowSubdomains = true
	in2.CanonicalURL = "https://deep.api.target.test/x"
	if d := e.Decide(p2, in2); d.Effect != policy.EffectAllow {
		t.Errorf("allow_subdomains 应 ALLOW，得 %s (%s)", d.Effect, d.Code)
	}
}

// 端口不在范围 → deny。
func TestPortNotAllowed(t *testing.T) {
	e := policy.NewEngine()
	p, in := basePolicy(), baseInput()
	p.Target.Ports = []int{443}
	in.CanonicalURL = "https://api.target.test:8443/x"
	if d := e.Decide(p, in); d.Code != "port_not_allowed" {
		t.Errorf("越界端口应 port_not_allowed，得 %s (%s)", d.Effect, d.Code)
	}
}

// 坏授权时间 fail-closed。
func TestBadAuthTimeFailClosed(t *testing.T) {
	e := policy.NewEngine()
	p, in := basePolicy(), baseInput()
	p.Authorization.ValidUntil = "not-a-date"
	if d := e.Decide(p, in); d.Code != "auth_window_invalid" {
		t.Errorf("坏时间应 fail-closed auth_window_invalid，得 %s (%s)", d.Effect, d.Code)
	}
}

// 确定性：同输入多次同结果。
func TestDeterministic(t *testing.T) {
	e := policy.NewEngine()
	p, in := basePolicy(), baseInput()
	in.ActionClasses = []string{"write_mode"}
	in.Method = "POST"
	first := e.Decide(p, in)
	for i := 0; i < 50; i++ {
		if d := e.Decide(p, in); d != first {
			t.Fatalf("判定不确定：%+v vs %+v", first, d)
		}
	}
}

// LoadPolicy 从 canonical_json 解析。
func TestLoadPolicy(t *testing.T) {
	cj := []byte(`{"version":1,"target":{"hosts":[{"value":"api.target.test","match":"exact"}]},
		"authorization":{"valid_from":"2026-01-01T00:00:00Z","valid_until":"2026-12-31T23:59:59Z"},
		"network":{"require_proxy":true},"actions":{"write_mode":"deny"}}`)
	p, err := policy.LoadPolicy(cj)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != 1 || len(p.Target.Hosts) != 1 || p.Actions["write_mode"] != "deny" {
		t.Errorf("解析不符: %+v", p)
	}
}
