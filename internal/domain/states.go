package domain

// states.go —— 全系统状态与枚举的单一来源（对照主文档 §43.5 术语与枚举登记表）。
//
// 设计不变量：
//   - 所有状态都是「字符串枚举」，不是 int。字符串在日志、事件、SSE、报告里自解释，
//     且新增取值不会因数值错位而静默出错（§4.1 runs 字段约束、Day 01 §4）。
//   - 本文件是唯一登记处，任何模块不得再私自定义同义枚举，防止「枚举漂移」（§43.5 标题）。
//
// 逐一核对口径：若 §43.5 新增/修改取值，只改这里。

// RunState —— Run 生命周期状态（§43.5 Run 状态；+QUEUED 全局资源排队 §33）。
type RunState string

const (
	RunDraft     RunState = "DRAFT"
	RunPreflight RunState = "PREFLIGHT"
	RunRunning   RunState = "RUNNING"
	RunPausing   RunState = "PAUSING"
	RunPaused    RunState = "PAUSED"
	RunResuming  RunState = "RESUMING"
	RunBlocked   RunState = "BLOCKED"
	RunExiting   RunState = "EXITING"
	RunCompleted RunState = "COMPLETED"
	RunPartial   RunState = "PARTIAL"
	RunStopping  RunState = "STOPPING"
	RunStopped   RunState = "STOPPED"
	RunFailed    RunState = "FAILED"
	RunQueued    RunState = "QUEUED" // 全局资源排队（§33）
)

// JobState —— Job 生命周期状态（§43.5 Job 状态）。
type JobState string

const (
	JobPending         JobState = "PENDING"
	JobBlocked         JobState = "BLOCKED"
	JobReady           JobState = "READY"
	JobClaimed         JobState = "CLAIMED"
	JobRunning         JobState = "RUNNING"
	JobReconciling     JobState = "RECONCILING" // 存在未决 OUTCOME_UNKNOWN 动作时先入此态（§5.2）
	JobWaitingApproval JobState = "WAITING_APPROVAL"
	JobSubmitted       JobState = "SUBMITTED"
	JobVerifying       JobState = "VERIFYING"
	JobDone            JobState = "DONE"
	JobRetrying        JobState = "RETRYING"
	JobCancelled       JobState = "CANCELLED"
	JobFailed          JobState = "FAILED"
)

// ActionState —— 单个动作（一次具体请求/工具调用）的状态（§43.5 Action 状态）。
type ActionState string

const (
	ActionProposed        ActionState = "PROPOSED"
	ActionPolicyDenied    ActionState = "POLICY_DENIED"
	ActionWaitingApproval ActionState = "WAITING_APPROVAL"
	ActionAuthorized      ActionState = "AUTHORIZED"
	ActionDispatched      ActionState = "DISPATCHED"
	ActionExecuted        ActionState = "EXECUTED"
	ActionRecorded        ActionState = "RECORDED"
	ActionOutcomeUnknown  ActionState = "OUTCOME_UNKNOWN" // 已发出但结果未确认，需对账（§5.2）
	ActionCancelled       ActionState = "CANCELLED"
)

// FindingState —— 漏洞发现的晋级状态（§43.5 FindingState）。
type FindingState string

const (
	FindingCandidate     FindingState = "CANDIDATE"
	FindingNeedsEvidence FindingState = "NEEDS_EVIDENCE"
	FindingRejected      FindingState = "REJECTED"
	FindingConfirmed     FindingState = "CONFIRMED"
	FindingTriaged       FindingState = "TRIAGED"
	FindingReportable    FindingState = "REPORTABLE"
)

// VerificationVerdict —— 独立 Verifier 的裁决（§43.5 VerificationVerdict）。
type VerificationVerdict string

const (
	VerdictConfirmed      VerificationVerdict = "CONFIRMED"
	VerdictRejected       VerificationVerdict = "REJECTED"
	VerdictUnableToVerify VerificationVerdict = "UNABLE_TO_VERIFY"
)

// CancelCode —— 取消/终止的稳定机器码，Run 与 Job 共用（§4.1 jobs 取消语义、§43.5 cancel_code）。
// 取消只影响未来动作；已 DISPATCHED 的动作继续进入对账，不得用 CANCELLED 掩盖已发生副作用。
type CancelCode string

const (
	CancelUserPause         CancelCode = "USER_PAUSE"
	CancelUserStop          CancelCode = "USER_STOP"
	CancelPlannerSuperseded CancelCode = "PLANNER_SUPERSEDED"
	CancelBudgetExhausted   CancelCode = "BUDGET_EXHAUSTED"
	CancelAuthExpired       CancelCode = "AUTH_EXPIRED"
	CancelTimeoutDrain      CancelCode = "TIMEOUT_DRAIN"
	CancelObjectiveDone     CancelCode = "OBJECTIVE_DONE"
	CancelShutdown          CancelCode = "SHUTDOWN"
	CancelPauseRace         CancelCode = "PAUSE_RACE"
	CancelOutcomeUnresolved CancelCode = "OUTCOME_UNRESOLVED"
)

// RecoveryResult —— 暂停/崩溃后恢复一个 Run 的结果（§43.5 恢复结果）。
type RecoveryResult string

