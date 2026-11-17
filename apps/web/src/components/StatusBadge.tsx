import { statusMeta } from "./status";
import { cx } from "./cx";

export interface StatusBadgeProps {
  /** 状态枚举键（Run/Job/Finding 通用，见 status.ts STATUS_MAP）。 */
  status: string;
  className?: string;
}

// StatusBadge（D2.6）：全站统一状态映射。颜色+图标(圆点)+文字三者齐全（色盲可辨）；
// RUNNING 轻脉动、不确定态斜纹。未登记状态兜底为 unknown，不静默吞。
export function StatusBadge({ status, className }: StatusBadgeProps) {
  const m = statusMeta(status);
  return (
    <span
      className={cx("sn-badge", className)}
      data-tone={m.tone}
      data-pulse={m.pulse || undefined}
      data-hatched={m.hatched || undefined}
      role="status"
      title={m.label}
    >
      <span className="sn-badge-dot" aria-hidden="true" />
      <span>{m.label}</span>
    </span>
  );
}
