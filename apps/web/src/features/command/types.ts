// command/types.ts —— 编排中心作战视图的领域类型（ARTEX 式多 Agent 作战，风格用本仓暖纸墨底）。
// 数据来源：预览夹具（mockCommand）→ daemon 接线后由事件流/REST 提供。仅类型，无副作用。

// —— 多 Agent 会话树（主 Agent + 规划器 + 工人） ——
export type AgentRole = "main" | "planner" | "worker";
export type AgentState = "RUNNING" | "WAITING" | "BLOCKED" | "DONE" | "FAILED" | "PAUSED";

export interface AgentSession {
  id: string;
  role: AgentRole;
  title: string;
  state: AgentState;
  model: string; // 模型档位（profile）
  currentAction?: string; // 当前在做什么（Redacted Projection）
  tokensUsed: number;
  budgetPct?: number; // 该 Agent 续航（已用预算%）
  parentId?: string; // 构树；主 Agent 无 parent
  jobId?: string; // 关联 Job（可跳工作台 Jobs 细节）
  depth?: number; // 运行期由组件计算，非数据字段
}

export interface SessionTreeModel {
  sessions: AgentSession[]; // 扁平列表，parentId 构树
  focusedId: string;
}

// —— 编排对话 / 推理流（聚焦 Agent 的实时转录 + 人工转向） ——
export type OrchEntryKind =
  | "reasoning" // 模型思考（摘要）
  | "tool_call" // 发起工具调用
  | "tool_result" // 工具返回
  | "directive" // 人工下发的转向指令
  | "approval_request" // 命中拦截→待审批
  | "note"; // 系统/里程碑

export interface OrchEntry {
  id: string;
  kind: OrchEntryKind;
  at: string; // 时间戳（展示用）
  agentId: string; // 归属 Agent
  text: string;
  tool?: string; // tool_call/tool_result 的工具名
  tokens?: number; // 该步 token
  objectRef?: { type: string; id: string }; // 可跳 Inspector 的关联对象
}

export interface OrchestrationModel {
  entries: OrchEntry[];
  focusedId: string; // 当前聚焦 Agent（与 SessionTree 联动）
  canSteer: boolean; // 是否允许人工下发指令（暂停/运行态）
}

// —— 探索知识图谱（目标/意图/事实/发现，spawns/yields/proves 边） ——
export type GraphNodeKind = "goal" | "intent" | "fact" | "finding";
export type GraphEdgeKind = "spawns" | "yields" | "proves";

export interface GraphNode {
  id: string;
  kind: GraphNodeKind;
  label: string;
  state?: string; // 如 finding 的 REPORTABLE / CANDIDATE
  objectRef?: { type: string; id: string };
}

export interface GraphEdge {
  from: string;
  to: string;
  kind: GraphEdgeKind;
}

export interface ExplorationGraphModel {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

// —— 统一作战台（Run 级一屏：任务/发现/资产/流量/Token） ——
export interface DeckStat {
  key: string;
  label: string;
  value: string | number;
  sub?: string; // 次要说明
  tone?: "ok" | "warn" | "danger" | "info" | "unknown" | "accent";
}

export interface DeckActivity {
  id: string;
  at: string;
  label: string;
  kind: "finding" | "action" | "approval" | "coverage" | "budget" | "plan";
}

export interface DeckFinding {
  id: string;
  title: string;
  severity: "Critical" | "High" | "Medium" | "Low" | "Info";
  state: string;
}

export interface CommandDeckModel {
  stats: DeckStat[];
  tokenSeries: number[]; // 每点为一个时间窗的累计/增量 token（展示用 sparkline）
  tokenWindows?: string[]; // 可选的窗口标签
  activity: DeckActivity[];
  findings: DeckFinding[];
}

// —— 拦截规则 + 审批日志（守卫） ——
export type RuleDisposition = "approve" | "deny" | "allow";
export type MatchKind = "regex" | "string";

export interface InterceptionRule {
  id: string;
  match: string;
  matchKind: MatchKind;
  scope: string; // 作用域（如 工具/HTTP 方法/域）
  disposition: RuleDisposition;
  hits: number;
  enabled: boolean;
  note?: string;
}

export type ApprovalDecision = "approved" | "denied" | "denied_cancel" | "expired" | "pending";

export interface ApprovalLogEntry {
  id: string;
  at: string;
  action: string; // 被拦动作
  identity?: string;
  decision: ApprovalDecision;
  decidedBy?: string; // 决策者（人工/策略）
  reason?: string;
  ruleId?: string; // 命中的拦截规则
}

export interface GuardModel {
  rules: InterceptionRule[];
  log: ApprovalLogEntry[];
}
