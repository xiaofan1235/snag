import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Markdown } from "./Markdown";

describe("Markdown（轻量安全渲染）", () => {
  it("标题 / 粗体 / 行内代码", () => {
    render(<Markdown source={"# 标题\n\n正文 **粗** 与 `code`。"} />);
    expect(screen.getByRole("heading", { level: 1, name: "标题" })).toBeInTheDocument();
    expect(screen.getByText("粗").tagName).toBe("STRONG");
    expect(screen.getByText("code").tagName).toBe("CODE");
  });

  it("无序 / 有序列表与围栏代码块", () => {
    const { container } = render(<Markdown source={"- a\n- b\n\n1. x\n2. y\n\n```\nraw code\n```"} />);
    expect(container.querySelectorAll("ul li").length).toBe(2);
    expect(container.querySelectorAll("ol li").length).toBe(2);
    expect(container.querySelector("pre code")?.textContent).toBe("raw code");
  });

  it("安全链接：放行 http（带 noopener），拒绝 javascript: 退化为纯文本", () => {
    render(<Markdown source={"[good](https://example.com) [bad](javascript:alert(1))"} />);
    const good = screen.getByRole("link", { name: "good" });
    expect(good).toHaveAttribute("href", "https://example.com");
    expect(good.getAttribute("rel")).toContain("noopener");
    expect(screen.queryByRole("link", { name: "bad" })).not.toBeInTheDocument();
    expect(screen.getByText("bad")).toBeInTheDocument();
  });
});
