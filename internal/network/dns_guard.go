// Package network 实现 Policy Proxy——所有目标请求的咽喉（§9.5 执行时序 / §31 P1x / §13.2 留证 / §9.0）。
//
// 红线第二道闸门 + 证据产生点。逐请求回查 Permit/Fencing/Scope/授权窗/撤销代际，原始留证；
// 进程内 epoch 缓存把热路径降为 O(1)。**不信任 Runner 持有的 Permit 快照**，每跳独立回查（§9.5）。
package network

import "net/netip"

// isPrivateHost 报告（已规范化的）host 是否为不可出网的私有/环回/链路本地/元数据地址（SSRF egress 守卫）。
// host 若是域名（非 IP 字面量）返回 false——域名的私网解析属连接时 DNS 守卫（DNS 重绑定），
// 需在实际拨号时按解析结果再查；本函数守住 IP 字面量这一层。
func isPrivateHost(host string) bool {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	// 云元数据端点（169.254.169.254）落在链路本地内，下面一并覆盖。
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsUnspecified() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() {
		return true
	}
	// CGNAT 100.64.0.0/10（RFC 6598）。
	if addr.Is4() {
		cgnat := netip.MustParsePrefix("100.64.0.0/10")
		if cgnat.Contains(addr) {
			return true
		}
	}
	return false
}
