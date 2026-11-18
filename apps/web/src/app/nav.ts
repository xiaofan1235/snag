// nav.ts —— 界面地图（UI 文档 D4 + ARTEX 式编排中心重构）。全局左导航 + Run 主模式 + 细节子导航。
// 重构（2026-11）：Run 工作台从 15 个平铺标签改为「主模式（指挥台/探索图谱/作战台/守卫）+ 细节层」。
// 原 15 页全部保留，降为「细节」模式下的二级导航（不删，仅重组 §16.34 全部已实现）。

export interface NavItem {
  to: string;
  label: string;
  icon: string; // 线性图标占位符号
  badge?: number; // 待办/阻塞角标
  implemented?: boolean; // false → 空态占位页（当前全部 true）
}

// 全局左导航（D4）。
export const GLOBAL_NAV: NavItem[] = [
  { to: "/", label: "值班台", icon: "◫", implemented: true },
  { to: "/runs", label: "运行", icon: "▤", implemented: true },
  { to: "/runs/new", label: "新建运行", icon: "＋", implemented: true },
  { to: "/inbox", label: "收件箱", icon: "✉", implemented: true },
  { to: "/knowledge", label: "知识库", icon: "▦", implemented: true },
  { to: "/skills", label: "技能", icon: "✦", implemented: true },
  { to: "/tools", label: "工具", icon: "⚒", implemented: true },
  { to: "/operations", label: "运维", icon: "⚙", implemented: true },
  { to: "/settings", label: "设置", icon: "⚑", implemented: true },
];

export interface RunNavItem extends NavItem {
  seg: string; // 路径段
}

// Run 主模式（ARTEX 式作战视图入口）。默认进入「指挥台」。
export const RUN_MODES: RunNavItem[] = [
  { seg: "command", to: "", label: "指挥台", icon: "◉", implemented: true },
  { seg: "graph", to: "", label: "探索图谱", icon: "✦", implemented: true },
  { seg: "deck", to: "", label: "作战台", icon: "▦", implemented: true },
  { seg: "guard", to: "", label: "守卫", icon: "⛉", implemented: true },
  { seg: "detail", to: "", label: "细节", icon: "≣", implemented: true },
];

// 主模式对应的实际视图段（「detail」是容器，落到具体细节段）。
export const RUN_MODE_SEGS = ["command", "graph", "deck", "guard"] as const;
export type RunModeSeg = (typeof RUN_MODE_SEGS)[number];
export function isRunModeSeg(seg: string): seg is RunModeSeg {
  return (RUN_MODE_SEGS as readonly string[]).includes(seg);
}

// 细节层二级导航（原 15 页，全部保留为细节视图）。
export const RUN_NAV: RunNavItem[] = [
  { seg: "overview", to: "", label: "概览", icon: "◉", implemented: true },
  { seg: "activity", to: "", label: "活动", icon: "≡", implemented: true },
  { seg: "strategy", to: "", label: "策略", icon: "✦", implemented: true },
  { seg: "jobs", to: "", label: "智能体/作业", icon: "⬡", implemented: true },
  { seg: "surface", to: "", label: "攻击面", icon: "◈", implemented: true },
  { seg: "hypotheses", to: "", label: "假设", icon: "☰", implemented: true },
  { seg: "findings", to: "", label: "发现", icon: "⚑", implemented: true },
  { seg: "traffic", to: "", label: "流量", icon: "⇅", implemented: true },
  { seg: "evidence", to: "", label: "证据", icon: "▣", implemented: true },
  { seg: "browser", to: "", label: "浏览器", icon: "◎", implemented: true },
  { seg: "tools", to: "", label: "工具", icon: "⚒", implemented: true },
  { seg: "files", to: "", label: "文件", icon: "▤", implemented: true },
  { seg: "coverage", to: "", label: "覆盖", icon: "▦", implemented: true },
  { seg: "report", to: "", label: "报告", icon: "▧", implemented: true },
  { seg: "timeline", to: "", label: "时间线", icon: "◷", implemented: true },
];

// Run 的默认进入段（主模式·指挥台）。
export const RUN_DEFAULT_SEG = "command";
// 「细节」模式点击时落到的默认细节段。
export const RUN_DETAIL_DEFAULT = "overview";

export function runNavTo(runId: string, seg: string): string {
  return `/runs/${runId}/${seg}`;
}
