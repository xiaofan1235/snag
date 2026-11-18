// mockCommand.ts —— 编排中心作战视图的预览夹具（daemon 接线前）。与 run-1「acme.com authz / IDOR on
// /orders」故事线一致。接线后由事件流（会话/推理/图谱/审批）替换；当前仅预览，不以假数据冒充完成（§16.34）。

import type {
  SessionTreeModel, OrchestrationModel, ExplorationGraphModel, CommandDeckModel, GuardModel, DeckStat,
} from "../features/command";

// —— 全局作战总览（跨 Run 聚合，ARTEX 式一屏指标；放在值班台顶部） ——
export const mkGlobalStats: DeckStat[] = [
  { key: "runs", label: "活跃 Run", value: 2, sub: "3 历史", tone: "info" },
  { key: "agents", label: "Agent 会话", value: 9, sub: "跨 Run", tone: "accent" },
  { key: "findings", label: "确认发现", value: 5, sub: "11 候选", tone: "danger" },
  { key: "assets", label: "端点 / 资产", value: 72, sub: "31 已测", tone: "ok" },
  { key: "traffic", label: "录制流量", value: 486, sub: "MITM exchanges", tone: "unknown" },
  { key: "token", label: "Token 消耗", value: "128k", sub: "$1.84", tone: "warn" },
];
export const mkGlobalTokenWindows = ["一", "二", "三", "四", "五", "六", "日"];
export const mkGlobalTokenSeries = [12400, 18200, 15600, 22100, 19800, 8400, 31500];

// —— 多 Agent 会话树：主 + 规划 + 5 工人（对齐 ARTEX task session view） ——
export const mkSessionTree: SessionTreeModel = {
  focusedId: "w1",
  sessions: [
    { id: "main", role: "main", title: "主 Agent · 编排", state: "RUNNING", model: "default", currentAction: "编排 authz 攻击面，监督工人", tokensUsed: 18400, budgetPct: 24 },
    { id: "planner", role: "planner", title: "规划器", state: "RUNNING", model: "default", currentAction: "分解 /api/orders 实验为变体序列", tokensUsed: 9200, budgetPct: 12, parentId: "main" },
    { id: "w1", role: "worker", title: "authz · /api/orders IDOR", state: "RUNNING", model: "fast", currentAction: "重放 DELETE 变体（命中拦截→待审批）", tokensUsed: 6100, budgetPct: 9, parentId: "planner", jobId: "job-2" },
    { id: "w2", role: "worker", title: "recon · /api/v2/*", state: "WAITING", model: "fast", currentAction: "等待调度配额", tokensUsed: 2400, budgetPct: 3, parentId: "planner", jobId: "job-3" },
    { id: "w3", role: "worker", title: "evidence · 采集 CEBS", state: "DONE", model: "fast", currentAction: "已归档 ev-1（脱敏投影）", tokensUsed: 1800, parentId: "planner", jobId: "job-4" },
    { id: "w4", role: "worker", title: "browser · 登录态验证", state: "BLOCKED", model: "default", currentAction: "人工接管审批中", tokensUsed: 3300, budgetPct: 5, parentId: "planner", jobId: "job-5" },
    { id: "w5", role: "worker", title: "sqli · /api/orders probe", state: "FAILED", model: "fast", currentAction: "无注入点，反证降级", tokensUsed: 900, parentId: "planner", jobId: "job-6" },
  ],
};

// —— 编排对话 / 推理流（聚焦 w1，含人工指令与待审批） ——
export const mkOrchestration: OrchestrationModel = {
  focusedId: "w1",
  canSteer: true,
  entries: [
    { id: "e1", kind: "note", at: "09:02", agentId: "main", text: "Run 启动，锚定审计链（seq=1 run_created）" },
    { id: "e2", kind: "reasoning", at: "09:03", agentId: "main", text: "authz 模块优先：/api/orders 存在对象归属风险，派发规划器分解", tokens: 420 },
    { id: "e3", kind: "note", at: "09:04", agentId: "planner", text: "分解为 5 个 worker：IDOR / recon / evidence / browser / sqli" },
    { id: "e4", kind: "directive", at: "09:06", agentId: "w1", text: "【人工】聚焦 /api/orders，先读后写，写操作必须等我确认" },
    { id: "e5", kind: "tool_call", at: "09:07", agentId: "w1", tool: "http", text: "GET /api/orders/42  身份=账号A", tokens: 110 },
    { id: "e6", kind: "tool_result", at: "09:07", agentId: "w1", tool: "http", text: "200 OK · 响应体含账号B 订单 42（跨身份读取成立）", tokens: 180, objectRef: { type: "evidence", id: "ev-1" } },
    { id: "e7", kind: "reasoning", at: "09:08", agentId: "w1", text: "对象归属校验缺失，升级为候选 Finding，请求独立 Verifier 复核", tokens: 260, objectRef: { type: "finding", id: "f-1" } },
    { id: "e8", kind: "tool_call", at: "09:10", agentId: "w1", tool: "http", text: "DELETE /api/orders/42（破坏性变体）", tokens: 90 },
    { id: "e9", kind: "approval_request", at: "09:10", agentId: "w1", text: "命中写操作拦截规则 r1 → 待人工审批（未执行）", objectRef: { type: "finding", id: "f-1" } },
    { id: "e10", kind: "tool_call", at: "09:11", agentId: "w2", tool: "crawl", text: "枚举 /api/v2/*（recon）", tokens: 140 },
    { id: "e11", kind: "reasoning", at: "09:12", agentId: "main", text: "IDOR 证据充分，指示报告器纳入 REPORTABLE；sqli worker 无注入点，降级", tokens: 300 },
  ],
};

