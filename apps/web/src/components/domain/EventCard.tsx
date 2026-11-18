// EventCard —— 统一事件卡族（UI 文档 D2.13 / §16.13）。统一头（类型图标 + 时间）+ 体（Typed 内容）+
// 脚（跳转链）。PlanCard/ActionCard/ObservationCard/DecisionCard/ApprovalCard/ErrorCard/BudgetCard/UserCard
// 复用同一壳，只换头图标与 Typed 体。错误卡脱敏（不含秘密/原始响应，§16.33）。

import type { ReactNode } from "react";
import { Card } from "../Card";
import { StatusBadge } from "../StatusBadge";

export type EventCardKind =
  | "plan"
  | "action"
  | "observation"
  | "decision"
  | "approval"
  | "error"
  | "budget"
  | "user";

const KIND_META: Record<EventCardKind, { icon: string; label: string }> = {
  plan: { icon: "◇", label: "计划" },
  action: { icon: "▶", label: "动作" },
  observation: { icon: "◎", label: "观察" },
  decision: { icon: "⚖", label: "决策" },
  approval: { icon: "✔", label: "审批" },
  error: { icon: "⚠", label: "错误" },
  budget: { icon: "◴", label: "预算" },
  user: { icon: "✎", label: "指令" },
};

export interface EventCardProps {
  kind: EventCardKind;
  title: ReactNode;
  time?: string;
  status?: string;
  children?: ReactNode;
  /** 脚部跳转链（ObjectRef）。 */
  onJump?: () => void;
  jumpLabel?: string;
}

export function EventCard({ kind, title, time, status, children, onJump, jumpLabel }: EventCardProps) {
  const meta = KIND_META[kind];
  return (
    <Card>
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-2)" }}>
        <span aria-hidden="true" style={{ color: "var(--ink-3)" }}>
          {meta.icon}
        </span>
        <strong style={{ fontSize: "var(--fs-sm)" }}>{title}</strong>
        <span className="sn-badge" data-tone="ink-3" style={{ marginLeft: "var(--sp-1)" }}>
          {meta.label}
        </span>
        {status && <StatusBadge status={status} />}
        {time && (
          <span className="mono" style={{ marginLeft: "auto", fontSize: "var(--fs-xs)", color: "var(--ink-3)" }}>
            {time}
          </span>
        )}
      </div>
      {children && <div style={{ fontSize: "var(--fs-sm)", color: "var(--ink-2)" }}>{children}</div>}
      {onJump && (
        <div style={{ marginTop: "var(--sp-2)" }}>
          <button
            className="sn-btn sn-focus"
            data-variant="link"
            data-size="sm"
            onClick={onJump}
          >
            {jumpLabel ?? "查看详情"}
          </button>
        </div>
      )}
    </Card>
  );
}
