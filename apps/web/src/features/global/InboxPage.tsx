// InboxPage —— 统一收件箱 / 通知 / 阻塞恢复（UI 文档 §16.23）。分类：APPROVAL_REQUIRED/AUTH_REQUIRED/
// OUTCOME_UNKNOWN/BUDGET_WARNING/RUN_BLOCKED/FINDING_CONFIRMED/CLEANUP_REQUIRED/SYSTEM_DEGRADED。
// 不支持批量批准高风险；Snooze 只影响提醒不延长 Approval/Lease。阻塞恢复向导给出原因+可执行恢复动作。

import { Tabs } from "../../components/Tabs";
import { Card } from "../../components/Card";
import { Button } from "../../components/Button";
import { StatusBadge } from "../../components/StatusBadge";
import { ApprovalCenter, type ApprovalItem } from "../approvals/ApprovalCenter";

export type NotifCategory =
  | "APPROVAL_REQUIRED" | "AUTH_REQUIRED" | "OUTCOME_UNKNOWN" | "BUDGET_WARNING"
  | "RUN_BLOCKED" | "FINDING_CONFIRMED" | "CLEANUP_REQUIRED" | "SYSTEM_DEGRADED";

export interface Notification {
  id: string;
  category: NotifCategory;
  summary: string;
  runId?: string;
  read: boolean;
  at: string;
}
export interface Blocker {
  id: string;
  runId: string;
  reason: string;
  scope: string;
  waitedFor: string;
  recoveryActions: string[];
  restartsJobs: string[];
}
export interface InboxModel {
  notifications: Notification[];
  approvals: ApprovalItem[];
  blockers: Blocker[];
}

const CAT_LABEL: Record<NotifCategory, string> = {
  APPROVAL_REQUIRED: "待审批", AUTH_REQUIRED: "需认证", OUTCOME_UNKNOWN: "结果未知", BUDGET_WARNING: "预算预警",
  RUN_BLOCKED: "Run 受阻", FINDING_CONFIRMED: "新确认 Finding", CLEANUP_REQUIRED: "需清理", SYSTEM_DEGRADED: "系统降级",
};
const CAT_TONE: Record<NotifCategory, string> = {
  APPROVAL_REQUIRED: "warn", AUTH_REQUIRED: "warn", OUTCOME_UNKNOWN: "unknown", BUDGET_WARNING: "warn",
  RUN_BLOCKED: "danger", FINDING_CONFIRMED: "danger", CLEANUP_REQUIRED: "info", SYSTEM_DEGRADED: "danger",
};

export interface InboxPageProps {
  model: InboxModel;
  onRead?: (id: string) => void;
  onSnooze?: (id: string) => void;
  onApprove?: (id: string, revision: number, actionDigest: string) => void;
  onDeny?: (id: string) => void;
  onDenyAndCancel?: (id: string) => void;
  onRecover?: (blockerId: string, action: string) => void;
}

export function InboxPage(props: InboxPageProps) {
  const { model } = props;
  const unread = model.notifications.filter((n) => !n.read).length;
  return (
    <Tabs
      items={[
        {
          id: "all",
          label: "全部通知",
          badge: unread,
          content: (
            <div style={{ display: "grid", gap: "var(--sp-2)" }}>
              {model.notifications.length === 0 && <div className="sn-empty">暂无通知</div>}
              {model.notifications.map((n) => (
                <Card key={n.id}>
                  <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
                    <span className="sn-badge" data-tone={CAT_TONE[n.category]}>{CAT_LABEL[n.category]}</span>
                    {!n.read && <span className="sn-badge" data-tone="accent">未读</span>}
                    <span style={{ fontSize: "var(--fs-sm)" }}>{n.summary}</span>
                    <span style={{ marginLeft: "auto", color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>{n.at}</span>
                    <Button size="sm" variant="ghost" onClick={() => props.onRead?.(n.id)}>已读</Button>
                    <Button size="sm" variant="ghost" onClick={() => props.onSnooze?.(n.id)}>稍后</Button>
                  </div>
                </Card>
              ))}
              <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>不支持批量批准高风险动作；Snooze 只影响提醒，不延长 Approval 或 Lease。</div>
            </div>
          ),
        },
        {
          id: "approvals",
          label: "待审批",
          badge: model.approvals.filter((a) => a.state === "PENDING").length,
          content: (
            <ApprovalCenter items={model.approvals} onApprove={props.onApprove} onDeny={props.onDeny} onDenyAndCancel={props.onDenyAndCancel} />
          ),
        },
        {
          id: "blockers",
          label: "阻塞恢复",
          badge: model.blockers.length,
          content: (
            <div style={{ display: "grid", gap: "var(--sp-3)" }}>
              {model.blockers.length === 0 && <div className="sn-empty">当前无阻塞</div>}
              {model.blockers.map((b) => (
                <Card key={b.id}>
                  <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-2)" }}>
                    <StatusBadge status="BLOCKED" />
                    <strong>{b.reason}</strong>
                    <span style={{ marginLeft: "auto", color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>已等待 {b.waitedFor}</span>
                  </div>
                  <dl className="sn-kv">
                    <dt>影响范围</dt><dd>{b.scope}</dd>
                    <dt>恢复后重启 Job</dt><dd className="mono">{b.restartsJobs.join(", ") || "无"}</dd>
                  </dl>
                  <div style={{ display: "flex", gap: "var(--sp-2)", marginTop: "var(--sp-2)", flexWrap: "wrap" }}>
                    {b.recoveryActions.map((a) => (
                      <Button key={a} size="sm" variant="secondary" onClick={() => props.onRecover?.(b.id, a)}>{a}</Button>
                    ))}
                  </div>
                </Card>
              ))}
              <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>例如代理过期只允许重新提取/绑定出口并 Preflight，不提供「临时直连目标」按钮。</div>
            </div>
          ),
        },
      ]}
    />
  );
}
