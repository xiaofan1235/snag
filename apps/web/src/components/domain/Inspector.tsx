// Inspector —— 右栏统一对象详情（UI 文档 D2.13 / D3）。ObjectRef{type,id} 解析，双向跳转。
// 右侧抽屉即 Claude 的 Artifacts 侧栏模式；本组件是其内容渲染器（壳由 Drawer 提供）。

import type { ReactNode } from "react";

export interface ObjectRef {
  type: string; // run/job/finding/evidence/endpoint/...
  id: string;
}

export interface InspectorField {
  label: string;
  value: ReactNode;
}

export interface InspectorModel {
  ref: ObjectRef;
  title: string;
  fields: InspectorField[];
  /** 关联对象，点击可双向跳转。 */
  related?: Array<{ label: string; ref: ObjectRef }>;
}

export interface InspectorProps {
  model: InspectorModel | null;
  onNavigate?: (ref: ObjectRef) => void;
  emptyHint?: ReactNode;
}

const TYPE_LABEL: Record<string, string> = {
  run: "Run",
  job: "Job",
  finding: "Finding",
  evidence: "Evidence",
  endpoint: "Endpoint",
  hypothesis: "Hypothesis",
};

export function Inspector({ model, onNavigate, emptyHint }: InspectorProps) {
  if (!model) {
    return <div className="sn-empty">{emptyHint ?? "选择一个对象查看详情"}</div>;
  }
  return (
    <div data-testid="inspector">
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-3)" }}>
        <span className="sn-badge" data-tone="info">
          {TYPE_LABEL[model.ref.type] ?? model.ref.type}
        </span>
        <strong>{model.title}</strong>
      </div>
      <dl style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: "var(--sp-1) var(--sp-3)", margin: 0 }}>
        {model.fields.map((f, i) => (
          <div key={i} style={{ display: "contents" }}>
            <dt style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>{f.label}</dt>
            <dd style={{ margin: 0, fontSize: "var(--fs-sm)", wordBreak: "break-all" }}>{f.value}</dd>
          </div>
        ))}
      </dl>
      {model.related && model.related.length > 0 && (
        <div style={{ marginTop: "var(--sp-4)" }}>
          <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-1)" }}>关联对象</div>
          <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--sp-2)" }}>
            {model.related.map((r, i) => (
              <button
                key={i}
                className="sn-btn sn-focus"
                data-variant="secondary"
                data-size="sm"
                onClick={() => onNavigate?.(r.ref)}
              >
                {r.label}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
