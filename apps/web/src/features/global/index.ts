// 全局页面（UI 文档 §16.20/§16.23/§16.24–16.29）统一出口。
export { KnowledgePanel, SkillsPanel, ToolsPanel } from "./ResourcesPage";
export type { ResourcesModel, KnowledgeItem, SkillItem, RunTemplate } from "./ResourcesPage";
export { OperationsPage } from "./OperationsPage";
export type { OperationsModel, ProviderItem, ModelProfile, IdentityItem, EgressPool, SandboxItem, ToolReg, DiagnosticsItem } from "./OperationsPage";
export { SettingsPage } from "./SettingsPage";
export type { SettingsModel, PreflightCheckRow, PreflightStatus } from "./SettingsPage";
export { InboxPage } from "./InboxPage";
export type { InboxPageProps, InboxModel, Notification, Blocker, NotifCategory } from "./InboxPage";
