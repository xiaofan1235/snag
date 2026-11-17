import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Input, Textarea } from "./Input";

describe("Input", () => {
  it("label 关联 control（for=id）", () => {
    render(<Input label="目标" />);
    expect(screen.getByLabelText("目标")).toBeInTheDocument();
  });

  it("error 态设 aria-invalid 且 role=alert 关联 describedby", () => {
    render(<Input label="URL" error="不合法" />);
    const input = screen.getByLabelText("URL");
    expect(input).toHaveAttribute("aria-invalid", "true");
    const err = screen.getByRole("alert");
    expect(err).toHaveTextContent("不合法");
    expect(input).toHaveAttribute("aria-describedby", err.id);
  });

  it("hint 在无 error 时作为 describedby", () => {
    render(<Input label="URL" hint="仅授权目标" />);
    const input = screen.getByLabelText("URL");
    expect(input.getAttribute("aria-describedby")).toBeTruthy();
    expect(screen.getByText("仅授权目标")).toBeInTheDocument();
  });

  it("Textarea 渲染并可带 error", () => {
    render(<Textarea label="备注" error="太长" />);
    expect(screen.getByLabelText("备注").tagName).toBe("TEXTAREA");
    expect(screen.getByRole("alert")).toHaveTextContent("太长");
  });
});
