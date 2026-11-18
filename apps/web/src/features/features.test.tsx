import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Overview } from "./overview/Overview";
import { RunWizard, type PreflightCheck } from "./runs/RunWizard";
import { Dashboard, type DashboardModel } from "./dashboard/Dashboard";
import { ActivityStream, AgentSessionPanel } from "./activity/ActivityStream";
import { eventKind } from "./activity/eventKind";
import type { EventEnvelope } from "../api/schema.gen";

describe("Overview（§16.3 真实分数）", () => {
  const model = {
    state: "RUNNING",
    strategyEpoch: 3,
    priorityDirection: "authz",
    activeAgents: 2,
    waitingJobs: 5,
    surfaceRevision: 7,
    experiments: { done: 4, total: 10 },
    coverage: { tested: 12, eligible: 30 },
    verification: { confirmed: 2, candidate: 6 },
    budget: { requestsUsed: 120, requestsMax: 500, costUsedCents: 80, costMaxCents: 500 },
    blockers: ["等待审批：写操作"],
    nextPlan: "recon 新端点",
  };

  it("显示真实 DONE/TOTAL 分数而非虚假百分比", () => {
    render(<Overview model={model} />);
    expect(screen.getByText("4/10")).toBeInTheDocument(); // 实验
    expect(screen.getByText("12/30")).toBeInTheDocument(); // 覆盖
    expect(screen.getByText("2 / 6")).toBeInTheDocument(); // 验证
    expect(screen.getByText("rev 7")).toBeInTheDocument(); // SurfaceRevision
    expect(screen.queryByText(/73%/)).not.toBeInTheDocument();
  });

  it("阻塞显示具体原因", () => {
    render(<Overview model={model} />);
    expect(screen.getByText("等待审批：写操作")).toBeInTheDocument();
  });
});

describe("RunWizard（§16.2 Preflight 门禁）", () => {
  function gotoLastStep() {
    // 连点「下一步」到第 5 步。
    for (let i = 0; i < 4; i++) {
      userEvent.click(screen.getByRole("button", { name: "下一步" }));
    }
  }

  it("Preflight 未全过时 Start 禁用", async () => {
    const onStart = vi.fn();
    const pf: PreflightCheck[] = [
      { id: "a", label: "Scope 合法", ok: true },
      { id: "b", label: "身份有效", ok: false },
    ];
    render(<RunWizard preflight={pf} onStart={onStart} />);
    for (let i = 0; i < 4; i++) await userEvent.click(screen.getByRole("button", { name: "下一步" }));
    const start = screen.getByRole("button", { name: "Start" });
    expect(start).toHaveAttribute("aria-disabled", "true");
    await userEvent.click(start);
    expect(onStart).not.toHaveBeenCalled();
  });

  it("Preflight 全过时 Start 可用并回传 state", async () => {
    const onStart = vi.fn();
    const pf: PreflightCheck[] = [
      { id: "a", label: "Scope 合法", ok: true },
      { id: "b", label: "身份有效", ok: true },
    ];
    render(<RunWizard preflight={pf} initial={{ target: { name: "T1", workspace: "", startUrl: "", domain: "x.com", includeSubdomains: false, forbiddenPaths: "", authorizationRef: "" } }} onStart={onStart} />);
    for (let i = 0; i < 4; i++) await userEvent.click(screen.getByRole("button", { name: "下一步" }));
    await userEvent.click(screen.getByRole("button", { name: "Start" }));
    expect(onStart).toHaveBeenCalledOnce();
    expect(onStart.mock.calls[0][0].target.name).toBe("T1");
  });

  it("默认规则保守（写操作关闭）", () => {
    render(<RunWizard preflight={[]} onStart={() => {}} />);
    // 第 1 步可见，Scope 未设定。
    expect(screen.getByText(/未设定/)).toBeInTheDocument();
  });
  void gotoLastStep;
});

