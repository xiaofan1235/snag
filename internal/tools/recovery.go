// Package tools 实现工具错误恢复协议（§8.5，吸收 HexStrike 分类→恢复→替代→降级）。
//
// 工具失败不是把 error 塞回模型了事：每工具 Manifest 声明 ToolRecoveryPolicy，由**确定性代码**执行。
// 关键红线：替代/降级产生**新 CanonicalActionDigest + 新 Permit**，逐动作重过 Policy，仍走 Sandbox/Proxy；
// 降级不放宽 Scope/身份/速率/预算；已 DISPATCHED 结果未知的动作**不得自动换工具重打**（先对账，§5.3）。
package tools

// 错误类别（§8.5）。
type ErrorClass string

const (
	ErrTimeout         ErrorClass = "timeout"
	ErrRateLimited429  ErrorClass = "rate_limited_429"
	ErrAuthFailed      ErrorClass = "auth_failed"
	ErrToolUnavailable ErrorClass = "tool_unavailable"
	ErrSandboxCrash    ErrorClass = "sandbox_crash"
	ErrOutcomeUnknown  ErrorClass = "outcome_unknown" // DISPATCHED 结果未知：先对账，不换工具
)

// 恢复策略。
type Strategy string

const (
	StratRetry         Strategy = "retry"
	StratAlternative   Strategy = "alternative"    // 换替代工具（新 Digest+新 Permit）
	StratDegrade       Strategy = "degrade"        // 降级实现（新 Digest+新 Permit，仍走执法）
	StratBlockIdentity Strategy = "block_identity" // auth 失败：转 Identity BLOCKED，不重试
	StratReconcile     Strategy = "reconcile"      // 结果未知：先对账，不自动换工具
	StratExhausted     Strategy = "exhausted"      // 恢复耗尽：写 failure_class
)

// Backoff 退避方式。
type Backoff string

const (
	BackoffNone        Backoff = "none"
	BackoffExponential Backoff = "exponential"
	BackoffRetryAfter  Backoff = "retry_after"
)

// ToolRecoveryPolicy 是一个工具对各错误类别的恢复策略（Manifest 声明，§8.5）。
type ToolRecoveryPolicy struct {
	RetryLimit         map[ErrorClass]int
	Backoff            map[ErrorClass]Backoff
	AlternativeToolIDs []string // tool_unavailable 的显式白名单
	DegradationID      string   // 降级实现（必须是注册工具）
	RequireFreshBox    map[ErrorClass]bool
}

// DefaultPolicy 给 §8.5 示例的合理默认。
func DefaultPolicy() ToolRecoveryPolicy {
	return ToolRecoveryPolicy{
		RetryLimit: map[ErrorClass]int{
			ErrTimeout: 2, ErrRateLimited429: 1, ErrAuthFailed: 0, ErrSandboxCrash: 1,
		},
		Backoff: map[ErrorClass]Backoff{
			ErrTimeout: BackoffExponential, ErrRateLimited429: BackoffRetryAfter,
		},
		RequireFreshBox: map[ErrorClass]bool{ErrSandboxCrash: true},
	}
}

// RecoveryAction 是确定性恢复决策的结果。
type RecoveryAction struct {
	Strategy        Strategy
	Backoff         Backoff
	RequireFreshBox bool
	NewDigest       bool // 替代/降级需新 CanonicalActionDigest + 新 Permit
	AlternativeID   string
	Reason          string
}

// Plan 按错误类别 + 已尝试次数 + 策略，确定性给出下一步恢复动作（§8.5）。
func Plan(class ErrorClass, attempts int, p ToolRecoveryPolicy) RecoveryAction {
	switch class {
	case ErrOutcomeUnknown:
		// 已 DISPATCHED 结果未知：绝不自动换工具重打，先对账（§5.3）。
		return RecoveryAction{Strategy: StratReconcile, Reason: "dispatched outcome unknown; reconcile before any retry/switch"}
	case ErrAuthFailed:
		// 不重试，转 Identity BLOCKED。
		return RecoveryAction{Strategy: StratBlockIdentity, Reason: "auth failed; block identity, no retry"}
	case ErrTimeout, ErrRateLimited429, ErrSandboxCrash:
		limit := p.RetryLimit[class]
		if attempts < limit {
			return RecoveryAction{
				Strategy: StratRetry, Backoff: p.Backoff[class],
				RequireFreshBox: p.RequireFreshBox[class], Reason: "retry within limit",
			}
		}
		// 重试耗尽 → 尝试替代/降级，否则 exhausted。
		return afterRetries(p)
	case ErrToolUnavailable:
		return afterRetries(p)
	default:
		return RecoveryAction{Strategy: StratExhausted, Reason: "unknown error class"}
	}
}

// afterRetries：重试耗尽/工具不可用时，按 Manifest 显式声明走替代→降级→exhausted。替代/降级需新 Digest+Permit。
func afterRetries(p ToolRecoveryPolicy) RecoveryAction {
	if len(p.AlternativeToolIDs) > 0 {
		return RecoveryAction{Strategy: StratAlternative, NewDigest: true, AlternativeID: p.AlternativeToolIDs[0], Reason: "switch to whitelisted alternative tool (new digest+permit)"}
	}
	if p.DegradationID != "" {
		return RecoveryAction{Strategy: StratDegrade, NewDigest: true, AlternativeID: p.DegradationID, Reason: "degrade to declared implementation (new digest+permit, still enforced)"}
	}
	return RecoveryAction{Strategy: StratExhausted, Reason: "recovery exhausted; write failure_class"}
}
