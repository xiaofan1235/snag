package domain_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"pentagent/internal/domain"
)

// domain_test.go —— Day 01 §7 验收测试。
// 命令：go test ./internal/domain/...
// 对照：主文档 §43.5（枚举逐一核对）、§4.1（聚合可构造）、Day 01 DoD。

// --- 枚举：对照 §43.5 逐一核对，不缺 ---

func TestEnumValues(t *testing.T) {
	// 每组：实际常量 -> 期望字符串；并校验集合与 §43.5 完全一致（不多不少）。
	cases := []struct {
		name string
		got  []string
		want []string
	}{
		{"RunState", []string{
			string(domain.RunDraft), string(domain.RunPreflight), string(domain.RunRunning),
			string(domain.RunPausing), string(domain.RunPaused), string(domain.RunResuming),
			string(domain.RunBlocked), string(domain.RunExiting), string(domain.RunCompleted),
			string(domain.RunPartial), string(domain.RunStopping), string(domain.RunStopped),
			string(domain.RunFailed), string(domain.RunQueued),
		}, []string{
			"DRAFT", "PREFLIGHT", "RUNNING", "PAUSING", "PAUSED", "RESUMING", "BLOCKED",
			"EXITING", "COMPLETED", "PARTIAL", "STOPPING", "STOPPED", "FAILED", "QUEUED",
		}},
		{"JobState", []string{
			string(domain.JobPending), string(domain.JobBlocked), string(domain.JobReady),
			string(domain.JobClaimed), string(domain.JobRunning), string(domain.JobReconciling),
			string(domain.JobWaitingApproval), string(domain.JobSubmitted), string(domain.JobVerifying),
			string(domain.JobDone), string(domain.JobRetrying), string(domain.JobCancelled),
			string(domain.JobFailed),
		}, []string{
			"PENDING", "BLOCKED", "READY", "CLAIMED", "RUNNING", "RECONCILING", "WAITING_APPROVAL",
			"SUBMITTED", "VERIFYING", "DONE", "RETRYING", "CANCELLED", "FAILED",
		}},
		{"ActionState", []string{
			string(domain.ActionProposed), string(domain.ActionPolicyDenied), string(domain.ActionWaitingApproval),
			string(domain.ActionAuthorized), string(domain.ActionDispatched), string(domain.ActionExecuted),
			string(domain.ActionRecorded), string(domain.ActionOutcomeUnknown), string(domain.ActionCancelled),
		}, []string{
			"PROPOSED", "POLICY_DENIED", "WAITING_APPROVAL", "AUTHORIZED", "DISPATCHED",
			"EXECUTED", "RECORDED", "OUTCOME_UNKNOWN", "CANCELLED",
		}},
		{"FindingState", []string{
			string(domain.FindingCandidate), string(domain.FindingNeedsEvidence), string(domain.FindingRejected),
			string(domain.FindingConfirmed), string(domain.FindingTriaged), string(domain.FindingReportable),
		}, []string{
			"CANDIDATE", "NEEDS_EVIDENCE", "REJECTED", "CONFIRMED", "TRIAGED", "REPORTABLE",
		}},
		{"VerificationVerdict", []string{
			string(domain.VerdictConfirmed), string(domain.VerdictRejected), string(domain.VerdictUnableToVerify),
		}, []string{"CONFIRMED", "REJECTED", "UNABLE_TO_VERIFY"}},
		{"CancelCode", []string{
			string(domain.CancelUserPause), string(domain.CancelUserStop), string(domain.CancelPlannerSuperseded),
			string(domain.CancelBudgetExhausted), string(domain.CancelAuthExpired), string(domain.CancelTimeoutDrain),
			string(domain.CancelObjectiveDone), string(domain.CancelShutdown), string(domain.CancelPauseRace),
			string(domain.CancelOutcomeUnresolved),
		}, []string{
			"USER_PAUSE", "USER_STOP", "PLANNER_SUPERSEDED", "BUDGET_EXHAUSTED", "AUTH_EXPIRED",
			"TIMEOUT_DRAIN", "OBJECTIVE_DONE", "SHUTDOWN", "PAUSE_RACE", "OUTCOME_UNRESOLVED",
		}},
		{"RecoveryResult", []string{
			string(domain.RecoveryResumedExact), string(domain.RecoveryRebuiltFromState), string(domain.RecoveryFreshPrimed),
			string(domain.RecoveryAwaitingDecision), string(domain.RecoveryAttentionReq), string(domain.RecoveryFailed),
			string(domain.RecoveryUnconfirmed),
		}, []string{
			"RESUMED_EXACT", "REBUILT_FROM_STATE", "FRESH_PRIMED", "AWAITING_DECISION",
			"ATTENTION_REQUIRED", "FAILED", "UNCONFIRMED",
		}},
		{"KnowledgeTrust", []string{
			string(domain.TrustVerifiedFact), string(domain.TrustOperatorNote), string(domain.TrustUnverifiedReference),
		}, []string{"VERIFIED_FACT", "OPERATOR_NOTE", "UNVERIFIED_REFERENCE"}},
		{"EgressMode", []string{
			string(domain.EgressProxyPool), string(domain.EgressLocal),
		}, []string{"proxy_pool", "local"}},
		{"EgressChannel", []string{
			string(domain.ChannelModelAPI), string(domain.ChannelTarget), string(domain.ChannelUtility), string(domain.ChannelOAST),
		}, []string{"model_api", "target", "utility", "oast"}},
		{"EnforcementCompleteness", []string{
			string(domain.EnforcementFull), string(domain.EnforcementPartial), string(domain.EnforcementNone), string(domain.EnforcementUnknown),
		}, []string{"FULL", "PARTIAL", "NONE", "UNKNOWN"}},
		{"FindingEvidenceRole", []string{
			string(domain.EvidenceBaseline), string(domain.EvidenceVariant), string(domain.EvidenceOwnership),
			string(domain.EvidenceImpact), string(domain.EvidenceReplay), string(domain.EvidenceScreenshot),
			string(domain.EvidenceCleanup), string(domain.EvidenceCounterEvidence),
		}, []string{
			"baseline", "variant", "ownership", "impact", "replay", "screenshot", "cleanup", "counterevidence",
		}},
	}
	for _, c := range cases {
		if len(c.got) != len(c.want) {
			t.Errorf("%s: 常量个数 %d != 期望 %d", c.name, len(c.got), len(c.want))
			continue
		}
		for i := range c.got {
			if c.got[i] != c.want[i] {
				t.Errorf("%s[%d]: 得到 %q, 期望 %q", c.name, i, c.got[i], c.want[i])
			}
		}
	}
}

