import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Modal } from "./Modal";
import { Drawer } from "./Drawer";

describe("Modal", () => {
  it("open=false 不渲染", () => {
    render(<Modal open={false} onClose={() => {}} title="t" />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("open 时 role=dialog + aria-modal，标题关联", () => {
    render(
      <Modal open onClose={() => {}} title="停止 Run">
        内容
      </Modal>
    );
    const dlg = screen.getByRole("dialog");
    expect(dlg).toHaveAttribute("aria-modal", "true");
    expect(dlg).toHaveAttribute("aria-labelledby");
    expect(screen.getByText("停止 Run")).toBeInTheDocument();
  });

  it("Esc 触发 onClose", async () => {
    const fn = vi.fn();
    render(<Modal open onClose={fn} title="t" />);
    await userEvent.keyboard("{Escape}");
    expect(fn).toHaveBeenCalled();
  });

  it("danger 确认按钮触发 onConfirm", async () => {
    const fn = vi.fn();
    render(<Modal open onClose={() => {}} title="t" danger confirmText="停止" onConfirm={fn} />);
    await userEvent.click(screen.getByRole("button", { name: "停止" }));
    expect(fn).toHaveBeenCalled();
  });
});

describe("Drawer", () => {
  it("open 时 role=dialog，关闭按钮触发 onClose", async () => {
    const fn = vi.fn();
    render(
      <Drawer open onClose={fn} title="详情">
        body
      </Drawer>
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(fn).toHaveBeenCalled();
  });
});
