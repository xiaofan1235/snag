package js

import (
	"strings"
	"testing"
)

// DoD：JS 里的接口被（正则启发式）提取，方法/路径归一化、来源 static。
func TestExtractEndpoints(t *testing.T) {
	js := `
	  fetch("/api/users");
	  axios.post("/api/login", {u, p});
	  var x = new XMLHttpRequest(); x.open("DELETE", "/api/users/42");
	  const u = "/v1/orders/abc-123";
	`
	eps := RegexExtractor{}.ExtractEndpoints(js)
	got := map[string]bool{}
	for _, e := range eps {
		if e.Source != "static" {
			t.Fatalf("来源应 static，实得 %s", e.Source)
		}
		got[e.Method+" "+e.URLPattern] = true
	}
	for _, want := range []string{"GET /api/users", "POST /api/login", "DELETE /api/users/{id}", "GET /v1/orders/abc-123"} {
		if !got[want] {
			t.Fatalf("应提取到 %q，实得 %v", want, got)
		}
	}
}

// secrets 被检出并脱敏（不外泄明文）。
func TestExtractSecretsRedacted(t *testing.T) {
	js := `const a = "AKIAIOSFODNN7EXAMPLE"; const b = 'apikey="abcdef1234567890ABCDEF"';`
	secs := ExtractSecrets(js)
	kinds := map[string]bool{}
	for _, s := range secs {
		kinds[s.Kind] = true
		if strings.Contains(s.Snippet, "EXAMPLE") || strings.Contains(s.Snippet, "1234567890ABCDEF") {
			t.Fatalf("snippet 未脱敏，泄露明文：%q", s.Snippet)
		}
	}
	if !kinds["aws_access_key"] {
		t.Fatalf("应检出 aws_access_key，实得 %v", kinds)
	}
	if !kinds["generic_api_key"] {
		t.Fatalf("应检出 generic_api_key，实得 %v", kinds)
	}
}
