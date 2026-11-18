// FindingDetail —— Finding 详情（UI 文档 §16.7/D6/§3.4）。核心：**事实与推断视觉分区**——
// 事实（挂 EvidenceCitation，可点击→右栏 CEBS 字节高亮）与推断（AI 结论，不作 grounding）分开呈现。
// baseline/variant 并排 diff；Verifier 意见/反对证据/定级理由/清理状态/报告预览。
// 「请求重放」必须创建新 Job（onReplay 走命令），不是前端直接发请求。

import { useState } from "react";
import { Card } from "../../components/Card";
import { StatusBadge } from "../../components/StatusBadge";
import { Button } from "../../components/Button";
import { DiffViewer } from "../../components/domain/DiffViewer";

// 一条事实陈述：挂一条逐字 EvidenceCitation（byte 区间），点击在右栏高亮对应字节（§3.4 CEBS）。
export interface FactStatement {
  id: string;
  text: string;
  evidenceId: string;
  byteStart: number;
  byteEnd: number;
}
// 一条推断（AI 结论，无 grounding）。
export interface InferenceStatement {
  id: string;
  text: string;
}
export interface CitationView {
  evidenceId: string;
  content: string; // 脱敏投影全文（右栏展示，高亮 [byteStart,byteEnd)）
}
export interface FindingModel {
  id: string;
  title: string;
  vulnType: string;
  severity: string; // CVSS 分档：Critical/High/...（走 StatusBadge? 用普通徽章）
  state: string;
  affectedEndpoints: string[];
  identityObjectRelation: string;
  facts: FactStatement[];
  inferences: InferenceStatement[];
  baseline?: string;
  variant?: string;
  verifierOpinion?: string;
  counterEvidence?: string[];
  severityRationale?: string;
  cleanupStatus?: string;
  citations: CitationView[];
}

export interface FindingDetailProps {
  model: FindingModel;
  /** 请求重放——创建新 Verification Job（§16.7：不是前端直接发请求）。 */
  onReplay?: (findingId: string) => void;
  onJump?: (kind: "verifier" | "action" | "exchange" | "evidence", id: string) => void;
}

function Highlighted({ content, start, end }: { content: string; start: number; end: number }) {
  const s = Math.max(0, Math.min(start, content.length));
  const e = Math.max(s, Math.min(end, content.length));
  return (
    <pre className="mono" data-testid="cebs-highlight" style={{ margin: 0, whiteSpace: "pre-wrap", wordBreak: "break-all", fontSize: "var(--fs-xs)" }}>
      {content.slice(0, s)}
      <mark style={{ background: "var(--accent-weak)", color: "var(--ink-1)" }}>{content.slice(s, e)}</mark>
      {content.slice(e)}
    </pre>
  );
}

export function FindingDetail({ model, onReplay, onJump }: FindingDetailProps) {
  const [activeFact, setActiveFact] = useState<FactStatement | null>(null);
  const activeCitation = activeFact ? model.citations.find((c) => c.evidenceId === activeFact.evidenceId) : null;

  return (
    <div style={{ display: "grid", gridTemplateColumns: "1fr 320px", gap: "var(--sp-4)" }}>
      {/* 主区 */}
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-3)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
          <strong style={{ fontSize: "var(--fs-lg)" }}>{model.title}</strong>
          <StatusBadge status={model.state} />
          <span className="sn-badge" data-tone="warn">
            {model.severity}
          </span>
          <span className="mono" style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>
            {model.vulnType}
          </span>
        </div>

        {/* 事实区（可点击→证据） */}
        <Card>
          <div style={{ color: "var(--ok)", fontSize: "var(--fs-xs)", fontWeight: 600, marginBottom: "var(--sp-2)" }}>
            事实（有证据来源 · 点击查看 CEBS 字节）
          </div>
          <ul data-testid="facts" style={{ listStyle: "none", padding: 0, margin: 0, display: "grid", gap: "var(--sp-1)" }}>
            {model.facts.map((f) => (
              <li key={f.id}>
                <button
                  className="sn-btn sn-focus"
                  data-variant="ghost"
                  data-size="sm"
                  aria-pressed={activeFact?.id === f.id}
                  style={{ textAlign: "left", width: "100%", justifyContent: "flex-start", borderLeft: "2px solid var(--ok)" }}
                  onClick={() => setActiveFact(f)}
                >
                  {f.text}
                </button>
              </li>
            ))}
          </ul>
        </Card>

        {/* 推断区（视觉分开，不作证据） */}
        <Card>
          <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", fontWeight: 600, marginBottom: "var(--sp-2)" }}>
            推断（AI 结论 · 无证据 grounding）
          </div>
          <ul data-testid="inferences" style={{ margin: 0, paddingLeft: "var(--sp-5)", color: "var(--ink-2)" }}>
            {model.inferences.map((inf) => (
              <li key={inf.id} style={{ fontSize: "var(--fs-sm)" }}>
                {inf.text}
              </li>
            ))}
          </ul>
        </Card>

        {model.baseline != null && model.variant != null && (
          <Card>
            <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-2)" }}>Baseline / Variant 对比</div>
            <DiffViewer baseline={model.baseline} variant={model.variant} sync />
          </Card>
        )}

        <Card>
          <div style={{ fontSize: "var(--fs-sm)" }}>
            <div>受影响接口：<span className="mono">{model.affectedEndpoints.join(", ")}</span></div>
            <div>账号/对象关系：{model.identityObjectRelation}</div>
            {model.verifierOpinion && <div>Verifier 意见：{model.verifierOpinion}</div>}
            {model.severityRationale && <div>定级理由：{model.severityRationale}</div>}
            {model.cleanupStatus && <div>清理状态：{model.cleanupStatus}</div>}
            {model.counterEvidence && model.counterEvidence.length > 0 && (
              <div style={{ color: "var(--danger)" }}>反对证据：{model.counterEvidence.join("；")}</div>
            )}
          </div>
          <div style={{ display: "flex", gap: "var(--sp-2)", marginTop: "var(--sp-3)" }}>
            {onReplay && (
              <Button size="sm" variant="secondary" onClick={() => onReplay(model.id)}>
                请求重放（新 Job）
              </Button>
            )}
            {onJump && (
              <Button size="sm" variant="ghost" onClick={() => onJump("verifier", model.id)}>
                查看 Verifier
              </Button>
            )}
          </div>
        </Card>
      </div>

      {/* 右栏：CEBS 字节高亮 */}
      <aside>
        <Card>
          <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-2)" }}>原始 Evidence（CEBS 字节高亮）</div>
          {activeFact && activeCitation ? (
            <>
              <div className="mono" style={{ fontSize: "var(--fs-xs)", color: "var(--ink-3)", marginBottom: "var(--sp-1)" }}>
                {activeFact.evidenceId} bytes[{activeFact.byteStart}:{activeFact.byteEnd}]
              </div>
              <Highlighted content={activeCitation.content} start={activeFact.byteStart} end={activeFact.byteEnd} />
            </>
          ) : (
            <div className="sn-empty">点击左侧事实查看对应证据字节</div>
          )}
        </Card>
      </aside>
    </div>
  );
}
