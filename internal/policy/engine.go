// Package policy 实现红线判定点 PDP（Policy Decision Point，§9.0/§9.1/§9.2）。
//
// PDP 是红线三闸门的第一道（判定）：对每个 ActionProposal 做**确定性**判定——同一 (Policy, Input)
// 永远得同一 Decision，可审计、可回放。判定维度（§9.2，固定顺序、fail-closed、首个 deny 即返回）：
// §6.3 元操作 → 授权窗口 → 身份/租户 → Scope/Host → Method → Ownership/IDOR → 配额/预算 → 动作类红线。
//
// 设计边界：
//   - Scope 的 URL **规范化**是 Day 16（最关键，单独做）；本包接收已规范化的 CanonicalURL，只做
//     host/scheme/port 匹配。规范化不到位会让匹配失真——那是 Day 16 的职责。
//   - 动作→红线类的**分类**由上游提供（PolicyInput.ActionClasses）；PDP 只按类查 policy.actions。
//   - 计数态（已发请求数、IDOR 记录数）由上游注入，PDP 只做确定性比较，不自己计数。
package policy

import (
	"encoding/json"
	"time"
)

// Effect 是判定结果。
type Effect string

const (
	EffectAllow           Effect = "ALLOW"
	EffectDeny            Effect = "DENY"
	EffectRequireApproval Effect = "REQUIRE_APPROVAL"
)

// Decision 是一次判定的输出：结果 + 机读 code + 人读原因。
type Decision struct {
	Effect Effect `json:"effect"`
	Code   string `json:"code"`   // 稳定机读码，如 scope_violation / method_denied / redline_write_mode
	Reason string `json:"reason"` // 人读说明
}

func allow() Decision { return Decision{Effect: EffectAllow, Code: "allow"} }
func deny(code, reason string) Decision {
	return Decision{Effect: EffectDeny, Code: code, Reason: reason}
}
func requireApproval(code, reason string) Decision {
	return Decision{Effect: EffectRequireApproval, Code: code, Reason: reason}
}

// IsAllowed 报告是否放行（含需审批前的判定通过，但 Effect 仍需调用方据审批流处理）。
func (d Decision) IsAllowed() bool { return d.Effect == EffectAllow }

// Policy 是解析后的授权策略（§9.1，对应 policy.schema.json；只保留 PDP 用到的字段）。
type Policy struct {
	Version       int               `json:"version"`
	Target        Target            `json:"target"`
	Authorization AuthWindow        `json:"authorization"`
	Methods       Methods           `json:"methods"`
	Network       Network           `json:"network"`
	Identities    Identities        `json:"identities"`
	Limits        Limits            `json:"limits"`
	Actions       map[string]string `json:"actions"` // 红线类 → deny|allow|require_approval（缺省/deny=拒）
}

// Target 是授权范围（host/scheme/port）。
type Target struct {
	Hosts           []HostRule `json:"hosts"`
	AllowSubdomains bool       `json:"allow_subdomains"`
	Schemes         []string   `json:"schemes"`
	Ports           []int      `json:"ports"`
	RedirectPolicy  string     `json:"redirect_policy"`
}

// HostRule 是一条 host 匹配规则。
type HostRule struct {
	Value string `json:"value"`
	Match string `json:"match"` // exact | wildcard
}

// AuthWindow 是授权时间窗（§45 硬停依据）。
type AuthWindow struct {
	ValidFrom  string `json:"valid_from"`
	ValidUntil string `json:"valid_until"`
}

// Methods 是方法允许规则。
type Methods struct {
	UnverifiedEndpoints []string          `json:"unverified_endpoints"`
	Default             []string          `json:"default"`
	EndpointRules       []json.RawMessage `json:"endpoint_rules"` // Day 15 暂不逐条解析（见 methods.go 注释）
}

// Network 是网络/速率/配额。
type Network struct {
	RequireProxy bool `json:"require_proxy"`
	MaxRequests  int  `json:"max_requests"`
}

// Identities 是允许的身份与租户。
type Identities struct {
	Allowed        []string `json:"allowed"`
	AllowedTenants []string `json:"allowed_tenants"`
}

// Limits 是各类上限。
type Limits struct {
	IdorOtherRecords         int `json:"idor_other_records"`
	SensitiveEvidenceRecords int `json:"sensitive_evidence_records"`
	MaxResponseBytes         int `json:"max_response_bytes"`
	MaxDownloadBytes         int `json:"max_download_bytes"`
}

// LoadPolicy 从 policy_revisions.canonical_json 解析 Policy。
func LoadPolicy(canonicalJSON []byte) (Policy, error) {
	var p Policy
	if err := json.Unmarshal(canonicalJSON, &p); err != nil {
		return Policy{}, err
	}
	return p, nil
}

// PolicyInput 是一次判定的输入（§9.2：远不止 URL+Method）。
type PolicyInput struct {
	Now time.Time // 判定时刻（授权窗口用）

	CanonicalURL string // 已由 Day 16 规范化的 URL
	Method       string

	IdentityID string
	Tenant     string

	EndpointVerified bool   // EndpointVerificationState：端点是否已核验
	ObjectOwnership  string // self | other | none（跨租户对象=other，IDOR 判定）

	Tool            string
	RiskClass       string
	ActionClasses   []string // 本动作触碰的红线类（上游分类），如 ["write_mode","persistence"]
	IsMetaOperation bool     // §6.3：改 Scope/定级/结束 Run 等元操作（一律拒）

	RequestsSoFar    int // 配额计数（上游注入）
	IdorRecordsSoFar int // 已访问的他方记录数（IDOR 配额）
}

// Engine 是 PDP。无状态：Decide 是纯函数。
type Engine struct{}

// NewEngine 构造 PDP。
func NewEngine() *Engine { return &Engine{} }

// Decide 对 (policy, input) 做确定性判定。固定顺序、fail-closed、首个 deny 即返回；
// 全程无 deny 但有动作类需审批 → REQUIRE_APPROVAL；否则 ALLOW。
func (e *Engine) Decide(p Policy, in PolicyInput) Decision {
	// §6.3：模型不得通过提案改 Scope/定级/结束 Run。
	if in.IsMetaOperation {
		return deny("redline_meta_operation", "模型不得通过提案修改 Scope/定级或结束 Run（§6.3）")
	}
	// 授权时间窗。
	if d, ok := checkAuthWindow(p, in); !ok {
		return d
	}
	// 身份 + 租户。
	if d, ok := checkIdentity(p, in); !ok {
		return d
	}
	// Scope / Host。
	if d, ok := checkScope(p, in); !ok {
		return d
	}
	// Method。
	if d, ok := checkMethod(p, in); !ok {
		return d
	}
	// Ownership / IDOR 配额。
	if d, ok := checkOwnership(p, in); !ok {
		return d
	}
	// 请求配额。
	if d, ok := checkQuota(p, in); !ok {
		return d
	}
	// 动作类红线（§9.1，默认 deny；可能需审批）。
	approvalNeeded := false
	var approvalClass string
	for _, class := range in.ActionClasses {
		switch p.Actions[class] {
		case "allow":
			// 放行
		case "require_approval":
			approvalNeeded = true
			approvalClass = class
		default: // "deny" 或缺省
			return deny("redline_"+class, "动作命中红线类 "+class+"，策略默认拒绝（§9.1）")
		}
	}
	if approvalNeeded {
		return requireApproval("approval_"+approvalClass, "动作命中红线类 "+approvalClass+"，需人工审批（§9.1）")
	}
	return allow()
}
