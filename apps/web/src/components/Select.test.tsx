import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Select } from "./Select";
import type { SelectOption } from "./Select";

const opts: SelectOption[] = [
  { value: "a", label: "Alpha" },
  { value: "b", label: "Bravo" },
  { value: "c", label: "Charlie" },
];

describe("Select", () => {
  it("trigger 带 aria-expanded/haspopup，默认关闭", () => {
    render(<Select options={opts} aria-label="x" />);
    const trg = screen.getByRole("button", { name: "x" });
    expect(trg).toHaveAttribute("aria-expanded", "false");
    expect(trg).toHaveAttribute("aria-haspopup", "listbox");
  });

  it("点击打开，选项有 role=option，选择触发 onChange 并回填", async () => {
    const fn = vi.fn();
    render(<Select options={opts} aria-label="x" onChange={fn} />);
    await userEvent.click(screen.getByRole("button", { name: "x" }));
    expect(screen.getByRole("listbox")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("option", { name: "Bravo" }));
    expect(fn).toHaveBeenCalledWith("b");
  });

  it("键盘：ArrowDown 打开 + 移动，Enter 选择", async () => {
    const fn = vi.fn();
    render(<Select options={opts} aria-label="x" onChange={fn} />);
    const trg = screen.getByRole("button", { name: "x" });
    trg.focus();
    await userEvent.keyboard("{ArrowDown}"); // 打开
    await userEvent.keyboard("{ArrowDown}"); // active -> index1
    await userEvent.keyboard("{Enter}");
    expect(fn).toHaveBeenCalledWith("b");
  });

  it("Esc 关闭面板", async () => {
    render(<Select options={opts} aria-label="x" />);
    const trg = screen.getByRole("button", { name: "x" });
    await userEvent.click(trg);
    expect(screen.getByRole("listbox")).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("selected 项标 aria-selected", async () => {
    render(<Select options={opts} value="c" aria-label="x" />);
    await userEvent.click(screen.getByRole("button", { name: "x" }));
    expect(screen.getByRole("option", { name: "Charlie" })).toHaveAttribute("aria-selected", "true");
  });

  it("searchable 过滤选项", async () => {
    render(<Select options={opts} searchable aria-label="x" />);
    await userEvent.click(screen.getByRole("button", { name: "x" }));
    await userEvent.type(screen.getByPlaceholderText("搜索…"), "brav");
    expect(screen.getByRole("option", { name: "Bravo" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Alpha" })).not.toBeInTheDocument();
  });

  it("disabled 不展开", async () => {
    render(<Select options={opts} disabled aria-label="x" />);
    await userEvent.click(screen.getByRole("button", { name: "x" }));
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });
});
