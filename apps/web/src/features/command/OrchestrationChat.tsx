// OrchestrationChat —— 编排对话 / 推理流（ARTEX 式作战视图中栏）。聚焦 Agent 的实时转录：
// 推理/工具调用/返回/人工指令/待审批 分型渲染；底部人工转向输入（危险指令仍走审批 §16.7）。

import { useMemo, useState } from "react";
import type { OrchEntry, OrchEntryKind, OrchestrationModel } from "./types";

const KIND_META: Record<OrchEntryKind, { label: string; tone: string }> = {
  reasoning: { label: "推理", tone: "ink-3" },
  tool_call: { label: "工具", tone: "info" },
  tool_result: { label: "返回", tone: "ok" },
  directive: { label: "指令", tone: "accent" },
  approval_request: { label: "待审批", tone: "warn" },
  note: { label: "记录", tone: "ink-3" },
};

// 预置转向指令（点击即填入输入框，人工可改）。
const QUICK_DIRECTIVES = [
  "聚焦当前端点深挖",
  "转向 recon 新端点",
  "暂停并等待我确认",
  "跳过该假设",
];

export interface OrchestrationChatProps {
  model: OrchestrationModel;
  focusedTitle: string; // 聚焦 Agent 的标题（抬头展示）
  onSteer?: (agentId: string, text: string) => void;
  onJump?: (ref: { type: string; id: string }) => void;
}

export function OrchestrationChat({ model, focusedTitle, onSteer, onJump }: OrchestrationChatProps) {
  const [scope, setScope] = useState<"all" | "focused">("all");
  const [draft, setDraft] = useState("");
  // 人工在此发出的消息（乐观追加进对话流，接线后由事件流回灌真实往返）。
  const [sent, setSent] = useState<OrchEntry[]>([]);

  const all = useMemo(() => [...model.entries, ...sent], [model.entries, sent]);
  const entries = useMemo(
    () => (scope === "focused" ? all.filter((e) => e.agentId === model.focusedId) : all),
    [all, model.focusedId, scope]
  );

  const send = () => {
    const t = draft.trim();
    if (!t) return;
    const now = new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
    const base = Date.now();
    // 两段：人工消息 + Agent 收到回执（预览模拟；真实回复由 daemon 推流）。
    const human: OrchEntry = { id: `u-${base}`, kind: "directive", at: now, agentId: model.focusedId, text: `【你】${t}` };
    const ack: OrchEntry = { id: `a-${base}`, kind: "reasoning", at: now, agentId: model.focusedId, text: "收到，正在据此调整当前计划…（真实回复待 daemon 接线）" };
    setSent((prev) => [...prev, human, ack]);
    onSteer?.(model.focusedId, t);
    setDraft("");
  };

  return (
    <div className="sn-orch" aria-label="与 AI 对话">
      <div className="sn-orch-head">
        <div>
          <div className="sn-orch-title">与 Agent 对话 · <strong>{focusedTitle}</strong></div>
          <div className="sn-orch-sub">你在这里向 AI 下达指令 / 转向，消息进入对话流；危险命令仍走审批</div>
        </div>
        <div className="sn-seg" role="group" aria-label="范围">
          <button data-active={scope === "all" || undefined} onClick={() => setScope("all")}>全部</button>
          <button data-active={scope === "focused" || undefined} onClick={() => setScope("focused")}>仅聚焦</button>
        </div>
      </div>

      <div className="sn-orch-stream" role="log" aria-live="polite" aria-label="推理流">
        {entries.length === 0 ? (
          <div className="sn-empty">该 Agent 暂无转录</div>
        ) : (
          entries.map((e) => <OrchLine key={e.id} entry={e} onJump={onJump} />)
        )}
      </div>

      <div className="sn-orch-input">
        <div className="sn-orch-quick">
          {QUICK_DIRECTIVES.map((q) => (
            <button key={q} className="sn-chip" disabled={!model.canSteer} onClick={() => setDraft(q)}>
              {q}
            </button>
          ))}
        </div>
        <div className="sn-orch-compose">
          <textarea
            className="sn-input"
            rows={2}
            placeholder={model.canSteer ? "给 AI 发消息 / 下达指令…（Ctrl/⌘+Enter 发送）" : "当前不可对话（运行非运行/暂停态）"}
            aria-label="与 AI 对话输入"
            value={draft}
            disabled={!model.canSteer}
            onChange={(ev) => setDraft(ev.target.value)}
            onKeyDown={(ev) => {
              if (ev.key === "Enter" && (ev.metaKey || ev.ctrlKey)) {
                ev.preventDefault();
                send();
              }
            }}
          />
          <button className="sn-btn" data-variant="primary" disabled={!model.canSteer || !draft.trim()} onClick={send}>
            发送
          </button>
        </div>
      </div>
    </div>
  );
}

function OrchLine({ entry, onJump }: { entry: OrchEntry; onJump?: (ref: { type: string; id: string }) => void }) {
  const meta = KIND_META[entry.kind];
  return (
    <div className="sn-orch-entry" data-kind={entry.kind}>
      <div className="sn-orch-gutter">
        <span className="sn-badge" data-tone={meta.tone}>{meta.label}</span>
      </div>
      <div className="sn-orch-body">
        <div className="sn-orch-line1">
          {entry.tool && <span className="mono sn-orch-tool">{entry.tool}</span>}
          <span className="sn-orch-text">{entry.text}</span>
        </div>
        <div className="sn-orch-foot">
          <span>{entry.at}</span>
          {typeof entry.tokens === "number" && <span>· {entry.tokens} tok</span>}
          {entry.objectRef && onJump && (
            <button className="sn-link-btn" onClick={() => onJump(entry.objectRef!)}>
              {entry.objectRef.type} →
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
