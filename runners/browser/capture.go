// Package browser 处理浏览器运行时捕获的攻击面（§12.1 运行时 XHR/Fetch 捕获）。
//
// 有据偏离（网络关闭）：§12.1/§49 规定用 CDP（Playwright/rod）实时捕获；本环境无浏览器且出网受限，
// 无法起浏览器。故退化为「摄取一批已捕获请求」的接口——真实 CDP sidecar 产出的正是这种记录；具备
// 环境后把 Capture() 接到真实 CDP 即可，不动归一化/归因逻辑。实时捕获须经 Policy Proxy、禁 Service
// Worker 以保归因（§42）——这些约束在接真实 CDP 时于 Proxy/启动参数侧施加。
package browser

import (
	"net/url"
	"sort"
	"strings"

	"pentagent/internal/knowledge/surface"
)

// CapturedRequest 是一条运行时捕获的请求（CDP sidecar 产出的记录形态）。
type CapturedRequest struct {
	Method string
	URL    string // 绝对 URL
}

// IngestResult 是摄取结果。
type IngestResult struct {
	Endpoints  []surface.Endpoint // 同源接口（Source=runtime）
	ThirdParty []string           // 第三方域名：只记录不访问（§12.1）
}

// Ingest 把捕获请求归一化为同源接口（Source=runtime）；第三方域只登记域名、不转成接口（§12.1 第三方
// 只记录不访问）。baseOrigin 为目标源（scheme://host[:port]）。
func Ingest(baseOrigin string, reqs []CapturedRequest) IngestResult {
	baseHost := hostOf(baseOrigin)
	seenEp := map[string]surface.Endpoint{}
	seenTP := map[string]struct{}{}

	for _, r := range reqs {
		u, err := url.Parse(r.URL)
		if err != nil || u.Host == "" {
			continue
		}
		if !sameHost(u.Hostname(), baseHost) {
			seenTP[u.Hostname()] = struct{}{} // 第三方：只记录
			continue
		}
		ep := surface.Normalize(surface.Endpoint{Method: r.Method, Path: u.Path, Source: surface.SrcRuntime})
		// 查询参数作为 query 参数记录。
		for name := range u.Query() {
			ep.Params = append(ep.Params, surface.Param{Name: name, Location: surface.InQuery})
		}
		key := ep.Method + " " + ep.URLPattern
		if _, ok := seenEp[key]; !ok {
			seenEp[key] = ep
		}
	}

	var res IngestResult
	for _, e := range seenEp {
		res.Endpoints = append(res.Endpoints, e)
	}
	sort.Slice(res.Endpoints, func(i, j int) bool {
		if res.Endpoints[i].URLPattern != res.Endpoints[j].URLPattern {
			return res.Endpoints[i].URLPattern < res.Endpoints[j].URLPattern
		}
		return res.Endpoints[i].Method < res.Endpoints[j].Method
	})
	for h := range seenTP {
		res.ThirdParty = append(res.ThirdParty, h)
	}
	sort.Strings(res.ThirdParty)
	return res
}

func hostOf(origin string) string {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		// 容错：origin 可能只给了 host。
		return strings.ToLower(strings.TrimSpace(origin))
	}
	return strings.ToLower(u.Hostname())
}

func sameHost(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
