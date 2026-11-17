import { useState, type ReactNode } from "react";
import { cx } from "./cx";

export interface TabItem {
  id: string;
  label: ReactNode;
  content: ReactNode;
  badge?: number;
}

export interface TabsProps {
  items: TabItem[];
  initial?: string;
  value?: string;
  onChange?: (id: string) => void;
  className?: string;
  /** 下划线式（默认，Claude 气质）或分段式。 */
  variant?: "underline" | "segmented";
}

// Tabs（D2.9）：下划线式，当前 Tab accent 下划线；键盘 ←→；懒渲染当前面板。role=tablist/tab/tabpanel。
export function Tabs({ items, initial, value, onChange, className, variant = "underline" }: TabsProps) {
  const [inner, setInner] = useState(initial ?? items[0]?.id);
  const active = value ?? inner;
  const idx = Math.max(0, items.findIndex((t) => t.id === active));

  function select(id: string) {
    if (value === undefined) setInner(id);
    onChange?.(id);
  }
  function onKey(e: React.KeyboardEvent) {
    if (e.key === "ArrowRight") select(items[(idx + 1) % items.length].id);
    else if (e.key === "ArrowLeft") select(items[(idx - 1 + items.length) % items.length].id);
  }

  return (
    <div className={cx("sn-tabs", className)}>
      <div className="sn-tablist" role="tablist" onKeyDown={onKey} data-variant={variant}>
        {items.map((t) => (
          <button
            key={t.id}
            role="tab"
            aria-selected={t.id === active}
            tabIndex={t.id === active ? 0 : -1}
            className="sn-tab sn-focus"
            data-active={t.id === active || undefined}
            onClick={() => select(t.id)}
          >
            {t.label}
            {t.badge != null && t.badge > 0 && <span className="sn-tab-badge">{t.badge}</span>}
          </button>
        ))}
      </div>
      <div role="tabpanel" className="sn-tabpanel">
        {items[idx]?.content}
      </div>
    </div>
  );
}
