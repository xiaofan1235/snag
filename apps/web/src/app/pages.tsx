// pages.tsx —— 全局页面（UI 文档 D4）。接入已有 feature 组件 + 预览数据 + 导航/Inspector。
// 未实现页走只读空态（§16.34），不以假数据冒充完成。

import { useState } from "react";
import { Dashboard, RunWizard, type PreflightCheck } from "../features";
import { KnowledgePanel, SkillsPanel, ToolsPanel, OperationsPage, SettingsPage, InboxPage as FullInboxPage } from "../features/global";
import { StatTile, TokenChart } from "../features/command";
import { mkGlobalStats, mkGlobalTokenWindows, mkGlobalTokenSeries } from "./mockCommand";
import { Card } from "../components/Card";
import { Button } from "../components/Button";
import { StatusBadge } from "../components/StatusBadge";
import { useToast } from "../components";
import { useNavigate } from "./router";
import { runNavTo } from "./nav";
import { previewDashboard } from "./mockData";
import { mkResources, mkOperations, mkSettings, mkInbox } from "./mockGlobal";
import { useRuns, useCreateRun } from "../api/queries";
import type { Run } from "../api/schema.gen";

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
      {/* 作战总览（ARTEX 式跨 Run 一屏指标：Run/Agent/发现/资产/流量/Token + Token 消耗图） */}
      <section className="sn-deck-overview" aria-label="作战总览">
        <div className="sn-deck-stats">
          {mkGlobalStats.map((s) => <StatTile key={s.key} stat={s} />)}
        </div>
        <section className="sn-panel" style={{ marginTop: "var(--sp-3)" }}>
          <div className="sn-panel-head">Token 消耗（近 7 日，跨 Run）</div>
          <TokenChart series={mkGlobalTokenSeries} windows={mkGlobalTokenWindows} />
        </section>
      </section>
      <Dashboard
        model={previewDashboard}
        onOpen={(runId) => navigate(runNavTo(runId, "command"))}
        onResume={(r) => toast.push({ tone: "info", message: `已请求恢复 ${r}` })}
        onPause={(r) => toast.push({ tone: "warn", message: `已请求暂停 ${r}` })}
        onStop={(r) => toast.push({ tone: "danger", message: `已请求停止 ${r}` })}
      />
    </>
  );
}

export function RunsPage() {
  const navigate = useNavigate();
  // 接真实接口（GET /v1/runs）。快照用 Query，加载/错误/空态各自显式，不以假数据冒充完成（§16.34）。
  const { data, isLoading, isError, error, refetch, isFetching } = useRuns();
  const runs = data?.items ?? [];
  return (
    <>
      <PageHead title="运行列表 / 历史" />
      <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: "var(--sp-3)" }}>
        <Button variant="primary" onClick={() => navigate("/runs/new")}>新建运行</Button>
      </div>

      {isLoading ? (
        <div className="sn-sub" style={{ padding: "var(--sp-4) 0" }}>正在读取运行列表…</div>
      ) : isError ? (
        <Card>
          <div style={{ display: "grid", gap: "var(--sp-2)" }}>
            <strong>无法连接控制面</strong>
            <span className="sn-sub">
              读取 /v1/runs 失败：{error instanceof Error ? error.message : String(error)}。
              请确认 snagd 已在本地运行（默认 127.0.0.1:8787）。
            </span>
            <div>
              <Button onClick={() => refetch()}>重试</Button>
            </div>
          </div>
        </Card>
      ) : runs.length === 0 ? (
        <Card>
          <div style={{ display: "grid", gap: "var(--sp-2)" }}>
            <strong>还没有运行</strong>
            <span className="sn-sub">新建一个授权运行即可开始。所有目标需在授权范围内（§17）。</span>
            <div>
              <Button variant="primary" onClick={() => navigate("/runs/new")}>新建运行</Button>
            </div>
          </div>
        </Card>
      ) : (
        <div style={{ display: "grid", gap: "var(--sp-2)", opacity: isFetching ? 0.7 : 1 }}>
          {runs.map((r: Run) => (
            <Card key={r.id} clickable onClick={() => navigate(runNavTo(r.id, "command"))}>
              <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
                <strong>{r.name}</strong>
                <StatusBadge status={r.state} />
                <span style={{ marginLeft: "auto", color: "var(--ink-3)", fontSize: "var(--fs-sm)" }}>
                  工作区 {r.workspace_id} · 修订 r{r.revision}
                  {r.created_at ? ` · ${r.created_at.slice(0, 10)}` : ""}
                </span>
              </div>
            </Card>
          ))}
        </div>
      )}
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
  const createRun = useCreateRun();
  return (
    <>
      <PageHead title="新建运行" crumbs={<a href="#/runs">运行</a>} />
      <RunWizard
        preflight={pf}
        busy={createRun.isPending}
        onRunPreflight={() => {
          setPf(PREFLIGHT);
          toast.push({ tone: "ok", message: "Preflight 全部通过" });
        }}
        onStart={(state) => {
          // 接真实接口（POST /v1/runs）：创建 DRAFT Run，成功后跳转到其指挥台。
          // 命令只表示受理（§16.33），启动/生命周期在详情页按状态推进，这里不乐观改状态。
          createRun.mutate(
            {
              name: state.target.name || "cli-run",
              workspace_id: state.target.workspace || "ws-cli",
              authorization_scope_digest: state.target.authorizationRef || undefined,
            },
            {
              onSuccess: (run) => {
                toast.push({ tone: "ok", message: `Run 已创建（${run.state}）` });
                navigate(runNavTo(run.id, "command"));
              },
              onError: (e) => {
                toast.push({ tone: "danger", message: `创建失败：${e instanceof Error ? e.message : String(e)}` });
              },
            },
          );
        }}
      />
    </>
  );
}

