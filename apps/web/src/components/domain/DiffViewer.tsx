// DiffViewer —— baseline/variant 并排差异（UI 文档 D2.13）。关键字段高亮；计算优先放 Web Worker
// （§16.33），Worker 不可用时同步回退（测试/SSR 友好）。

import { useEffect, useRef, useState } from "react";
import { computeLineDiff, diffStats, type DiffRow } from "./diff";

export interface DiffViewerProps {
  baseline: string;
  variant: string;
  /** 强制同步计算（测试用）；默认尝试 Worker。 */
  sync?: boolean;
}

// 懒建 Worker；失败则返回 null（回退同步）。
function tryCreateWorker(): Worker | null {
  try {
    if (typeof Worker === "undefined") return null;
    return new Worker(new URL("./diff.worker.ts", import.meta.url), { type: "module" });
  } catch {
    return null;
  }
}

export function DiffViewer({ baseline, variant, sync = false }: DiffViewerProps) {
  const [rows, setRows] = useState<DiffRow[]>(() => (sync ? computeLineDiff(baseline, variant) : []));
  const reqId = useRef(0);

  useEffect(() => {
    if (sync) {
      setRows(computeLineDiff(baseline, variant));
      return;
    }
    const worker = tryCreateWorker();
    if (!worker) {
      setRows(computeLineDiff(baseline, variant)); // 回退同步
      return;
    }
    const id = ++reqId.current;
    const onMsg = (e: MessageEvent<{ id: number; rows: DiffRow[] }>) => {
      if (e.data.id === id) setRows(e.data.rows);
    };
    worker.addEventListener("message", onMsg);
    worker.postMessage({ id, baseline, variant });
    return () => {
      worker.removeEventListener("message", onMsg);
      worker.terminate();
    };
  }, [baseline, variant, sync]);

  const stats = diffStats(rows);

  return (
    <div>
      <div style={{ display: "flex", gap: "var(--sp-2)", marginBottom: "var(--sp-2)", fontSize: "var(--fs-xs)" }}>
        <span className="sn-badge" data-tone="ok">
          +{stats.added}
        </span>
        <span className="sn-badge" data-tone="danger">
          -{stats.deleted}
        </span>
      </div>
      <div
        className="mono"
        role="table"
        aria-label="差异对比"
        style={{ border: "1px solid var(--border-subtle)", borderRadius: "var(--radius-sm)", overflow: "auto", fontSize: "var(--fs-xs)" }}
      >
        {rows.map((r, i) => (
          <div
            key={i}
            role="row"
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              background:
                r.op === "add" ? "var(--ok-bg)" : r.op === "del" ? "var(--danger-bg)" : "transparent",
            }}
          >
            <span role="cell" style={{ padding: "2px var(--sp-2)", borderRight: "1px solid var(--border-subtle)" }}>
              {r.left ?? ""}
            </span>
            <span role="cell" style={{ padding: "2px var(--sp-2)" }}>
              {r.right ?? ""}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
