import { describe, it, expect, vi } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ToastProvider, useToast } from "./Toast";

function Trigger() {
  const t = useToast();
  return (
    <button onClick={() => t.push({ tone: "ok", message: "完成", duration: 0 })}>push</button>
  );
}

describe("Toast", () => {
  it("push 显示通知，手动关闭移除", async () => {
    render(
      <ToastProvider>
        <Trigger />
      </ToastProvider>
    );
    await userEvent.click(screen.getByRole("button", { name: "push" }));
    expect(screen.getByText("完成")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "关闭通知" }));
    expect(screen.queryByText("完成")).not.toBeInTheDocument();
  });

  it("通知区 role=region + aria-live=polite", () => {
    render(
      <ToastProvider>
        <span />
      </ToastProvider>
    );
    const region = screen.getByRole("region", { name: "通知" });
    expect(region).toHaveAttribute("aria-live", "polite");
  });

  it("useToast 在 Provider 外抛错", () => {
    // 预期抛错——静默 React 的渲染错误日志，保持测试输出干净。
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() =>
      act(() => {
        render(<Trigger />);
      })
    ).toThrow();
    spy.mockRestore();
  });
});
