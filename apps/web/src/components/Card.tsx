import type { HTMLAttributes, ReactNode } from "react";
import { cx } from "./cx";

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  clickable?: boolean;
  children?: ReactNode;
}

// Card（D2.7）：纸质感，阴影极淡；可点卡片 hover 抬升到 shadow-2。
export function Card({ clickable = false, className, children, ...rest }: CardProps) {
  return (
    <div
      className={cx("sn-card", clickable && "sn-focus", className)}
      data-clickable={clickable || undefined}
      tabIndex={clickable ? 0 : undefined}
      role={clickable ? "button" : undefined}
      {...rest}
    >
      {children}
    </div>
  );
}
