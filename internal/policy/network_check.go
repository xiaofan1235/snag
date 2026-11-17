package policy

import "time"

// network_check.go —— Policy Proxy 逐请求的网络层判定（§9.5 执行时序）。
//
// 复用 PDP 的同一套检查（Canonicalize / HostInTarget / checkAuthWindow / checkMethod），
// 避免 Proxy 自建第二套解析/判定逻辑造成分歧（Q-20：Proxy 与 PDP 同源）。
// identity/动作类红线已在 PDP（提案时）判定，这里只做网络层逐请求回查：Scope + 授权窗 + Method。

// CheckNetwork 对一个在线请求做网络层判定。endpointVerified 表示该端点是否已核验
// （决定按 unverified_endpoints 还是 methods.default 放行方法）。
func CheckNetwork(p Policy, rawURL, method string, endpointVerified bool, now time.Time) Decision {
	c, err := Canonicalize(rawURL)
	if err != nil {
		return deny("scope_url_invalid", "URL 规范化失败：fail-closed 拒绝（"+err.Error()+"）")
	}
	if !HostInTarget(c, p.Target) {
		return deny("scope_violation", "host 不在授权 Scope 内")
	}
	if d, ok := checkAuthWindow(p, PolicyInput{Now: now}); !ok {
		return d
	}
	if d, ok := checkMethod(p, PolicyInput{Method: method, EndpointVerified: endpointVerified}); !ok {
		return d
	}
	return allow()
}
