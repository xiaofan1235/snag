// diff.worker.ts —— 在 Web Worker 里跑行级 diff（UI 文档 §16.33：Diff 计算放 Worker 不卡 UI）。
// 消息协议：{ id, baseline, variant } → { id, rows }。
import { computeLineDiff, type DiffRow } from "./diff";

export interface DiffRequest {
  id: number;
  baseline: string;
  variant: string;
}
export interface DiffResponse {
  id: number;
  rows: DiffRow[];
}

self.onmessage = (e: MessageEvent<DiffRequest>) => {
  const { id, baseline, variant } = e.data;
  const rows = computeLineDiff(baseline, variant);
  (self as unknown as Worker).postMessage({ id, rows } satisfies DiffResponse);
};
