// TimelinePage —— Timeline / Audit / 全局搜索（UI 文档 §16.28）。统一时间轴（Strategy/Job/Action/
// Approval/Evidence/Finding/系统事件）+ Audit Explorer（只读，Reveal/审批/策略/接管/导出/清理重点标记）+ 全局搜索。
// Timeline 是查询投影，不是另一份可编辑状态。

import { useMemo, useState } from "react";
import { Input } from "../../components/Input";
import { Tabs } from "../../components/Tabs";

export interface TimelineEntry {
  at: string;
  kind: string; // strategy/job/action/approval/evidence/finding/system
  summary: string;
  key?: boolean; // 关键事件
}
export interface AuditEntry {
  at: string;
  actor: string;
  command: string;
  ref: string; // Run/Job/Action
  policyRevision: number;
  idempotencyKey: string;
  result: string;
  errorCode?: string;
  auditHash: string;
  flagged?: boolean; // Reveal/审批/策略/接管/导出/清理
}
export interface TimelineModel {
  timeline: TimelineEntry[];
  audit: AuditEntry[];
}

const KIND_TONE: Record<string, string> = {
  strategy: "info", job: "accent", action: "ink-2", approval: "warn",
  evidence: "ok", finding: "danger", system: "unknown",
};

export function TimelinePage({ model }: { model: TimelineModel }) {
  const [q, setQ] = useState("");
  const [keyOnly, setKeyOnly] = useState(false);

  const tl = useMemo(
    () => model.timeline.filter((e) => (!keyOnly || e.key) && (!q || e.summary.toLowerCase().includes(q.toLowerCase()))),
    [model.timeline, keyOnly, q]
  );

  return (
    <Tabs
      items={[
        {
          id: "timeline",
          label: "Run Timeline",
          content: (
            <>
              <div style={{ display: "flex", gap: "var(--sp-3)", alignItems: "center", marginBottom: "var(--sp-3)" }}>
                <div style={{ minWidth: 200 }}><Input placeholder="搜索时间轴…" value={q} onChange={(e) => setQ(e.target.value)} aria-label="搜索时间轴" /></div>
                <label style={{ display: "flex", alignItems: "center", gap: "var(--sp-1)", fontSize: "var(--fs-sm)" }}>
                  <input type="checkbox" checked={keyOnly} onChange={(e) => setKeyOnly(e.target.checked)} /> 只看关键事件
                </label>
              </div>
              <div style={{ display: "grid", gap: "var(--sp-1)" }}>
                {tl.map((e, i) => (
                  <div key={i} style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", padding: "var(--sp-1) var(--sp-2)", borderLeft: "2px solid var(--border-strong)" }}>
                    <span className="mono" style={{ fontSize: "var(--fs-xs)", color: "var(--ink-3)", width: 110 }}>{e.at}</span>
                    <span className="sn-badge" data-tone={KIND_TONE[e.kind] ?? "ink-3"}>{e.kind}</span>
                    <span style={{ fontSize: "var(--fs-sm)" }}>{e.summary}</span>
                  </div>
                ))}
              </div>
            </>
          ),
        },
        {
          id: "audit",
          label: "Audit Explorer",
          content: (
            <div className="sn-table-wrap">
              <table className="sn-table">
                <thead><tr><th>时间</th><th>Actor</th><th>Command</th><th>对象</th><th>PolRev</th><th>结果</th><th>审计哈希</th></tr></thead>
                <tbody>
                  {model.audit.map((a, i) => (
                    <tr key={i} style={a.flagged ? { background: "var(--warn-bg)" } : undefined}>
                      <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{a.at}</td>
                      <td>{a.actor}</td>
                      <td className="mono">{a.command}{a.flagged && <span className="sn-badge" data-tone="warn" style={{ marginLeft: 4 }}>重点</span>}</td>
                      <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{a.ref}</td>
                      <td>{a.policyRevision}</td>
                      <td style={{ color: a.errorCode ? "var(--danger)" : "var(--ok)" }}>{a.errorCode ?? a.result}</td>
                      <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{a.auditHash.slice(0, 12)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", padding: "var(--sp-2)" }}>审计记录只读，无编辑/单条删除；哈希链可离线验（§47.2）。</div>
            </div>
          ),
        },
      ]}
    />
  );
}
