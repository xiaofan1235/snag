import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Progress, FractionProgress } from "./Progress";

describe("Progress", () => {
  it("determinate 设 aria-valuenow/min/max", () => {
    render(<Progress value={42} aria-label="up" />);
    const p = screen.getByRole("progressbar", { name: "up" });
    expect(p).toHaveAttribute("aria-valuenow", "42");
    expect(p).toHaveAttribute("aria-valuemin", "0");
    expect(p).toHaveAttribute("aria-valuemax", "100");
  });

  it("indeterminate 省略 valuenow", () => {
    render(<Progress indeterminate aria-label="busy" />);
    const p = screen.getByRole("progressbar", { name: "busy" });
    expect(p).not.toHaveAttribute("aria-valuenow");
  });

  it("FractionProgress 显示真实 DONE/TOTAL 分数（不伪造百分比）", () => {
    render(<FractionProgress done={7} total={12} />);
    expect(screen.getByText("7/12")).toBeInTheDocument();
  });
});
