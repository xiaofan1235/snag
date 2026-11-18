// ExplorationGraph —— 探索知识图谱（ARTEX 式）。目标→意图→事实→发现分层，spawns/yields/proves 边。
// 手写 SVG 分层布局（不引 cytoscape/reactflow 等大依赖，与本仓「手写优于大依赖」一致）。节点点击推 Inspector。

import { useMemo, useState } from "react";
import type { ExplorationGraphModel, GraphNode, GraphNodeKind } from "./types";

const LAYER_ORDER: GraphNodeKind[] = ["goal", "intent", "fact", "finding"];
const KIND_LABEL: Record<GraphNodeKind, string> = { goal: "目标", intent: "意图", fact: "事实", finding: "发现" };
const EDGE_LABEL = { spawns: "spawns", yields: "yields", proves: "proves" } as const;

const NODE_W = 168;
const NODE_H = 48;
const COL_W = 224;
const ROW_H = 68;
const PAD = 24;

interface Pos {
  x: number;
  y: number;
  node: GraphNode;
}

function layout(model: ExplorationGraphModel): { pos: Map<string, Pos>; width: number; height: number } {
  const byLayer = new Map<number, GraphNode[]>();
  for (const n of model.nodes) {
    const layer = Math.max(0, LAYER_ORDER.indexOf(n.kind));
    if (!byLayer.has(layer)) byLayer.set(layer, []);
    byLayer.get(layer)!.push(n);
  }
  const maxCount = Math.max(1, ...[...byLayer.values()].map((a) => a.length));
  const pos = new Map<string, Pos>();
  for (let layer = 0; layer < LAYER_ORDER.length; layer++) {
    const nodes = byLayer.get(layer) ?? [];
    const startY = PAD + ((maxCount - nodes.length) * ROW_H) / 2;
    nodes.forEach((node, i) => {
      pos.set(node.id, { x: PAD + layer * COL_W, y: startY + i * ROW_H, node });
    });
  }
  const width = PAD * 2 + (LAYER_ORDER.length - 1) * COL_W + NODE_W;
  const height = PAD * 2 + maxCount * ROW_H;
  return { pos, width, height };
}

function truncate(s: string, n = 20): string {
  return s.length > n ? s.slice(0, n - 1) + "…" : s;
}

export interface ExplorationGraphProps {
  model: ExplorationGraphModel;
  onSelect?: (node: GraphNode) => void;
}

export function ExplorationGraph({ model, onSelect }: ExplorationGraphProps) {
  const { pos, width, height } = useMemo(() => layout(model), [model]);
  const [sel, setSel] = useState<string | null>(null);

  if (model.nodes.length === 0) {
    return <div className="sn-empty">暂无探索节点（随 Agent 推进填充）</div>;
  }

  return (
    <div className="sn-graph">
      <div className="sn-graph-legend">
        {LAYER_ORDER.map((k) => (
          <span key={k} className="sn-graph-legend-item">
            <span className="sn-graph-swatch" data-kind={k} /> {KIND_LABEL[k]}
          </span>
        ))}
        <span className="sn-graph-legend-sep" />
        {(Object.keys(EDGE_LABEL) as Array<keyof typeof EDGE_LABEL>).map((e) => (
          <span key={e} className="sn-graph-legend-item mono" data-edge={e}>
            — {EDGE_LABEL[e]}
          </span>
        ))}
      </div>
      <div className="sn-graph-scroll">
        <svg
          viewBox={`0 0 ${width} ${height}`}
          width={width}
          height={height}
          role="img"
          aria-label="探索知识图谱"
          className="sn-graph-svg"
        >
          <defs>
            <marker id="sn-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
              <path d="M0,0 L8,4 L0,8 z" className="sn-graph-arrow" />
            </marker>
          </defs>
          {/* 边 */}
          {model.edges.map((edge, i) => {
            const a = pos.get(edge.from);
            const b = pos.get(edge.to);
            if (!a || !b) return null;
            const x1 = a.x + NODE_W;
            const y1 = a.y + NODE_H / 2;
            const x2 = b.x;
            const y2 = b.y + NODE_H / 2;
            const mx = (x1 + x2) / 2;
            const active = sel === edge.from || sel === edge.to;
            return (
              <path
                key={i}
                className="sn-gedge"
                data-kind={edge.kind}
                data-active={active || undefined}
                d={`M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`}
                markerEnd="url(#sn-arrow)"
              />
            );
          })}
          {/* 节点 */}
          {[...pos.values()].map(({ x, y, node }) => {
            const selected = sel === node.id;
            return (
              <g
                key={node.id}
                className="sn-gnode"
                data-kind={node.kind}
                data-selected={selected || undefined}
                transform={`translate(${x},${y})`}
                tabIndex={0}
                role="button"
                aria-label={`${KIND_LABEL[node.kind]}：${node.label}`}
                onClick={() => {
                  setSel(node.id);
                  onSelect?.(node);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    setSel(node.id);
                    onSelect?.(node);
                  }
                }}
              >
                <rect width={NODE_W} height={NODE_H} rx={10} className="sn-gnode-box" />
                <text x={12} y={19} className="sn-gnode-kind">{KIND_LABEL[node.kind]}</text>
                <text x={12} y={37} className="sn-gnode-label">{truncate(node.label)}</text>
                {node.state && <text x={NODE_W - 12} y={19} className="sn-gnode-state" textAnchor="end">{node.state}</text>}
                <title>{node.label}{node.state ? `（${node.state}）` : ""}</title>
              </g>
            );
          })}
        </svg>
      </div>
    </div>
  );
}
