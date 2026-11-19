import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrategyPage } from "./StrategyPage";
import { JobsPage } from "./JobsPage";
import { HypothesesPage } from "./HypothesesPage";
import { SurfacePage } from "./SurfacePage";
import { TrafficPage } from "./TrafficPage";
import { EvidencePage } from "./EvidencePage";
import { ToolsPage } from "./ToolsPage";
import { TimelinePage } from "./TimelinePage";
import {
  mkStrategy, mkJobs, mkHyp, mkSurface, mkTraffic, mkEvidence, mkTools, mkTimeline,
} from "../../app/mockWorkbench";

describe("StrategyPage（§16.14）", () => {
  it("展示 Epoch + PlanCompiler 逐项结果，方向操作生成 UserDirective", async () => {
    const onD = vi.fn();
    render(<StrategyPage model={mkStrategy} onDirective={onD} />);
    expect(screen.getByText("Epoch 3")).toBeInTheDocument();
    expect(screen.getAllByText("ACCEPTED").length).toBeGreaterThan(0);
    expect(screen.getByText("NEEDS_APPROVAL")).toBeInTheDocument();
    expect(screen.getByText("DEDUPED")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "优先此方向" }));
    expect(onD).toHaveBeenCalledWith("STEER_RUN", expect.stringContaining("优先"));
  });
});

describe("JobsPage（§16.4）", () => {
  it("表格视图 + 点击 Job 打开 Drawer（JobSpec/Cancel）", async () => {
    const onCancel = vi.fn();
    render(<JobsPage model={mkJobs} onCancel={onCancel} />);
    await userEvent.click(screen.getByText("GET /api/orders/42 as A"));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("experiment.idor")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "取消" }));
    expect(onCancel).toHaveBeenCalledWith("job-2");
  });
  it("图视图按模块聚合", async () => {
    render(<JobsPage model={mkJobs} />);
    await userEvent.click(screen.getByRole("tab", { name: /图视图/ }));
    expect(screen.getByText("order")).toBeInTheDocument();
  });
});

describe("HypothesesPage（§16.6）", () => {
  it("看板分列 + 卡片详情", async () => {
    render(<HypothesesPage model={mkHyp} />);
    expect(screen.getByText("已证实")).toBeInTheDocument();
    expect(screen.getByText("红线阻塞")).toBeInTheDocument();
    await userEvent.click(screen.getByText(/账号A 可读账号B 订单/));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("对象归属")).toBeInTheDocument();
  });
});

describe("SurfacePage（§16.5/§16.15）", () => {
  it("Endpoint 表 + 范围外标记 + Discovery Inbox", async () => {
    const onEp = vi.fn();
    render(<SurfacePage model={mkSurface} onEndpointClick={onEp} />);
    expect(screen.getByText("SurfaceRevision rev 7")).toBeInTheDocument();
    expect(screen.getAllByText("范围外").length).toBeGreaterThan(0);
    await userEvent.click(screen.getAllByText("/api/orders/{id}")[0]);
    expect(onEp).toHaveBeenCalled();
    await userEvent.click(screen.getByRole("tab", { name: /发现收件箱/ }));
    expect(screen.getByText(/身份差异/)).toBeInTheDocument();
  });
});

describe("TrafficPage（§16.16）", () => {
  it("三栏 + Replay 显示实际 Job ID（非假设已发送）", async () => {
    const onReplay = vi.fn();
    render(<TrafficPage model={mkTraffic} onReplay={onReplay} />);
    await userEvent.click(screen.getByRole("button", { name: /重放/ }));
    expect(onReplay).toHaveBeenCalledWith("x1");
    expect(screen.getByText(/已创建重放 Job/)).toBeInTheDocument();
  });
});

describe("EvidencePage（§16.9）", () => {
  it("默认脱敏，血缘三元组展示", () => {
    render(<EvidencePage model={mkEvidence} />);
    expect(screen.getByText("脱敏投影")).toBeInTheDocument();
    expect(screen.getByText("身份（血缘）")).toBeInTheDocument();
  });
});

describe("ToolsPage（§16.18）", () => {
  it("四视图：Action/Timeline/Terminal/Error", async () => {
    render(<ToolsPage model={mkTools} />);
    expect(screen.getByRole("tab", { name: "动作视图" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: /终端/ }));
    expect(screen.getByText(/probe \/api\/orders/)).toBeInTheDocument();
  });
});

describe("TimelinePage（§16.28）", () => {
  it("时间轴 + Audit Explorer（重点标记）", async () => {
    render(<TimelinePage model={mkTimeline} />);
    expect(screen.getByText(/Epoch 1 启动/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: "审计浏览器" }));
    expect(screen.getAllByText("重点").length).toBeGreaterThan(0);
  });
});
