// SessionTree —— 多 Agent 会话树（ARTEX 式作战视图左栏）。主 Agent → 规划器 → 工人会话，
// 层级缩进、可聚焦；每节点示角色、状态、当前动作、token/续航。聚焦联动编排对话（center）。

import type { AgentSession, AgentState, SessionTreeModel } from "./types";

const ROLE_LABEL: Record<AgentSession["role"], string> = {
  main: "主",
  planner: "规划",
  worker: "工人",
};

const STATE_TONE: Record<AgentState, string> = {
  RUNNING: "info",
  WAITING: "unknown",
  BLOCKED: "warn",
  DONE: "ok",
  FAILED: "danger",
  PAUSED: "unknown",
};

const STATE_LABEL: Record<AgentState, string> = {
  RUNNING: "运行中",
  WAITING: "等待",
  BLOCKED: "阻塞",
  DONE: "完成",
  FAILED: "失败",
  PAUSED: "暂停",
};

// 扁平列表按 parentId 构树，深度优先展开为有序行（带 depth）。
function flattenTree(sessions: AgentSession[]): AgentSession[] {
  const byParent = new Map<string | undefined, AgentSession[]>();
  for (const s of sessions) {
    const key = s.parentId;
    if (!byParent.has(key)) byParent.set(key, []);
    byParent.get(key)!.push(s);
  }
  const out: AgentSession[] = [];
  const walk = (parentId: string | undefined, depth: number) => {
    for (const s of byParent.get(parentId) ?? []) {
      out.push({ ...s, depth });
      walk(s.id, depth + 1);
    }
  };
  walk(undefined, 0);
  return out;
}

export interface SessionTreeProps {
  model: SessionTreeModel;
  onFocus?: (id: string) => void;
  onOpenJob?: (jobId: string) => void;
}

export function SessionTree({ model, onFocus, onOpenJob }: SessionTreeProps) {
  const rows = flattenTree(model.sessions);
  const workers = model.sessions.filter((s) => s.role === "worker").length;

  return (
    <div className="sn-sessiontree" aria-label="多 Agent 会话树">
      <div className="sn-sessiontree-head">
        <span>会话树</span>
        <span className="sn-badge" data-tone="ink-3">{workers} 工人</span>
      </div>
      <div role="tree" aria-label="Agent 会话">
        {rows.map((s) => {
          const focused = s.id === model.focusedId;
          return (
            <div
              key={s.id}
              role="treeitem"
              aria-selected={focused}
              aria-level={(s.depth ?? 0) + 1}
              tabIndex={0}
              className="sn-session-node sn-focus"
              data-role={s.role}
              data-focused={focused || undefined}
              style={{ paddingLeft: `calc(var(--sp-3) + ${(s.depth ?? 0) * 16}px)` }}
              onClick={() => onFocus?.(s.id)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  onFocus?.(s.id);
                }
              }}
            >
              <div className="sn-session-row">
                <span className="sn-session-role" data-role={s.role}>{ROLE_LABEL[s.role]}</span>
                <span className="sn-session-title" title={s.title}>{s.title}</span>
                <span className="sn-badge" data-tone={STATE_TONE[s.state]} title={STATE_LABEL[s.state]}>
                  {STATE_LABEL[s.state]}
                </span>
              </div>
              {s.currentAction && (
                <div className="sn-session-action" title={s.currentAction}>
                  ▸ {s.currentAction}
                </div>
              )}
              <div className="sn-session-meta">
                <span className="mono">{s.model}</span>
                <span>· {s.tokensUsed.toLocaleString()} tok</span>
                {typeof s.budgetPct === "number" && (
                  <span className="sn-session-gauge" title={`续航：已用 ${s.budgetPct}%`}>
                    <span style={{ width: `${Math.min(100, s.budgetPct)}%` }} />
                  </span>
                )}
                {s.jobId && onOpenJob && (
                  <button
                    className="sn-link-btn"
                    onClick={(e) => {
                      e.stopPropagation();
                      onOpenJob(s.jobId!);
                    }}
                  >
                    作业
                  </button>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
