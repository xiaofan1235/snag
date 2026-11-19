import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KnowledgePanel, SkillsPanel, ToolsPanel } from "./ResourcesPage";
import { OperationsPage } from "./OperationsPage";
import { SettingsPage } from "./SettingsPage";
import { InboxPage } from "./InboxPage";
import { mkResources, mkOperations, mkSettings, mkInbox } from "../../app/mockGlobal";

describe("资源三分面板（§16.20，知识库 / 技能 / 工具）", () => {
  it("知识库：列表 + 默认渲染首个文档（Markdown）", () => {
    render(<KnowledgePanel items={mkResources.knowledge} />);
    expect(screen.getByText("VERIFIED_FACT")).toBeInTheDocument(); // 右侧元信息
    // 默认选中首个文档，其 Markdown 标题（h1）被渲染。
    expect(screen.getByRole("heading", { level: 1, name: /对象归属缺陷/ })).toBeInTheDocument();
  });
  it("知识库：切换文档更新阅读器", async () => {
    render(<KnowledgePanel items={mkResources.knowledge} />);
    await userEvent.click(screen.getByText("Express 常见 IDOR 模式"));
    expect(screen.getByRole("heading", { level: 1, name: /Express 常见 IDOR 模式/ })).toBeInTheDocument();
  });
  it("技能：列出技能", () => {
    render(<SkillsPanel items={mkResources.skills} />);
    expect(screen.getByText("idor-probe")).toBeInTheDocument();
  });
  it("工具（原运行模板）：列出模板", () => {
    render(<ToolsPanel items={mkResources.templates} />);
    expect(screen.getByText(mkResources.templates[0].name)).toBeInTheDocument();
  });
});

describe("OperationsPage（§16.24–16.27/16.29）", () => {
  it("Providers 连接检查回调 + 各子 Tab", async () => {
    const onCheck = vi.fn();
    render(<OperationsPage model={mkOperations} onCheck={onCheck} />);
    await userEvent.click(screen.getByRole("button", { name: "连接检查" }));
    expect(onCheck).toHaveBeenCalledWith("p1");
    await userEvent.click(screen.getByRole("tab", { name: /代理/ }));
    expect(screen.getByText(/临时直连/)).toBeInTheDocument(); // 安全说明
  });
});

describe("SettingsPage（§16.29）", () => {
  it("Preflight 分组 + FAIL/WARN 状态", () => {
    render(<SettingsPage model={mkSettings} />);
    expect(screen.getByText("授权")).toBeInTheDocument();
    expect(screen.getAllByText("PASS").length).toBeGreaterThan(0);
    expect(screen.getByText("WARN")).toBeInTheDocument();
  });
});

describe("InboxPage（§16.23）", () => {
  it("分类通知 + 审批 Tab + 阻塞恢复向导", async () => {
    const onRecover = vi.fn();
    render(<InboxPage model={mkInbox} onRecover={onRecover} />);
    expect(screen.getByText(/DELETE \/api\/orders\/42 待审批/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: /阻塞恢复/ }));
    expect(screen.getByText("代理 Lease 过期")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "预检" }));
    expect(onRecover).toHaveBeenCalledWith("b1", "预检");
  });
});
