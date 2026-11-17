import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cx } from "./cx";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "link";
export type ControlSize = "sm" | "md" | "lg";

export interface ButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "disabled"> {
  variant?: ButtonVariant;
  size?: ControlSize;
  loading?: boolean;
  disabled?: boolean;
  fullWidth?: boolean;
  /** icon-only 必须给 aria-label（D2.1 无障碍）。 */
  iconOnly?: boolean;
  leadingIcon?: ReactNode;
  children?: ReactNode;
}

// Button（D2.1）：5 变体 × 3 尺寸 × 全状态。disabled 用 aria-disabled 并仍可聚焦读提示；
// loading 冻结宽度、内联 spinner 占位防跳。纯 CSS 状态（hover/active/focus）。
export function Button({
  variant = "secondary",
  size = "md",
  loading = false,
  disabled = false,
  fullWidth = false,
  iconOnly = false,
  leadingIcon,
  children,
  className,
  onClick,
  type = "button",
  ...rest
}: ButtonProps) {
  const inert = disabled || loading;
  return (
    <button
      type={type}
      className={cx("sn-btn", "sn-focus", className)}
      data-variant={variant}
      data-size={size}
      data-loading={loading || undefined}
      data-disabled={inert || undefined}
      data-full={fullWidth || undefined}
      aria-disabled={inert || undefined}
      aria-busy={loading || undefined}
      onClick={(e) => {
        if (inert) {
          e.preventDefault();
          return;
        }
        onClick?.(e);
      }}
      {...rest}
    >
      {loading ? <span className="sn-spinner" aria-hidden="true" /> : leadingIcon}
      {!iconOnly && children}
    </button>
  );
}
