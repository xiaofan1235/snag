// JobsPage —— Agent/Job 页面（UI 文档 §16.4）。表格视图 + 图视图（简化为分组节点）+ Job Drawer。
// 颜色区分状态但不以颜色为唯一信息（StatusBadge 文字+图标）。大任务默认按模块聚合。
// 点击 Job 打开 Drawer：JobSpec/输入 Evidence/进度/最近事件/失败原因/子 Job/Cancel·Reprioritize。

import { useState } from "react";
import { Card } from "../../components/Card";
import { Button } from "../../components/Button";
import { StatusBadge } from "../../components/StatusBadge";
import { Drawer } from "../../components/Drawer";
import { Tabs } from "../../components/Tabs";

export interface JobRow {
  id: string;
  kind: string; // Strategist/Recon/Module/Experiment/Verifier/Report
  module: string;
  state: string;
  attempt: number;
  currentAction?: string;
  priority: number;
  budget: string;
  lease?: string;
  reason: string; // 创建原因
  spec?: string;
  inputEvidence?: string[];
  progress?: string;
  recentEvents?: string[];
  failure?: string;
  subJobs?: string[];
}
export interface JobsModel {
  jobs: JobRow[];
}

export interface JobsPageProps {
  model: JobsModel;
  onCancel?: (id: string) => void;
  onReprioritize?: (id: string, priority: number) => void;
}

export function JobsPage({ model, onCancel, onReprioritize }: JobsPageProps) {
  const [sel, setSel] = useState<JobRow | null>(null);

  const table = (
    <div className="sn-table-wrap">
      <table className="sn-table">
        <thead>
          <tr>
            <th>类型</th><th>模块</th><th>状态</th><th>尝试</th><th>当前动作</th><th>优先级</th><th>预算</th><th>租约</th><th>创建原因</th>
          </tr>
        </thead>
        <tbody>
          {model.jobs.map((j) => (
            <tr key={j.id} style={{ cursor: "pointer" }} onClick={() => setSel(j)}>
              <td>{j.kind}</td>
              <td className="mono">{j.module}</td>
              <td><StatusBadge status={j.state} /></td>
              <td>#{j.attempt}</td>
              <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{j.currentAction ?? "-"}</td>
              <td>{j.priority}</td>
              <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{j.budget}</td>
              <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{j.lease ?? "-"}</td>
              <td style={{ color: "var(--ink-2)", fontSize: "var(--fs-xs)" }}>{j.reason}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );

  // 图视图：按模块聚合的节点（简化，大任务不一次渲染数千节点）。
  const byModule = model.jobs.reduce<Record<string, JobRow[]>>((acc, j) => {
    (acc[j.module] ??= []).push(j);
    return acc;
  }, {});
  const graph = (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill,minmax(200px,1fr))", gap: "var(--sp-3)" }}>
      {Object.entries(byModule).map(([mod, jobs]) => (
        <Card key={mod}>
          <div style={{ fontWeight: 600, marginBottom: "var(--sp-2)" }}>{mod}</div>
          <div style={{ display: "grid", gap: "var(--sp-1)" }}>
            {jobs.map((j) => (
              <button key={j.id} className="sn-btn sn-focus" data-variant="ghost" data-size="sm"
                style={{ justifyContent: "space-between", width: "100%" }} onClick={() => setSel(j)}>
                <span>{j.kind}</span>
                <StatusBadge status={j.state} />
              </button>
            ))}
          </div>
        </Card>
      ))}
    </div>
  );

  return (
    <>
      <Tabs
        items={[
          { id: "table", label: "表格视图", content: table },
          { id: "graph", label: "图视图（按模块聚合）", content: graph },
        ]}
      />
      <Drawer open={sel != null} onClose={() => setSel(null)} title={sel ? `Job ${sel.id}` : ""}>
        {sel && (
          <div style={{ display: "grid", gap: "var(--sp-3)" }}>
            <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
              <StatusBadge status={sel.state} />
              <span>{sel.kind} · {sel.module}</span>
            </div>
            <dl className="sn-kv">
              <dt>JobSpec</dt><dd className="mono">{sel.spec ?? "-"}</dd>
              <dt>输入 Evidence</dt><dd className="mono">{(sel.inputEvidence ?? []).join(", ") || "无"}</dd>
              <dt>进度摘要</dt><dd>{sel.progress ?? "-"}</dd>
              <dt>创建原因</dt><dd>{sel.reason}</dd>
              {sel.failure && (<><dt style={{ color: "var(--danger)" }}>失败原因</dt><dd style={{ color: "var(--danger)" }}>{sel.failure}</dd></>)}
              <dt>子 Job</dt><dd className="mono">{(sel.subJobs ?? []).join(", ") || "无"}</dd>
            </dl>
            {sel.recentEvents && sel.recentEvents.length > 0 && (
              <div>
                <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-1)" }}>最近事件</div>
                <ul style={{ margin: 0, paddingLeft: "var(--sp-5)", fontSize: "var(--fs-sm)" }}>
                  {sel.recentEvents.map((ev, i) => <li key={i}>{ev}</li>)}
                </ul>
              </div>
            )}
            <div style={{ display: "flex", gap: "var(--sp-2)" }}>
              <Button size="sm" variant="secondary" onClick={() => onReprioritize?.(sel.id, sel.priority + 10)}>提升优先级</Button>
              <Button size="sm" variant="danger" onClick={() => { onCancel?.(sel.id); setSel(null); }}>取消</Button>
            </div>
          </div>
        )}
      </Drawer>
    </>
  );
}
