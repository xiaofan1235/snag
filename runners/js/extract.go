// Package js 从 JavaScript 源码提取攻击面（端点、参数）与 secrets（§12.1 静态接口提取）。
//
// 有据偏离（网络关闭）：§12.1 规定 JS AST 用 tree-sitter/Babel；本环境无法拉取 tree-sitter 的 CGO 语法，
// 故退化为 stdlib 正则启发式提取（spec §12.1 原本也把「文本正则」列为补充手段，这里因约束升为主路）。
// Extractor 做成可插拔接口：具备环境后可换真实 AST 实现而不动上层。
package js

import (
	"regexp"
	"sort"
	"strings"

	"pentagent/internal/knowledge/surface"
)

// Extractor 是 JS 端点提取器接口（可插拔：正则启发式 or 真实 AST）。
type Extractor interface {
	ExtractEndpoints(jsSource string) []surface.Endpoint
}

// RegexExtractor 是正则启发式提取器（tree-sitter 的离线替代）。
type RegexExtractor struct{}

var (
	// fetch("URL") / fetch('URL') / fetch(`URL`)
	reFetch = regexp.MustCompile(`fetch\(\s*["'` + "`" + `]([^"'` + "`" + `]+)["'` + "`" + `]`)
	// axios.get("URL") / axios.post("URL") / axios({url:"URL", method:"post"})
	reAxios = regexp.MustCompile(`axios\.(get|post|put|patch|delete|head)\(\s*["'` + "`" + `]([^"'` + "`" + `]+)["'` + "`" + `]`)
	// XMLHttpRequest: .open("METHOD","URL")
	reXHR = regexp.MustCompile(`\.open\(\s*["'](GET|POST|PUT|PATCH|DELETE|HEAD)["']\s*,\s*["'` + "`" + `]([^"'` + "`" + `]+)["'` + "`" + `]`)
	// 裸 API 路径字面量 "/api/...", "/v1/..." 等（补充）。
	rePathLit = regexp.MustCompile(`["'` + "`" + `](/(?:api|v\d+|graphql|rest|internal)/[A-Za-z0-9_\-/{}.:]*)["'` + "`" + `]`)
)

// ExtractEndpoints 从 JS 源码正则启发式提取端点（Source=static，方法尽量从调用上下文推断）。
func (RegexExtractor) ExtractEndpoints(js string) []surface.Endpoint {
	seen := map[string]surface.Endpoint{}
	add := func(method, url string) {
		if url == "" || strings.HasPrefix(url, "data:") {
			return
		}
		ep := surface.Normalize(surface.Endpoint{Method: method, Path: url, Source: surface.SrcStatic})
		key := ep.Method + " " + ep.URLPattern
		if _, ok := seen[key]; !ok {
			seen[key] = ep
		}
	}
	for _, m := range reFetch.FindAllStringSubmatch(js, -1) {
		add("GET", m[1]) // fetch 默认 GET（无法从字面确知时保守取 GET）
	}
	for _, m := range reAxios.FindAllStringSubmatch(js, -1) {
		add(m[1], m[2])
	}
	for _, m := range reXHR.FindAllStringSubmatch(js, -1) {
		add(m[1], m[2])
	}
	for _, m := range rePathLit.FindAllStringSubmatch(js, -1) {
		add("GET", m[1])
	}
	out := make([]surface.Endpoint, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].URLPattern != out[j].URLPattern {
			return out[i].URLPattern < out[j].URLPattern
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// Secret 是在 JS 里命中的疑似凭据（只记类别与位置，不外泄明文）。
type Secret struct {
	Kind    string // aws_access_key / generic_api_key / bearer_token / private_key
	Snippet string // 脱敏片段（仅前缀，供人工核验）
}

var secretPatterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"aws_access_key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"generic_api_key", regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|secret|access[_-]?token)["'` + "`" + `]?\s*[:=]\s*["'` + "`" + `][A-Za-z0-9_\-]{16,}["'` + "`" + `]`)},
	{"bearer_token", regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9_\-\.]{20,}`)},
	{"private_key", regexp.MustCompile(`-----BEGIN (?:RSA |EC )?PRIVATE KEY-----`)},
}

// ExtractSecrets 正则扫描 JS 里的疑似凭据（§12.1 secrets），脱敏后返回类别。
func ExtractSecrets(js string) []Secret {
	var out []Secret
	for _, sp := range secretPatterns {
		for _, m := range sp.re.FindAllString(js, -1) {
			out = append(out, Secret{Kind: sp.kind, Snippet: redact(m)})
		}
	}
	return out
}

// redact 只保留前 6 字符，其余打码（不外泄明文凭据）。
func redact(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 6 {
		return "******"
	}
	return s[:6] + "…(" + itoa(len(s)) + " chars)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
