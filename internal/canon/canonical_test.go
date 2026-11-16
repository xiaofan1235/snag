package canon_test

import (
	"testing"

	"pentagent/api/schemas"
	"pentagent/internal/canon"
)

// canonical_test.go —— Day 04 §7 验收：
//   - 同内容对象（键序打乱、数字写法不同）Digest 必须相等（§47.5 的立身不变量）
//   - 五个 Schema 能接受合法样例、拒绝非法样例

// 键序打乱（含嵌套）→ Digest 相等。
func TestDigestKeyOrderInvariant(t *testing.T) {
	a := []byte(`{"b":2,"a":1,"nested":{"y":10,"x":20},"arr":[1,2,3]}`)
	b := []byte(`{"arr":[1,2,3],"a":1,"nested":{"x":20,"y":10},"b":2}`)
	da, err := canon.DigestJSON(a)
	if err != nil {
		t.Fatal(err)
	}
	db, err := canon.DigestJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if da != db {
		t.Errorf("键序不同应得同 digest:\n a=%s\n b=%s", da, db)
	}
}

// 数字归一：1 / 1.0 / 1.00 / 1e0 应得同 digest；10 / 1e1 同；且与 2 不同。
func TestDigestNumberNormalization(t *testing.T) {
	forms := []string{`{"n":1}`, `{"n":1.0}`, `{"n":1.00}`, `{"n":1e0}`}
	var first string
	for i, f := range forms {
		d, err := canon.DigestJSON([]byte(f))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if i == 0 {
			first = d
		} else if d != first {
			t.Errorf("数字归一失败: %s 的 digest 与 {\"n\":1} 不同", f)
		}
	}
	ten1, _ := canon.DigestJSON([]byte(`{"n":10}`))
	ten2, _ := canon.DigestJSON([]byte(`{"n":1e1}`))
	if ten1 != ten2 {
		t.Error("10 与 1e1 应归一为同 digest")
	}
	if ten1 == first {
		t.Error("不同数值(10 vs 1)不应得同 digest")
	}
}

// 不同内容 → 不同 digest；且 Digest(任意 Go 值) 与等价 JSON 一致。
func TestDigestContentSensitivityAndGoValue(t *testing.T) {
	d1, _ := canon.DigestJSON([]byte(`{"a":1}`))
	d2, _ := canon.DigestJSON([]byte(`{"a":2}`))
	if d1 == d2 {
		t.Error("不同内容应得不同 digest")
	}
	// Go map（键序由 json.Marshal 决定）应与手写 JSON 归一后相等。
	dGo, err := canon.Digest(map[string]any{"b": 2, "a": 1})
	if err != nil {
		t.Fatal(err)
	}
	dJSON, _ := canon.DigestJSON([]byte(`{"a":1,"b":2}`))
	if dGo != dJSON {
		t.Errorf("Digest(Go值) 应与等价 JSON 相等: %s vs %s", dGo, dJSON)
	}
}

func TestCanonicalizeRejectsInvalidJSON(t *testing.T) {
	if _, err := canon.Canonicalize([]byte(`{not json`)); err == nil {
		t.Error("非法 JSON 应报错")
	}
	// RFC 8785：重复键应被拒。
	if _, err := canon.Canonicalize([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Error("重复键应被 JCS 拒绝")
	}
}

func TestRawSHA256(t *testing.T) {
	// 空输入的 SHA-256 是已知常量，锚一下实现正确。
	if got := canon.RawSHA256(nil); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("RawSHA256(nil) 不符: %s", got)
	}
}