// --- ID：UUIDv7 格式、唯一、单调 ---

func isUUIDShape(s string) bool {
	if len(s) != 36 {
		return false
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isHex {
			return false
		}
	}
	return true
}

func TestNewIDShapeAndVersion(t *testing.T) {
	id := domain.NewID().String()
	if !isUUIDShape(id) {
		t.Fatalf("NewID 形状非法: %q", id)
	}
	if id[14] != '7' { // 版本位（UUIDv7）
		t.Errorf("NewID 版本位应为 7, 得到 %c (id=%s)", id[14], id)
	}
	if v := id[19]; v != '8' && v != '9' && v != 'a' && v != 'b' {
		t.Errorf("NewID 变体位应为 8/9/a/b, 得到 %c", v)
	}
}

func TestNewIDUniqueAndMonotonic(t *testing.T) {
	const n = 5000
	seen := make(map[string]struct{}, n)
	prev := ""
	for i := 0; i < n; i++ {
		id := domain.NewID().String()
		if _, dup := seen[id]; dup {
			t.Fatalf("第 %d 个 id 重复: %s", i, id)
		}
		seen[id] = struct{}{}
		if prev != "" && id <= prev {
			t.Fatalf("id 非单调: 第 %d 个 %s <= 前一个 %s", i, id, prev)
		}
		prev = id
	}
}

