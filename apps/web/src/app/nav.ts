// nav.ts —— 界面地图（UI 文档 D4）。全局左导航 + Run 工作台子导航。
// 未实现页走只读空态占位（§16.34），不以假数据冒充完成。

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
  { to: "/resources", label: "Resources", icon: "▦", implemented: false },
  { to: "/operations", label: "Operations", icon: "⚙", implemented: false },
  { to: "/settings", label: "Settings", icon: "⚑", implemented: false },
];

// Run 工作台子导航（D4，进入某 Run 后）。runId 替换 :id。
export interface RunNavItem extends NavItem {
  seg: string; // 路径段
}

export const RUN_NAV: RunNavItem[] = [
  { seg: "overview", to: "", label: "Overview", icon: "◉", implemented: true },
  { seg: "activity", to: "", label: "Activity", icon: "≡", implemented: true },
  { seg: "strategy", to: "", label: "Strategy", icon: "✦", implemented: false },
  { seg: "jobs", to: "", label: "Agent/Jobs", icon: "⬡", implemented: false },
  { seg: "surface", to: "", label: "Surface", icon: "◈", implemented: false },
  { seg: "hypotheses", to: "", label: "Hypotheses", icon: "☰", implemented: false },
  { seg: "findings", to: "", label: "Findings", icon: "⚑", implemented: true },
  { seg: "traffic", to: "", label: "Traffic", icon: "⇅", implemented: false },
  { seg: "evidence", to: "", label: "Evidence", icon: "▣", implemented: false },
  { seg: "coverage", to: "", label: "Coverage", icon: "▦", implemented: true },
  { seg: "report", to: "", label: "Report", icon: "▧", implemented: true },
  { seg: "timeline", to: "", label: "Timeline", icon: "◷", implemented: false },
];

export function runNavTo(runId: string, seg: string): string {
  return `/runs/${runId}/${seg}`;
}
