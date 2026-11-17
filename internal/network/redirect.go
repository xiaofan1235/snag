package network

import (
	"net/url"

	"pentagent/internal/policy"
)

// redirect.go —— 重定向守卫（§9.5）：重定向目标必须重新过 Scope——**越域重定向被拦**，须重新授权。

// RedirectTarget 把 Location（可能是相对 URL）相对 base 解析为绝对 URL。
func RedirectTarget(base, location string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	l, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return b.ResolveReference(l).String(), nil
}

// RedirectInScope 判断一个重定向的目标是否仍在授权 Scope 内（规范化后匹配 Target）。
// 越域返回 false——调用方应拦截并要求重新授权，而非自动跟随。
func RedirectInScope(base, location string, t policy.Target) (bool, error) {
	abs, err := RedirectTarget(base, location)
	if err != nil {
		return false, err
	}
	in, _, err := policy.InScope(abs, t)
	return in, err
}
