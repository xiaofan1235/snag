import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EvidenceViewer } from "./EvidenceViewer";
import { DiffViewer } from "./DiffViewer";
import { HTTPViewer } from "./HTTPViewer";
import { Inspector } from "./Inspector";
import { EventCard } from "./EventCard";
import { computeLineDiff, diffStats } from "./diff";

describe("EvidenceViewer（默认脱敏 + Reveal）", () => {
  it("默认展示脱敏投影，标「脱敏投影」", () => {
    render(<EvidenceViewer evidenceId="ev-1" redacted={{ response: "token=***REDACTED***" }} />);
    expect(screen.getByText("脱敏投影")).toBeInTheDocument();
    expect(screen.getByTestId("evidence-body")).toHaveTextContent("REDACTED");
  });

  it("Reveal 需二次确认，确认后展示原始", async () => {
    const onReveal = vi.fn().mockResolvedValue("token=SECRET123");
    render(<EvidenceViewer evidenceId="ev-1" redacted={{ response: "token=***" }} onReveal={onReveal} />);
    await userEvent.click(screen.getByRole("button", { name: "Reveal 原始" }));
    // 确认 Modal 出现。
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "确认 Reveal" }));
    await waitFor(() => expect(screen.getByTestId("evidence-body")).toHaveTextContent("SECRET123"));
    expect(onReveal).toHaveBeenCalledWith("ev-1", "response");
    expect(screen.getByText("原始（已 Reveal）")).toBeInTheDocument();
  });

  it("无 onReveal 时不显示 Reveal 按钮（列表 API 不顺带返回原秘密）", () => {
    render(<EvidenceViewer evidenceId="ev-1" redacted={{ response: "x" }} />);
    expect(screen.queryByRole("button", { name: "Reveal 原始" })).not.toBeInTheDocument();
  });
});

describe("DiffViewer / diff", () => {
  it("computeLineDiff 纯函数：增删等识别", () => {
    const rows = computeLineDiff("a\nb\nc", "a\nx\nc");
    const s = diffStats(rows);
    expect(s.equal).toBe(2); // a, c
    expect(s.added).toBe(1); // x
    expect(s.deleted).toBe(1); // b
  });

  it("sync 渲染并排差异 + 统计", () => {
    render(<DiffViewer baseline={"a\nb"} variant={"a\nc"} sync />);
    expect(screen.getByText("+1")).toBeInTheDocument();
    expect(screen.getByText("-1")).toBeInTheDocument();
  });
});

describe("HTTPViewer", () => {
  it("Headers tab 展示起始行与头", () => {
    render(
      <HTTPViewer
        message={{ startLine: "GET /api HTTP/1.1", headers: [["Host", "example.com"]], body: '{"a":1}' }}
      />
    );
    expect(screen.getByText("GET /api HTTP/1.1")).toBeInTheDocument();
    expect(screen.getByText("Host")).toBeInTheDocument();
  });

  it("Body tab 美化 JSON", async () => {
    render(<HTTPViewer message={{ startLine: "HTTP/1.1 200 OK", headers: [], body: '{"a":1}' }} />);
    await userEvent.click(screen.getByRole("tab", { name: "Body" }));
    expect(screen.getByTestId("http-body").textContent).toContain('"a": 1');
  });
});

describe("Inspector", () => {
  it("空态提示", () => {
    render(<Inspector model={null} />);
    expect(screen.getByText("选择一个对象查看详情")).toBeInTheDocument();
  });

  it("渲染字段 + 关联对象双向跳转", async () => {
    const onNav = vi.fn();
    render(
      <Inspector
        model={{
          ref: { type: "finding", id: "f-1" },
          title: "订单越权",
          fields: [{ label: "类型", value: "IDOR" }],
          related: [{ label: "证据 ev-1", ref: { type: "evidence", id: "ev-1" } }],
        }}
        onNavigate={onNav}
      />
    );
    expect(screen.getByText("订单越权")).toBeInTheDocument();
    expect(screen.getByText("IDOR")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "证据 ev-1" }));
    expect(onNav).toHaveBeenCalledWith({ type: "evidence", id: "ev-1" });
  });
});

describe("EventCard", () => {
  it("渲染类型标签与跳转", async () => {
    const onJump = vi.fn();
    render(
      <EventCard kind="action" title="GET /api/orders" time="12:00" status="RUNNING" onJump={onJump}>
        动作体
      </EventCard>
    );
    expect(screen.getByText("动作")).toBeInTheDocument();
    expect(screen.getByText("运行中")).toBeInTheDocument(); // StatusBadge RUNNING
    await userEvent.click(screen.getByRole("button", { name: "查看详情" }));
    expect(onJump).toHaveBeenCalled();
  });
});
