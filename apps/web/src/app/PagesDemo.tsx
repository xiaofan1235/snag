// PagesDemo —— P0 页面预览（Day 41）。用 mock 数据演示 Dashboard/Overview/RunWizard/ActivityStream。
// 非生产接线（真实后端 daemon 无 HTTP 监听，见 PROGRESS 集成待接线清单）；页面逻辑由 Vitest 覆盖。

import { useState } from "react";
import { Dashboard, Overview, RunWizard, ActivityStream, type DashboardModel, type PreflightCheck } from "../features";
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

type Tab = "dashboard" | "overview" | "wizard" | "activity";

export function PagesDemo() {
  const [tab, setTab] = useState<Tab>("dashboard");
  const [session, setSession] = useState(false);
  return (
    <div>
      <div style={{ display: "flex", gap: "var(--sp-1)", marginBottom: "var(--sp-4)" }}>
        {(["dashboard", "overview", "wizard", "activity"] as Tab[]).map((t) => (
          <button
            key={t}
            className="sn-btn sn-focus"
            data-variant={t === tab ? "secondary" : "ghost"}
            data-size="sm"
            onClick={() => setTab(t)}
          >
            {t === "dashboard" ? "Dashboard" : t === "overview" ? "Overview" : t === "wizard" ? "Run 向导" : "Activity"}
          </button>
        ))}
      </div>
      {tab === "dashboard" && <Dashboard model={dash} onOpen={() => {}} onResume={() => {}} onPause={() => {}} onStop={() => {}} />}
      {tab === "overview" && <Overview model={overview} />}
      {tab === "wizard" && <RunWizard preflight={preflight} onStart={() => {}} onRunPreflight={() => {}} />}
      {tab === "activity" && <ActivityStream events={activity} onJump={() => {}} onOpenSession={() => setSession(true)} />}
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
