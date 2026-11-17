// P0 页面组（UI 文档 §16.34 P0）统一出口。
export { Dashboard } from "./dashboard/Dashboard";
export type { DashboardModel, DashboardProps, AttentionItem, RunningRun, RecentResult, SystemHealth } from "./dashboard/Dashboard";
export { RunWizard } from "./runs/RunWizard";
export type { RunWizardProps, WizardState, PreflightCheck } from "./runs/RunWizard";
export { Overview } from "./overview/Overview";
export type { OverviewModel } from "./overview/Overview";
export { ActivityStream, AgentSessionPanel } from "./activity/ActivityStream";
export type { ActivityStreamProps, AgentSessionModel } from "./activity/ActivityStream";
export { eventKind } from "./activity/eventKind";
