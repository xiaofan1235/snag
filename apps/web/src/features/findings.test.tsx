import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FindingDetail, type FindingModel } from "./findings/FindingDetail";
import { ApprovalCenter, type ApprovalItem } from "./approvals/ApprovalCenter";
import { CoverageMatrix, type CoverageModel } from "./coverage/CoverageMatrix";

const finding: FindingModel = {
  id: "f-1",
  title: "IDOR on /orders",
  vulnType: "IDOR",
  severity: "High",
  state: "REPORTABLE",
  affectedEndpoints: ["/api/orders/{id}"],
  identityObjectRelation: "账号A 访问 账号B 的订单",
  facts: [
    { id: "fa1", text: "账号A 读取到账号B 订单 42 的 JSON", evidenceId: "ev-1", byteStart: 10, byteEnd: 20 },
  ],
  inferences: [{ id: "in1", text: "推断：对象归属校验缺失" }],
  baseline: "HTTP/1.1 403 Forbidden",
  variant: "HTTP/1.1 200 OK",
  verifierOpinion: "确认跨身份",
  severityRationale: "机密性高影响",
  cleanupStatus: "无需清理",
  citations: [{ evidenceId: "ev-1", content: "0123456789SECRETDATA0123456789" }],
};

describe("FindingDetail（§16.7 事实/推断分区 + 点击即证据）", () => {
  it("事实区与推断区分开渲染", () => {
    render(<FindingDetail model={finding} />);
    const facts = screen.getByTestId("facts");
    const inferences = screen.getByTestId("inferences");
    expect(within(facts).getByText(/账号A 读取到账号B 订单/)).toBeInTheDocument();
    expect(within(inferences).getByText(/对象归属校验缺失/)).toBeInTheDocument();
  });

  it("点击事实 → 右栏 CEBS 字节高亮对应区间", async () => {
    render(<FindingDetail model={finding} />);
    expect(screen.queryByTestId("cebs-highlight")).not.toBeInTheDocument();
    await userEvent.click(screen.getByText(/账号A 读取到账号B 订单/));
    const hl = screen.getByTestId("cebs-highlight");
    // byte[10:20] = "SECRETDATA"
    expect(within(hl).getByText("SECRETDATA").tagName).toBe("MARK");
  });

  it("请求重放走命令回调（不前端直接发请求）", async () => {
    const onReplay = vi.fn();
    render(<FindingDetail model={finding} onReplay={onReplay} />);
    await userEvent.click(screen.getByRole("button", { name: /请求重放/ }));
    expect(onReplay).toHaveBeenCalledWith("f-1");
  });
});

describe("ApprovalCenter（§16.10）", () => {
  const items: ApprovalItem[] = [
    {
      id: "ap-1",
      revision: 2,
      actionDigest: "digest-abc",
      normalAction: "GET /api/orders/42",
      pendingAction: "DELETE /api/orders/42",
      identity: "账号A",
      object: "order:42",
      sideEffects: ["删除订单"],
      policyReason: "写操作需审批",
      expiresAt: "12:30",
      state: "PENDING",
      highRisk: true,
    },
  ];

  it("展示动作 Diff（正常→待审批）+ 身份/对象/副作用/策略原因", () => {
    render(<ApprovalCenter items={items} />);
    expect(screen.getByText("GET /api/orders/42")).toBeInTheDocument();
    expect(screen.getByText("DELETE /api/orders/42")).toBeInTheDocument();
    expect(screen.getByText("删除订单")).toBeInTheDocument();
    expect(screen.getByText("写操作需审批")).toBeInTheDocument();
  });

  it("Approve 带 revision + actionDigest（冲突检测用）", async () => {
    const onApprove = vi.fn();
    render(<ApprovalCenter items={items} onApprove={onApprove} />);
    await userEvent.click(screen.getByRole("button", { name: "Approve Once" }));
    expect(onApprove).toHaveBeenCalledWith("ap-1", 2, "digest-abc");
  });

  it("Deny and Cancel Related 回调", async () => {
    const onDenyCancel = vi.fn();
    render(<ApprovalCenter items={items} onDenyAndCancel={onDenyCancel} />);
    await userEvent.click(screen.getByRole("button", { name: "Deny and Cancel Related" }));
    expect(onDenyCancel).toHaveBeenCalledWith("ap-1");
  });

  it("非 PENDING 审批不显示操作按钮", () => {
    render(<ApprovalCenter items={[{ ...items[0], state: "APPROVED" }]} />);
    expect(screen.queryByRole("button", { name: "Approve Once" })).not.toBeInTheDocument();
  });
});

describe("CoverageMatrix（§16.8）", () => {
  const model: CoverageModel = {
    surfaceRevision: 7,
    cells: [
      { endpoint: "/api/orders", attackType: "IDOR", identity: "账号A", state: "TESTED", actionId: "act-1", evidenceId: "ev-1" },
      { endpoint: "/api/orders", attackType: "SQLI", identity: "账号A", state: "UNTESTED", untestedReason: "无注入点" },
      { endpoint: "/api/login", attackType: "IDOR", identity: "账号A", state: "NA", untestedReason: "无对象归属" },
    ],
  };

  it("展示 SurfaceRevision 与矩阵", () => {
    render(<CoverageMatrix model={model} />);
    expect(screen.getByText("SurfaceRevision rev 7")).toBeInTheDocument();
    expect(screen.getByTestId("coverage-matrix")).toBeInTheDocument();
  });

  it("单元格点击下钻显示 Action + Evidence", async () => {
    const onDrill = vi.fn();
    render(<CoverageMatrix model={model} onDrillDown={onDrill} />);
    await userEvent.click(screen.getByRole("button", { name: "/api/orders IDOR TESTED" }));
    const dd = screen.getByTestId("cell-drilldown");
    expect(within(dd).getByText("act-1")).toBeInTheDocument();
    expect(within(dd).getByText("ev-1")).toBeInTheDocument();
    expect(onDrill).toHaveBeenCalled();
  });

  it("未测单元格显示未测原因", async () => {
    render(<CoverageMatrix model={model} />);
    await userEvent.click(screen.getByRole("button", { name: "/api/orders SQLI UNTESTED" }));
    expect(screen.getByText(/无注入点/)).toBeInTheDocument();
  });
});
