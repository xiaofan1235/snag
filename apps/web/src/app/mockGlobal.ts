// mockGlobal.ts —— 全局页预览夹具（daemon 接线前）。
import type { ResourcesModel, OperationsModel, SettingsModel, InboxModel } from "../features/global";
import { previewApprovals } from "./mockData";

export const mkResources: ResourcesModel = {
  knowledge: [
    { id: "k1", level: "VERIFIED_FACT", text: "该站 /api/orders 按 Bearer 鉴权但不校验对象归属", source: "ev-1", techStack: "Node/Express", ttl: "30d", lastHit: "12:00", scope: "run" },
    { id: "k2", level: "OPERATOR_NOTE", text: "优先测 authz，recon 已充分", source: "owner", techStack: "-", ttl: "∞", lastHit: "11:58", scope: "run" },
    { id: "k3", level: "UNVERIFIED_REFERENCE", text: "Express 常见 IDOR 模式（方法知识）", source: "knowledge/global", techStack: "Express", ttl: "90d", lastHit: "11:50", scope: "global" },
  ],
  skills: [
    { id: "s1", name: "idor-probe", version: "1.2.0", source: "local", sha256: "aa11bb22", appliesTo: "EXPERIMENT", tools: ["http"], enabled: true },
    { id: "s2", name: "auth-enum", version: "0.9.1", source: "local", sha256: "cc33dd44", appliesTo: "RECON", tools: ["http"], enabled: false },
  ],
  templates: [
    { id: "t1", name: "SRC Web 标准", scope: "*.example.com", budget: "500 req / $5", tools: "http, browser" },
  ],
};

export const mkOperations: OperationsModel = {
  providers: [
    { id: "p1", type: "Anthropic", baseUrl: "https://api.anthropic.com", enabled: true, models: ["claude-opus", "claude-sonnet"], checkResult: "DNS✓ TLS✓ 认证✓ 模型✓" },
  ],
  profiles: [
    { slot: "Strategist", model: "claude-opus", fallback: "claude-sonnet", temperature: 0.2, maxOutput: 4096 },
    { slot: "Executor", model: "claude-sonnet", fallback: "claude-sonnet", temperature: 0.0, maxOutput: 2048 },
    { slot: "Verifier", model: "claude-opus", fallback: "-", temperature: 0.0, maxOutput: 2048 },
  ],
  identities: [
    { id: "i1", alias: "账号A", role: "user", tenant: "t1", auth: "Bearer（SecretRef）", expires: "12:50", lastValidated: "11:50" },
    { id: "i2", alias: "账号B", role: "user", tenant: "t2", auth: "Bearer（SecretRef）", expires: "12:50", lastValidated: "11:50" },
    { id: "i3", alias: "Anonymous", role: "-", tenant: "-", auth: "none", expires: "-", lastValidated: "-" },
  ],
  egress: [
    { id: "eg1", provider: "ProxyCo", region: "eu-west", quota: "充足", currentLease: "lease-1", health: "RUNNING", boundRun: "run-1" },
  ],
  sandboxes: [
    { id: "sbx-12", run: "run-1", imageDigest: "sha256:abcd…", state: "RUNNING", cpu: "8%", mem: "210MB", lastHeartbeat: "刚刚" },
  ],
  tools: [
    { id: "http", name: "http", version: "1.0", manifestHash: "mh-http", sideEffect: "read", successRate: "99.2%", enabled: true },
    { id: "browser", name: "browser", version: "1.0", manifestHash: "mh-br", sideEffect: "read", successRate: "97.0%", enabled: true },
  ],
  diagnostics: [
    { label: "进程版本", value: "snag 0.1.0", ok: true },
    { label: "数据库迁移", value: "0001–0016 已应用", ok: true },
    { label: "Outbox 积压", value: "0", ok: true },
    { label: "OutcomeUnknown", value: "0 未对账", ok: true },
    { label: "SSE 重连", value: "LIVE", ok: true },
  ],
  budget: { requestsUsed: 120, requestsMax: 500, costUsedCents: 80, costMaxCents: 500 },
};

export const mkSettings: SettingsModel = {
  preflight: [
    { group: "Authorization", item: "Scope 规范化", status: "PASS", detail: "app.example.com https:443" },
    { group: "Authorization", item: "授权时间窗", status: "PASS", detail: "在窗口内" },
    { group: "Provider", item: "认证 + 模型能力", status: "PASS", detail: "Anthropic 连接正常" },
    { group: "Identity", item: "账号 A/B 绑定", status: "PASS", detail: "SecretRef 可解析" },
    { group: "Egress", item: "付费代理 + 禁止直连", status: "PASS", detail: "lease-1 绑定" },
    { group: "Sandbox", item: "镜像 + 隔离 + 工具", status: "WARN", detail: "浏览器镜像较旧", fix: "更新 runner 镜像" },
    { group: "Storage", item: "SQLite + Blob + Vault", status: "PASS", detail: "WAL 正常" },
    { group: "Protocol", item: "Schema 版本 + 恢复能力", status: "PASS", detail: "契约兼容" },
  ],
};

export const mkInbox: InboxModel = {
  notifications: [
    { id: "n1", category: "APPROVAL_REQUIRED", summary: "DELETE /api/orders/42 待审批（run-1）", runId: "run-1", read: false, at: "11:59" },
    { id: "n2", category: "BUDGET_WARNING", summary: "run-1 请求预算已用 78%", runId: "run-1", read: false, at: "12:05" },
    { id: "n3", category: "FINDING_CONFIRMED", summary: "新确认 Finding：IDOR on /orders（High）", runId: "run-1", read: true, at: "12:10" },
    { id: "n4", category: "OUTCOME_UNKNOWN", summary: "job-5 一个动作结果未知，待对账", runId: "run-1", read: false, at: "12:06" },
  ],
  approvals: previewApprovals,
  blockers: [
    { id: "b1", runId: "run-2", reason: "代理 Lease 过期", scope: "run-2 全部目标 Egress", waitedFor: "4 分钟", recoveryActions: ["重新提取出口并绑定", "Preflight"], restartsJobs: ["job-2", "job-3"] },
  ],
};
