// router.tsx —— 轻量 hash 路由（不引三方依赖，与本仓「手写优于大依赖」取舍一致）。
// 支持静态段与 :param 动态段；useRoute 订阅 hashchange，navigate/Link 切换。

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

export interface RouteMatch {
  path: string; // 当前规范化路径，如 /runs/run-1/findings
  params: Record<string, string>;
}

function currentPath(): string {
  const h = typeof window !== "undefined" ? window.location.hash : "";
  const p = h.startsWith("#") ? h.slice(1) : h;
  return p === "" ? "/" : p;
}

const RouteCtx = createContext<{ path: string; navigate: (to: string) => void }>({
  path: "/",
  navigate: () => {},
});

export function RouterProvider({ children }: { children: ReactNode }) {
  const [path, setPath] = useState<string>(() => currentPath());
  useEffect(() => {
    const onHash = () => setPath(currentPath());
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);
  const navigate = useCallback((to: string) => {
    window.location.hash = to;
  }, []);
  const value = useMemo(() => ({ path, navigate }), [path, navigate]);
  return <RouteCtx.Provider value={value}>{children}</RouteCtx.Provider>;
}

export function useNavigate(): (to: string) => void {
  return useContext(RouteCtx).navigate;
}

export function usePath(): string {
  return useContext(RouteCtx).path;
}

// matchRoute：把 pattern（含 :param）与实际路径匹配，返回 params 或 null。
export function matchRoute(pattern: string, path: string): Record<string, string> | null {
  const ps = pattern.split("/").filter(Boolean);
  const as = path.split("/").filter(Boolean);
  if (ps.length !== as.length) return null;
  const params: Record<string, string> = {};
  for (let i = 0; i < ps.length; i++) {
    if (ps[i].startsWith(":")) {
      params[ps[i].slice(1)] = decodeURIComponent(as[i]);
    } else if (ps[i] !== as[i]) {
      return null;
    }
  }
  return params;
}

// useRoute：对一组 pattern 做首个匹配，返回 {pattern, params}。
export function useRoute(patterns: string[]): { pattern: string; params: Record<string, string> } | null {
  const path = usePath();
  return useMemo(() => {
    for (const pat of patterns) {
      const params = matchRoute(pat, path);
      if (params) return { pattern: pat, params };
    }
    return null;
  }, [patterns, path]);
}

// Link：声明式导航（a[href=#...]，支持 Ctrl/Cmd+点击新开）。
export function Link({ to, children, className, style, onClick }: {
  to: string;
  children: ReactNode;
  className?: string;
  style?: React.CSSProperties;
  onClick?: () => void;
}) {
  const navigate = useNavigate();
  return (
    <a
      href={"#" + to}
      className={className}
      style={style}
      onClick={(e) => {
        if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
        e.preventDefault();
        navigate(to);
        onClick?.();
      }}
    >
      {children}
    </a>
  );
}
