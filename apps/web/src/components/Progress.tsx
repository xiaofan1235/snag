import { cx } from "./cx";

export type ProgressTone = "accent" | "ok" | "warn" | "danger";

export interface ProgressProps {
  /** 0–100；省略或 indeterminate 时忽略。 */
  value?: number;
  max?: number;
  indeterminate?: boolean;
  tone?: ProgressTone;
  size?: "sm" | "md";
  className?: string;
  "aria-label"?: string;
}

// Progress（D2.5）：linear determinate / indeterminate。CSS width/transform 过渡，禁每帧 JS 改 DOM。
// role=progressbar + aria-valuenow/min/max；indeterminate 省略 valuenow。
export function Progress({
  value,
  max = 100,
  indeterminate = false,
  tone = "accent",
  size = "md",
  className,
  "aria-label": ariaLabel,
}: ProgressProps) {
  const pct = indeterminate || value == null ? 0 : Math.max(0, Math.min(100, (value / max) * 100));
  return (
    <div
      className={cx("sn-progress-track", className)}
      data-size={size}
      role="progressbar"
      aria-label={ariaLabel}
      aria-valuemin={0}
      aria-valuemax={max}
      aria-valuenow={indeterminate ? undefined : value}
    >
      <div
        className="sn-progress-bar"
        data-tone={tone}
        data-indeterminate={indeterminate || undefined}
        style={indeterminate ? undefined : { width: `${pct}%` }}
      />
    </div>
  );
}

export interface FractionProgressProps {
  done: number;
  total: number;
  tone?: ProgressTone;
  size?: "sm" | "md";
}

// FractionProgress（D2.5 Do）：覆盖率/实验用真实 DONE/TOTAL 分数，不显示虚假百分比。
export function FractionProgress({ done, total, tone = "accent", size = "md" }: FractionProgressProps) {
  const label = `${done}/${total}`;
  return (
    <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
      <Progress value={done} max={Math.max(total, 1)} tone={tone} size={size} aria-label={label} />
      <span className="mono" style={{ fontSize: "var(--fs-xs)", color: "var(--ink-2)" }}>
        {label}
      </span>
    </div>
  );
}
