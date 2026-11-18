// RunWorkbench —— 进入某 Run 后的作战视图（ARTEX 式编排中心重构 + UI 文档 §16.3–16.28 / D4）。
// 顶部主模式：指挥台（会话树+编排对话）/ 探索图谱 / 作战台 / 守卫；「细节」模式收纳原 15 页二级导航。
// 跨对象跳转统一经右栏 Inspector（Finding↔Evidence↔Verifier↔Endpoint）。

import { useState } from "react";
import { Overview, ActivityStream, FindingDetail, CoverageMatrix, ReportCenter } from "../features";
import { AgentSessionPanel } from "../features/activity/ActivityStream";
import {
  StrategyPage, JobsPage, HypothesesPage, SurfacePage, TrafficPage,
  EvidencePage, BrowserPage, ToolsPage, FilesPage, TimelinePage,
} from "../features/workbench";
import { SessionTree, OrchestrationChat, ExplorationGraph, CommandDeck, Guard } from "../features/command";
import {
  RUN_MODES, RUN_NAV, RUN_DETAIL_DEFAULT, isRunModeSeg, runNavTo,
} from "./nav";
import { useNavigate } from "./router";
import { useInspector } from "./inspector";
import { useToast } from "../components";
import { Drawer } from "../components/Drawer";
import type { InspectorModel } from "../components/domain/Inspector";
import { PageHead } from "./pages";
import { previewOverview, previewActivity, previewFinding, previewCoverage, previewReport } from "./mockData";
import {
  mkStrategy, mkJobs, mkHyp, mkSurface, mkTraffic, mkEvidence, mkBrowser, mkTools, mkFiles, mkTimeline,
} from "./mockWorkbench";
import { mkSessionTree, mkOrchestration, mkGraph, mkDeck, mkGuard } from "./mockCommand";

// 通用：由 objectRef 构造 Inspector 模型（跨对象跳转的统一出口）。
function inspectFromRef(ref: { type: string; id: string }, runId: string): InspectorModel {
  const related =
    ref.type === "evidence"
      ? [{ label: "查看关联 Finding", ref: { type: "finding", id: "f-1" } }]
      : ref.type === "finding"
      ? [{ label: "打开 Findings 细节", ref: { type: "finding", id: ref.id } }]
      : undefined;
  return {
    ref,
    title: `${ref.type} · ${ref.id}`,
    fields: [
      { label: "类型", value: ref.type },
      { label: "ID", value: ref.id },
      { label: "Run", value: runId },
    ],
    related,
  };
}

export function RunWorkbench({ runId, seg }: { runId: string; seg: string }) {
  const navigate = useNavigate();
  const mode = isRunModeSeg(seg) ? seg : "detail";
  const crumbs = (<><a href="#/runs">Runs</a> › <span>{runId}</span></>);

  return (
    <>
      <PageHead title={`Run ${runId}`} crumbs={crumbs} />

      {/* 主模式导航（ARTEX 式作战入口） */}
      <nav className="run-modebar" role="tablist" aria-label="Run 作战模式">
        {RUN_MODES.map((m) => {
          const active = m.seg === "detail" ? mode === "detail" : m.seg === seg;
          const target = m.seg === "detail" ? RUN_DETAIL_DEFAULT : m.seg;
          return (
            <a
              key={m.seg}
              href={"#" + runNavTo(runId, target)}
              role="tab"
              aria-selected={active}
              data-active={active || undefined}
              className="run-mode"
              onClick={(e) => {
                if (e.metaKey || e.ctrlKey || e.button !== 0) return;
                e.preventDefault();
                navigate(runNavTo(runId, target));
              }}
            >
              <span className="run-mode-icon" aria-hidden="true">{m.icon}</span>
              {m.label}
            </a>
          );
        })}
      </nav>

      {seg === "command" && <CommandMode runId={runId} />}
      {seg === "graph" && <GraphMode />}
      {seg === "deck" && <DeckMode runId={runId} />}
      {seg === "guard" && <GuardMode />}
      {mode === "detail" && <RunDetail runId={runId} seg={seg} />}
    </>
  );
}

// ========== 主模式：指挥台（会话树 + 编排对话） ==========
function CommandMode({ runId }: { runId: string }) {
  const navigate = useNavigate();
  const insp = useInspector();
  const toast = useToast();
  const [focusedId, setFocusedId] = useState(mkSessionTree.focusedId);
  const focused = mkSessionTree.sessions.find((s) => s.id === focusedId);

  return (
    <div className="sn-command-layout">
      <SessionTree
        model={{ ...mkSessionTree, focusedId }}
        onFocus={setFocusedId}
        onOpenJob={() => navigate(runNavTo(runId, "jobs"))}
      />
      <OrchestrationChat
        model={{ ...mkOrchestration, focusedId }}
        focusedTitle={focused?.title ?? "—"}
        onSteer={(agentId, text) => toast.push({ tone: "info", title: "转向指令已下发", message: `${agentId}：${text}` })}
        onJump={(ref) => insp.show(inspectFromRef(ref, runId))}
      />
    </div>
  );
}

