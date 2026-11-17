package policy

import (
	"net/url"
	"strings"
)

// checkScope 校验 URL 是否在授权 Scope 内（host/scheme/port，§9.2）。
// 接收**已规范化**的 CanonicalURL（规范化本身是 Day 16）；此处做结构匹配，host 大小写不敏感。
func checkScope(p Policy, in PolicyInput) (Decision, bool) {
	u, err := url.Parse(in.CanonicalURL)
	if err != nil || u.Host == "" {
		return deny("scope_url_invalid", "URL 非法或缺 host：fail-closed 拒绝"), false
	}
	host := strings.ToLower(u.Hostname())
	scheme := strings.ToLower(u.Scheme)

	// scheme：指定则必须命中；未指定默认允许 http/https。
	if len(p.Target.Schemes) > 0 {
		if !contains(p.Target.Schemes, scheme) {
			return deny("scheme_not_allowed", "scheme 不在授权范围"), false
		}
	} else if scheme != "http" && scheme != "https" {
		return deny("scheme_not_allowed", "仅允许 http/https"), false
	}

	// host：必须命中某条 Hosts 规则。
	matched := false
	for _, rule := range p.Target.Hosts {
		if hostMatches(host, rule, p.Target.AllowSubdomains) {
			matched = true
			break
		}
	}
	if !matched {
		return deny("scope_violation", "host 不在授权 Scope 内"), false
	}

	// port：指定则必须命中（取显式端口，缺省按 scheme 推默认）。
	if len(p.Target.Ports) > 0 {
		port := effectivePort(u, scheme)
		if !containsInt(p.Target.Ports, port) {
			return deny("port_not_allowed", "端口不在授权范围"), false
		}
	}
	return allow(), true
}

// hostMatches 判断 host 是否匹配一条规则（exact / wildcard，含 allow_subdomains）。
func hostMatches(host string, rule HostRule, allowSubdomains bool) bool {
	val := strings.ToLower(rule.Value)
	switch rule.Match {
	case "wildcard":
		base := strings.TrimPrefix(val, "*.")
		return host == base || strings.HasSuffix(host, "."+base)
	default: // exact
		if host == val {
			return true
		}
		if allowSubdomains && strings.HasSuffix(host, "."+val) {
			return true
		}
		return false
	}
}

func effectivePort(u *url.URL, scheme string) int {
	if ps := u.Port(); ps != "" {
		n := 0
		for _, c := range ps {
			if c < '0' || c > '9' {
				return -1
			}
			n = n*10 + int(c-'0')
		}
		return n
	}
	if scheme == "https" {
		return 443
	}
	return 80
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// safeDefaultMethods 是 Default/UnverifiedEndpoints 未配置时的 fail-closed 兜底：只放行安全只读。
var safeDefaultMethods = []string{"GET", "HEAD"}

// checkMethod 校验请求方法（§9.2）。未核验端点只许 unverified_endpoints 的方法；已核验端点按
// methods.default。两者未配置时兜底仅 GET/HEAD（fail-closed）。
// 注：endpoint_rules（逐端点细则）Day 15 暂不解析——默认集已足够覆盖「越权 Method 被拒」的验收；
// 逐端点覆盖随 Scope 规范化（Day 16）与 Proxy（Day 18）接入时再细化。
func checkMethod(p Policy, in PolicyInput) (Decision, bool) {
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if method == "" {
		return deny("method_required", "缺少 HTTP 方法"), false
	}
	var allowed []string
	if !in.EndpointVerified {
		allowed = p.Methods.UnverifiedEndpoints
		if len(allowed) == 0 {
			allowed = safeDefaultMethods
		}
		if !contains(allowed, method) {
			return deny("method_unverified_endpoint", "未核验端点不允许此方法（仅限安全只读）"), false
		}
		return allow(), true
	}
	allowed = p.Methods.Default
	if len(allowed) == 0 {
		allowed = safeDefaultMethods
	}
	if !contains(allowed, method) {
		return deny("method_denied", "方法不在授权允许列表内"), false
	}
	return allow(), true
}
