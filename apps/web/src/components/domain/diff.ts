// diff.ts —— 纯行级 diff（LCS），供 DiffViewer 使用。纯函数，可独立单测，也可在 Web Worker 里跑
// （UI 文档 §16.33：大响应/Diff/JSON Tree 放 Web Worker）。

export type DiffOp = "equal" | "add" | "del";

export interface DiffRow {
  op: DiffOp;
  left?: string; // baseline 行（del/equal）
  right?: string; // variant 行（add/equal）
}

// computeLineDiff 对两段文本做行级 LCS diff，返回并排行序列。
export function computeLineDiff(baseline: string, variant: string): DiffRow[] {
  const a = baseline.split("\n");
  const b = variant.split("\n");
  const n = a.length;
  const m = b.length;
  // LCS 长度表。
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const rows: DiffRow[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      rows.push({ op: "equal", left: a[i], right: b[j] });
      i++;
      j++;
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      rows.push({ op: "del", left: a[i] });
      i++;
    } else {
      rows.push({ op: "add", right: b[j] });
      j++;
    }
  }
  while (i < n) rows.push({ op: "del", left: a[i++] });
  while (j < m) rows.push({ op: "add", right: b[j++] });
  return rows;
}

export interface DiffStats {
  added: number;
  deleted: number;
  equal: number;
}

export function diffStats(rows: DiffRow[]): DiffStats {
  const s: DiffStats = { added: 0, deleted: 0, equal: 0 };
  for (const r of rows) {
    if (r.op === "add") s.added++;
    else if (r.op === "del") s.deleted++;
    else s.equal++;
  }
  return s;
}