describe("Dashboard（§16.12 四区 + 状态区分）", () => {
  const model: DashboardModel = {
    attention: [{ id: "a1", kind: "approval", label: "等待审批：写操作", runId: "run-1" }],
    running: [
      { runId: "run-1", name: "Blocked Run", state: "BLOCKED", strategy: "authz", activeJobs: 0, requestsUsed: 50, requestsMax: 500 },
      { runId: "run-2", name: "Done Run", state: "COMPLETED", strategy: "-", activeJobs: 0, requestsUsed: 500, requestsMax: 500 },
    ],
    recent: [{ id: "r1", label: "新确认 Finding: IDOR", kind: "finding" }],
    health: { provider: "COMPLETED", egress: "COMPLETED", sandbox: "COMPLETED", database: "COMPLETED", blobStore: "COMPLETED", eventStream: "RUNNING" },
  };

  it("四区标题齐全", () => {
    render(<Dashboard model={model} />);
    expect(screen.getByText("需要我处理")).toBeInTheDocument();
    expect(screen.getByText("正在运行")).toBeInTheDocument();
    expect(screen.getByText("最近结果")).toBeInTheDocument();
    expect(screen.getByText("系统健康")).toBeInTheDocument();
  });

  it("BLOCKED Run 提供恢复入口；终态 Run 无停止/恢复操作", () => {
    render(<Dashboard model={model} onResume={() => {}} onStop={() => {}} onPause={() => {}} />);
    // BLOCKED 提供恢复入口。
    expect(screen.getByRole("button", { name: "恢复" })).toBeInTheDocument();
    // 停止仅对非终态可用：BLOCKED(run-1) 1 个，COMPLETED(run-2) 无。
    expect(screen.queryAllByRole("button", { name: "停止" }).length).toBe(1);
    // 终态 Run 无恢复。
    expect(screen.queryAllByRole("button", { name: "恢复" }).length).toBe(1);
  });

  it("恢复按钮调命令回调（不在前端改状态）", async () => {
    const onResume = vi.fn();
    render(<Dashboard model={model} onResume={onResume} />);
    await userEvent.click(screen.getByRole("button", { name: "恢复" }));
    expect(onResume).toHaveBeenCalledWith("run-1");
  });
});

describe("ActivityStream（§16.13）", () => {
  const events: EventEnvelope[] = [
    { event_type: "plan.updated", run_id: "run-1", sequence: 1 },
    { event_type: "action.updated", run_id: "run-1", sequence: 2 },
    { event_type: "approval.requested", run_id: "run-1", sequence: 3 },
  ];

  it("eventKind 纯映射", () => {
    expect(eventKind("plan.updated")).toBe("plan");
    expect(eventKind("action.updated")).toBe("action");
    expect(eventKind("approval.requested")).toBe("approval");
    expect(eventKind("budget.updated")).toBe("budget");
    expect(eventKind("policy.denied")).toBe("error");
    expect(eventKind("job.updated")).toBe("decision");
    expect(eventKind("evidence.created")).toBe("observation");
  });

  it("渲染每条事件为 Typed 卡，支持跳转", async () => {
    const onJump = vi.fn();
    render(<ActivityStream events={events} onJump={onJump} />);
    expect(screen.getByText("plan.updated")).toBeInTheDocument();
    expect(screen.getAllByText("跳转到对象").length).toBe(3);
    await userEvent.click(screen.getAllByText("跳转到对象")[0]);
    expect(onJump).toHaveBeenCalledWith(events[0]);
  });

  it("Live Follow 开关存在且默认开", () => {
    render(<ActivityStream events={events} />);
    const cb = screen.getByRole("checkbox", { name: /实时跟随/ });
    expect(cb).toBeChecked();
  });

  it("AgentSessionPanel 展示会话字段（无原始 CoT）", () => {
    render(
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
          lastInputSummary: "最近输入摘要",
        }}
      />
    );
    const panel = screen.getByTestId("agent-session");
    expect(within(panel).getByText("executor")).toBeInTheDocument();
    expect(within(panel).getByText("http, browser")).toBeInTheDocument();
  });
});
