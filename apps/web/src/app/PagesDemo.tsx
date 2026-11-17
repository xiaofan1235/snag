// PagesDemo —— P0 页面预览（Day 41）。用 mock 数据演示 Dashboard/Overview/RunWizard/ActivityStream。
// 非生产接线（真实后端 daemon 无 HTTP 监听，见 PROGRESS 集成待接线清单）；页面逻辑由 Vitest 覆盖。

import { useState } from "react";
import {
  Dashboard,
  Overview,
  RunWizard,
  ActivityStream,
  FindingDetail,
  ApprovalCenter,
  CoverageMatrix,
  type DashboardModel,
  type PreflightCheck,
  type FindingModel,
  type ApprovalItem,
  type CoverageModel,
} from "../features";
import { Drawer } from "../components";
import { AgentSessionPanel } from "../features/activity/ActivityStream";
import type { EventEnvelope } from "../api/schema.gen";

const dash: DashboardModel = {
  attention: [{ id: "a1", kind: "approval", label: "等待审批：写操作（run-1）", runId: "run-1" }],
  running: [
    { runId: "run-1", name: "acme.com 授权测试", state: "RUNNING", strategy: "authz", activeJobs: 3, lastAction: "GET /api/orders/42", requestsUsed: 120, requestsMax: 500 },
    { runId: "run-2", name: "staging 复测", state: "BLOCKED", strategy: "recon", activeJobs: 0, requestsUsed: 80, requestsMax: 500 },
    { runId: "run-3", name: "历史 Run", state: "COMPLETED", strategy: "-", activeJobs: 0, requestsUsed: 500, requestsMax: 500 },
  ],
  recent: [
    { id: "r1", label: "新确认 Finding：IDOR on /orders", kind: "finding" },
    { id: "r2", label: "报告生成完成（run-3）", kind: "report" },
  ],
  health: { provider: "RUNNING", egress: "RUNNING", sandbox: "RUNNING", database: "RUNNING", blobStore: "RUNNING", eventStream: "RUNNING" },
};

const overview = {
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
  blockers: ["等待审批：写操作"],
  nextPlan: "recon 新端点 /api/v2",
};

const preflight: PreflightCheck[] = [
  { id: "scope", label: "Scope 规范化合法", ok: true },
  { id: "auth", label: "授权材料引用存在", ok: true },
  { id: "identity", label: "身份有效（SecretRef 可解析）", ok: true },
  { id: "budget", label: "预算与停止条件已设", ok: true },
];

const activity: EventEnvelope[] = [
  { event_type: "plan.updated", run_id: "run-1", sequence: 101 },
  { event_type: "action.updated", run_id: "run-1", sequence: 102 },
  { event_type: "observation.created", run_id: "run-1", sequence: 103 },
  { event_type: "approval.requested", run_id: "run-1", sequence: 104 },
];

const finding: FindingModel = {
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
  verifierOpinion: "确认跨身份",
  severityRationale: "机密性高影响（CVSS 7.5）",
  cleanupStatus: "无需清理",
  citations: [{ evidenceId: "ev-1", content: "0123456789SECRETDATA0123456789" }],
};
const approvals: ApprovalItem[] = [
  {
    id: "ap-1",
    revision: 2,
    actionDigest: "digest-abc",
    normalAction: "GET /api/orders/42",
    pendingAction: "DELETE /api/orders/42",
    identity: "账号A",
    object: "order:42",
    sideEffects: ["删除订单"],
    policyReason: "写操作需审批",
    expiresAt: "12:30",
    state: "PENDING",
    highRisk: true,
  },
];
const coverage: CoverageModel = {
  surfaceRevision: 7,
  cells: [
    { endpoint: "/api/orders", attackType: "IDOR", identity: "账号A", state: "TESTED", actionId: "act-1", evidenceId: "ev-1" },
    { endpoint: "/api/orders", attackType: "SQLI", identity: "账号A", state: "UNTESTED", untestedReason: "无注入点" },
    { endpoint: "/api/login", attackType: "IDOR", identity: "账号A", state: "NA", untestedReason: "无对象归属" },
  ],
};

type Tab = "dashboard" | "overview" | "wizard" | "activity" | "finding" | "approval" | "coverage";
const TABS: Array<[Tab, string]> = [
  ["dashboard", "Dashboard"],
  ["overview", "Overview"],
  ["wizard", "Run 向导"],
  ["activity", "Activity"],
  ["finding", "Finding"],
  ["approval", "审批"],
  ["coverage", "覆盖率"],
];

export function PagesDemo() {
  const [tab, setTab] = useState<Tab>("dashboard");
  const [session, setSession] = useState(false);
  return (
    <div>
      <div style={{ display: "flex", gap: "var(--sp-1)", marginBottom: "var(--sp-4)", flexWrap: "wrap" }}>
        {TABS.map(([t, label]) => (
          <button
            key={t}
            className="sn-btn sn-focus"
            data-variant={t === tab ? "secondary" : "ghost"}
            data-size="sm"
            onClick={() => setTab(t)}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === "dashboard" && <Dashboard model={dash} onOpen={() => {}} onResume={() => {}} onPause={() => {}} onStop={() => {}} />}
      {tab === "overview" && <Overview model={overview} />}
      {tab === "wizard" && <RunWizard preflight={preflight} onStart={() => {}} onRunPreflight={() => {}} />}
      {tab === "activity" && <ActivityStream events={activity} onJump={() => {}} onOpenSession={() => setSession(true)} />}
      {tab === "finding" && <FindingDetail model={finding} onReplay={() => {}} onJump={() => {}} />}
      {tab === "approval" && <ApprovalCenter items={approvals} onApprove={() => {}} onDeny={() => {}} onDenyAndCancel={() => {}} />}
      {tab === "coverage" && <CoverageMatrix model={coverage} onDrillDown={() => {}} />}
      <Drawer open={session} onClose={() => setSession(false)} title="Agent Session">
        <AgentSessionPanel
          model={{
            sessionId: "s1",
            role: "executor",
            modelProfile: "default",
            jobId: "job-1",
            attempt: 2,
            generation: 1,
            contextRevision: 5,
            compactions: 1,
            tokenUsed: 1200,
            tools: ["http", "browser"],
            lastInputSummary: "最近输入摘要（Redacted Projection）",
          }}
        />
      </Drawer>
    </div>
  );
}
