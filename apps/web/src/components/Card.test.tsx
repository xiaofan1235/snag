import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Card } from "./Card";

describe("Card", () => {
  it("静态卡片无 role=button", () => {
    render(<Card>内容</Card>);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.getByText("内容")).toBeInTheDocument();
  });

  it("clickable 卡片可聚焦可点（role=button）", async () => {
    const fn = vi.fn();
    render(
      <Card clickable onClick={fn}>
        可点
      </Card>
    );
    const card = screen.getByRole("button", { name: "可点" });
    expect(card).toHaveAttribute("tabindex", "0");
    await userEvent.click(card);
    expect(fn).toHaveBeenCalled();
  });
});
