// mockData.ts —— 预览夹具（daemon 接线前）。真实数据由 api client + useRunStream 提供（§16.33）；
// 当前 daemon 无 HTTP 监听（见 PROGRESS「集成待接线清单」），故应用外壳以预览数据演示导航与布局，
// 未实现/无数据的页走只读空态（§16.34「不用假数据冒充完成」）。标记 preview，接线后替换为真实查询。

import type { DashboardModel, OverviewModel, ApprovalItem, CoverageModel, FindingModel, ReportModel, RecoveryPreviewModel } from "../features";
import type { EventEnvelope } from "../api/schema.gen";

export const previewDashboard: DashboardModel = {
  attention: [
    { id: "a1", kind: "approval", label: "等待审批：DELETE /api/orders/42（run-1）", runId: "run-1" },
    { id: "a2", kind: "budget_low", label: "run-1 请求预算已用 78%", runId: "run-1" },
  ],
  running: [
    { runId: "run-1", name: "acme.com 授权测试", state: "RUNNING", strategy: "authz", activeJobs: 3, lastAction: "GET /api/orders/42", requestsUsed: 120, requestsMax: 500 },
    { runId: "run-2", name: "staging 复测", state: "BLOCKED", strategy: "recon", activeJobs: 0, requestsUsed: 80, requestsMax: 500 },
    { runId: "run-3", name: "历史 Run（11-10）", state: "COMPLETED", strategy: "-", activeJobs: 0, requestsUsed: 500, requestsMax: 500 },
  ],
  recent: [
    { id: "r1", label: "新确认 Finding：IDOR on /orders（High）", kind: "finding" },
    { id: "r2", label: "报告生成完成（run-3）", kind: "report" },
    { id: "r3", label: "覆盖门禁变化：authz 模块 TESTED 12/30", kind: "coverage" },
  ],
  health: { provider: "RUNNING", egress: "RUNNING", sandbox: "RUNNING", database: "RUNNING", blobStore: "RUNNING", eventStream: "RUNNING" },
};

export const previewRuns = previewDashboard.running;

export const previewOverview: OverviewModel = {
  state: "RUNNING",
  strategyEpoch: 3,
  priorityDirection: "authz",
  activeAgents: 2,
  waitingJobs: 5,
  surfaceRevision: 7,
  experiments: { done: 4, total: 10 },
  coverage: { tested: 12, eligible: 30 },
  verification: { confirmed: 2, candidate: 6 },
  budget: { requestsUsed: 120, requestsMax: 500, costUsedCents: 80, costMaxCents: 500 },
  blockers: ["等待审批：写操作 DELETE /api/orders/42"],
  nextPlan: "recon 新端点 /api/v2/*",
  recentHighValue: ["账号A 读到账号B 订单 42"],
};

export const previewActivity: EventEnvelope[] = [
  { event_type: "plan.updated", run_id: "run-1", sequence: 101 },
  { event_type: "action.updated", run_id: "run-1", sequence: 102 },
  { event_type: "observation.created", run_id: "run-1", sequence: 103 },
  { event_type: "finding.updated", run_id: "run-1", sequence: 104 },
  { event_type: "approval.requested", run_id: "run-1", sequence: 105 },
  { event_type: "budget.updated", run_id: "run-1", sequence: 106 },
];

export const previewFinding: FindingModel = {
  id: "f-1",
  title: "IDOR on /orders",
  vulnType: "IDOR",
  severity: "High",
  state: "REPORTABLE",
  affectedEndpoints: ["/api/orders/{id}"],
  identityObjectRelation: "账号A 访问 账号B 的订单",
  facts: [{ id: "fa1", text: "账号A 读取到账号B 订单 42 的 JSON", evidenceId: "ev-1", byteStart: 10, byteEnd: 20 }],
  inferences: [{ id: "in1", text: "推断：对象归属校验缺失" }],
  baseline: "HTTP/1.1 403 Forbidden\nX-Owner: B",
  variant: "HTTP/1.1 200 OK\nX-Owner: B",
  verifierOpinion: "独立 Verifier 确认跨身份",
  severityRationale: "机密性高影响（CVSS 7.5 High）",
  cleanupStatus: "无需清理",
  citations: [{ evidenceId: "ev-1", content: "0123456789SECRETDATA account B order 42" }],
};

export const previewApprovals: ApprovalItem[] = [
  {
    id: "ap-1",
    revision: 2,
    actionDigest: "digest-abc",
    normalAction: "GET /api/orders/42",
    pendingAction: "DELETE /api/orders/42",
    identity: "账号A",
    object: "order:42",
    sideEffects: ["删除订单"],
    policyReason: "写操作需审批（§6.3）",
    expiresAt: "12:30",
    state: "PENDING",
    highRisk: true,
  },
];

export const previewCoverage: CoverageModel = {
  surfaceRevision: 7,
  cells: [
    { endpoint: "/api/orders", attackType: "IDOR", identity: "账号A", state: "TESTED", actionId: "act-1", evidenceId: "ev-1" },
    { endpoint: "/api/orders", attackType: "SQLI", identity: "账号A", state: "UNTESTED", untestedReason: "无注入点" },
    { endpoint: "/api/login", attackType: "IDOR", identity: "账号A", state: "NA", untestedReason: "无对象归属" },
    { endpoint: "/api/login", attackType: "AUTH", identity: "账号A", state: "BLOCKED", untestedReason: "需审批" },
  ],
};

export const previewReport: ReportModel = {
  id: "rep-1",
  state: "FINALIZED",
  reportable: [{ id: "f-1", title: "IDOR on /orders", severity: "High" }],
  excluded: [{ id: "f-2", title: "可能的 XSS", state: "UNABLE_TO_VERIFY", reason: "零交互要求未满足" }],
  exitGateResult: "PASSED",
  coverageSnapshot: "rev 7",
  partialReasons: [],
  markdownPreview:
    "# 渗透测试报告\n\n## 1. IDOR on /orders\n\n账号A 可读取账号B 的订单 42（跨身份确认）。\n\n### 复现步骤 / 证据引用\n\n- Evidence ev-1 bytes[10:20] sha256=…\n\n### 修复建议\n\n服务端按 session 身份校验对象归属。\n",
};

export const previewRecovery: RecoveryPreviewModel = {
  unfinishedJobs: 2,
  outcomeUnknownActions: 1,
  expiredIdentities: ["账号A"],
  staleProxies: [],
  policyRevisionChanged: true,
  schemaCompatible: true,
};

// 顶部状态条预览。
export const previewAgentStatus = {
  doing: "实验 /api/orders IDOR 变体",
  epoch: 3,
  endurancePct: 24, // 续航：已用预算%
  proxy: "RUNNING",
  runState: "RUNNING",
};
