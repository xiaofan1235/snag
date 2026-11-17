import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Button } from "./Button";

describe("Button", () => {
  it("渲染变体与尺寸的 data 属性", () => {
    render(
      <Button variant="danger" size="lg">
        删除
      </Button>
    );
    const btn = screen.getByRole("button", { name: "删除" });
    expect(btn).toHaveAttribute("data-variant", "danger");
    expect(btn).toHaveAttribute("data-size", "lg");
  });

  it("点击触发 onClick", async () => {
    const fn = vi.fn();
    render(<Button onClick={fn}>ok</Button>);
    await userEvent.click(screen.getByRole("button"));
    expect(fn).toHaveBeenCalledOnce();
  });

  it("disabled 时 aria-disabled 且不触发 onClick（仍可聚焦读提示）", async () => {
    const fn = vi.fn();
    render(
      <Button disabled onClick={fn}>
        禁用
      </Button>
    );
    const btn = screen.getByRole("button");
    expect(btn).toHaveAttribute("aria-disabled", "true");
    await userEvent.click(btn);
    expect(fn).not.toHaveBeenCalled();
  });

  it("loading 时 aria-busy 且吞掉点击、显示 spinner", async () => {
    const fn = vi.fn();
    render(
      <Button loading onClick={fn}>
        提交
      </Button>
    );
    const btn = screen.getByRole("button");
    expect(btn).toHaveAttribute("aria-busy", "true");
    await userEvent.click(btn);
    expect(fn).not.toHaveBeenCalled();
  });

  it("iconOnly 不渲染文字（靠 aria-label）", () => {
    render(<Button iconOnly aria-label="关闭" leadingIcon={<span>×</span>} />);
    expect(screen.getByRole("button", { name: "关闭" })).toBeInTheDocument();
  });
});
