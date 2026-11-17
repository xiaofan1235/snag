// Dashboard —— 全局值班台（UI 文档 §16.12/D6）。四区：需要我处理 / 正在运行 / 最近结果 / 系统健康。
// Run 卡片显示**真实状态**：终态 COMPLETED/PARTIAL/STOPPED/FAILED 与可恢复非终态 BLOCKED/PAUSED
// 视觉区分（形状+图标+色，不只用色；BLOCKED 提供恢复入口）。卡片操作调命令 API，不在前端改状态。

import { Card } from "../../components/Card";
import { StatusBadge } from "../../components/StatusBadge";
import { Button } from "../../components/Button";

export interface AttentionItem {
  id: string;
  kind: "approval" | "auth_expired" | "proxy_expired" | "budget_low" | "outcome_unknown" | "cleanup_failed";
  label: string;
  runId?: string;
}
export interface RunningRun {
  runId: string;
  name: string;
  state: string;
  strategy: string;
  activeJobs: number;
  lastAction?: string;
  requestsUsed: number;
  requestsMax: number;
}
export interface RecentResult {
  id: string;
  label: string;
  kind: "finding" | "report" | "replay" | "coverage";
}
export interface SystemHealth {
  provider: string;
  egress: string;
  sandbox: string;
  database: string;
  blobStore: string;
  eventStream: string;
}
export interface DashboardModel {
  attention: AttentionItem[];
  running: RunningRun[];
  recent: RecentResult[];
  health: SystemHealth;
}

export interface DashboardProps {
  model: DashboardModel;
  onOpen?: (runId: string) => void;
  onPause?: (runId: string) => void;
  onResume?: (runId: string) => void;
  onStop?: (runId: string) => void;
}

const TERMINAL = new Set(["COMPLETED", "PARTIAL", "STOPPED", "FAILED"]);

export function Dashboard({ model, onOpen, onPause, onResume, onStop }: DashboardProps) {
  return (
    <div style={{ display: "grid", gap: "var(--sp-4)" }}>
      {/* 1. 需要我处理（首屏第一区） */}
      <section aria-labelledby="dash-attention">
        <h2 id="dash-attention" style={{ fontSize: "var(--fs-lg)", margin: "0 0 var(--sp-2)" }}>
          需要我处理
        </h2>
        {model.attention.length === 0 ? (
          <Card>
            <span style={{ color: "var(--ok)" }}>当前没有待处理事项</span>
          </Card>
        ) : (
          <div style={{ display: "grid", gap: "var(--sp-2)" }}>
            {model.attention.map((a) => (
              <Card key={a.id}>
                <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
                  <StatusBadge status={a.kind === "approval" ? "QUEUED" : "BLOCKED"} />
                  <span>{a.label}</span>
                  {a.runId && onOpen && (
                    <Button size="sm" variant="secondary" style={{ marginLeft: "auto" }} onClick={() => onOpen(a.runId!)}>
                      处理
                    </Button>
                  )}
                </div>
              </Card>
            ))}
          </div>
        )}
      </section>

      {/* 2. 正在运行 */}
      <section aria-labelledby="dash-running">
        <h2 id="dash-running" style={{ fontSize: "var(--fs-lg)", margin: "0 0 var(--sp-2)" }}>
          正在运行
        </h2>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill,minmax(280px,1fr))", gap: "var(--sp-3)" }}>
          {model.running.map((r) => {
            const terminal = TERMINAL.has(r.state);
            const blocked = r.state === "BLOCKED";
            return (
              <Card key={r.runId} clickable onClick={() => onOpen?.(r.runId)} data-terminal={terminal}>
                <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-2)" }}>
                  <strong>{r.name}</strong>
                  <span style={{ marginLeft: "auto" }}>
                    <StatusBadge status={r.state} />
                  </span>
                </div>
                <div style={{ color: "var(--ink-2)", fontSize: "var(--fs-sm)" }}>
                  策略 {r.strategy} · 活跃 Job {r.activeJobs}
                  {r.lastAction && <div className="mono" style={{ fontSize: "var(--fs-xs)", color: "var(--ink-3)" }}>{r.lastAction}</div>}
                </div>
                <div style={{ marginTop: "var(--sp-2)", fontSize: "var(--fs-xs)", color: "var(--ink-3)" }}>
                  请求 {r.requestsUsed}/{r.requestsMax}
                </div>
                <div style={{ display: "flex", gap: "var(--sp-1)", marginTop: "var(--sp-2)" }} onClick={(e) => e.stopPropagation()}>
                  {blocked && onResume && (
                    <Button size="sm" variant="primary" onClick={() => onResume(r.runId)}>
                      恢复
                    </Button>
                  )}
                  {!terminal && !blocked && onPause && (
                    <Button size="sm" variant="ghost" onClick={() => onPause(r.runId)}>
                      暂停
                    </Button>
                  )}
                  {!terminal && onStop && (
                    <Button size="sm" variant="ghost" onClick={() => onStop(r.runId)}>
                      停止
                    </Button>
                  )}
                </div>
              </Card>
            );
          })}
        </div>
      </section>

      {/* 3. 最近结果 */}
      <section aria-labelledby="dash-recent">
        <h2 id="dash-recent" style={{ fontSize: "var(--fs-lg)", margin: "0 0 var(--sp-2)" }}>
          最近结果
        </h2>
        <Card>
          {model.recent.length === 0 ? (
            <span style={{ color: "var(--ink-3)" }}>暂无最近结果</span>
          ) : (
            <ul style={{ margin: 0, paddingLeft: "var(--sp-5)" }}>
              {model.recent.map((r) => (
                <li key={r.id} style={{ fontSize: "var(--fs-sm)" }}>
                  {r.label}
                </li>
              ))}
            </ul>
          )}
        </Card>
      </section>

      {/* 4. 系统健康 */}
      <section aria-labelledby="dash-health">
        <h2 id="dash-health" style={{ fontSize: "var(--fs-lg)", margin: "0 0 var(--sp-2)" }}>
          系统健康
        </h2>
        <Card>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(120px,1fr))", gap: "var(--sp-3)", fontSize: "var(--fs-sm)" }}>
            {Object.entries({
              Provider: model.health.provider,
              Egress: model.health.egress,
              Sandbox: model.health.sandbox,
              数据库: model.health.database,
              BlobStore: model.health.blobStore,
              事件流: model.health.eventStream,
            }).map(([k, v]) => (
              <div key={k}>
                <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>{k}</div>
                <StatusBadge status={v} />
              </div>
            ))}
          </div>
        </Card>
      </section>
    </div>
  );
}
