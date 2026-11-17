// ActivityStream —— 实时活动流（UI 文档 §16.13）。统一展示 Planner/Executor/Verifier/Runner/Policy/用户
// 行为的 Typed 事件卡。Live Follow 自动滚动；用户向上阅读时暂停跟随并显示「有 N 条新事件」。
// 不展示原始隐藏思维链（只展示简短计划/结论/可审计理由——由事件 payload 决定，本组件不含 CoT）。
// 支持从任一事件跳转到对应 Job/Exchange/Evidence/Finding（onJump），不能从旧事件分叉执行状态。

import { useEffect, useRef, useState } from "react";
import type { EventEnvelope } from "../../api/schema.gen";
import { EventCard } from "../../components/domain/EventCard";
import { eventKind } from "./eventKind";

export interface ActivityStreamProps {
  events: EventEnvelope[];
  /** 从事件跳转到对应对象。 */
  onJump?: (ev: EventEnvelope) => void;
  /** 打开 Agent Session Drawer。 */
  onOpenSession?: (ev: EventEnvelope) => void;
}

function title(ev: EventEnvelope): string {
  return ev.event_type;
}

export function ActivityStream({ events, onJump, onOpenSession }: ActivityStreamProps) {
  const [following, setFollowing] = useState(true);
  const [seenCount, setSeenCount] = useState(events.length);
  const scrollRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  // 新事件到达：跟随则滚到底并同步 seenCount；否则累积「有 N 条新事件」。
  useEffect(() => {
    if (following) {
      bottomRef.current?.scrollIntoView?.({ block: "end" });
      setSeenCount(events.length);
    }
  }, [events.length, following]);

  const pending = events.length - seenCount;

  function onScroll() {
    const el = scrollRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
    // 用户向上阅读 → 暂停跟随；回到底部 → 恢复跟随。
    setFollowing(atBottom);
    if (atBottom) setSeenCount(events.length);
  }

  function resumeFollow() {
    setFollowing(true);
    setSeenCount(events.length);
    bottomRef.current?.scrollIntoView?.({ block: "end" });
  }

  return (
    <div style={{ position: "relative" }}>
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-2)" }}>
        <label style={{ display: "flex", alignItems: "center", gap: "var(--sp-1)", fontSize: "var(--fs-sm)" }}>
          <input type="checkbox" checked={following} onChange={(e) => setFollowing(e.target.checked)} />
          Live Follow
        </label>
        <span style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>{events.length} 条事件</span>
      </div>

      <div
        ref={scrollRef}
        onScroll={onScroll}
        data-testid="activity-scroll"
        style={{ maxHeight: 420, overflow: "auto", display: "flex", flexDirection: "column", gap: "var(--sp-2)" }}
      >
        {events.map((ev, i) => (
          <EventCard
            key={`${ev.sequence}-${i}`}
            kind={eventKind(ev.event_type)}
            title={title(ev)}
            onJump={onJump ? () => onJump(ev) : undefined}
            jumpLabel="跳转到对象 →"
          >
            {onOpenSession && (
              <button className="sn-btn sn-focus" data-variant="link" data-size="sm" onClick={() => onOpenSession(ev)}>
                查看 Agent Session
              </button>
            )}
          </EventCard>
        ))}
        <div ref={bottomRef} />
      </div>

      {!following && pending > 0 && (
        <button
          className="sn-btn sn-focus"
          data-variant="primary"
          data-size="sm"
          style={{ position: "absolute", bottom: "var(--sp-3)", left: "50%", transform: "translateX(-50%)" }}
          onClick={resumeFollow}
        >
          有 {pending} 条新事件 ↓
        </button>
      )}
    </div>
  );
}

// AgentSessionModel —— Agent Session Drawer 内容（§16.13）。展示角色/Profile/Job/Attempt/Generation/
// 上下文 Revision/压缩次数/Token/当前工具集/最近输入摘要/最后错误。上下文查看器展示 Redacted Projection。
export interface AgentSessionModel {
  sessionId: string;
  role: string;
  modelProfile: string;
  jobId: string;
  attempt: number;
  generation: number;
  contextRevision: number;
  compactions: number;
  tokenUsed: number;
  tools: string[];
  lastInputSummary?: string;
  lastError?: string;
}

export function AgentSessionPanel({ model }: { model: AgentSessionModel }) {
  const rows: Array<[string, React.ReactNode]> = [
    ["角色", model.role],
    ["Model Profile", model.modelProfile],
    ["Job / Attempt", `${model.jobId} / #${model.attempt}`],
    ["Generation", model.generation],
    ["上下文 Revision", model.contextRevision],
    ["压缩次数", model.compactions],
    ["Token 用量", model.tokenUsed],
    ["当前工具集", model.tools.join(", ")],
  ];
  return (
    <div data-testid="agent-session">
      <dl style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: "var(--sp-1) var(--sp-3)", margin: 0 }}>
        {rows.map(([k, v]) => (
          <div key={k} style={{ display: "contents" }}>
            <dt style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>{k}</dt>
            <dd style={{ margin: 0, fontSize: "var(--fs-sm)" }} className="mono">
              {v}
            </dd>
          </div>
        ))}
      </dl>
      {model.lastInputSummary && (
        <div style={{ marginTop: "var(--sp-3)" }}>
          <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>最近输入摘要（Redacted Projection）</div>
          <div style={{ fontSize: "var(--fs-sm)", color: "var(--ink-2)" }}>{model.lastInputSummary}</div>
        </div>
      )}
      {model.lastError && (
        <div style={{ marginTop: "var(--sp-2)", color: "var(--danger)", fontSize: "var(--fs-sm)" }}>最后错误：{model.lastError}</div>
      )}
    </div>
  );
}
