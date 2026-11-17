// EvidencePage —— Evidence 页面（UI 文档 §16.9）。左证据列表 + 右 EvidenceViewer（Tab：请求/响应/差异/
// 时间信息/截图/元数据/哈希/关联 Finding）。默认脱敏 Projection，原始需 Reveal；HTML/JS 不执行、链接不自动打开。

import { Fragment, useState } from "react";
import { EvidenceViewer, type EvidenceTab } from "../../components/domain/EvidenceViewer";

export interface EvidenceRow {
  id: string;
  kind: string;
  capturedAt: string;
  sha256: string;
  identity: string;
  policyRevision: number;
  linkedFinding?: string;
  redacted: Partial<Record<EvidenceTab, string>>;
  meta: Record<string, string>;
}
export interface EvidenceModel {
  items: EvidenceRow[];
}

export interface EvidencePageProps {
  model: EvidenceModel;
  onReveal?: (evidenceId: string, tab: EvidenceTab) => Promise<string>;
}

export function EvidencePage({ model, onReveal }: EvidencePageProps) {
  const [sel, setSel] = useState<EvidenceRow | null>(model.items[0] ?? null);
  return (
    <div className="sn-three-col" style={{ gridTemplateColumns: "280px 1fr" }}>
      <div className="sn-col-scroll">
        <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-1)" }}>Evidence 列表</div>
        {model.items.map((e) => (
          <button key={e.id} className="sn-btn sn-focus" data-variant={sel?.id === e.id ? "secondary" : "ghost"} data-size="sm"
            style={{ width: "100%", justifyContent: "flex-start", marginBottom: 2 }} onClick={() => setSel(e)}>
            <span className="mono" style={{ fontSize: "var(--fs-xs)" }}>{e.id} · {e.kind}</span>
          </button>
        ))}
      </div>
      <div>
        {sel ? (
          <>
            <EvidenceViewer evidenceId={sel.id} redacted={sel.redacted} onReveal={onReveal} contentType={sel.kind} />
            <div style={{ marginTop: "var(--sp-3)" }}>
              <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-1)" }}>元数据 / 哈希 / 血缘 / 关联</div>
              <dl className="sn-kv">
                <dt>SHA-256</dt><dd className="mono" style={{ fontSize: "var(--fs-xs)" }}>{sel.sha256}</dd>
                <dt>采集时间</dt><dd>{sel.capturedAt}</dd>
                <dt>身份（血缘）</dt><dd>{sel.identity}</dd>
                <dt>PolicyRevision（血缘）</dt><dd>{sel.policyRevision}</dd>
                <dt>关联 Finding</dt><dd className="mono">{sel.linkedFinding ?? "无"}</dd>
                {Object.entries(sel.meta).map(([k, v]) => (<Fragment key={k}><dt>{k}</dt><dd>{v}</dd></Fragment>))}
              </dl>
            </div>
          </>
        ) : (
          <div className="sn-empty">选择左侧 Evidence</div>
        )}
      </div>
    </div>
  );
}
