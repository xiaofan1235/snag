package policy

import "time"

// checkAuthWindow 校验授权时间窗（§45：valid_until 是硬停依据）。
// 坏日期一律 fail-closed 拒绝（Q-8：此处显式 time.Parse，不依赖 schema 的 format 断言）。
func checkAuthWindow(p Policy, in PolicyInput) (Decision, bool) {
	from, err := time.Parse(time.RFC3339, p.Authorization.ValidFrom)
	if err != nil {
		return deny("auth_window_invalid", "授权 valid_from 非法时间：fail-closed 拒绝"), false
	}
	until, err := time.Parse(time.RFC3339, p.Authorization.ValidUntil)
	if err != nil {
		return deny("auth_window_invalid", "授权 valid_until 非法时间：fail-closed 拒绝"), false
	}
	now := in.Now
	if now.Before(from) {
		return deny("auth_not_yet_valid", "当前早于授权生效时间 valid_from"), false
	}
	if now.After(until) {
		return deny("auth_window_expired", "授权已过期（晚于 valid_until，§45 硬停）"), false
	}
	return allow(), true
}

// checkQuota 校验请求配额（§9.2：network.max_requests）。max_requests=0 视为不限。
func checkQuota(p Policy, in PolicyInput) (Decision, bool) {
	if p.Network.MaxRequests > 0 && in.RequestsSoFar >= p.Network.MaxRequests {
		return deny("quota_exhausted", "已达请求配额上限 max_requests"), false
	}
	return allow(), true
}
