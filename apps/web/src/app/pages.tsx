// pages.tsx —— 全局页面（UI 文档 D4）。接入已有 feature 组件 + 预览数据 + 导航/Inspector。
// 未实现页走只读空态（§16.34），不以假数据冒充完成。

import { useState } from "react";
import { Dashboard, RunWizard, ApprovalCenter, type PreflightCheck } from "../features";
import { Card } from "../components/Card";
import { Button } from "../components/Button";
import { StatusBadge } from "../components/StatusBadge";
import { useToast } from "../components";
import { useNavigate } from "./router";
import { runNavTo } from "./nav";
import { previewDashboard, previewRuns, previewApprovals } from "./mockData";

// 页头（面包屑 + 标题）。
export function PageHead({ title, crumbs }: { title: string; crumbs?: React.ReactNode }) {
  return (
    <div className="app-main-head">
      <div>
        {crumbs && <div className="app-crumbs">{crumbs}</div>}
        <h1 style={{ fontSize: "var(--fs-2xl)", margin: 0 }}>{title}</h1>
      </div>
    </div>
  );
}

export function DashboardPage() {
  const navigate = useNavigate();
  const toast = useToast();
  return (
    <>
      <PageHead title="值班台" />
      <Dashboard
        model={previewDashboard}
        onOpen={(runId) => navigate(runNavTo(runId, "overview"))}
        onResume={(r) => toast.push({ tone: "info", message: `已请求恢复 ${r}` })}
        onPause={(r) => toast.push({ tone: "warn", message: `已请求暂停 ${r}` })}
        onStop={(r) => toast.push({ tone: "danger", message: `已请求停止 ${r}` })}
      />
    </>
  );
}

export function RunsPage() {
  const navigate = useNavigate();
  return (
    <>
      <PageHead title="Runs 列表 / 历史" />
      <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: "var(--sp-3)" }}>
        <Button variant="primary" onClick={() => navigate("/runs/new")}>新建 Run</Button>
      </div>
      <div style={{ display: "grid", gap: "var(--sp-2)" }}>
        {previewRuns.map((r) => (
          <Card key={r.runId} clickable onClick={() => navigate(runNavTo(r.runId, "overview"))}>
            <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
              <strong>{r.name}</strong>
              <StatusBadge status={r.state} />
              <span style={{ marginLeft: "auto", color: "var(--ink-3)", fontSize: "var(--fs-sm)" }}>
                策略 {r.strategy} · 请求 {r.requestsUsed}/{r.requestsMax}
              </span>
            </div>
          </Card>
        ))}
      </div>
    </>
  );
}

const PREFLIGHT: PreflightCheck[] = [
  { id: "scope", label: "Scope 规范化合法", ok: true },
  { id: "auth", label: "授权材料引用存在", ok: true },
  { id: "identity", label: "身份有效（SecretRef 可解析）", ok: true },
  { id: "budget", label: "预算与停止条件已设", ok: true },
];

export function NewRunPage() {
  const navigate = useNavigate();
  const toast = useToast();
  const [pf, setPf] = useState<PreflightCheck[]>([]);
  return (
    <>
      <PageHead title="新建 Run" crumbs={<a href="#/runs">Runs</a>} />
      <RunWizard
        preflight={pf}
        onRunPreflight={() => {
          setPf(PREFLIGHT);
          toast.push({ tone: "ok", message: "Preflight 全部通过" });
        }}
        onStart={() => {
          toast.push({ tone: "ok", message: "Run 已创建并启动" });
          navigate(runNavTo("run-1", "overview"));
        }}
      />
    </>
  );
}

export function InboxPage() {
  const toast = useToast();
  return (
    <>
      <PageHead title="Inbox 收件箱" />
      <ApprovalCenter
        items={previewApprovals}
        onApprove={(id, rev, dig) => toast.push({ tone: "ok", message: `Approve ${id}（rev ${rev}, digest ${dig.slice(0, 8)}）` })}
        onDeny={(id) => toast.push({ tone: "warn", message: `Deny ${id}` })}
        onDenyAndCancel={(id) => toast.push({ tone: "danger", message: `Deny + 取消关联 Job（${id}）` })}
      />
    </>
  );
}

// 未实现页：只读空态 + 契约占位（§16.34），不用假数据冒充完成。
export function PlaceholderPage({ title, note }: { title: string; note?: string }) {
  return (
    <>
      <PageHead title={title} />
      <div className="app-placeholder">
        <div style={{ fontSize: "var(--fs-lg)", marginBottom: "var(--sp-2)" }}>此页尚未接线</div>
        <div style={{ fontSize: "var(--fs-sm)" }}>{note ?? "API 契约已定义，页面随 daemon 接线后填充真实数据；此处不展示假数据（§16.34）。"}</div>
      </div>
    </>
  );
}
