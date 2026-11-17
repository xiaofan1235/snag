import { useId, useRef, type ReactNode } from "react";
import { useFocusTrap } from "./useFocusTrap";
import { Button } from "./Button";

export interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children?: ReactNode;
}

// Drawer（D2.10）：右侧抽屉 = Inspector（对象详情），不阻断主内容。role=dialog + 焦点陷阱 + Esc；
// 遮罩较淡（非完全阻断视觉）。宽屏可固定/收起由布局层决定（Day 41+）。
export function Drawer({ open, onClose, title, children }: DrawerProps) {
  const ref = useRef<HTMLDivElement>(null);
  const titleId = useId();
  useFocusTrap(open, ref, onClose);
  if (!open) return null;
  return (
    <div className="sn-drawer-overlay" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div
        className="sn-drawer"
        ref={ref}
        role="dialog"
        aria-label={typeof title === "string" ? title : "详情"}
        aria-labelledby={title ? titleId : undefined}
        tabIndex={-1}
      >
        <div className="sn-dialog-head">
          {title && (
            <h2 className="sn-dialog-title" id={titleId}>
              {title}
            </h2>
          )}
          <Button variant="ghost" size="sm" iconOnly aria-label="关闭" onClick={onClose}>
            ✕
          </Button>
        </div>
        <div>{children}</div>
      </div>
    </div>
  );
}
