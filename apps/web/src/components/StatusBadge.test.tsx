import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StatusBadge } from "./StatusBadge";
import { ALL_STATUSES, statusMeta } from "./status";

describe("StatusBadge", () => {
  it("覆盖所有已登记状态枚举，渲染文字（文字+颜色+图标三者）", () => {
    for (const s of ALL_STATUSES) {
      const { unmount } = render(<StatusBadge status={s} />);
      const meta = statusMeta(s);
      expect(screen.getByText(meta.label)).toBeInTheDocument();
      unmount();
    }
  });

  it("RUNNING 轻脉动（data-pulse），tone=accent", () => {
    render(<StatusBadge status="RUNNING" />);
    const b = screen.getByRole("status");
    expect(b).toHaveAttribute("data-pulse", "true");
    expect(b).toHaveAttribute("data-tone", "accent");
  });

  it("不确定态斜纹（data-hatched）", () => {
    render(<StatusBadge status="OUTCOME_UNKNOWN" />);
    expect(screen.getByRole("status")).toHaveAttribute("data-hatched", "true");
  });

  it("未登记状态兜底 unknown 且显示原文", () => {
    render(<StatusBadge status="WEIRD_STATE" />);
    const b = screen.getByRole("status");
    expect(b).toHaveAttribute("data-tone", "unknown");
    expect(b).toHaveTextContent("WEIRD_STATE");
  });
});
