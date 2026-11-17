package browser

import "testing"

// 运行时捕获摄取：同源→接口（Source=runtime，query 参数提取）；第三方域只记录不访问（§12.1/§42）。
func TestIngestSameOriginAndThirdParty(t *testing.T) {
	res := Ingest("https://app.target.test", []CapturedRequest{
		{Method: "GET", URL: "https://app.target.test/api/profile?tab=1"},
		{Method: "POST", URL: "https://app.target.test/api/orders/99"},
		{Method: "GET", URL: "https://cdn.thirdparty.com/lib.js"},
	})

	got := map[string]bool{}
	for _, e := range res.Endpoints {
		if e.Source != "runtime" {
			t.Fatalf("来源应 runtime，实得 %s", e.Source)
		}
		got[e.Method+" "+e.URLPattern] = true
	}
	if !got["GET /api/profile"] || !got["POST /api/orders/{id}"] {
		t.Fatalf("同源接口归一化错误：%v", got)
	}
	// 第三方域只记录不转成接口。
	if len(res.ThirdParty) != 1 || res.ThirdParty[0] != "cdn.thirdparty.com" {
		t.Fatalf("第三方域应只记录 cdn.thirdparty.com，实得 %v", res.ThirdParty)
	}
	for _, e := range res.Endpoints {
		if e.URLPattern == "/lib.js" {
			t.Fatal("第三方请求不应转成接口（只记录不访问）")
		}
	}
	// query 参数提取。
	foundQ := false
	for _, e := range res.Endpoints {
		if e.URLPattern == "/api/profile" {
			for _, p := range e.Params {
				if p.Name == "tab" && p.Location == "query" {
					foundQ = true
				}
			}
		}
	}
	if !foundQ {
		t.Fatal("query 参数 tab 应被提取")
	}
}
