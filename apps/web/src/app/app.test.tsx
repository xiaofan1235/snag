import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { RouterProvider, matchRoute } from "./router";
import { InspectorProvider } from "./inspector";
import { ToastProvider } from "../components";

function renderApp() {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <ToastProvider>
        <RouterProvider>
          <InspectorProvider>
            <App />
          </InspectorProvider>
        </RouterProvider>
      </ToastProvider>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  window.location.hash = "/";
});

describe("matchRoute", () => {
  it("静态段精确匹配", () => {
    expect(matchRoute("/runs", "/runs")).toEqual({});
    expect(matchRoute("/runs", "/inbox")).toBeNull();
  });
  it(":param 动态段提取", () => {
    expect(matchRoute("/runs/:id/:seg", "/runs/run-1/findings")).toEqual({ id: "run-1", seg: "findings" });
  });
  it("段数不符不匹配", () => {
    expect(matchRoute("/runs/:id", "/runs/run-1/findings")).toBeNull();
  });
});

describe("App 外壳与导航", () => {
  it("渲染三区外壳：状态条 + 主导航 + 底部指令输入", () => {
    renderApp();
    expect(screen.getByRole("navigation", { name: "主导航" })).toBeInTheDocument();
    expect(screen.getByLabelText("指令输入")).toBeInTheDocument();
    // 状态条里有 Epoch。
    expect(screen.getByText(/Epoch 3/)).toBeInTheDocument();
  });

  it("默认路由渲染值班台（Dashboard 四区）", () => {
    renderApp();
    expect(screen.getByText("值班台")).toBeInTheDocument();
    expect(screen.getByText("需要我处理")).toBeInTheDocument();
  });

  it("点击左导航 Runs → 进入 Runs 列表", async () => {
    renderApp();
    const nav = screen.getByRole("navigation", { name: "主导航" });
    await userEvent.click(within(nav).getByText("Runs"));
    expect(await screen.findByText("Runs 列表 / 历史")).toBeInTheDocument();
  });

  it("从 Runs 列表点一个 Run → 进入工作台 Overview", async () => {
    window.location.hash = "/runs";
    renderApp();
    await userEvent.click(screen.getByText("acme.com 授权测试"));
    // 工作台标题 + 子导航。
    expect(await screen.findByText(/Run run-1/)).toBeInTheDocument();
    expect(screen.getByRole("tablist", { name: "Run 工作台" })).toBeInTheDocument();
  });

  it("工作台子导航切到 Findings → 事实/推断分区呈现", async () => {
    window.location.hash = "/runs/run-1/overview";
    renderApp();
    const subnav = screen.getByRole("tablist", { name: "Run 工作台" });
    await userEvent.click(within(subnav).getByText("Findings"));
    expect(await screen.findByTestId("facts")).toBeInTheDocument();
    expect(screen.getByTestId("inferences")).toBeInTheDocument();
  });

  it("Inbox 待办角标显示，进入显示审批中心", async () => {
    renderApp();
    const nav = screen.getByRole("navigation", { name: "主导航" });
    await userEvent.click(within(nav).getByText("Inbox"));
    expect(await screen.findByText("Inbox 收件箱")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Approve Once" })).toBeInTheDocument();
  });

  it("未实现页走只读空态（不假数据冒充完成）", async () => {
    window.location.hash = "/resources";
    renderApp();
    expect(await screen.findByText("此页尚未接线")).toBeInTheDocument();
  });

  it("Coverage 单元格下钻推送到右栏 Inspector", async () => {
    window.location.hash = "/runs/run-1/coverage";
    renderApp();
    await userEvent.click(await screen.findByRole("button", { name: "/api/orders IDOR TESTED" }));
    // 右栏 Inspector 打开并显示对象。
    expect(await screen.findByRole("complementary", { name: "Inspector" })).toBeInTheDocument();
    expect(screen.getByTestId("inspector")).toBeInTheDocument();
  });
});
