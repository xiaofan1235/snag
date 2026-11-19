// HypothesesPage —— Hypothesis 看板（UI 文档 §16.6）。列：未测/需要上下文/测试中/需要补证据/已证实/已证伪/红线阻塞。
// 卡片：假设/信任边界/预期影响/反证条件/当前证据/下一步最小实验/停止条件。

import { useState } from "react";
import { Drawer } from "../../components/Drawer";

export type HypColumn = "UNTESTED" | "NEEDS_CONTEXT" | "TESTING" | "NEEDS_EVIDENCE" | "CONFIRMED" | "REFUTED" | "REDLINE_BLOCKED";

export interface Hypothesis {
  id: string;
  column: HypColumn;
  statement: string;
  trustBoundary: string;
  expectedImpact: string;
  falsifier: string;
  currentEvidence: string;
  minimumExperiment: string;
  stopCondition: string;
}
export interface HypothesesModel {
  items: Hypothesis[];
}

const COLS: Array<{ key: HypColumn; label: string }> = [
  { key: "UNTESTED", label: "未测" },
  { key: "NEEDS_CONTEXT", label: "需要上下文" },
  { key: "TESTING", label: "测试中" },
  { key: "NEEDS_EVIDENCE", label: "需要补证据" },
  { key: "CONFIRMED", label: "已证实" },
  { key: "REFUTED", label: "已证伪" },
  { key: "REDLINE_BLOCKED", label: "红线阻塞" },
];

export function HypothesesPage({ model }: { model: HypothesesModel }) {
  const [sel, setSel] = useState<Hypothesis | null>(null);
  return (
    <>
      <div className="sn-kanban">
        {COLS.map((c) => {
          const items = model.items.filter((h) => h.column === c.key);
          return (
            <div key={c.key} className="sn-kanban-col">
              <div className="sn-kanban-col-head">
                <span>{c.label}</span>
                <span>{items.length}</span>
              </div>
              {items.map((h) => (
                <div key={h.id} className="sn-kanban-card" onClick={() => setSel(h)}>
                  {h.statement}
                </div>
              ))}
            </div>
          );
        })}
      </div>
      <Drawer open={sel != null} onClose={() => setSel(null)} title="假设">
        {sel && (
          <dl className="sn-kv">
            <dt>假设</dt><dd>{sel.statement}</dd>
            <dt>信任边界</dt><dd>{sel.trustBoundary}</dd>
            <dt>预期影响</dt><dd>{sel.expectedImpact}</dd>
            <dt>反证条件</dt><dd>{sel.falsifier}</dd>
            <dt>当前证据</dt><dd>{sel.currentEvidence}</dd>
            <dt>下一步最小实验</dt><dd>{sel.minimumExperiment}</dd>
            <dt>停止条件</dt><dd>{sel.stopCondition}</dd>
          </dl>
        )}
      </Drawer>
    </>
  );
}
