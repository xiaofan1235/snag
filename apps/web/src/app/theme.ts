import { useCallback, useEffect, useState } from "react";

export type Theme = "light" | "dark";

// useTheme：亮/暗切换，写 <html data-theme>。不入 LocalStorage 存敏感数据，仅存主题偏好（非敏感）。
export function useTheme(): [Theme, () => void] {
  const [theme, setTheme] = useState<Theme>(() => {
    if (typeof document !== "undefined") {
      const attr = document.documentElement.getAttribute("data-theme");
      if (attr === "dark" || attr === "light") return attr;
    }
    return "light";
  });

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
  }, [theme]);

  const toggle = useCallback(() => setTheme((t) => (t === "light" ? "dark" : "light")), []);
  return [theme, toggle];
}
