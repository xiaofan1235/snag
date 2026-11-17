// inspector.tsx —— 右栏 Inspector 的全局上下文（UI 文档 D3）。任何页面可 show(model) 推送对象详情到右栏，
// close() 收起。跨对象跳转（Finding↔Evidence↔Action↔Job↔Exchange）统一经此（D4）。

import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import type { InspectorModel } from "../components/domain/Inspector";

interface InspectorCtx {
  model: InspectorModel | null;
  show: (m: InspectorModel) => void;
  close: () => void;
  isOpen: boolean;
}

const Ctx = createContext<InspectorCtx | null>(null);

export function InspectorProvider({ children }: { children: ReactNode }) {
  const [model, setModel] = useState<InspectorModel | null>(null);
  const show = useCallback((m: InspectorModel) => setModel(m), []);
  const close = useCallback(() => setModel(null), []);
  const value = useMemo<InspectorCtx>(() => ({ model, show, close, isOpen: model != null }), [model, show, close]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useInspector(): InspectorCtx {
  const c = useContext(Ctx);
  if (!c) throw new Error("useInspector 必须在 <InspectorProvider> 内使用");
  return c;
}
