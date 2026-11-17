package network

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// egress.go —— Egress 管理（§10.1）：代理租约，禁本机直连——沙箱的唯一出口是 Policy Proxy。
//
// Runner 只拿到一个 EgressLease（指向 Proxy），据此构造的 http.Client 把**所有**流量强制走代理、
// 无直连路由。租约过期即不能再出网。本机直连模式留到 §38/后续。

// EgressLease 是一次代理出网租约。
type EgressLease struct {
	ProxyURL   string
	ValidUntil time.Time
}

// Valid 报告租约在 now 是否有效。
func (l EgressLease) Valid(now time.Time) bool {
	return l.ProxyURL != "" && now.Before(l.ValidUntil)
}

// Client 据租约构造一个**只能经代理出网**的 http.Client（无直连）。租约无效则返回错误（fail-closed）。
func (l EgressLease) Client(now time.Time) (*http.Client, error) {
	if !l.Valid(now) {
		return nil, fmt.Errorf("egress 租约无效或已过期：禁止出网")
	}
	pu, err := url.Parse(l.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理地址非法: %w", err)
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(pu), // 唯一出口：所有请求走该代理
			// 不设 DialContext 直连兜底——无代理即无路由（禁本机直连，§10.1）。
		},
	}, nil
}
