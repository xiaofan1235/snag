// RunWorkbench —— 进入某 Run 后的工作台（UI 文档 §16.3–16.28 / D4）。横向子导航 + 全部子页；
// 跨对象跳转经右栏 Inspector（Finding↔Evidence↔Verifier↔Endpoint）。

import { useState } from "react";
import { Overview, ActivityStream, FindingDetail, CoverageMatrix, ReportCenter } from "../features";
import { AgentSessionPanel } from "../features/activity/ActivityStream";
import {
  StrategyPage, JobsPage, HypothesesPage, SurfacePage, TrafficPage,
  EvidencePage, BrowserPage, ToolsPage, FilesPage, TimelinePage,
} from "../features/workbench";
import { RUN_NAV, runNavTo } from "./nav";
import { useNavigate } from "./router";
import { useInspector } from "./inspector";
import { useToast } from "../components";
import { Drawer } from "../components/Drawer";
import { PageHead } from "./pages";
import { previewOverview, previewActivity, previewFinding, previewCoverage, previewReport } from "./mockData";
import {
  mkStrategy, mkJobs, mkHyp, mkSurface, mkTraffic, mkEvidence, mkBrowser, mkTools, mkFiles, mkTimeline,
} from "./mockWorkbench";

export function RunWorkbench({ runId, seg }: { runId: string; seg: string }) {
  const navigate = useNavigate();
  const insp = useInspector();
  const toast = useToast();
  const [sessionOpen, setSessionOpen] = useState(false);

  const crumbs = (<><a href="#/runs">Runs</a> › <span>{runId}</span></>);

  return (
    <>
      <PageHead title={`Run ${runId}`} crumbs={crumbs} />

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
            </a>
          );
        })}
      </div>

      {seg === "overview" && <Overview model={previewOverview} />}

      {seg === "strategy" && (
        <StrategyPage model={mkStrategy} onDirective={(k, d) => toast.push({ tone: "info", message: `UserDirective ${k}: ${d}` })} />
      )}

      {seg === "jobs" && (
        <JobsPage
          model={mkJobs}
          onCancel={(id) => toast.push({ tone: "warn", message: `已请求取消 ${id}` })}
          onReprioritize={(id, p) => toast.push({ tone: "info", message: `${id} 优先级→${p}` })}
        />
      )}

      {seg === "surface" && (
        <SurfacePage
          model={mkSurface}
          onEndpointClick={(ep) =>
            insp.show({
              ref: { type: "endpoint", id: ep.id },
              title: `${ep.method} ${ep.url}`,
              fields: [
                { label: "模块", value: ep.module },
                { label: "认证", value: ep.authType },
                { label: "状态码", value: ep.statusCode },
                { label: "范围", value: ep.inScope ? "范围内" : "范围外（只记录）" },
              ],
              related: ep.hasFinding ? [{ label: "查看 Finding", ref: { type: "finding", id: "f-1" } }] : undefined,
            })
          }
          onDiscoveryDirective={(id, a) => toast.push({ tone: "info", message: `Discovery ${id}: ${a}` })}
        />
      )}

      {seg === "hypotheses" && <HypothesesPage model={mkHyp} />}

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
                ],
                related: [{ label: "查看 Finding", ref: { type: "finding", id: "f-1" } }],
              })
            }
            onOpenSession={() => setSessionOpen(true)}
          />
          <Drawer open={sessionOpen} onClose={() => setSessionOpen(false)} title="Agent Session">
            <AgentSessionPanel
              model={{ sessionId: "s1", role: "executor", modelProfile: "default", jobId: "job-2", attempt: 2, generation: 1, contextRevision: 5, compactions: 1, tokenUsed: 1200, tools: ["http", "browser"], lastInputSummary: "最近输入摘要（Redacted Projection）" }}
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
              fields: [{ label: "对象", value: `${kind}:${id}` }, { label: "Run", value: runId }],
              related: [{ label: "回到 Finding", ref: { type: "finding", id: previewFinding.id } }],
            })
          }
        />
      )}

      {seg === "traffic" && (
        <TrafficPage model={mkTraffic} onReplay={(id) => toast.push({ tone: "info", message: `Replay Job 创建（exchange ${id}）` })} />
      )}

      {seg === "evidence" && (
        <EvidencePage
          model={mkEvidence}
          onReveal={async (evId, tab) => {
            toast.push({ tone: "warn", message: `Reveal ${evId}/${tab}（短时授权+审计）` });
            return `原始内容（已 Reveal，${evId}/${tab}）`;
          }}
        />
      )}

      {seg === "browser" && (
        <BrowserPage model={mkBrowser} onTakeover={(id) => toast.push({ tone: "danger", message: `已发起人工接管审批（${id}）` })} />
      )}

      {seg === "tools" && <ToolsPage model={mkTools} />}

      {seg === "files" && (
        <FilesPage model={mkFiles} onPreview={(p) => toast.push({ tone: "info", message: `安全预览 ${p}` })} />
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

      {seg === "timeline" && <TimelinePage model={mkTimeline} />}
    </>
  );
}
