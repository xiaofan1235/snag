package tools

import "testing"

// DoD / §8.5 验收：超时、429、认证失败、工具不存在、容器崩溃分别走预期策略。
func TestRecoveryFiveClasses(t *testing.T) {
	p := DefaultPolicy()

	// timeout：限额内重试、指数退避。
	if a := Plan(ErrTimeout, 0, p); a.Strategy != StratRetry || a.Backoff != BackoffExponential {
		t.Fatalf("timeout 应重试+指数退避，实得 %+v", a)
	}
	// 429：按 retry_after 重试。
	if a := Plan(ErrRateLimited429, 0, p); a.Strategy != StratRetry || a.Backoff != BackoffRetryAfter {
		t.Fatalf("429 应重试+retry_after，实得 %+v", a)
	}
	// auth_failed：不重试，转 Identity BLOCKED。
	if a := Plan(ErrAuthFailed, 0, p); a.Strategy != StratBlockIdentity {
		t.Fatalf("auth_failed 应 block_identity 不重试，实得 %+v", a)
	}
	// sandbox_crash：重试 + 要求全新沙箱。
	if a := Plan(ErrSandboxCrash, 0, p); a.Strategy != StratRetry || !a.RequireFreshBox {
		t.Fatalf("sandbox_crash 应重试+fresh box，实得 %+v", a)
	}
	// tool_unavailable 无替代/降级 → exhausted。
	if a := Plan(ErrToolUnavailable, 0, p); a.Strategy != StratExhausted {
		t.Fatalf("工具不可用且无替代应 exhausted，实得 %+v", a)
	}
}

// OUTCOME_UNKNOWN：已 DISPATCHED 结果未知，先对账、**不自动换工具重打**（§5.3/§8.5）。
func TestRecoveryOutcomeUnknownNoSwitch(t *testing.T) {
	if a := Plan(ErrOutcomeUnknown, 5, DefaultPolicy()); a.Strategy != StratReconcile {
		t.Fatalf("结果未知应 reconcile 不换工具，实得 %+v", a)
	}
}

// 替代/降级产生新 Digest（+新 Permit，逐动作重过 Policy）。
func TestRecoveryAlternativeAndDegrade(t *testing.T) {
	p := DefaultPolicy()
	p.AlternativeToolIDs = []string{"alt-1"}
	if a := Plan(ErrToolUnavailable, 0, p); a.Strategy != StratAlternative || !a.NewDigest || a.AlternativeID != "alt-1" {
		t.Fatalf("有替代应换替代工具+新 Digest，实得 %+v", a)
	}
	p2 := DefaultPolicy()
	p2.DegradationID = "py-fallback"
	if a := Plan(ErrToolUnavailable, 0, p2); a.Strategy != StratDegrade || !a.NewDigest {
		t.Fatalf("无替代有降级应降级+新 Digest，实得 %+v", a)
	}
	// 超时重试耗尽后也走替代。
	if a := Plan(ErrTimeout, 2, p); a.Strategy != StratAlternative {
		t.Fatalf("超时重试耗尽应转替代，实得 %+v", a)
	}
}
