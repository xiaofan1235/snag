import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ResourcesPage } from "./ResourcesPage";
import { OperationsPage } from "./OperationsPage";
import { SettingsPage } from "./SettingsPage";
import { InboxPage } from "./InboxPage";
import { mkResources, mkOperations, mkSettings, mkInbox } from "../../app/mockGlobal";

describe("ResourcesPage（§16.20）", () => {
  it("Knowledge 可信级别 + Skills/Templates Tab", async () => {
    render(<ResourcesPage model={mkResources} />);
    expect(screen.getByText("VERIFIED_FACT")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: /技能/ }));
    expect(screen.getByText("idor-probe")).toBeInTheDocument();
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
    expect(screen.getByText("Authorization")).toBeInTheDocument();
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
