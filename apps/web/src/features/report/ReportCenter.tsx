// ReportCenter —— 报告中心（UI 文档 §16.22）。阅读气质（衬线/留白）；只纳入 Reportable Finding；
// 被排除的 Candidate/Blocked/UnableToVerify **单独列出及原因，不静默消失**；导出 Markdown/JSON
// （导出文件移除外链图片/Raw HTML/活动内容）。**首版只下载本地，不自动上传 SRC/第三方**。
// 报告状态 DRAFT/FINALIZED/STALE——STALE（依赖失效）不得继续分发（§13.4），导出按钮禁用。

import { Card } from "../../components/Card";
import { StatusBadge } from "../../components/StatusBadge";
import { Button } from "../../components/Button";

export interface ReportFinding {
  id: string;
  title: string;
  severity: string;
}
export interface ExcludedFinding {
  id: string;
  title: string;
  state: string; // CANDIDATE/BLOCKED/UNABLE_TO_VERIFY
  reason: string;
}
export interface ReportModel {
  id: string;
  state: "DRAFT" | "FINALIZED" | "STALE";
  reportable: ReportFinding[];
  excluded: ExcludedFinding[];
  exitGateResult?: string;
  coverageSnapshot?: string;
  partialReasons?: string[];
  markdownPreview: string;
}

export interface ReportCenterProps {
  model: ReportModel;
  /** 导出（本地下载）。STALE 时不应可点。 */
  onExport?: (reportId: string, format: "markdown" | "json") => void;
}

export function ReportCenter({ model, onExport }: ReportCenterProps) {
  const stale = model.state === "STALE";
  const exportable = !stale; // STALE 依赖失效，禁止分发/导出

  return (
    <div style={{ display: "grid", gridTemplateColumns: "1fr 300px", gap: "var(--sp-4)" }}>
      {/* 中：安全 Markdown 预览（阅读气质） */}
      <div>
        <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-3)" }}>
          <StatusBadge status={model.state} />
          {stale && <span style={{ color: "var(--danger)", fontSize: "var(--fs-sm)" }}>依赖失效，不得分发</span>}
          <div style={{ marginLeft: "auto", display: "flex", gap: "var(--sp-2)" }}>
            <Button size="sm" variant="secondary" disabled={!exportable} onClick={() => onExport?.(model.id, "markdown")}>
              导出 Markdown
            </Button>
            <Button size="sm" variant="secondary" disabled={!exportable} onClick={() => onExport?.(model.id, "json")}>
              导出 JSON
            </Button>
          </div>
        </div>
        <Card>
          <article
            data-testid="report-preview"
            style={{ fontFamily: "var(--font-read)", lineHeight: "var(--lh-normal)", whiteSpace: "pre-wrap", color: "var(--ink-1)" }}
          >
            {model.markdownPreview}
          </article>
        </Card>
      </div>

      {/* 右：来源 Finding + 被排除项 */}
      <aside style={{ display: "grid", gap: "var(--sp-3)" }}>
        <Card>
          <div style={{ color: "var(--ok)", fontSize: "var(--fs-xs)", fontWeight: 600, marginBottom: "var(--sp-2)" }}>
            纳入（Reportable {model.reportable.length}）
          </div>
          <ul data-testid="report-included" style={{ margin: 0, paddingLeft: "var(--sp-5)", fontSize: "var(--fs-sm)" }}>
            {model.reportable.map((f) => (
              <li key={f.id}>
                {f.title} <span className="sn-badge" data-tone="warn">{f.severity}</span>
              </li>
            ))}
          </ul>
        </Card>

        {/* 被排除项单独列出及原因（不静默消失） */}
        <Card>
          <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", fontWeight: 600, marginBottom: "var(--sp-2)" }}>
            被排除（{model.excluded.length}）
          </div>
          {model.excluded.length === 0 ? (
            <span style={{ color: "var(--ink-3)", fontSize: "var(--fs-sm)" }}>无</span>
          ) : (
            <ul data-testid="report-excluded" style={{ margin: 0, padding: 0, listStyle: "none", display: "grid", gap: "var(--sp-1)" }}>
              {model.excluded.map((f) => (
                <li key={f.id} style={{ fontSize: "var(--fs-sm)" }}>
                  <StatusBadge status={f.state} /> {f.title}
                  <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>原因：{f.reason}</div>
                </li>
              ))}
            </ul>
          )}
        </Card>

        {(model.exitGateResult || model.partialReasons?.length) && (
          <Card>
            <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", fontWeight: 600, marginBottom: "var(--sp-2)" }}>绑定</div>
            <div style={{ fontSize: "var(--fs-sm)" }}>
              {model.exitGateResult && <div>Exit Gate：{model.exitGateResult}</div>}
              {model.coverageSnapshot && <div>Coverage 快照：{model.coverageSnapshot}</div>}
              {model.partialReasons && model.partialReasons.length > 0 && (
                <div style={{ color: "var(--warn)" }}>PartialReasons：{model.partialReasons.join("；")}</div>
              )}
            </div>
          </Card>
        )}
      </aside>
    </div>
  );
}
