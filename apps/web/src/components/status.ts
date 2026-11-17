// status.ts —— 全站统一状态映射（UI 文档 D2.6）。关键不变量：Run/Job/Finding 状态语义在所有页面
// 用这**一套**枚举与配色，不允许同一状态在不同页用不同颜色/文案。颜色+图标+文字三者齐全（色盲可辨）。

// 语义色族（对应 Token 变量名后缀）。
export type StatusTone = "ok" | "warn" | "danger" | "accent" | "info" | "ink-2" | "ink-3" | "unknown";

export interface StatusMeta {
  tone: StatusTone;
  label: string;
  icon: string; // 线性图标名（currentColor 继承）；此处用简短符号占位，Day 40+ 换 Lucide
  pulse?: boolean; // 轻脉动（RUNNING）
  hatched?: boolean; // 斜纹（不确定态）
}

// 规范映射表（D2.6）：终态 / 非终态 / 不确定三组。
export const STATUS_MAP: Record<string, StatusMeta> = {
  // 终态
  COMPLETED: { tone: "ok", label: "已完成", icon: "check" },
  PARTIAL: { tone: "warn", label: "部分完成", icon: "slash" },
  STOPPED: { tone: "ink-3", label: "已停止", icon: "square" },
  FAILED: { tone: "danger", label: "失败", icon: "x" },
  // 非终态
  RUNNING: { tone: "accent", label: "运行中", icon: "play", pulse: true },
  PAUSED: { tone: "ink-2", label: "已暂停", icon: "pause" },
  BLOCKED: { tone: "warn", label: "受阻", icon: "lock" },
  QUEUED: { tone: "info", label: "排队中", icon: "clock" },
  // 不确定（一等公民）
  OUTCOME_UNKNOWN: { tone: "unknown", label: "结果未知", icon: "help", hatched: true },
  UNABLE_TO_VERIFY: { tone: "unknown", label: "无法验证", icon: "help", hatched: true },
  UNCONFIRMED: { tone: "unknown", label: "未确认", icon: "help", hatched: true },
};

// 兜底：未登记状态以 unknown 呈现（不静默吞掉陌生状态）。
export function statusMeta(state: string): StatusMeta {
  return STATUS_MAP[state] ?? { tone: "unknown", label: state, icon: "help", hatched: true };
}

// 全部已登记状态键（测试覆盖所有枚举用）。
export const ALL_STATUSES = Object.keys(STATUS_MAP);
