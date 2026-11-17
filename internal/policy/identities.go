package policy

// contains 报告 slice 是否含 item（精确匹配）。
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// checkIdentity 校验身份与租户（§9.2）。IdentityID 必填；若策略给了白名单则必须命中。
func checkIdentity(p Policy, in PolicyInput) (Decision, bool) {
	if in.IdentityID == "" {
		return deny("identity_required", "动作缺少 identity_id（§4.1 必填）"), false
	}
	if len(p.Identities.Allowed) > 0 && !contains(p.Identities.Allowed, in.IdentityID) {
		return deny("identity_not_allowed", "身份不在授权白名单内"), false
	}
	// 租户边界 fail-closed（Q-19）：策略配了 allowed_tenants 即表示租户是硬边界——
	// 此时空租户无法核验，一律拒（去掉旧版 in.Tenant!="" 的 fail-open 逃生门）。
	if len(p.Identities.AllowedTenants) > 0 && !contains(p.Identities.AllowedTenants, in.Tenant) {
		if in.Tenant == "" {
			return deny("tenant_required", "策略限定了 allowed_tenants，但请求缺租户：fail-closed 拒绝"), false
		}
		return deny("tenant_not_allowed", "租户不在授权白名单内"), false
	}
	return allow(), true
}

// checkOwnership 校验对象归属与 IDOR 配额（§9.2）。
// 访问他方对象（ObjectOwnership=other）受 limits.idor_other_records 约束：
// 上限为 0 则任何他方访问即拒；否则已访问数达上限即拒。self/none 放行。
func checkOwnership(p Policy, in PolicyInput) (Decision, bool) {
	if in.ObjectOwnership != "other" {
		return allow(), true
	}
	if p.Limits.IdorOtherRecords <= 0 {
		return deny("idor_denied", "策略不允许访问他方对象（idor_other_records=0）"), false
	}
	if in.IdorRecordsSoFar >= p.Limits.IdorOtherRecords {
		return deny("idor_quota_exhausted", "已达他方记录访问上限 idor_other_records"), false
	}
	return allow(), true
}