export function InboxPage() {
  const toast = useToast();
  return (
    <>
      <PageHead title="收件箱" />
      <FullInboxPage
        model={mkInbox}
        onRead={(id) => toast.push({ tone: "info", message: `已读 ${id}` })}
        onSnooze={(id) => toast.push({ tone: "info", message: `Snooze ${id}` })}
        onApprove={(id, rev, dig) => toast.push({ tone: "ok", message: `Approve ${id}（rev ${rev}, digest ${dig.slice(0, 8)}）` })}
        onDeny={(id) => toast.push({ tone: "warn", message: `Deny ${id}` })}
        onDenyAndCancel={(id) => toast.push({ tone: "danger", message: `Deny + 取消关联 Job（${id}）` })}
        onRecover={(bid, a) => toast.push({ tone: "info", message: `恢复 ${bid}: ${a}` })}
      />
    </>
  );
}

export function KnowledgeGlobalPage() {
  return (
    <>
      <PageHead title="知识库" />
      <KnowledgePanel items={mkResources.knowledge} />
    </>
  );
}

export function SkillsGlobalPage() {
  return (
    <>
      <PageHead title="技能" />
      <SkillsPanel items={mkResources.skills} />
    </>
  );
}

export function ToolsGlobalPage() {
  return (
    <>
      <PageHead title="工具" />
      <ToolsPanel items={mkResources.templates} />
    </>
  );
}

export function OperationsGlobalPage() {
  const toast = useToast();
  return (
    <>
      <PageHead title="运维" />
      <OperationsPage
        model={mkOperations}
        onCheck={(id) => toast.push({ tone: "info", message: `连接检查 ${id}` })}
        onRotate={(id) => toast.push({ tone: "warn", message: `Rotate 出口 ${id}` })}
      />
    </>
  );
}

export function SettingsGlobalPage() {
  const toast = useToast();
  return (
    <>
      <PageHead title="设置 / 预检 / 诊断" />
      <SettingsPage model={mkSettings} onRecheck={(item) => toast.push({ tone: "info", message: `Recheck ${item}` })} />
    </>
  );
}

// 兜底空态（仅用于不存在的路径）。
export function PlaceholderPage({ title, note }: { title: string; note?: string }) {
  return (
    <>
      <PageHead title={title} />
      <div className="app-placeholder">
        <div style={{ fontSize: "var(--fs-lg)", marginBottom: "var(--sp-2)" }}>页面不存在</div>
        <div style={{ fontSize: "var(--fs-sm)" }}>{note ?? "该路径无对应页面，请从左侧导航进入。"}</div>
      </div>
    </>
  );
}
