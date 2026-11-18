import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionTree } from "./SessionTree";
import { OrchestrationChat } from "./OrchestrationChat";
import { ExplorationGraph } from "./ExplorationGraph";
import { CommandDeck } from "./CommandDeck";
import { Guard } from "./Guard";
import { mkSessionTree, mkOrchestration, mkGraph, mkDeck, mkGuard } from "../../app/mockCommand";

describe("SessionTree（多 Agent 会话树）", () => {
  it("渲染主/规划/工人层级，点击节点触发聚焦", async () => {
    const onFocus = vi.fn();
    render(<SessionTree model={mkSessionTree} onFocus={onFocus} />);
    expect(screen.getByRole("tree", { name: "Agent 会话" })).toBeInTheDocument();
    expect(screen.getByText("主 Agent · 编排")).toBeInTheDocument();
    expect(screen.getByText("规划器")).toBeInTheDocument();
    // 5 个工人（ARTEX task session view：main+planner+5 workers）。
    expect(screen.getByText("5 工人")).toBeInTheDocument();
    await userEvent.click(screen.getByText("recon · /api/v2/*"));
    expect(onFocus).toHaveBeenCalledWith("w2");
  });

  it("工人节点的 Job 链接触发打开（不冒泡到聚焦）", async () => {
    const onFocus = vi.fn();
    const onJob = vi.fn();
    render(<SessionTree model={mkSessionTree} onFocus={onFocus} onOpenJob={onJob} />);
    await userEvent.click(within(screen.getByRole("tree")).getAllByText("作业")[0]);
    expect(onJob).toHaveBeenCalled();
    expect(onFocus).not.toHaveBeenCalled();
  });
});

describe("OrchestrationChat（编排对话 + 人工转向）", () => {
  it("渲染分型转录，下发转向指令触发回调并清空", async () => {
    const onSteer = vi.fn();
    render(<OrchestrationChat model={mkOrchestration} focusedTitle="authz · /api/orders IDOR" onSteer={onSteer} />);
    expect(screen.getByRole("log", { name: "推理流" })).toBeInTheDocument();
    // 分型徽标存在（待审批）。
    expect(screen.getAllByText("待审批").length).toBeGreaterThan(0);
    const box = screen.getByLabelText("与 AI 对话输入");
    await userEvent.type(box, "聚焦 /api/orders");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));
    expect(onSteer).toHaveBeenCalledWith("w1", "聚焦 /api/orders");
    expect((box as HTMLTextAreaElement).value).toBe("");
    // 发送后，人工消息进入对话流（两段：你 + Agent 回执）。
    expect(screen.getByText(/【你】聚焦 \/api\/orders/)).toBeInTheDocument();
  });

  it("不可转向时输入禁用", () => {
    render(<OrchestrationChat model={{ ...mkOrchestration, canSteer: false }} focusedTitle="x" />);
    expect(screen.getByLabelText("与 AI 对话输入")).toBeDisabled();
  });

  it("「仅聚焦」范围过滤到聚焦 Agent", async () => {
    render(<OrchestrationChat model={mkOrchestration} focusedTitle="x" />);
    // 全部范围下存在 main 的推理条目文案。
    expect(screen.getByText(/authz 模块优先/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "仅聚焦" }));
    // 切到仅聚焦（w1）后，main 的「派发规划器」文案应消失。
    expect(screen.queryByText(/派发规划器/)).not.toBeInTheDocument();
  });
});

describe("ExplorationGraph（探索知识图谱）", () => {
  it("渲染四层节点与图例，点击节点触发选择", async () => {
    const onSelect = vi.fn();
    render(<ExplorationGraph model={mkGraph} onSelect={onSelect} />);
    expect(screen.getByRole("img", { name: "探索知识图谱" })).toBeInTheDocument();
    // finding 节点可点击（label 以 aria-label 暴露）。
    await userEvent.click(screen.getByRole("button", { name: /IDOR on \/orders/ }));
    expect(onSelect).toHaveBeenCalled();
    expect(onSelect.mock.calls[0][0].kind).toBe("finding");
  });

  it("空图谱走只读空态", () => {
    render(<ExplorationGraph model={{ nodes: [], edges: [] }} />);
    expect(screen.getByText(/暂无探索节点/)).toBeInTheDocument();
  });
});

describe("CommandDeck（统一作战台）", () => {
  it("渲染统计瓦片 + Token 图 + 发现，点击发现触发回调", async () => {
    const onF = vi.fn();
    render(<CommandDeck model={mkDeck} onOpenFinding={onF} />);
    expect(screen.getByText("活跃 Jobs")).toBeInTheDocument();
    expect(screen.getByText("录制流量")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Token 消耗柱状图" })).toBeInTheDocument();
    await userEvent.click(screen.getByText("IDOR on /orders"));
    expect(onF).toHaveBeenCalledWith("f-1");
  });
});

describe("Guard（拦截规则 + 审批日志）", () => {
  it("渲染拦截规则表，开关切换翻转状态", async () => {
    const onToggle = vi.fn();
    render(<Guard model={mkGuard} onToggleRule={onToggle} />);
    expect(screen.getByRole("tab", { name: /拦截规则/ })).toBeInTheDocument();
    const firstSwitch = screen.getAllByRole("switch")[0];
    expect(firstSwitch).toBeChecked();
    await userEvent.click(firstSwitch);
    expect(onToggle).toHaveBeenCalled();
    expect(firstSwitch).not.toBeChecked();
  });

  it("切到审批日志 Tab 显示决定记录", async () => {
    render(<Guard model={mkGuard} />);
    await userEvent.click(screen.getByRole("tab", { name: /审批日志/ }));
    expect(await screen.findByText("拒绝+取消")).toBeInTheDocument();
    expect(screen.getByText(/DELETE \/api\/orders\/42/)).toBeInTheDocument();
  });
});