const (
	RecoveryResumedExact     RecoveryResult = "RESUMED_EXACT"
	RecoveryRebuiltFromState RecoveryResult = "REBUILT_FROM_STATE"
	RecoveryFreshPrimed      RecoveryResult = "FRESH_PRIMED"
	RecoveryAwaitingDecision RecoveryResult = "AWAITING_DECISION"
	RecoveryAttentionReq     RecoveryResult = "ATTENTION_REQUIRED"
	RecoveryFailed           RecoveryResult = "FAILED"
	RecoveryUnconfirmed      RecoveryResult = "UNCONFIRMED"
)

// KnowledgeTrust —— 知识条目的信任级（§43.5 信任级(知识)）。
// 决定一条知识能否影响判定：VERIFIED_FACT 可据以晋级，UNVERIFIED_REFERENCE 仅作线索。
type KnowledgeTrust string

const (
	TrustVerifiedFact        KnowledgeTrust = "VERIFIED_FACT"
	TrustOperatorNote        KnowledgeTrust = "OPERATOR_NOTE"
	TrustUnverifiedReference KnowledgeTrust = "UNVERIFIED_REFERENCE"
)

// EgressMode —— 目标流量出口模式（§43.5 egress 模式、§38）。
type EgressMode string

const (
	EgressProxyPool EgressMode = "proxy_pool"
	EgressLocal     EgressMode = "local"
)

// EgressChannel —— 全系统四类固定出口通道，互不混用 Scope（§43.6）。
type EgressChannel string

const (
	ChannelModelAPI EgressChannel = "model_api" // 调模型 Provider
	ChannelTarget   EgressChannel = "target"    // 目标流量（经 Policy Proxy + 代理池/本机）
	ChannelUtility  EgressChannel = "utility"   // 非目标工具通道：知识联网 + 远程 MCP + 其它非目标 HTTP
	ChannelOAST     EgressChannel = "oast"      // 带外交互服务
)

// EnforcementCompleteness —— 某条执行路径的「执法完备度」（§43.5 执法完备度）。
// 报告导出时据此标注证据可信度：只有 FULL 才能无保留晋级。
type EnforcementCompleteness string

const (
	EnforcementFull    EnforcementCompleteness = "FULL"
	EnforcementPartial EnforcementCompleteness = "PARTIAL"
	EnforcementNone    EnforcementCompleteness = "NONE"
	EnforcementUnknown EnforcementCompleteness = "UNKNOWN"
)

// FindingEvidenceRole —— 证据在某条 Finding 中扮演的角色（§4.1 finding_evidence.role）。
type FindingEvidenceRole string

const (
	EvidenceBaseline        FindingEvidenceRole = "baseline"
	EvidenceVariant         FindingEvidenceRole = "variant"
	EvidenceOwnership       FindingEvidenceRole = "ownership"
	EvidenceImpact          FindingEvidenceRole = "impact"
	EvidenceReplay          FindingEvidenceRole = "replay"
	EvidenceScreenshot      FindingEvidenceRole = "screenshot"
	EvidenceCleanup         FindingEvidenceRole = "cleanup"
	EvidenceCounterEvidence FindingEvidenceRole = "counterevidence"
)

// 以下字段在 §43.5 尚未登记固定取值集合，按 §43.8「别空想，实现期按真实数据细化」的
// 原则，先声明为开放字符串类型（命名以便后续集中登记），不预埋可能漂移的常量：
//   - AttemptState    attempts.state
//   - SessionState    agent_sessions.state
//   - HypothesisState hypotheses.state
//   - RiskClass       actions.risk_class
//   - IdempotencyClass actions.idempotency_class（仅语义：幂等/非幂等决定对账方式 §4.1）
//   - DataClass       workspaces.data_classification
type (
	AttemptState     string
	SessionState     string
	HypothesisState  string
	RiskClass        string
	IdempotencyClass string
	DataClass        string
)

// IdempotencyClass 取值（§4.1/§967：决定 OUTCOME_UNKNOWN 的对账与可否重发）。
// §43.8 实现期细化：Day 14 落地这三类。
const (
	IdempotencyReadOnly      IdempotencyClass = "READ_ONLY"      // 只读：失败可直接重试
	IdempotencyIdempotent    IdempotencyClass = "IDEMPOTENT"     // 幂等写：重试须沿用目标侧 Idempotency Key
	IdempotencyNonIdempotent IdempotencyClass = "NON_IDEMPOTENT" // 非幂等写：未对账前禁止增代重发
)

// PermitState —— Action Permit 状态（§9.5，action_permits.state）。Day 17 落地。
type PermitState string

const (
	PermitIssued   PermitState = "ISSUED"   // 已签发，可消费一次
	PermitConsumed PermitState = "CONSUMED" // 已消费（一次性，§4.3 #4）
	PermitRevoked  PermitState = "REVOKED"  // 已撤销（如 Attempt 租约过期）
	PermitExpired  PermitState = "EXPIRED"  // 已过期
)

// AttemptState 取值——§43.8「实现期按真实数据细化」：Day 09 Claim/Lease/Fencing 实际用到的
// 子集，随后续天数需要再补（待 §43.5 集中登记）。不是穷举，仅登记已落地的。
const (
	AttemptRunning   AttemptState = "RUNNING"   // 活跃尝试：刚被 Claim / 执行中（持当代 fencing_token）
	AttemptSubmitted AttemptState = "SUBMITTED" // 结果已提交，待验证
	AttemptExpired   AttemptState = "EXPIRED"   // 租约过期被取代：其 fencing_token 失效，写入一律被拒
)
