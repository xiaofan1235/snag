// AppShell —— 应用布局（UI 文档 D3 调整版）：顶 Agent 状态条 / 左导航 / 中内容 / 右 Inspector。
// 左导航带待办角标；右 Inspector 随选中对象。原底部全局「指令输入」已删除（无功能且全局无 run
// 归属，语义含糊）——下发指令改由指挥台「与 Agent 对话」按 run/聚焦 Agent 进行；连接同步状态并入顶栏。

import type { ReactNode } from "react";
import { GLOBAL_NAV } from "./nav";
import { usePath, useNavigate } from "./router";
import { useInspector } from "./inspector";
import { Inspector } from "../components/domain/Inspector";
import { Button } from "../components/Button";
import { ConnectionStatus } from "../components/ConnectionStatus";
import { previewAgentStatus, previewDashboard } from "./mockData";
import { useTheme } from "./theme";

function navActive(path: string, to: string): boolean {
  if (to === "/") return path === "/";
  // 新建 Run 与 Runs 列表都以 /runs 开头，精确区分 /runs/new。
  if (to === "/runs") return path === "/runs" || (path.startsWith("/runs/") && path !== "/runs/new");
  return path === to || path.startsWith(to + "/");
}

export function AppShell({ children }: { children: ReactNode }) {
  const path = usePath();
  const insp = useInspector();
  const navigate = useNavigate();
  const [theme, toggleTheme] = useTheme();
  const st = previewAgentStatus;
  const attention = previewDashboard.attention.length;

  return (
    <div className="app-shell">
      {/* 顶：Agent 状态条（一行，平静） */}
      <header className="app-statusbar">
        <span className="live-dot" aria-hidden="true" />
        <span>{st.doing}</span>
        <span className="sep">·</span>
        <span>Epoch {st.epoch}</span>
        <span className="sep">·</span>
        <span title="续航：预算余量" style={{ display: "flex", alignItems: "center", gap: "var(--sp-1)" }}>
          续航
          <span className="gauge">
            <span style={{ width: `${100 - st.endurancePct}%` }} />
          </span>
          {100 - st.endurancePct}%
        </span>
        <span className="sep">·</span>
        <span style={{ display: "flex", alignItems: "center", gap: "var(--sp-1)" }}>
          代理 <ConnectionStatus state={st.proxy === "RUNNING" ? "LIVE" : "STALE"} />
        </span>
        <span className="sep">·</span>
        <span style={{ display: "flex", alignItems: "center", gap: "var(--sp-1)" }}>
          同步 <ConnectionStatus state="LIVE" />
        </span>
        <span style={{ marginLeft: "auto", display: "flex", gap: "var(--sp-2)" }}>
          <Button size="sm" variant="ghost" onClick={toggleTheme} aria-label="切换主题">
            {theme === "light" ? "☾" : "☀"}
          </Button>
          <Button size="sm" variant="ghost">暂停</Button>
          <Button size="sm" variant="danger">停止</Button>
        </span>
      </header>

      {/* 中部三栏 */}
      <div className="app-body" data-inspector={insp.isOpen ? "open" : "closed"}>
        <nav className="app-nav" aria-label="主导航">
          {GLOBAL_NAV.map((item) => {
            const active = navActive(path, item.to);
            return (
              <a
                key={item.to}
                href={"#" + item.to}
                className="app-nav-item"
                data-active={active || undefined}
                aria-current={active ? "page" : undefined}
                onClick={(e) => {
                  if (e.metaKey || e.ctrlKey || e.button !== 0) return;
                  e.preventDefault();
                  navigate(item.to);
                }}
              >
                <span className="nav-label">{item.label}</span>
                {item.to === "/inbox" && attention > 0 && <span className="nav-badge">{attention}</span>}
              </a>
            );
          })}
        </nav>

        <main className="app-main">{children}</main>

        {insp.isOpen && (
          <aside className="app-inspector" aria-label="检视器">
            <div className="app-inspector-head">
              <strong style={{ fontSize: "var(--fs-sm)" }}>检视器</strong>
              <Button size="sm" variant="ghost" iconOnly aria-label="关闭 Inspector" onClick={insp.close}>✕</Button>
            </div>
            <Inspector model={insp.model} onNavigate={(ref) => navigate(`/runs/run-1/${ref.type === "finding" ? "findings" : ref.type}`)} />
          </aside>
        )}
      </div>
    </div>
  );
}
