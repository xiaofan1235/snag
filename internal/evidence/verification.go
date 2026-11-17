package evidence

// verification.go —— 独立 Verifier 分两层（§13.4）：确定性验证器（哈希/真实执行/同 Action/身份/角色/策略/
// 引用完整/相反证据）+ AI 语义验证器（数据是否真敏感/是否真跨身份/差异是否支持结论/其他解释/定级是否过高）。
//
// 铁律（§13.4）：AI 验证器**不能直接改 Evidence，也不能绕过确定性验证**——两层都过才算通过；确定性层任一
// 不过即拒。语义层读独立证据投影，不读作者结论话术（§31 V4），并主动搜寻反证。

// DetChecks 是确定性验证器的各项检查结果（§13.4）。
type DetChecks struct {
	EvidenceHashOK     bool // Evidence 哈希一致
	ActionExecuted     bool // Action 真实执行（有 Receipt）
	SameActionReqResp  bool // 请求与响应属同一 Action
	IdentityMatch      bool // 身份匹配
	RolesComplete      bool // Evidence 角色齐全（baseline/variant/ownership）
	PolicyValid        bool // 策略有效
	CitationsComplete  bool // Finding 引用完整
	NoContraryEvidence bool // 无相反证据
}

// DeterministicVerify 运行确定性验证：所有检查必须通过，返回 (pass, 未过原因列表)。
func DeterministicVerify(c DetChecks) (bool, []string) {
	var fail []string
	add := func(ok bool, name string) {
		if !ok {
			fail = append(fail, name)
		}
	}
	add(c.EvidenceHashOK, "evidence_hash")
	add(c.ActionExecuted, "action_executed")
	add(c.SameActionReqResp, "same_action_req_resp")
	add(c.IdentityMatch, "identity_match")
	add(c.RolesComplete, "roles_complete")
	add(c.PolicyValid, "policy_valid")
	add(c.CitationsComplete, "citations_complete")
	add(c.NoContraryEvidence, "no_contrary_evidence")
	return len(fail) == 0, fail
}

// SemanticVerdict 是 AI 语义验证结论（§13.4）。
type SemanticVerdict struct {
	DataSensitive       bool // 数据确实敏感
	TrulyCrossIdentity  bool // 确实跨身份
	DiffSupportsClaim   bool // 响应差异支持结论
	NoAltExplanation    bool // 无其他合理解释
	PreconditionHonest  bool // 前置条件未被淡化
	ZeroInteractionMet  bool // 满足零交互要求（如适用）
	SeverityNotInflated bool // 定级未过高
	Notes               string
}

// SemanticVerifier 是 AI 语义验证器接口（异质模型实现；不能改 Evidence、不能绕过确定性层）。
type SemanticVerifier interface {
	Verify(detPassed bool, independentProjection []byte) (SemanticVerdict, error)
}

// Passed 报告语义结论是否全项通过。
func (v SemanticVerdict) Passed() bool {
	return v.DataSensitive && v.TrulyCrossIdentity && v.DiffSupportsClaim &&
		v.NoAltExplanation && v.PreconditionHonest && v.SeverityNotInflated
}

// VerifyResult 是两层验证的合并结果。
type VerifyResult struct {
	DetPassed      bool
	DetFailReasons []string
	SemPassed      bool
	Confirmed      bool
}

// Verify 组合两层：确定性层必须先过（语义层不能绕过）；两层都过才 Confirmed。语义层只在确定性通过后运行。
func Verify(det DetChecks, sem SemanticVerifier, projection []byte) (VerifyResult, error) {
	var res VerifyResult
	res.DetPassed, res.DetFailReasons = DeterministicVerify(det)
	if !res.DetPassed {
		return res, nil // 确定性不过即拒，语义层不能救
	}
	if sem == nil {
		return res, nil // 无语义层：仅确定性通过，未 Confirmed
	}
	v, err := sem.Verify(res.DetPassed, projection)
	if err != nil {
		return res, err
	}
	res.SemPassed = v.Passed()
	res.Confirmed = res.DetPassed && res.SemPassed
	return res, nil
}
