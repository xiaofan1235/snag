// ToolsPage —— 工具执行与终端观测（UI 文档 §16.18）。四视图：Action View / Tool Timeline /
// Terminal View（xterm 风格 ANSI 只读）/ Error View（按错误码聚合）。无 /exec?cmd= 直通。

import { Tabs } from "../../components/Tabs";
import { StatusBadge } from "../../components/StatusBadge";

export interface ActionOutput {
  id: string;
  tool: string;
  policyDecision: string;
  exitCode: number;
  stdout: string;
  stderr: string;
}
export interface ToolCall {
  seq: number;
  tool: string;
  durationMs: number;
  retries: number;
  degraded: boolean;
}
export interface ToolError {
  code: string; // TOOL_UNAVAILABLE/TIMEOUT/POLICY_DENIED/OUTCOME_UNKNOWN
  count: number;
  lastDetail: string;
}
export interface ToolsModel {
  actions: ActionOutput[];
  timeline: ToolCall[];
  terminal: string; // ANSI/纯文本
  errors: ToolError[];
}

export function ToolsPage({ model }: { model: ToolsModel }) {
  return (
    <Tabs
      items={[
        {
          id: "action",
          label: "动作视图",
          content: (
            <div style={{ display: "grid", gap: "var(--sp-2)" }}>
              {model.actions.map((a) => (
                <div key={a.id} className="sn-card">
                  <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-1)" }}>
                    <span className="mono">{a.tool}</span>
                    <StatusBadge status={a.policyDecision === "allow" ? "COMPLETED" : "FAILED"} />
                    <span className="mono" style={{ marginLeft: "auto", fontSize: "var(--fs-xs)", color: a.exitCode === 0 ? "var(--ok)" : "var(--danger)" }}>exit {a.exitCode}</span>
                  </div>
                  {a.stdout && <pre className="sn-terminal">{a.stdout}</pre>}
                  {a.stderr && <pre className="sn-terminal" style={{ color: "#e0a0a0" }}>{a.stderr}</pre>}
                </div>
              ))}
            </div>
          ),
        },
        {
          id: "timeline",
          label: "工具时间线",
          content: (
            <table className="sn-table">
              <thead><tr><th>#</th><th>工具</th><th>耗时</th><th>重试</th><th>降级</th></tr></thead>
              <tbody>
                {model.timeline.map((t) => (
                  <tr key={t.seq}><td>{t.seq}</td><td className="mono">{t.tool}</td><td>{t.durationMs}ms</td><td>{t.retries}</td><td>{t.degraded ? "是" : "—"}</td></tr>
                ))}
              </tbody>
            </table>
          ),
        },
        { id: "terminal", label: "终端视图（只读）", content: <pre className="sn-terminal">{model.terminal}</pre> },
        {
          id: "error",
          label: "错误视图",
          badge: model.errors.reduce((s, e) => s + e.count, 0),
          content: (
            <div style={{ display: "grid", gap: "var(--sp-2)" }}>
              {model.errors.map((e) => (
                <div key={e.code} className="sn-card" style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
                  <span className="sn-badge" data-tone={e.code === "OUTCOME_UNKNOWN" ? "unknown" : "danger"}>{e.code}</span>
                  <span>×{e.count}</span>
                  <span style={{ marginLeft: "auto", color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>{e.lastDetail}</span>
                </div>
              ))}
            </div>
          ),
        },
      ]}
    />
  );
}