// ========== 主模式：探索图谱 ==========
function GraphMode() {
  const insp = useInspector();
  return (
    <ExplorationGraph
      model={mkGraph}
      onSelect={(node) =>
        insp.show({
          ref: node.objectRef ?? { type: node.kind, id: node.id },
          title: node.label,
          fields: [
            { label: "类型", value: node.kind },
            ...(node.state ? [{ label: "状态", value: node.state }] : []),
          ],
          related: node.objectRef ? [{ label: `打开 ${node.objectRef.type}`, ref: node.objectRef }] : undefined,
        })
      }
    />
  );
}

// ========== 主模式：作战台 ==========
function DeckMode({ runId }: { runId: string }) {
  const navigate = useNavigate();
  const toast = useToast();
  return (
    <CommandDeck
      model={mkDeck}
      onOpenFinding={() => navigate(runNavTo(runId, "findings"))}
      onOpenActivity={(a) => toast.push({ tone: "info", message: a.label })}
    />
  );
}

// ========== 主模式：守卫（拦截规则 + 审批日志） ==========
function GuardMode() {
  const toast = useToast();
  return (
    <Guard
      model={mkGuard}
      onToggleRule={(id, enabled) =>
        toast.push({ tone: enabled ? "ok" : "warn", message: `拦截规则 ${id} 已${enabled ? "启用" : "停用"}` })
      }
    />
  );
}

// ========== 细节层：原 15 页（全部保留，二级导航） ==========
function RunDetail({ runId, seg }: { runId: string; seg: string }) {
  const navigate = useNavigate();
  const insp = useInspector();
  const toast = useToast();
  const [sessionOpen, setSessionOpen] = useState(false);
  const detailSeg = RUN_NAV.some((n) => n.seg === seg) ? seg : RUN_DETAIL_DEFAULT;

  return (
    <>
      <div className="run-detailbar" role="tablist" aria-label="Run 细节视图">
        {RUN_NAV.map((item) => {
          const active = item.seg === detailSeg;
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

      {detailSeg === "overview" && <Overview model={previewOverview} />}

      {detailSeg === "strategy" && (
        <StrategyPage model={mkStrategy} onDirective={(k, d) => toast.push({ tone: "info", message: `UserDirective ${k}: ${d}` })} />
      )}

      {detailSeg === "jobs" && (
        <JobsPage
          model={mkJobs}
          onCancel={(id) => toast.push({ tone: "warn", message: `已请求取消 ${id}` })}
          onReprioritize={(id, p) => toast.push({ tone: "info", message: `${id} 优先级→${p}` })}
        />
      )}

      {detailSeg === "surface" && (
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

      {detailSeg === "hypotheses" && <HypothesesPage model={mkHyp} />}

      {detailSeg === "activity" && (
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

      {detailSeg === "findings" && (
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

      {detailSeg === "traffic" && (
        <TrafficPage model={mkTraffic} onReplay={(id) => toast.push({ tone: "info", message: `Replay Job 创建（exchange ${id}）` })} />
      )}

      {detailSeg === "evidence" && (
        <EvidencePage
          model={mkEvidence}
          onReveal={async (evId, tab) => {
            toast.push({ tone: "warn", message: `Reveal ${evId}/${tab}（短时授权+审计）` });
            return `原始内容（已 Reveal，${evId}/${tab}）`;
          }}
        />
      )}

      {detailSeg === "browser" && (
        <BrowserPage model={mkBrowser} onTakeover={(id) => toast.push({ tone: "danger", message: `已发起人工接管审批（${id}）` })} />
      )}

      {detailSeg === "tools" && <ToolsPage model={mkTools} />}

      {detailSeg === "files" && (
        <FilesPage model={mkFiles} onPreview={(p) => toast.push({ tone: "info", message: `安全预览 ${p}` })} />
      )}

      {detailSeg === "coverage" && (
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

      {detailSeg === "report" && (
        <ReportCenter model={previewReport} onExport={(id, fmt) => toast.push({ tone: "ok", message: `导出 ${fmt}（${id}）` })} />
      )}

      {detailSeg === "timeline" && <TimelinePage model={mkTimeline} />}
    </>
  );
}
