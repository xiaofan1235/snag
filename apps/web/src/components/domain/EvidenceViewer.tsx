// EvidenceViewer —— 证据查看器（UI 文档 D2.13 / §16.33）。Tab：请求/响应/差异/截图/元数据。
// 安全核心：**默认只展示脱敏投影（Redacted Projection）**；原始 Raw 需显式 Reveal（短时授权 + 二次确认 +
// 审计），不通过普通列表 API 顺带返回原秘密。截图按敏感 Evidence 处理，不假设已自动脱敏。

import { useState } from "react";
import { Modal } from "../Modal";

export type EvidenceTab = "request" | "response" | "diff" | "screenshot" | "metadata";

export interface EvidenceViewerProps {
  evidenceId: string;
  /** 脱敏投影文本（服务端已脱敏；前端脱敏只是额外防线）。 */
  redacted: Partial<Record<EvidenceTab, string>>;
  /** Reveal 原始内容的回调（短时授权；由调用方走后端授权 API）。返回原文。 */
  onReveal?: (evidenceId: string, tab: EvidenceTab) => Promise<string>;
  contentType?: string;
}

const TAB_LABEL: Record<EvidenceTab, string> = {
  request: "请求",
  response: "响应",
  diff: "差异",
  screenshot: "截图",
  metadata: "元数据",
};

export function EvidenceViewer({ evidenceId, redacted, onReveal, contentType }: EvidenceViewerProps) {
  const tabs = (Object.keys(TAB_LABEL) as EvidenceTab[]).filter((t) => redacted[t] != null);
  const [tab, setTab] = useState<EvidenceTab>(tabs[0] ?? "metadata");
  const [confirming, setConfirming] = useState(false);
  const [revealed, setRevealed] = useState<Partial<Record<EvidenceTab, string>>>({});
  const [revealing, setRevealing] = useState(false);

  const showingRaw = revealed[tab] != null;
  const body = showingRaw ? revealed[tab]! : redacted[tab] ?? "";

  async function doReveal() {
    if (!onReveal) return;
    setRevealing(true);
    try {
      const raw = await onReveal(evidenceId, tab);
      setRevealed((r) => ({ ...r, [tab]: raw }));
    } finally {
      setRevealing(false);
      setConfirming(false);
    }
  }

  return (
    <div>
      <div role="tablist" aria-label="证据视图" style={{ display: "flex", gap: "var(--sp-1)", marginBottom: "var(--sp-2)" }}>
        {tabs.map((t) => (
          <button
            key={t}
            role="tab"
            aria-selected={t === tab}
            className="sn-btn sn-focus"
            data-variant={t === tab ? "secondary" : "ghost"}
            data-size="sm"
            onClick={() => setTab(t)}
          >
            {TAB_LABEL[t]}
          </button>
        ))}
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-1)" }}>
        <span className="sn-badge" data-tone={showingRaw ? "danger" : "ok"}>
          {showingRaw ? "原始（已 Reveal）" : "脱敏投影"}
        </span>
        {contentType && (
          <span className="mono" style={{ fontSize: "var(--fs-xs)", color: "var(--ink-3)" }}>
            {contentType}
          </span>
        )}
        {!showingRaw && onReveal && (
          <button
            className="sn-btn sn-focus"
            data-variant="danger"
            data-size="sm"
            style={{ marginLeft: "auto" }}
            onClick={() => setConfirming(true)}
          >
            Reveal 原始
          </button>
        )}
      </div>

      <pre
        className="mono"
        data-testid="evidence-body"
        style={{
          margin: 0,
          padding: "var(--sp-3)",
          background: "var(--bg-sunken)",
          borderRadius: "var(--radius-sm)",
          fontSize: "var(--fs-xs)",
          whiteSpace: "pre-wrap",
          wordBreak: "break-all",
          maxHeight: 320,
          overflow: "auto",
        }}
      >
        {body}
      </pre>

      <Modal
        open={confirming}
        onClose={() => setConfirming(false)}
        title="Reveal 原始证据？"
        danger
        confirmText={revealing ? "授权中…" : "确认 Reveal"}
        onConfirm={doReveal}
      >
        原始内容可能含敏感数据/秘密。Reveal 使用短时授权并记入审计；仅在确有必要时查看。
      </Modal>
    </div>
  );
}
