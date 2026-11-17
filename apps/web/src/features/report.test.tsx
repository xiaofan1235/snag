import { describe, it, expect, vi } from "vitest";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ReportCenter, type ReportModel } from "./report/ReportCenter";
import { RecoveryPreview, type RecoveryPreviewModel } from "./runs/RecoveryPreview";
import { CommandButton } from "./commands/CommandButton";
import { ConnectionStatus } from "../components/ConnectionStatus";

const baseReport: ReportModel = {
  id: "rep-1",
  state: "FINALIZED",
  reportable: [{ id: "f-1", title: "IDOR on /orders", severity: "High" }],
  excluded: [{ id: "f-2", title: "可能的 XSS", state: "UNABLE_TO_VERIFY", reason: "零交互要求未满足" }],
  exitGateResult: "PASSED",
  coverageSnapshot: "rev 7",
  partialReasons: [],
  markdownPreview: "# 渗透测试报告\n\n## 1. IDOR",
};

describe("ReportCenter（§16.22）", () => {
  it("纳入 Reportable；被排除项单独列出及原因（不静默消失）", () => {
    render(<ReportCenter model={baseReport} />);
    expect(within(screen.getByTestId("report-included")).getByText(/IDOR on \/orders/)).toBeInTheDocument();
    const excluded = screen.getByTestId("report-excluded");
    expect(within(excluded).getByText(/可能的 XSS/)).toBeInTheDocument();
    expect(within(excluded).getByText(/零交互要求未满足/)).toBeInTheDocument();
  });

  it("FINALIZED 可导出 Markdown/JSON（本地下载）", async () => {
    const onExport = vi.fn();
    render(<ReportCenter model={baseReport} onExport={onExport} />);
    await userEvent.click(screen.getByRole("button", { name: "导出 Markdown" }));
    expect(onExport).toHaveBeenCalledWith("rep-1", "markdown");
  });

  it("STALE 报告禁止导出（依赖失效不得分发）", () => {
    render(<ReportCenter model={{ ...baseReport, state: "STALE" }} onExport={() => {}} />);
    expect(screen.getByRole("button", { name: "导出 Markdown" })).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByText("依赖失效，不得分发")).toBeInTheDocument();
  });
});

describe("RecoveryPreview（§16.12）", () => {
  const model: RecoveryPreviewModel = {
    unfinishedJobs: 2,
    outcomeUnknownActions: 1,
    expiredIdentities: ["账号A"],
    staleProxies: [],
    policyRevisionChanged: true,
    schemaCompatible: true,
  };

  it("展示恢复前各项风险", () => {
    render(<RecoveryPreview model={model} />);
    const panel = screen.getByTestId("recovery-preview");
    expect(within(panel).getByText("未完成 Job")).toBeInTheDocument();
    expect(within(panel).getByText("账号A")).toBeInTheDocument();
  });

  it("确认恢复走命令回调", async () => {
    const onResume = vi.fn();
    render(<RecoveryPreview model={model} onResume={onResume} />);
    await userEvent.click(screen.getByRole("button", { name: "确认恢复" }));
    expect(onResume).toHaveBeenCalled();
  });

  it("Schema 不兼容阻断恢复", () => {
    render(<RecoveryPreview model={{ ...model, schemaCompatible: false }} />);
    expect(screen.getByRole("button", { name: "确认恢复" })).toHaveAttribute("aria-disabled", "true");
  });
});

describe("CommandButton（幂等 + 终态反映）", () => {
  it("点击执行命令，终态回调", async () => {
    const onSettled = vi.fn();
    render(
      <CommandButton
        label="暂停"
        deps={{
          submit: async () => ({ operationId: "op-1", terminal: true, state: "APPLIED", resultRef: "run-1" }),
          poll: async () => ({ state: "APPLIED", terminal: true }),
          sleep: () => Promise.resolve(),
        }}
        onSettled={onSettled}
      />
    );
    await userEvent.click(screen.getByRole("button", { name: "暂停" }));
    await waitFor(() => expect(onSettled).toHaveBeenCalledWith("DONE", "run-1"));
  });

  it("破坏性命令需二次确认", async () => {
    const onSettled = vi.fn();
    render(
      <CommandButton
        label="停止"
        variant="danger"
        confirm={{ title: "停止 Run？", body: "不可逆", danger: true }}
        deps={{
          submit: async () => ({ operationId: "op-1", terminal: true, state: "APPLIED" }),
          poll: async () => ({ state: "APPLIED", terminal: true }),
          sleep: () => Promise.resolve(),
        }}
        onSettled={onSettled}
      />
    );
    await userEvent.click(screen.getByRole("button", { name: "停止" }));
    // 确认 Modal 出现，命令尚未执行。
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(onSettled).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "确认" }));
    await waitFor(() => expect(onSettled).toHaveBeenCalledWith("DONE", null));
  });
});

describe("ConnectionStatus（§16.33）", () => {
  it("STALE 文案说明 Run 仍在后端运行（UI 断线≠Run 停止）", () => {
    render(<ConnectionStatus state="STALE" />);
    expect(screen.getByText(/Run 仍在后端运行/)).toBeInTheDocument();
  });

  it("LIVE 显示实时", () => {
    render(<ConnectionStatus state="LIVE" />);
    expect(screen.getByText("实时")).toBeInTheDocument();
  });
});