// 五个契约 Schema：合法样例通过、非法样例被拒。
func TestSchemasValidAndInvalid(t *testing.T) {
	cases := []struct {
		schema  string
		valid   string
		invalid string
		why     string
	}{
		{
			schema:  schemas.Policy,
			valid:   `{"version":1,"target":{"hosts":[{"value":"app.example.test","match":"exact"}]},"authorization":{"valid_from":"2026-11-16T00:00:00Z","valid_until":"2026-11-17T00:00:00Z"},"network":{"require_proxy":true},"actions":{"write_mode":"deny"}}`,
			invalid: `{"version":2,"target":{"hosts":[{"value":"app.example.test","match":"exact"}]},"authorization":{"valid_from":"2026-11-16T00:00:00Z","valid_until":"2026-11-17T00:00:00Z"},"network":{"require_proxy":true},"actions":{"write_mode":"deny"}}`,
			why:     "version 必须为 const 1",
		},
		{
			schema:  schemas.Policy,
			valid:   `{"version":1,"target":{"hosts":[{"value":"a.test","match":"wildcard"}]},"authorization":{"valid_from":"2026-11-16T00:00:00Z","valid_until":"2026-11-17T00:00:00Z"},"network":{"require_proxy":false},"actions":{}}`,
			invalid: `{"version":1,"target":{"hosts":[{"value":"a.test","match":"regex"}]},"authorization":{"valid_from":"2026-11-16T00:00:00Z","valid_until":"2026-11-17T00:00:00Z"},"network":{"require_proxy":false},"actions":{}}`,
			why:     "match 只能 exact/wildcard（精确vs泛域名两类，§9.3）",
		},
		{
			schema:  schemas.PlanPatch,
			valid:   `{"snapshot_revision":182,"operations":[{"op":"create_job"},{"op":"close_epoch"}]}`,
			invalid: `{"snapshot_revision":182,"operations":[{"op":"modify_scope"}]}`,
			why:     "op 不在 M0 允许清单（模型不能改 Scope）",
		},
		{
			schema:  schemas.Action,
			valid:   `{"tool_name":"http.request","action":{"method":"GET","url":"https://app.example.test/"},"identity_id":"id-1","risk_class":"read","idempotency_class":"idempotent"}`,
			invalid: `{"tool_name":"http.request","action":{},"identity_id":"id-1","risk_class":"read"}`,
			why:     "缺 idempotency_class（对账方式依赖它，§4.1）",
		},
		{
			schema:  schemas.Experiment,
			valid:   `{"hypothesis_id":"h-1","required_evidence":["baseline","variant"],"budget":{"max_requests":12,"max_duration_seconds":900}}`,
			invalid: `{"hypothesis_id":"h-1","required_evidence":[],"budget":{"max_requests":12,"max_duration_seconds":900}}`,
			why:     "required_evidence 至少一项",
		},
		{
			schema:  schemas.Evidence,
			valid:   `{"title":"IDOR","vulnerability_type":"idor","severity":"high","state":"CANDIDATE","evidence":[{"evidence_id":"e-1","role":"baseline"}]}`,
			invalid: `{"title":"IDOR","vulnerability_type":"idor","severity":"high","state":"FABRICATED","evidence":[{"evidence_id":"e-1","role":"baseline"}]}`,
			why:     "state 必须是 FindingState 枚举",
		},
	}

	for _, c := range cases {
		schemaBytes, err := schemas.FS.ReadFile(c.schema)
		if err != nil {
			t.Fatalf("读取 schema %s: %v", c.schema, err)
		}
		if err := canon.ValidateAgainst(schemaBytes, []byte(c.valid)); err != nil {
			t.Errorf("[%s] 合法样例被拒(%s): %v", c.schema, c.why, err)
		}
		if err := canon.ValidateAgainst(schemaBytes, []byte(c.invalid)); err == nil {
			t.Errorf("[%s] 非法样例应被拒但通过了（%s）", c.schema, c.why)
		}
	}
}

// Q-7/Q-8：契约冻结日收口——红线开关名拼错必须被拒；垃圾授权日期必须被拒。
func TestPolicyHardening(t *testing.T) {
	policy, err := schemas.FS.ReadFile(schemas.Policy)
	if err != nil {
		t.Fatal(err)
	}
	base := func(auth, actions string) string {
		return `{"version":1,"target":{"hosts":[{"value":"a.test","match":"exact"}]},` +
			`"authorization":` + auth + `,"network":{"require_proxy":true},"actions":` + actions + `}`
	}
	goodAuth := `{"valid_from":"2026-11-16T00:00:00Z","valid_until":"2026-11-17T00:00:00Z"}`

	// Q-7：拼错的开关名 write_mod（实为 write_mode）必须被拒，而非静默放行。
	if err := canon.ValidateAgainst(policy, []byte(base(goodAuth, `{"write_mod":"deny"}`))); err == nil {
		t.Error("Q-7：拼错的红线开关名应被拒（additionalProperties=false）")
	}
	// 正确开关名应通过。
	if err := canon.ValidateAgainst(policy, []byte(base(goodAuth, `{"write_mode":"deny"}`))); err != nil {
		t.Errorf("正确开关名不应被拒: %v", err)
	}
	// Q-8：垃圾授权日期必须被拒（format 断言已开启）。
	badAuth := `{"valid_from":"not-a-date","valid_until":"2026-11-17T00:00:00Z"}`
	if err := canon.ValidateAgainst(policy, []byte(base(badAuth, `{"write_mode":"deny"}`))); err == nil {
		t.Error("Q-8：垃圾 date-time 授权窗应被 format 断言拒绝")
	}
}
