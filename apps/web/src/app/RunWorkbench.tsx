// RunWorkbench —— 进入某 Run 后的工作台（UI 文档 D4）。横向子导航 + 子页；
// 跨对象跳转经右栏 Inspector（Finding↔Evidence↔Verifier）。

import { Overview, ActivityStream, FindingDetail, CoverageMatrix, ReportCenter } from "../features";
import { AgentSessionPanel } from "../features/activity/ActivityStream";
import { RUN_NAV, runNavTo } from "./nav";
import { useNavigate } from "./router";
import { useInspector } from "./inspector";
import { useToast } from "../components";
import { Drawer } from "../components/Drawer";
import { PageHead, PlaceholderPage } from "./pages";
import { previewOverview, previewActivity, previewFinding, previewCoverage, previewReport } from "./mockData";
import { useState } from "react";

export function RunWorkbench({ runId, seg }: { runId: string; seg: string }) {
  const navigate = useNavigate();
  const insp = useInspector();
  const toast = useToast();
  const [sessionOpen, setSessionOpen] = useState(false);

  const crumbs = (
    <>
      <a href="#/runs">Runs</a> › <span>{runId}</span>
    </>
  );

  return (
    <>
      <PageHead title={`Run ${runId}`} crumbs={crumbs} />

      {/* 子导航 */}
      <div className="run-subnav" role="tablist" aria-label="Run 工作台">
        {RUN_NAV.map((item) => {
          const active = item.seg === seg;
          return (
            <a
              key={item.seg}
              href={"#" + runNavTo(runId, item.seg)}
              role="tab"
              aria-selected={active}
              data-active={active || undefined}
              onClick={(e) => {
                if (e.metaKey || e.ctrlKey || e.button !== 0) return;
                e.preventDefault();
                navigate(runNavTo(runId, item.seg));
              }}
            >
              {item.label}
              {!item.implemented && <span style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}> ·空</span>}
            </a>
          );
        })}
      </div>

      {/* 子页 */}
      {seg === "overview" && <Overview model={previewOverview} />}

      {seg === "activity" && (
        <>
          <ActivityStream
            events={previewActivity}
            onJump={(ev) =>
              insp.show({
                ref: { type: "evidence", id: `seq-${ev.sequence}` },
                title: ev.event_type,
                fields: [
                  { label: "事件类型", value: ev.event_type },
                  { label: "序号", value: ev.sequence },
                  { label: "Run", value: ev.run_id ?? runId },
                ],
                related: [{ label: "查看 Finding", ref: { type: "finding", id: "f-1" } }],
              })
            }
            onOpenSession={() => setSessionOpen(true)}
          />
          <Drawer open={sessionOpen} onClose={() => setSessionOpen(false)} title="Agent Session">
            <AgentSessionPanel
              model={{
                sessionId: "s1",
                role: "executor",
                modelProfile: "default",
                jobId: "job-1",
                attempt: 2,
                generation: 1,
                contextRevision: 5,
                compactions: 1,
                tokenUsed: 1200,
                tools: ["http", "browser"],
                lastInputSummary: "最近输入摘要（Redacted Projection）",
              }}
            />
          </Drawer>
        </>
      )}

      {seg === "findings" && (
        <FindingDetail
          model={previewFinding}
          onReplay={(id) => toast.push({ tone: "info", message: `已创建重放 Verification Job（${id}）` })}
          onJump={(kind, id) =>
            insp.show({
              ref: { type: kind === "verifier" ? "finding" : kind, id },
              title: kind === "verifier" ? "Verifier 意见" : kind,
              fields: [
                { label: "对象", value: `${kind}:${id}` },
                { label: "Run", value: runId },
              ],
              related: [{ label: "回到 Finding", ref: { type: "finding", id: previewFinding.id } }],
            })
          }
        />
      )}

      {seg === "coverage" && (
        <CoverageMatrix
          model={previewCoverage}
          onDrillDown={(cell) =>
            insp.show({
              ref: { type: "endpoint", id: cell.endpoint },
              title: `${cell.endpoint} · ${cell.attackType}`,
              fields: [
                { label: "状态", value: cell.state },
                { label: "身份", value: cell.identity },
                ...(cell.actionId ? [{ label: "Action", value: cell.actionId }] : []),
                ...(cell.evidenceId ? [{ label: "Evidence", value: cell.evidenceId }] : []),
                ...(cell.untestedReason ? [{ label: "未测原因", value: cell.untestedReason }] : []),
              ],
            })
          }
        />
      )}

      {seg === "report" && (
        <ReportCenter model={previewReport} onExport={(id, fmt) => toast.push({ tone: "ok", message: `导出 ${fmt}（${id}）` })} />
      )}

      {/* 未实现子页空态 */}
      {!["overview", "activity", "findings", "coverage", "report"].includes(seg) && (
        <PlaceholderPage title={RUN_NAV.find((n) => n.seg === seg)?.label ?? seg} note={`Run 工作台「${seg}」页随对应子系统接线后填充；此处不展示假数据（§16.34）。`} />
      )}
    </>
  );
}
