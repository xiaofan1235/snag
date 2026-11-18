// ApprovalCenter —— 审批中心（UI 文档 §16.10）。每条审批显示动作 Diff（正常动作→待审批动作）+
// 身份 + 对象 + 副作用 + 策略原因 + 过期时间。操作：Approve Once / Deny / Deny and Cancel Related Jobs。
// 不提供「批量批准高风险」（§16.33：批量只允许只读/已读通知等低风险，审批逐条处理）。
// Approve 时带 Approval Revision + Action Digest；动作已变/过期/已消费则冲突（由 onApprove 回调处理）。

import { Card } from "../../components/Card";
import { Button } from "../../components/Button";
import { StatusBadge } from "../../components/StatusBadge";

export interface ApprovalItem {
  id: string;
  revision: number;
  actionDigest: string;
  normalAction: string; // 正常动作
  pendingAction: string; // 待审批动作（高风险）
  identity: string;
  object: string;
  sideEffects: string[];
  policyReason: string;
  expiresAt: string;
  state: string; // PENDING/APPROVED/DENIED/EXPIRED/CONSUMED
  highRisk?: boolean;
}

export interface ApprovalCenterProps {
  items: ApprovalItem[];
  onApprove?: (id: string, revision: number, actionDigest: string) => void;
  onDeny?: (id: string) => void;
  onDenyAndCancel?: (id: string) => void;
}

export function ApprovalCenter({ items, onApprove, onDeny, onDenyAndCancel }: ApprovalCenterProps) {
  const pending = items.filter((i) => i.state === "PENDING");
  return (
    <div style={{ display: "grid", gap: "var(--sp-3)" }}>
      <div style={{ color: "var(--ink-2)", fontSize: "var(--fs-sm)" }}>
        {pending.length} 条待处理审批（逐条处理，不支持批量批准高风险动作）
      </div>
      {items.map((it) => (
        <Card key={it.id} data-testid={`approval-${it.id}`}>
          <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-2)" }}>
            <StatusBadge status={it.state === "PENDING" ? "QUEUED" : it.state} />
            {it.highRisk && (
              <span className="sn-badge" data-tone="danger">
                高风险
              </span>
            )}
            <span className="mono" style={{ marginLeft: "auto", fontSize: "var(--fs-xs)", color: "var(--ink-3)" }}>
              过期 {it.expiresAt}
            </span>
          </div>

          {/* 动作 Diff */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr auto 1fr",
              gap: "var(--sp-2)",
              alignItems: "center",
              marginBottom: "var(--sp-2)",
            }}
          >
            <div className="mono" style={{ padding: "var(--sp-2)", background: "var(--ok-bg)", borderRadius: "var(--radius-sm)", fontSize: "var(--fs-xs)" }}>
              {it.normalAction}
            </div>
            <span aria-hidden="true" style={{ color: "var(--ink-3)" }}>→</span>
            <div className="mono" style={{ padding: "var(--sp-2)", background: "var(--warn-bg)", borderRadius: "var(--radius-sm)", fontSize: "var(--fs-xs)" }}>
              {it.pendingAction}
            </div>
          </div>

          <dl style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: "2px var(--sp-3)", margin: "0 0 var(--sp-3)", fontSize: "var(--fs-sm)" }}>
            <dt style={{ color: "var(--ink-3)" }}>身份</dt>
            <dd style={{ margin: 0 }}>{it.identity}</dd>
            <dt style={{ color: "var(--ink-3)" }}>对象</dt>
            <dd style={{ margin: 0 }}>{it.object}</dd>
            <dt style={{ color: "var(--ink-3)" }}>副作用</dt>
            <dd style={{ margin: 0 }}>{it.sideEffects.join("；") || "无"}</dd>
            <dt style={{ color: "var(--ink-3)" }}>策略原因</dt>
            <dd style={{ margin: 0 }}>{it.policyReason}</dd>
          </dl>

          {it.state === "PENDING" && (
            <div style={{ display: "flex", gap: "var(--sp-2)" }}>
              <Button size="sm" variant="primary" onClick={() => onApprove?.(it.id, it.revision, it.actionDigest)}>
                批准一次
              </Button>
              <Button size="sm" variant="secondary" onClick={() => onDeny?.(it.id)}>
                拒绝
              </Button>
              <Button size="sm" variant="danger" onClick={() => onDenyAndCancel?.(it.id)}>
                拒绝并取消关联作业
              </Button>
            </div>
          )}
        </Card>
      ))}
    </div>
  );
}
