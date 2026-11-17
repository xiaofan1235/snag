// sse.ts —— fetch-based SSE 线格式解析（UI 文档 §15.6：原生 EventSource 无法设首帧头/用 fetch 流）。
// 只做「字节流 → SSE 帧」的纯解析，不涉及一致性语义（见 consistency.ts）。

export interface SSEFrame {
  event?: string; // event: 字段，缺省为 "message"
  data: string; // data: 多行拼接（\n 连接）
  id?: string; // id: 字段
}

// createSSEParser 返回一个增量解析器：feed(chunk) 返回本次解析出的完整帧。
// SSE 规范：以空行分隔事件；每行 "field: value"；data 多行累积；行以 \n 或 \r\n 结束。
export function createSSEParser() {
  let buffer = "";
  let cur: { event?: string; data: string[]; id?: string } = { data: [] };
  const out: SSEFrame[] = [];

  function flush() {
    if (cur.data.length === 0 && cur.event === undefined && cur.id === undefined) {
      cur = { data: [] };
      return;
    }
    out.push({ event: cur.event, data: cur.data.join("\n"), id: cur.id });
    cur = { data: [] };
  }

  return {
    feed(chunk: string): SSEFrame[] {
      out.length = 0;
      buffer += chunk;
      const lines = buffer.split(/\r\n|\r|\n/);
      // 最后一段可能不完整，留回 buffer。
      buffer = lines.pop() ?? "";
      for (const line of lines) {
        if (line === "") {
          flush();
          continue;
        }
        if (line.startsWith(":")) continue; // 注释行
        const idx = line.indexOf(":");
        const field = idx === -1 ? line : line.slice(0, idx);
        let value = idx === -1 ? "" : line.slice(idx + 1);
        if (value.startsWith(" ")) value = value.slice(1);
        switch (field) {
          case "event":
            cur.event = value;
            break;
          case "data":
            cur.data.push(value);
            break;
          case "id":
            cur.id = value;
            break;
          // retry 等字段本层忽略（退避由上层控制）
        }
      }
      return [...out];
    },
  };
}
