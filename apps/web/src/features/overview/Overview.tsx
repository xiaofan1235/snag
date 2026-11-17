// Overview —— Run 总览（UI 文档 §16.3）。核心：**不显示虚假「73% 完成」**，而是真实分数——
// 接口发现（当前 SurfaceRevision）、实验 DONE/TOTAL、覆盖 TESTED/ELIGIBLE、验证 CONFIRMED/CANDIDATE、
// 阻塞具体原因。另展示策略 Epoch/优先方向/活跃 Agent/等待 Job/预算摘要/下一步计划。

import { Card } from "../../components/Card";
import { FractionProgress } from "../../components/Progress";
import { StatusBadge } from "../../components/StatusBadge";

export interface OverviewModel {
  state: string; // Run 状态（StatusBadge）
  strategyEpoch: number;
  priorityDirection: string;
  activeAgents: number;
  waitingJobs: number;
  surfaceRevision: number;
  experiments: { done: number; total: number };
  coverage: { tested: number; eligible: number };
  verification: { confirmed: number; candidate: number };
  budget: { requestsUsed: number; requestsMax: number; costUsedCents: number; costMaxCents: number };
  blockers: string[]; // 具体阻塞原因（空=无阻塞）
  nextPlan?: string;
  recentHighValue?: string[];
}

function Metric({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: 2 }}>{label}</div>
      <div style={{ fontSize: "var(--fs-sm)" }}>{children}</div>
    </div>
  );
}

export function Overview({ model }: { model: OverviewModel }) {
  const m = model;
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-4)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
        <StatusBadge status={m.state} />
        <span style={{ color: "var(--ink-2)" }}>
          策略 Epoch <strong>{m.strategyEpoch}</strong> · 优先方向 <strong>{m.priorityDirection}</strong>
        </span>
        <span style={{ marginLeft: "auto", color: "var(--ink-3)", fontSize: "var(--fs-sm)" }}>
          活跃 Agent {m.activeAgents} · 等待 Job {m.waitingJobs}
        </span>
      </div>

      {/* 真实进度分数（§16.3：绝不虚假百分比） */}
      <Card>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(180px,1fr))", gap: "var(--sp-4)" }}>
          <Metric label="接口发现（SurfaceRevision）">
            <span className="mono">rev {m.surfaceRevision}</span>
          </Metric>
          <Metric label="实验 DONE / TOTAL">
            <FractionProgress done={m.experiments.done} total={m.experiments.total} />
          </Metric>
          <Metric label="覆盖 TESTED / ELIGIBLE">
            <FractionProgress done={m.coverage.tested} total={m.coverage.eligible} tone="ok" />
          </Metric>
          <Metric label="验证 CONFIRMED / CANDIDATE">
            <span className="mono">
              {m.verification.confirmed} / {m.verification.candidate}
            </span>
          </Metric>
        </div>
      </Card>

      {/* 预算摘要 */}
      <Card>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--sp-4)" }}>
          <Metric label="请求预算">
            <FractionProgress
              done={m.budget.requestsUsed}
              total={m.budget.requestsMax}
              tone={m.budget.requestsUsed >= m.budget.requestsMax ? "danger" : "accent"}
            />
          </Metric>
          <Metric label="费用预算（分）">
            <FractionProgress
              done={m.budget.costUsedCents}
              total={m.budget.costMaxCents}
              tone={m.budget.costUsedCents >= m.budget.costMaxCents ? "danger" : "accent"}
            />
          </Metric>
        </div>
      </Card>

      {/* 阻塞——具体原因，不是笼统进度条 */}
      <Card>
        <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-1)" }}>阻塞</div>
        {m.blockers.length === 0 ? (
          <span style={{ color: "var(--ok)" }}>无阻塞</span>
        ) : (
          <ul style={{ margin: 0, paddingLeft: "var(--sp-5)" }}>
            {m.blockers.map((b, i) => (
              <li key={i} style={{ color: "var(--warn)" }}>
                {b}
              </li>
            ))}
          </ul>
        )}
        {m.nextPlan && (
          <div style={{ marginTop: "var(--sp-2)", color: "var(--ink-2)", fontSize: "var(--fs-sm)" }}>
            下一步调度：{m.nextPlan}
          </div>
        )}
      </Card>
    </div>
  );
}
