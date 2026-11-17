// RecoveryPreview —— Resume 前恢复预览（UI 文档 §16.12）。展示未完成 Job / 未知结果 Action /
// 过期身份 / 失效代理 / 策略版本 / Schema 兼容性，让用户知情后再恢复。Resume 走命令（不前端改状态）。

import { Card } from "../../components/Card";
import { Button } from "../../components/Button";
import { StatusBadge } from "../../components/StatusBadge";

export interface RecoveryPreviewModel {
  unfinishedJobs: number;
  outcomeUnknownActions: number;
  expiredIdentities: string[];
  staleProxies: string[];
  policyRevisionChanged: boolean;
  schemaCompatible: boolean;
}

export interface RecoveryPreviewProps {
  model: RecoveryPreviewModel;
  onResume?: () => void;
  onCancel?: () => void;
}

export function RecoveryPreview({ model, onResume, onCancel }: RecoveryPreviewProps) {
  const rows: Array<{ label: string; value: React.ReactNode; warn: boolean }> = [
    { label: "未完成 Job", value: model.unfinishedJobs, warn: model.unfinishedJobs > 0 },
    { label: "未知结果 Action（OUTCOME_UNKNOWN）", value: model.outcomeUnknownActions, warn: model.outcomeUnknownActions > 0 },
    { label: "过期身份", value: model.expiredIdentities.join(", ") || "无", warn: model.expiredIdentities.length > 0 },
    { label: "失效代理", value: model.staleProxies.join(", ") || "无", warn: model.staleProxies.length > 0 },
    { label: "策略版本变化", value: model.policyRevisionChanged ? "是" : "否", warn: model.policyRevisionChanged },
    { label: "Schema 兼容", value: model.schemaCompatible ? "兼容" : "不兼容", warn: !model.schemaCompatible },
  ];
  const blocking = !model.schemaCompatible; // Schema 不兼容应阻断恢复

  return (
    <Card data-testid="recovery-preview">
      <h3 style={{ marginTop: 0, fontSize: "var(--fs-lg)" }}>恢复预览</h3>
      <dl style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "var(--sp-2) var(--sp-3)", margin: "0 0 var(--sp-4)" }}>
        {rows.map((r) => (
          <div key={r.label} style={{ display: "contents" }}>
            <dt style={{ color: "var(--ink-2)", fontSize: "var(--fs-sm)" }}>{r.label}</dt>
            <dd style={{ margin: 0, color: r.warn ? "var(--warn)" : "var(--ink-1)", fontSize: "var(--fs-sm)" }}>{r.value}</dd>
          </div>
        ))}
      </dl>
      {blocking && (
        <div style={{ marginBottom: "var(--sp-3)" }}>
          <StatusBadge status="BLOCKED" /> <span style={{ color: "var(--danger)", fontSize: "var(--fs-sm)" }}>Schema 不兼容，无法直接恢复</span>
        </div>
      )}
      <div style={{ display: "flex", gap: "var(--sp-2)" }}>
        <Button variant="primary" disabled={blocking} onClick={() => onResume?.()}>
          确认恢复
        </Button>
        <Button variant="ghost" onClick={() => onCancel?.()}>
          取消
        </Button>
      </div>
    </Card>
  );
}
