// Guard —— 守卫：拦截规则表 + 审批日志（ARTEX 式）。拦截规则按 regex/string 匹配工具调用，
// 命中→审批/拒绝/放行（映射 Snag PDP/两层 Permit §6.3、ApprovalCenter §16.7）。审批日志记录每次决定。

import { useState } from "react";
import { Tabs } from "../../components/Tabs";
import type { ApprovalDecision, ApprovalLogEntry, GuardModel, InterceptionRule, RuleDisposition } from "./types";

const DISP_META: Record<RuleDisposition, { label: string; tone: string }> = {
  approve: { label: "需审批", tone: "warn" },
  deny: { label: "拒绝", tone: "danger" },
  allow: { label: "放行", tone: "ok" },
};

const DECISION_META: Record<ApprovalDecision, { label: string; tone: string }> = {
  approved: { label: "已批准", tone: "ok" },
  denied: { label: "已拒绝", tone: "danger" },
  denied_cancel: { label: "拒绝+取消", tone: "danger" },
  expired: { label: "已过期", tone: "unknown" },
  pending: { label: "待决", tone: "warn" },
};

export interface GuardProps {
  model: GuardModel;
  onToggleRule?: (id: string, enabled: boolean) => void;
}

export function Guard({ model, onToggleRule }: GuardProps) {
  const [rules, setRules] = useState<InterceptionRule[]>(model.rules);

  const toggle = (id: string) => {
    setRules((rs) => rs.map((r) => (r.id === id ? { ...r, enabled: !r.enabled } : r)));
    const r = rules.find((x) => x.id === id);
    if (r) onToggleRule?.(id, !r.enabled);
  };

  return (
    <Tabs
      items={[
        {
          id: "rules",
          label: `拦截规则 (${rules.length})`,
          content: <RulesTable rules={rules} onToggle={toggle} />,
        },
        {
          id: "log",
          label: `审批日志 (${model.log.length})`,
          content: <ApprovalLogTable log={model.log} />,
        },
      ]}
    />
  );
}

function RulesTable({ rules, onToggle }: { rules: InterceptionRule[]; onToggle: (id: string) => void }) {
  return (
    <div className="sn-table-wrap">
      <table className="sn-table">
        <thead>
          <tr>
            <th>匹配</th>
            <th>类型</th>
            <th>作用域</th>
            <th>处置</th>
            <th>命中</th>
            <th>启用</th>
          </tr>
        </thead>
        <tbody>
          {rules.length === 0 ? (
            <tr><td colSpan={6}><div className="sn-empty">暂无拦截规则</div></td></tr>
          ) : (
            rules.map((r) => {
              const d = DISP_META[r.disposition];
              return (
                <tr key={r.id} data-disabled={!r.enabled || undefined}>
                  <td className="mono" title={r.note}>{r.match}</td>
                  <td><span className="sn-badge" data-tone="ink-3">{r.matchKind}</span></td>
                  <td>{r.scope}</td>
                  <td><span className="sn-badge" data-tone={d.tone}>{d.label}</span></td>
                  <td className="mono">{r.hits}</td>
                  <td>
                    <button
                      className="sn-switch"
                      role="switch"
                      aria-checked={r.enabled}
                      aria-label={`${r.enabled ? "停用" : "启用"}规则 ${r.match}`}
                      data-on={r.enabled || undefined}
                      onClick={() => onToggle(r.id)}
                    >
                      <span />
                    </button>
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}

function ApprovalLogTable({ log }: { log: ApprovalLogEntry[] }) {
  return (
    <div className="sn-table-wrap">
      <table className="sn-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>动作</th>
            <th>身份</th>
            <th>决定</th>
            <th>决策者</th>
            <th>理由 / 规则</th>
          </tr>
        </thead>
        <tbody>
          {log.length === 0 ? (
            <tr><td colSpan={6}><div className="sn-empty">暂无审批记录</div></td></tr>
          ) : (
            log.map((e) => {
              const m = DECISION_META[e.decision];
              return (
                <tr key={e.id}>
                  <td className="mono">{e.at}</td>
                  <td className="mono">{e.action}</td>
                  <td>{e.identity ?? "—"}</td>
                  <td><span className="sn-badge" data-tone={m.tone}>{m.label}</span></td>
                  <td>{e.decidedBy ?? "—"}</td>
                  <td>
                    {e.reason ?? "—"}
                    {e.ruleId && <span className="sn-badge" data-tone="ink-3" style={{ marginLeft: "var(--sp-2)" }}>{e.ruleId}</span>}
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}
