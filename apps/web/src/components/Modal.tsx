import { useId, useRef, type ReactNode } from "react";
import { useFocusTrap } from "./useFocusTrap";
import { Button } from "./Button";

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  /** 破坏性确认（D2.10 Do）。 */
  danger?: boolean;
  confirmText?: string;
  cancelText?: string;
  onConfirm?: () => void;
}

// Modal（D2.10）：阻断式确认；role=dialog + aria-modal；焦点陷阱 + Esc 关 + 遮罩。
// 破坏性操作用 danger 二次确认。点击遮罩关闭（确认类对话框由调用方决定是否禁用）。
export function Modal({
  open,
  onClose,
  title,
  children,
  footer,
  danger = false,
  confirmText = "确认",
  cancelText = "取消",
  onConfirm,
}: ModalProps) {
  const ref = useRef<HTMLDivElement>(null);
  const titleId = useId();
  useFocusTrap(open, ref, onClose);
  if (!open) return null;
  return (
    <div className="sn-overlay" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div
        className="sn-modal"
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? titleId : undefined}
        tabIndex={-1}
      >
        {title && (
          <div className="sn-dialog-head">
            <h2 className="sn-dialog-title" id={titleId}>
              {title}
            </h2>
            <Button variant="ghost" size="sm" iconOnly aria-label="关闭" onClick={onClose}>
              ✕
            </Button>
          </div>
        )}
        <div>{children}</div>
        <div style={{ display: "flex", justifyContent: "flex-end", gap: "var(--sp-2)", marginTop: "var(--sp-5)" }}>
          {footer ?? (
            <>
              <Button variant="secondary" onClick={onClose}>
                {cancelText}
              </Button>
              {onConfirm && (
                <Button variant={danger ? "danger" : "primary"} onClick={onConfirm}>
                  {confirmText}
                </Button>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}
