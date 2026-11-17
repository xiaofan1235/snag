import { useEffect, useId, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { cx } from "./cx";

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps {
  options: SelectOption[];
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  error?: boolean;
  /** 可搜索（输入过滤）。 */
  searchable?: boolean;
  /** 超过此数量启用虚拟化（D2.3 性能：>50 必虚拟化）。 */
  virtualizeThreshold?: number;
  "aria-label"?: string;
}

// Select（D2.3）：自绘单选下拉。键盘 ↑↓/Enter/Esc/Home/End；点击外部/Esc 关闭、焦点回 trigger；
// role=listbox/option + aria-selected/expanded；选项 >50 虚拟化（TanStack Virtual）。
export function Select({
  options,
  value,
  onChange,
  placeholder = "请选择…",
  disabled = false,
  error = false,
  searchable = false,
  virtualizeThreshold = 50,
  "aria-label": ariaLabel,
}: SelectProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const listboxId = useId();

  const filtered = useMemo(() => {
    if (!searchable || !query) return options;
    const q = query.toLowerCase();
    return options.filter((o) => o.label.toLowerCase().includes(q));
  }, [options, query, searchable]);

  const selected = options.find((o) => o.value === value);
  const virtualize = filtered.length > virtualizeThreshold;

  const rowVirtualizer = useVirtualizer({
    count: filtered.length,
    getScrollElement: () => panelRef.current,
    estimateSize: () => 34,
    overscan: 8,
    enabled: open && virtualize,
  });

  // 点击外部关闭。
  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  function commit(opt: SelectOption) {
    if (opt.disabled) return;
    onChange?.(opt.value);
    setOpen(false);
    setQuery("");
    triggerRef.current?.focus();
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (disabled) return;
    if (!open && (e.key === "Enter" || e.key === " " || e.key === "ArrowDown")) {
      e.preventDefault();
      setOpen(true);
      return;
    }
    if (!open) return;
    switch (e.key) {
      case "Escape":
        e.preventDefault();
        setOpen(false);
        triggerRef.current?.focus();
        break;
      case "ArrowDown":
        e.preventDefault();
        setActive((i) => Math.min(i + 1, filtered.length - 1));
        break;
      case "ArrowUp":
        e.preventDefault();
        setActive((i) => Math.max(i - 1, 0));
        break;
      case "Home":
        e.preventDefault();
        setActive(0);
        break;
      case "End":
        e.preventDefault();
        setActive(filtered.length - 1);
        break;
      case "Enter":
        e.preventDefault();
        if (filtered[active]) commit(filtered[active]);
        break;
    }
  }

  useEffect(() => {
    if (open && virtualize) rowVirtualizer.scrollToIndex(active);
  }, [active, open, virtualize, rowVirtualizer]);

  const renderOption = (opt: SelectOption, idx: number, style?: React.CSSProperties) => (
    <div
      key={opt.value}
      role="option"
      aria-selected={opt.value === value}
      aria-disabled={opt.disabled || undefined}
      className="sn-option"
      data-active={idx === active || undefined}
      style={style}
      onMouseEnter={() => setActive(idx)}
      onClick={() => commit(opt)}
    >
      {opt.label}
    </div>
  );

  return (
    <div className="sn-select" ref={rootRef} onKeyDown={onKeyDown}>
      <button
        ref={triggerRef}
        type="button"
        className={cx("sn-select-trigger", "sn-focus")}
        data-disabled={disabled || undefined}
        aria-invalid={error || undefined}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={ariaLabel}
        disabled={disabled}
        onClick={() => !disabled && setOpen((v) => !v)}
      >
        <span style={{ color: selected ? "var(--ink-1)" : "var(--ink-3)" }}>
          {selected ? selected.label : placeholder}
        </span>
        <span aria-hidden="true">▾</span>
      </button>
      {open && (
        <div className="sn-select-panel" ref={panelRef} role="listbox" id={listboxId} tabIndex={-1}>
          {searchable && (
            <input
              className="sn-input sn-focus"
              style={{ width: "100%", marginBottom: "var(--sp-1)" }}
              placeholder="搜索…"
              value={query}
              autoFocus
              onChange={(e) => {
                setQuery(e.target.value);
                setActive(0);
              }}
            />
          )}
          {filtered.length === 0 && <div className="sn-empty">无匹配项</div>}
          {virtualize ? (
            <div style={{ height: rowVirtualizer.getTotalSize(), position: "relative" }}>
              {rowVirtualizer.getVirtualItems().map((vi) =>
                renderOption(filtered[vi.index], vi.index, {
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  transform: `translateY(${vi.start}px)`,
                  height: vi.size,
                })
              )}
            </div>
          ) : (
            filtered.map((opt, idx) => renderOption(opt, idx))
          )}
        </div>
      )}
    </div>
  );
}