// —— 探索知识图谱：目标→意图→事实→发现 ——
export const mkGraph: ExplorationGraphModel = {
  nodes: [
    { id: "g1", kind: "goal", label: "验证 acme.com 授权控制" },
    { id: "i1", kind: "intent", label: "探测 /api/orders 对象归属" },
    { id: "i2", kind: "intent", label: "recon /api/v2 新端点" },
    { id: "i3", kind: "intent", label: "验证会话令牌轮换" },
    { id: "fa1", kind: "fact", label: "账号A 读到账号B 订单42", objectRef: { type: "evidence", id: "ev-1" } },
    { id: "fa2", kind: "fact", label: "/api/v2/users 可枚举" },
    { id: "fa3", kind: "fact", label: "登录后 token 未轮换" },
    { id: "fd1", kind: "finding", label: "IDOR on /orders", state: "REPORTABLE", objectRef: { type: "finding", id: "f-1" } },
    { id: "fd2", kind: "finding", label: "可能的用户枚举", state: "CANDIDATE" },
  ],
  edges: [
    { from: "g1", to: "i1", kind: "spawns" },
    { from: "g1", to: "i2", kind: "spawns" },
    { from: "g1", to: "i3", kind: "spawns" },
    { from: "i1", to: "fa1", kind: "yields" },
    { from: "i2", to: "fa2", kind: "yields" },
    { from: "i3", to: "fa3", kind: "yields" },
    { from: "fa1", to: "fd1", kind: "proves" },
    { from: "fa2", to: "fd2", kind: "proves" },
  ],
};

// —— 统一作战台（Run 级一屏） ——
export const mkDeck: CommandDeckModel = {
  stats: [
    { key: "jobs", label: "活跃 Jobs", value: 3, sub: "7 Agent 会话", tone: "info" },
    { key: "findings", label: "确认发现", value: 2, sub: "6 候选", tone: "accent" },
    { key: "assets", label: "端点 / 资产", value: 30, sub: "12 已测", tone: "ok" },
    { key: "traffic", label: "录制流量", value: 148, sub: "MITM exchanges", tone: "unknown" },
    { key: "token", label: "Token 消耗", value: "42.1k", sub: "续航余量 76%", tone: "warn" },
  ],
  tokenWindows: ["09", "10", "11", "12", "13", "14", "15", "16"],
  tokenSeries: [1200, 3400, 2800, 5100, 4300, 6200, 3900, 7400],
  activity: [
    { id: "a1", at: "12:28", kind: "approval", label: "拒绝并取消：DELETE /api/orders/42（越权写）" },
    { id: "a2", at: "11:40", kind: "finding", label: "确认 Finding：IDOR on /orders（High）" },
    { id: "a3", at: "11:02", kind: "coverage", label: "覆盖门禁：authz 模块 TESTED 12/30" },
    { id: "a4", at: "10:30", kind: "action", label: "采集证据 ev-1（CEBS，脱敏投影）" },
    { id: "a5", at: "09:50", kind: "budget", label: "预算告警：请求预算已用 24%" },
  ],
  findings: [
    { id: "f-1", title: "IDOR on /orders", severity: "High", state: "REPORTABLE" },
    { id: "f-2", title: "可能的用户枚举 /api/v2/users", severity: "Medium", state: "CANDIDATE" },
  ],
};

// —— 守卫：拦截规则 + 审批日志 ——
export const mkGuard: GuardModel = {
  rules: [
    { id: "r1", match: "DELETE|PUT|PATCH", matchKind: "regex", scope: "HTTP 写方法", disposition: "approve", hits: 4, enabled: true, note: "破坏性写操作一律人工审批（§6.3）" },
    { id: "r2", match: "/admin", matchKind: "string", scope: "路径前缀", disposition: "approve", hits: 1, enabled: true },
    { id: "r3", match: "password|secret|token", matchKind: "regex", scope: "出站 body", disposition: "deny", hits: 2, enabled: true, note: "疑似凭据外泄→直接拒绝" },
    { id: "r4", match: "169.254.169.254", matchKind: "string", scope: "云元数据地址", disposition: "deny", hits: 0, enabled: true },
    { id: "r5", match: "GET|HEAD", matchKind: "regex", scope: "HTTP 读方法", disposition: "allow", hits: 120, enabled: true },
  ],
  log: [
    { id: "l1", at: "12:28", action: "DELETE /api/orders/42", identity: "账号A", decision: "denied_cancel", decidedBy: "人工", reason: "越权写，终止该 Job", ruleId: "r1" },
    { id: "l2", at: "11:10", action: "GET /admin/users", identity: "账号A", decision: "approved", decidedBy: "人工", reason: "范围内只读核查", ruleId: "r2" },
    { id: "l3", at: "10:30", action: "POST webhook（含 token）", identity: "账号A", decision: "denied", decidedBy: "策略", reason: "命中凭据外泄规则", ruleId: "r3" },
    { id: "l4", at: "09:40", action: "PUT /api/orders/42/status", identity: "账号A", decision: "expired", decidedBy: "—", reason: "审批超时自动失效", ruleId: "r1" },
  ],
};
