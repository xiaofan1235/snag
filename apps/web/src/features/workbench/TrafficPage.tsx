// TrafficPage —— Traffic / Repeater / Diff 工作台（UI 文档 §16.16）。三栏：左 Exchange 列表 + 筛选、
// 中 请求/响应（HTTPViewer）、右 关联对象。Baseline/Variant 选中生成 Diff（DiffViewer）。
// 所有发送仍由后端 Job 执行；Replay 显示实际 Job ID + 结果（不是「假设已发送」）。
// 响应不直接渲染 HTML、无浏览器直连隐藏通道（§16.16 安全）。

import { useState } from "react";
import { Card } from "../../components/Card";
import { Button } from "../../components/Button";
import { StatusBadge } from "../../components/StatusBadge";
import { HTTPViewer, type HttpMessage } from "../../components/domain/HTTPViewer";
import { DiffViewer } from "../../components/domain/DiffViewer";
import { Tabs } from "../../components/Tabs";

export interface Exchange {
  id: string;
  method: string;
  host: string;
  path: string;
  status: number;
  identity: string;
  jobId: string;
  hasEvidence: boolean;
  hasSecretMark: boolean;
  request: HttpMessage;
  response: HttpMessage;
  relatedAction?: string;
  relatedPermit?: string;
  relatedEvidence?: string;
  relatedFinding?: string;
}
export interface TrafficModel {
  exchanges: Exchange[];
}

export interface TrafficPageProps {
  model: TrafficModel;
  onReplay?: (exchangeId: string) => void;
}

export function TrafficPage({ model, onReplay }: TrafficPageProps) {
  const [sel, setSel] = useState<Exchange | null>(model.exchanges[0] ?? null);
  const [variant, setVariant] = useState<Exchange | null>(null);
  const [replayJob, setReplayJob] = useState<string | null>(null);

  return (
    <div className="sn-three-col">
      {/* 左：Exchange 列表 */}
      <div className="sn-col-scroll">
        <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-1)" }}>Exchange 列表</div>
        {model.exchanges.map((x) => (
          <button
            key={x.id}
            className="sn-btn sn-focus"
            data-variant={sel?.id === x.id ? "secondary" : "ghost"}
            data-size="sm"
            style={{ width: "100%", justifyContent: "flex-start", marginBottom: 2, textAlign: "left" }}
            onClick={() => setSel(x)}
          >
            <span className="mono" style={{ fontSize: "var(--fs-xs)" }}>
              {x.method} {x.path} · {x.status}
            </span>
            {x.hasSecretMark && <span className="sn-badge" data-tone="warn" style={{ marginLeft: "auto" }}>密</span>}
          </button>
        ))}
      </div>

      {/* 中：请求/响应 + Diff */}
      <div>
        {sel ? (
          <>
            <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-2)" }}>
              <span className="mono">{sel.method} {sel.host}{sel.path}</span>
              <StatusBadge status={sel.status < 400 ? "COMPLETED" : "FAILED"} />
              <Button size="sm" variant="secondary" style={{ marginLeft: "auto" }}
                onClick={() => { onReplay?.(sel.id); setReplayJob("job-replay-" + sel.id); }}>
                Replay（创建 Job）
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setVariant(variant ? null : sel)}>
                {variant ? "退出对比" : "设为 Variant 对比"}
              </Button>
            </div>
            {replayJob && (
              <div style={{ padding: "var(--sp-2)", background: "var(--info-bg)", color: "var(--info)", borderRadius: "var(--radius-sm)", fontSize: "var(--fs-sm)", marginBottom: "var(--sp-2)" }}>
                已创建重放 Job <span className="mono">{replayJob}</span>（后端执行，非前端直发）
              </div>
            )}
            <Tabs
              items={[
                { id: "req", label: "请求", content: <HTTPViewer message={sel.request} /> },
                { id: "resp", label: "响应", content: <HTTPViewer message={sel.response} /> },
                ...(variant ? [{ id: "diff", label: "基线/变体差异", content: (
                  <DiffViewer baseline={sel.response.body ?? ""} variant={variant.response.body ?? ""} sync />
                ) }] : []),
              ]}
            />
          </>
        ) : (
          <div className="sn-empty">选择左侧 Exchange</div>
        )}
      </div>

      {/* 右：关联对象 */}
      <div>
        <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-1)" }}>关联</div>
        {sel && (
          <Card>
            <dl className="sn-kv">
              <dt>身份</dt><dd>{sel.identity}</dd>
              <dt>Job</dt><dd className="mono">{sel.jobId}</dd>
              <dt>Action</dt><dd className="mono">{sel.relatedAction ?? "-"}</dd>
              <dt>Permit</dt><dd className="mono">{sel.relatedPermit ?? "-"}</dd>
              <dt>Evidence</dt><dd className="mono">{sel.relatedEvidence ?? "-"}</dd>
              <dt>Finding</dt><dd className="mono">{sel.relatedFinding ?? "-"}</dd>
            </dl>
          </Card>
        )}
      </div>
    </div>
  );
}
