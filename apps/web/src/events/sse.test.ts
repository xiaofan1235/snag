import { describe, it, expect } from "vitest";
import { createSSEParser } from "./sse";

describe("createSSEParser（SSE 线格式）", () => {
  it("解析单帧 event+data", () => {
    const p = createSSEParser();
    const frames = p.feed("event: job.updated\ndata: {\"sequence\":5}\n\n");
    expect(frames).toHaveLength(1);
    expect(frames[0].event).toBe("job.updated");
    expect(frames[0].data).toBe('{"sequence":5}');
  });

  it("跨 chunk 增量解析（半帧留在 buffer）", () => {
    const p = createSSEParser();
    expect(p.feed("data: hel")).toHaveLength(0);
    const frames = p.feed("lo\n\n");
    expect(frames).toHaveLength(1);
    expect(frames[0].data).toBe("hello");
  });

  it("多行 data 以 \\n 拼接", () => {
    const p = createSSEParser();
    const frames = p.feed("data: a\ndata: b\n\n");
    expect(frames[0].data).toBe("a\nb");
  });

  it("忽略注释行（: 开头）与 CRLF", () => {
    const p = createSSEParser();
    const frames = p.feed(": keep-alive\r\ndata: x\r\n\r\n");
    expect(frames).toHaveLength(1);
    expect(frames[0].data).toBe("x");
  });

  it("一次 feed 多帧", () => {
    const p = createSSEParser();
    const frames = p.feed("data: 1\n\ndata: 2\n\n");
    expect(frames.map((f) => f.data)).toEqual(["1", "2"]);
  });
});
