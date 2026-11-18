// CommandDeck —— 统一作战台（ARTEX 式 Run 级一屏）。任务/发现/资产/流量/Token 统计瓦片 +
// Token 消耗图（手写 SVG 柱，用暖纸墨底 Token）+ 活动流 + 发现列表。全部点击可跳 Inspector/细节页。

import type { CommandDeckModel, DeckActivity, DeckFinding, DeckStat } from "./types";

const SEV_TONE: Record<DeckFinding["severity"], string> = {
  Critical: "danger",
  High: "danger",
  Medium: "warn",
  Low: "info",
  Info: "unknown",
};

const ACT_LABEL: Record<DeckActivity["kind"], string> = {
  finding: "发现",
  action: "动作",
  approval: "审批",
  coverage: "覆盖",
  budget: "预算",
  plan: "计划",
};
const ACT_TONE: Record<DeckActivity["kind"], string> = {
  finding: "accent",
  action: "info",
  approval: "warn",
  coverage: "ok",
  budget: "unknown",
  plan: "info",
};

export interface CommandDeckProps {
  model: CommandDeckModel;
  onOpenFinding?: (id: string) => void;
  onOpenActivity?: (a: DeckActivity) => void;
}

export function CommandDeck({ model, onOpenFinding, onOpenActivity }: CommandDeckProps) {
  return (
    <div className="sn-deck">
      <div className="sn-deck-stats">
        {model.stats.map((s) => (
          <StatTile key={s.key} stat={s} />
        ))}
      </div>

      <div className="sn-deck-grid">
        <section className="sn-panel">
          <div className="sn-panel-head">Token 消耗（按时间窗）</div>
          <TokenChart series={model.tokenSeries} windows={model.tokenWindows} />
        </section>

        <section className="sn-panel">
          <div className="sn-panel-head">活动流</div>
          <div className="sn-deck-activity">
            {model.activity.length === 0 ? (
              <div className="sn-empty">暂无活动</div>
            ) : (
              model.activity.map((a) => (
                <button key={a.id} className="sn-deck-act sn-focus" onClick={() => onOpenActivity?.(a)}>
                  <span className="sn-badge" data-tone={ACT_TONE[a.kind]}>{ACT_LABEL[a.kind]}</span>
                  <span className="sn-deck-act-label">{a.label}</span>
                  <span className="sn-deck-act-at mono">{a.at}</span>
                </button>
              ))
            )}
          </div>
        </section>
      </div>

      <section className="sn-panel">
        <div className="sn-panel-head">发现（Findings）</div>
        <div className="sn-deck-findings">
          {model.findings.length === 0 ? (
            <div className="sn-empty">暂无发现</div>
          ) : (
            model.findings.map((f) => (
              <button key={f.id} className="sn-deck-finding sn-focus" onClick={() => onOpenFinding?.(f.id)}>
                <span className="sn-badge" data-tone={SEV_TONE[f.severity]}>{f.severity}</span>
                <span className="sn-deck-finding-title">{f.title}</span>
                <span className="sn-badge" data-tone="ink-3">{f.state}</span>
              </button>
            ))
          )}
        </div>
      </section>
    </div>
  );
}

export function StatTile({ stat }: { stat: DeckStat }) {
  return (
    <div className="sn-stat" data-tone={stat.tone ?? "ink-2"}>
      <div className="sn-stat-value">{typeof stat.value === "number" ? stat.value.toLocaleString() : stat.value}</div>
      <div className="sn-stat-label">{stat.label}</div>
      {stat.sub && <div className="sn-stat-sub">{stat.sub}</div>}
    </div>
  );
}

// 手写 SVG 柱状图：Token 随时间窗消耗。高度按最大值归一化；零值仍留 1px 以示“有窗无消耗”。
export function TokenChart({ series, windows }: { series: number[]; windows?: string[] }) {
  if (series.length === 0) return <div className="sn-empty">暂无 Token 数据</div>;
  const W = 520;
  const H = 120;
  const padB = 18;
  const max = Math.max(1, ...series);
  const n = series.length;
  const gap = 6;
  const barW = (W - gap * (n - 1)) / n;
  return (
    <div className="sn-chart-scroll">
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" height={H} role="img" aria-label="Token 消耗柱状图" className="sn-tokenchart">
        {series.map((v, i) => {
          const h = Math.max(1, ((v / max) * (H - padB - 6)));
          const x = i * (barW + gap);
          const y = H - padB - h;
          return (
            <g key={i}>
              <rect x={x} y={y} width={barW} height={h} rx={3} className="sn-tokenbar">
                <title>{windows?.[i] ?? `窗 ${i + 1}`}: {v.toLocaleString()} tok</title>
              </rect>
              {windows?.[i] && (
                <text x={x + barW / 2} y={H - 5} textAnchor="middle" className="sn-tokenlabel">{windows[i]}</text>
              )}
            </g>
          );
        })}
      </svg>
    </div>
  );
}
