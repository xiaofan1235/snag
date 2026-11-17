// nav.ts —— 界面地图（UI 文档 D4）。全局左导航 + Run 工作台子导航。
// 全部页面已实现为完整 UI（§16.4–16.29），当前以预览/mock 数据驱动，真实数据待 daemon 接线。
// implemented 标志保留：false 表示该页仍为只读空态占位（§16.34）。目前全部为 true。

export interface NavItem {
  to: string;
  label: string;
  icon: string; // 线性图标占位符号
  badge?: number; // 待办/阻塞角标
  implemented?: boolean; // false → 空态占位页
}

// 全局左导航（D4）。
export const GLOBAL_NAV: NavItem[] = [
  { to: "/", label: "Dashboard", icon: "◫", implemented: true },
  { to: "/runs", label: "Runs", icon: "▤", implemented: true },
  { to: "/runs/new", label: "新建 Run", icon: "＋", implemented: true },
  { to: "/inbox", label: "Inbox", icon: "✉", implemented: true },
  { to: "/resources", label: "Resources", icon: "▦", implemented: true },
  { to: "/operations", label: "Operations", icon: "⚙", implemented: true },
  { to: "/settings", label: "Settings", icon: "⚑", implemented: true },
];

// Run 工作台子导航（D4，进入某 Run 后）。runId 替换 :id。
export interface RunNavItem extends NavItem {
  seg: string; // 路径段
}

export const RUN_NAV: RunNavItem[] = [
  { seg: "overview", to: "", label: "Overview", icon: "◉", implemented: true },
  { seg: "activity", to: "", label: "Activity", icon: "≡", implemented: true },
  { seg: "strategy", to: "", label: "Strategy", icon: "✦", implemented: true },
  { seg: "jobs", to: "", label: "Agent/Jobs", icon: "⬡", implemented: true },
  { seg: "surface", to: "", label: "Surface", icon: "◈", implemented: true },
  { seg: "hypotheses", to: "", label: "Hypotheses", icon: "☰", implemented: true },
  { seg: "findings", to: "", label: "Findings", icon: "⚑", implemented: true },
  { seg: "traffic", to: "", label: "Traffic", icon: "⇅", implemented: true },
  { seg: "evidence", to: "", label: "Evidence", icon: "▣", implemented: true },
  { seg: "browser", to: "", label: "Browser", icon: "◎", implemented: true },
  { seg: "tools", to: "", label: "Tools", icon: "⚒", implemented: true },
  { seg: "files", to: "", label: "Files", icon: "▤", implemented: true },
  { seg: "coverage", to: "", label: "Coverage", icon: "▦", implemented: true },
  { seg: "report", to: "", label: "Report", icon: "▧", implemented: true },
  { seg: "timeline", to: "", label: "Timeline", icon: "◷", implemented: true },
];

export function runNavTo(runId: string, seg: string): string {
  return `/runs/${runId}/${seg}`;
}
