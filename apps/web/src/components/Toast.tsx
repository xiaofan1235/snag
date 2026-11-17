import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";

export type ToastTone = "ok" | "warn" | "danger" | "info" | "neutral";

export interface ToastItem {
  id: string;
  tone: ToastTone;
  message: ReactNode;
  /** 可选标题（§16.35：图标 + 标题 + 正文结构）。 */
  title?: ReactNode;
  /** 毫秒；0 表示不自动消失。 */
  duration?: number;
}

// tone → 图标符号（按 tone 着色，正文反白，§16.35 反馈二）。
const TONE_ICON: Record<ToastTone, string> = {
  ok: "✓",
  warn: "!",
  danger: "✕",
  info: "i",
  neutral: "·",
};

interface ToastCtx {
  push: (t: Omit<ToastItem, "id">) => string;
  dismiss: (id: string) => void;
}

const Ctx = createContext<ToastCtx | null>(null);

// useToast（D2.11）：非阻断通知。调用方务必对错误 Toast 脱敏——不放秘密/原始响应（§16.33）。
export function useToast(): ToastCtx {
  const c = useContext(Ctx);
  if (!c) throw new Error("useToast 必须在 <ToastProvider> 内使用");
  return c;
}

let seq = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const timers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  const dismiss = useCallback((id: string) => {
    setItems((xs) => xs.filter((x) => x.id !== id));
    const t = timers.current.get(id);
    if (t) {
      clearTimeout(t);
      timers.current.delete(id);
    }
  }, []);

  const push = useCallback(
    (t: Omit<ToastItem, "id">) => {
      const id = `toast-${++seq}`;
      const item: ToastItem = { duration: 4000, ...t, id };
      setItems((xs) => [...xs, item]);
      if (item.duration && item.duration > 0) {
        timers.current.set(
          id,
          setTimeout(() => dismiss(id), item.duration)
        );
      }
      return id;
    },
    [dismiss]
  );

  useEffect(() => {
    const map = timers.current;
    return () => {
      map.forEach((t) => clearTimeout(t));
      map.clear();
    };
  }, []);

  return (
    <Ctx.Provider value={{ push, dismiss }}>
      {children}
      <div className="sn-toast-region" role="region" aria-label="通知" aria-live="polite">
        {items.map((it) => (
          <div key={it.id} className="sn-toast" data-tone={it.tone} role="status">
            <span className="sn-toast-icon" aria-hidden="true">
              {TONE_ICON[it.tone]}
            </span>
            <div className="sn-toast-body">
              {it.title && <div className="sn-toast-title">{it.title}</div>}
              <div>{it.message}</div>
            </div>
            <button className="sn-toast-close" aria-label="关闭通知" onClick={() => dismiss(it.id)}>
              ✕
            </button>
          </div>
        ))}
      </div>
    </Ctx.Provider>
  );
}