func TestKeyDeterministic(t *testing.T) {
	a := domain.Key("run-1", "GET", "https://t/x")
	b := domain.Key("run-1", "GET", "https://t/x")
	c := domain.Key("run-1", "GET", "https://t/y")
	if a != b {
		t.Errorf("Key 对相同输入应相同: %s != %s", a, b)
	}
	if a == c {
		t.Errorf("Key 对不同输入应不同, 却都等于 %s", a)
	}
	if !isUUIDShape(a.String()) {
		t.Errorf("Key 形状非法: %s", a)
	}
	if a.String()[14] != '8' { // 版本 8：确定性/自定义
		t.Errorf("Key 版本位应为 8, 得到 %c", a.String()[14])
	}
	// 分隔符防碰撞：["a","bc"] 与 ["ab","c"] 不应相同
	if domain.Key("a", "bc") == domain.Key("ab", "c") {
		t.Error("Key 分隔符未能防止拼接碰撞")
	}
}

// --- 构造器与零值 ---

func TestConstructors(t *testing.T) {
	ws := domain.NewWorkspace("acme", "mpk-123")
	if ws.ID.IsZero() || ws.CreatedAt.IsZero() {
		t.Error("NewWorkspace 应填充 ID 与 CreatedAt")
	}

	r := domain.NewRun(ws.ID, "run-a", "tester", "digest-abc")
	if r.State != domain.RunDraft {
		t.Errorf("NewRun 初态应为 DRAFT, 得到 %s", r.State)
	}
	if r.Revision != 1 {
		t.Errorf("NewRun Revision 应为 1, 得到 %d", r.Revision)
	}
	if r.AuthorizationScopeDigest != "digest-abc" {
		t.Error("NewRun 应保留 scopeDigest")
	}
	if r.WorkspaceID != ws.ID || r.ID.IsZero() || r.CreatedAt.IsZero() {
		t.Error("NewRun 字段填充不完整")
	}

	j := domain.NewJob(r.ID, "recon", "枚举子域")
	if j.State != domain.JobPending || j.Generation != 1 || j.Revision != 1 {
		t.Errorf("NewJob 初值应为 PENDING/gen1/rev1, 得到 %s/%d/%d", j.State, j.Generation, j.Revision)
	}
}

// 零值构造各聚合不 panic，且能 JSON 序列化（Day 01 §7）。
func TestZeroValuesNoPanic(t *testing.T) {
	vals := []any{
		domain.Workspace{}, domain.Run{}, domain.PolicyRevision{}, domain.AgentSession{},
		domain.Job{}, domain.Attempt{}, domain.Action{}, domain.Evidence{},
		domain.Hypothesis{}, domain.Finding{}, domain.FindingEvidence{},
		domain.CoverageCell{}, domain.Event{},
	}
	for i, v := range vals {
		if _, err := json.Marshal(v); err != nil {
			t.Errorf("第 %d 个聚合零值 JSON 序列化失败: %v", i, err)
		}
	}
}

// Run 序列化应为 snake_case 列名，便于事件/API 映射。
func TestRunJSONSnakeCase(t *testing.T) {
	r := domain.NewRun("ws", "run-a", "tester", "d")
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{`"workspace_id"`, `"authorization_scope_digest"`, `"state":"DRAFT"`} {
		if !strings.Contains(s, key) {
			t.Errorf("Run JSON 缺少 %s: %s", key, s)
		}
	}
}

// --- 领域错误 ---

func TestDomainError(t *testing.T) {
	if domain.ErrNotFound.Error() != "NOT_FOUND: entity not found" {
		t.Errorf("Error() 文本异常: %s", domain.ErrNotFound.Error())
	}
	// Errorf 携带上下文但仍按 Code 匹配
	wrapped := domain.Errorf(domain.ErrNotFound, "run xyz")
	if !errors.Is(wrapped, domain.ErrNotFound) {
		t.Error("errors.Is 应按 Code 匹配 ErrNotFound")
	}
	if errors.Is(wrapped, domain.ErrConflict) {
		t.Error("不同 Code 不应匹配")
	}
	if wrapped.Error() != "NOT_FOUND: run xyz" {
		t.Errorf("Errorf 文本异常: %s", wrapped.Error())
	}
}
