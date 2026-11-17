// CoverageMatrix —— 覆盖率页面（UI 文档 §16.8）。矩阵（接口 × 攻击类型），按身份过滤；
// 单元格状态 TESTED/NA/BLOCKED/STALE/UNTESTED；单元格点击显示具体 Action 和 Evidence + 未测原因；
// 展示当前 SurfaceRevision。状态不只靠颜色——用 StatusBadge 文字+图标（无障碍，D8）。

import { useMemo, useState } from "react";
import { Card } from "../../components/Card";
import { Select } from "../../components/Select";

export type CellState = "TESTED" | "NA" | "BLOCKED" | "STALE" | "UNTESTED";

export interface CoverageCell {
  endpoint: string;
  attackType: string;
  identity: string;
  state: CellState;
  actionId?: string;
  evidenceId?: string;
  untestedReason?: string;
}

export interface CoverageModel {
  surfaceRevision: number;
  cells: CoverageCell[];
}

export interface CoverageMatrixProps {
  model: CoverageModel;
  onDrillDown?: (cell: CoverageCell) => void;
}

const STATE_TONE: Record<CellState, string> = {
  TESTED: "ok",
  NA: "ink-3",
  BLOCKED: "warn",
  STALE: "unknown",
  UNTESTED: "ink-2",
};
const STATE_MARK: Record<CellState, string> = {
  TESTED: "✓",
  NA: "—",
  BLOCKED: "⬒",
  STALE: "∼",
  UNTESTED: "·",
};

export function CoverageMatrix({ model, onDrillDown }: CoverageMatrixProps) {
  const identities = useMemo(() => Array.from(new Set(model.cells.map((c) => c.identity))), [model.cells]);
  const [identity, setIdentity] = useState<string>(identities[0] ?? "");
  const [selected, setSelected] = useState<CoverageCell | null>(null);

  const filtered = model.cells.filter((c) => !identity || c.identity === identity);
  const endpoints = Array.from(new Set(filtered.map((c) => c.endpoint)));
  const attacks = Array.from(new Set(filtered.map((c) => c.attackType)));
  const cellAt = (ep: string, at: string) => filtered.find((c) => c.endpoint === ep && c.attackType === at);

  function pick(cell: CoverageCell) {
    setSelected(cell);
    onDrillDown?.(cell);
  }

  return (
    <div style={{ display: "grid", gap: "var(--sp-3)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
        <span className="mono" style={{ color: "var(--ink-3)", fontSize: "var(--fs-sm)" }}>
          SurfaceRevision rev {model.surfaceRevision}
        </span>
        {identities.length > 0 && (
          <label style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", fontSize: "var(--fs-sm)" }}>
            身份
            <Select
              options={identities.map((i) => ({ value: i, label: i }))}
              value={identity}
              onChange={setIdentity}
              aria-label="按身份过滤"
            />
          </label>
        )}
      </div>

      <div className="sn-table-wrap">
        <table className="sn-table" data-testid="coverage-matrix">
          <thead>
            <tr>
              <th>接口 \ 攻击类型</th>
              {attacks.map((a) => (
                <th key={a}>{a}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {endpoints.map((ep) => (
              <tr key={ep}>
                <td className="mono" style={{ color: "var(--ink-2)" }}>
                  {ep}
                </td>
                {attacks.map((a) => {
                  const cell = cellAt(ep, a);
                  if (!cell) return <td key={a} />;
                  return (
                    <td key={a}>
                      <button
                        className="sn-badge sn-focus"
                        data-tone={STATE_TONE[cell.state]}
                        aria-label={`${ep} ${a} ${cell.state}`}
                        style={{ cursor: "pointer", border: "none" }}
                        onClick={() => pick(cell)}
                      >
                        <span aria-hidden="true">{STATE_MARK[cell.state]}</span>
                        {cell.state}
                      </button>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {selected && (
        <Card data-testid="cell-drilldown">
          <div style={{ fontSize: "var(--fs-sm)" }}>
            <div>
              单元格：<span className="mono">{selected.endpoint} · {selected.attackType} · {selected.identity}</span>
            </div>
            <div>状态：{selected.state}</div>
            {selected.actionId && <div>Action：<span className="mono">{selected.actionId}</span></div>}
            {selected.evidenceId && <div>Evidence：<span className="mono">{selected.evidenceId}</span></div>}
            {selected.state !== "TESTED" && selected.untestedReason && (
              <div style={{ color: "var(--warn)" }}>未测原因：{selected.untestedReason}</div>
            )}
          </div>
        </Card>
      )}
    </div>
  );
}
