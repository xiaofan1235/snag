import { describe, it, expect, vi } from "vitest";
import { ApiClient, ApiError } from "./client";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
}

describe("ApiClient", () => {
  it("GET 带 same-origin 凭据，解析 JSON", async () => {
    const f = vi.fn().mockResolvedValue(jsonResponse({ items: [] }));
    const c = new ApiClient({ fetchImpl: f as unknown as typeof fetch });
    const res = await c.listRuns();
    expect(res).toEqual({ items: [] });
    expect(f).toHaveBeenCalledWith(expect.stringContaining("/v1/runs"), expect.objectContaining({ credentials: "same-origin" }));
  });

  it("错误响应抛 ApiError，携带 code/message（不拼原始响应）", async () => {
    const f = vi.fn().mockResolvedValue(jsonResponse({ code: "not_found", message: "run 不存在" }, 404));
    const c = new ApiClient({ fetchImpl: f as unknown as typeof fetch });
    await expect(c.getRun("x")).rejects.toMatchObject({ name: "ApiError", status: 404, code: "not_found" });
  });

  it("非 JSON 错误体不回显原文（防泄漏）", async () => {
    const f = vi.fn().mockResolvedValue(new Response("<html>secret</html>", { status: 500 }));
    const c = new ApiClient({ fetchImpl: f as unknown as typeof fetch });
    const err = (await c.getRun("x").catch((e) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).not.toContain("secret");
  });

  it("POST 创建 Run 带 JSON body", async () => {
    const f = vi.fn().mockResolvedValue(jsonResponse({ id: "run-1", name: "t", workspace_id: "ws", revision: 1, state: "DRAFT" }));
    const c = new ApiClient({ fetchImpl: f as unknown as typeof fetch });
    await c.createRun({ name: "t", workspace_id: "ws" });
    expect(f).toHaveBeenCalledWith(
      expect.stringContaining("/v1/runs"),
      expect.objectContaining({ method: "POST", body: JSON.stringify({ name: "t", workspace_id: "ws" }) })
    );
  });

  it("eventStreamUrl 用 cursor 查询参数（不放敏感 token）", () => {
    const c = new ApiClient({ baseUrl: "" });
    expect(c.eventStreamUrl("run-1", 42)).toBe("/v1/runs/run-1/events?cursor=42");
  });
});
