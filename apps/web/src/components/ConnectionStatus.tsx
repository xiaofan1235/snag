// ConnectionStatus —— SSE 连接状态徽章（UI 文档 §16.33）。显示 LIVE/RECONNECTING/STALE/RESYNCING。
// 关键：浏览器离线/UI 断线**不代表 Run 已停止**——文案只描述连接，不描述 Run 状态。

import type { StreamState } from "../events/stream";
import { cx } from "./cx";

const META: Record<StreamState, { tone: string; label: string; pulse?: boolean }> = {
  CONNECTING: { tone: "info", label: "连接中", pulse: true },
  LIVE: { tone: "ok", label: "实时", pulse: true },
  RECONNECTING: { tone: "warn", label: "重连中", pulse: true },
  STALE: { tone: "unknown", label: "连接中断（Run 仍在后端运行）" },
  RESYNCING: { tone: "info", label: "重新同步中", pulse: true },
};

export function ConnectionStatus({ state, className }: { state: StreamState; className?: string }) {
  const m = META[state];
  return (
    <span className={cx("sn-badge", className)} data-tone={m.tone} data-pulse={m.pulse || undefined} role="status" title="UI 连接状态，不代表 Run 状态">
      <span className="sn-badge-dot" aria-hidden="true" />
      <span>{m.label}</span>
    </span>
  );
}
