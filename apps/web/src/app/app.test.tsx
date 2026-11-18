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

  it("值班台顶部渲染作战总览（跨 Run 指标 + Token 消耗图）", () => {
    renderApp();
    const overview = screen.getByRole("region", { name: "作战总览" });
    expect(within(overview).getByText("活跃 Run")).toBeInTheDocument();
    expect(within(overview).getByRole("img", { name: "Token 消耗柱状图" })).toBeInTheDocument();
  });

  it("点击左导航 Runs → 进入 Runs 列表", async () => {
    renderApp();
    const nav = screen.getByRole("navigation", { name: "主导航" });
    await userEvent.click(within(nav).getByText("Runs"));
    expect(await screen.findByText("Runs 列表 / 历史")).toBeInTheDocument();
  });

  it("从 Runs 列表点一个 Run → 进入指挥台（作战模式，默认会话树）", async () => {
    window.location.hash = "/runs";
    renderApp();
    await userEvent.click(screen.getByText("acme.com 授权测试"));
    // 作战视图标题 + 主模式导航 + 指挥台默认内容（会话树）。
    expect(await screen.findByText(/Run run-1/)).toBeInTheDocument();
    expect(screen.getByRole("tablist", { name: "Run 作战模式" })).toBeInTheDocument();
    expect(screen.getByRole("tree", { name: "Agent 会话" })).toBeInTheDocument();
  });

  it("主模式切到「探索图谱 / 作战台 / 守卫」各渲染对应视图", async () => {
    window.location.hash = "/runs/run-1/command";
    renderApp();
    const modebar = screen.getByRole("tablist", { name: "Run 作战模式" });
    await userEvent.click(within(modebar).getByRole("tab", { name: /探索图谱/ }));
    expect(await screen.findByRole("img", { name: "探索知识图谱" })).toBeInTheDocument();
    await userEvent.click(within(modebar).getByRole("tab", { name: /作战台/ }));
    expect(await screen.findByText("录制流量")).toBeInTheDocument();
    await userEvent.click(within(modebar).getByRole("tab", { name: /守卫/ }));
    expect(await screen.findByRole("tab", { name: /拦截规则/ })).toBeInTheDocument();
  });

  it("细节层子导航切到 Findings → 事实/推断分区呈现", async () => {
    window.location.hash = "/runs/run-1/overview";
    renderApp();
    const subnav = screen.getByRole("tablist", { name: "Run 细节视图" });
    await userEvent.click(within(subnav).getByText("Findings"));
    expect(await screen.findByTestId("facts")).toBeInTheDocument();
    expect(screen.getByTestId("inferences")).toBeInTheDocument();
  });

  it("Inbox 待办角标显示，进入显示分类通知 + 审批 Tab", async () => {
    renderApp();
    const nav = screen.getByRole("navigation", { name: "主导航" });
    await userEvent.click(within(nav).getByText("Inbox"));
    expect(await screen.findByText("Inbox 收件箱")).toBeInTheDocument();
    // 默认「全部通知」Tab 显示分类通知。
    expect(screen.getByText(/run-1 请求预算已用 78%/)).toBeInTheDocument();
    // 切到待审批 Tab → ApprovalCenter。
    await userEvent.click(screen.getByRole("tab", { name: /待审批/ }));
    expect(screen.getByRole("button", { name: "Approve Once" })).toBeInTheDocument();
  });

  it("Resources 页渲染（Knowledge/Skills/Templates）", async () => {
    window.location.hash = "/resources";
    renderApp();
    expect(await screen.findByText(/Resources（Knowledge/)).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Knowledge" })).toBeInTheDocument();
  });

  it("Operations 页渲染（Providers/Egress/Sandbox/Diagnostics）", async () => {
    window.location.hash = "/operations";
    renderApp();
    expect(await screen.findByRole("heading", { name: "Operations" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Providers/ })).toBeInTheDocument();
  });

  it("Run 工作台所有子导航项均可进入真实页（无空态占位）", async () => {
    for (const seg of ["strategy", "jobs", "surface", "hypotheses", "traffic", "evidence", "browser", "tools", "files", "timeline"]) {
      window.location.hash = `/runs/run-1/${seg}`;
      const { unmount } = renderApp();
      // 不应出现「页面不存在」兜底。
      expect(screen.queryByText("页面不存在")).not.toBeInTheDocument();
      unmount();
    }
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
