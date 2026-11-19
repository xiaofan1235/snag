// StrategyPage —— Strategy / PlanPatch（UI 文档 §16.14）。回答「AI 为什么现在做这件事」。
// 展示当前 Epoch/触发原因/Snapshot、优先/暂停方向/无进展计数/最近 Reframe、PlanPatch 操作 Diff +
// PlanCompiler 逐项结果（ACCEPTED/DEDUPED/REJECTED/NEEDS_APPROVAL）。交互生成 UserDirective，不直改 Patch/DB。
// 已拒绝 Patch 只读保留；切历史 Epoch 顶部持续显示「正在查看历史」。

import { Card } from "../../components/Card";
import { Button } from "../../components/Button";

export type CompilerResult = "ACCEPTED" | "DEDUPED" | "REJECTED" | "NEEDS_APPROVAL";

export interface PlanPatchOp {
  kind: string; // add_hypothesis / create_job / reprioritize / request_approval / close_epoch
  summary: string;
  result: CompilerResult;
  reason?: string;
}
export interface StrategyEpoch {
  id: string;
  epoch: number;
  trigger: string;
  createdAt: string;
  snapshotRevision: number;
  priorityDirection: string;
  pausedDirections: string[];
  noProgressCount: number;
  lastReframe?: string;
  patches: Array<{ id: string; createdAt: string; ops: PlanPatchOp[] }>;
  historical?: boolean;
}
export interface StrategyModel {
  current: StrategyEpoch;
  history: Array<{ id: string; epoch: number; trigger: string }>;
}

const RESULT_TONE: Record<CompilerResult, string> = {
  ACCEPTED: "ok",
  DEDUPED: "ink-2",
  REJECTED: "danger",
  NEEDS_APPROVAL: "warn",
};

export interface StrategyPageProps {
  model: StrategyModel;
  onDirective?: (kind: string, detail: string) => void;
}

export function StrategyPage({ model, onDirective }: StrategyPageProps) {
  const e = model.current;
  return (
    <div style={{ display: "grid", gap: "var(--sp-4)" }}>
      {e.historical && (
        <div style={{ padding: "var(--sp-2) var(--sp-3)", background: "var(--warn-bg)", color: "var(--warn)", borderRadius: "var(--radius-sm)", fontSize: "var(--fs-sm)" }}>
          正在查看历史 Epoch（只读回看）
        </div>
      )}

      {/* Epoch 头 */}
      <Card>
        <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)", flexWrap: "wrap" }}>
          <strong style={{ fontSize: "var(--fs-lg)" }}>Epoch {e.epoch}</strong>
          <span className="sn-badge" data-tone="info">触发：{e.trigger}</span>
          <span className="mono" style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>快照 rev {e.snapshotRevision} · {e.createdAt}</span>
        </div>
        <dl className="sn-kv" style={{ marginTop: "var(--sp-3)" }}>
          <dt>当前优先方向</dt>
          <dd><strong>{e.priorityDirection}</strong></dd>
          <dt>暂停方向</dt>
          <dd>{e.pausedDirections.join("、") || "无"}</dd>
          <dt>无进展计数</dt>
          <dd>{e.noProgressCount}</dd>
          {e.lastReframe && (<><dt>最近转向</dt><dd>{e.lastReframe}</dd></>)}
        </dl>
        {/* 用户干预：生成 UserDirective（§16.14），不直改 Patch/DB */}
        <div style={{ display: "flex", gap: "var(--sp-2)", marginTop: "var(--sp-3)", flexWrap: "wrap" }}>
          <Button size="sm" variant="secondary" onClick={() => onDirective?.("STEER_RUN", "优先此方向:" + e.priorityDirection)}>优先此方向</Button>
          <Button size="sm" variant="ghost" onClick={() => onDirective?.("STEER_RUN", "降低此方向优先级:" + e.priorityDirection)}>降低优先级</Button>
          <Button size="sm" variant="ghost" onClick={() => onDirective?.("STEER_RUN", "停止同类新作业")}>停止同类新作业</Button>
        </div>
      </Card>

      {/* PlanPatch 操作 Diff + PlanCompiler 结果 */}
      <div>
        <h3 style={{ fontSize: "var(--fs-lg)", margin: "0 0 var(--sp-2)" }}>PlanPatch 与编译结果</h3>
        <div style={{ display: "grid", gap: "var(--sp-3)" }}>
          {e.patches.map((p) => (
            <Card key={p.id}>
              <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-2)" }}>
                <span className="mono" style={{ fontSize: "var(--fs-xs)", color: "var(--ink-3)" }}>{p.id}</span>
                <span style={{ marginLeft: "auto", color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>{p.createdAt}</span>
              </div>
              <table className="sn-table">
                <thead>
                  <tr><th>操作</th><th>摘要</th><th>编译结果</th></tr>
                </thead>
                <tbody>
                  {p.ops.map((op, i) => (
                    <tr key={i}>
                      <td className="mono">{op.kind}</td>
                      <td>{op.summary}</td>
                      <td>
                        <span className="sn-badge" data-tone={RESULT_TONE[op.result]}>{op.result}</span>
                        {op.reason && <span style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginLeft: "var(--sp-1)" }}>{op.reason}</span>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </Card>
          ))}
        </div>
      </div>

      {/* 历史 Epoch 切换（只读回看） */}
      {model.history.length > 0 && (
        <Card>
          <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-2)" }}>历史 Epoch（只读回看）</div>
          <div style={{ display: "flex", gap: "var(--sp-2)", flexWrap: "wrap" }}>
            {model.history.map((h) => (
              <span key={h.id} className="sn-badge" data-tone="ink-2">
                Epoch {h.epoch} · {h.trigger}
              </span>
            ))}
          </div>
        </Card>
      )}
    </div>
  );
}
