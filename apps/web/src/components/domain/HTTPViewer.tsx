// HTTPViewer —— HTTP 交换查看（UI 文档 D2.13）。Tab：Headers / Body / Raw；JSON 体美化；行内搜索。
// 内容视为不可信（§16.33）：纯文本渲染，不执行/不外链。

import { useMemo, useState } from "react";
import { Input } from "../Input";

export interface HttpMessage {
  startLine: string; // 如 "GET /api HTTP/1.1" 或 "HTTP/1.1 200 OK"
  headers: Array<[string, string]>;
  body?: string;
}

export interface HTTPViewerProps {
  message: HttpMessage;
}

type HttpTab = "headers" | "body" | "raw";

function tryPrettyJSON(s: string): string {
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

function rawOf(m: HttpMessage): string {
  const head = [m.startLine, ...m.headers.map(([k, v]) => `${k}: ${v}`)].join("\n");
  return m.body != null ? `${head}\n\n${m.body}` : head;
}

export function HTTPViewer({ message }: HTTPViewerProps) {
  const [tab, setTab] = useState<HttpTab>("headers");
  const [q, setQ] = useState("");

  const bodyText = useMemo(() => (message.body ? tryPrettyJSON(message.body) : ""), [message.body]);
  const rawText = useMemo(() => rawOf(message), [message]);

  const filterLines = (text: string) =>
    q ? text.split("\n").filter((l) => l.toLowerCase().includes(q.toLowerCase())).join("\n") : text;

  return (
    <div>
      <div style={{ display: "flex", gap: "var(--sp-1)", marginBottom: "var(--sp-2)" }}>
        {(["headers", "body", "raw"] as HttpTab[]).map((t) => (
          <button
            key={t}
            role="tab"
            aria-selected={t === tab}
            className="sn-btn sn-focus"
            data-variant={t === tab ? "secondary" : "ghost"}
            data-size="sm"
            onClick={() => setTab(t)}
          >
            {t === "headers" ? "Headers" : t === "body" ? "Body" : "Raw"}
          </button>
        ))}
        <div style={{ marginLeft: "auto", minWidth: 160 }}>
          <Input placeholder="搜索…" value={q} onChange={(e) => setQ(e.target.value)} aria-label="搜索交换内容" />
        </div>
      </div>

      {tab === "headers" ? (
        <table className="sn-table">
          <tbody>
            <tr>
              <td className="mono" colSpan={2} style={{ color: "var(--ink-2)" }}>
                {message.startLine}
              </td>
            </tr>
            {message.headers
              .filter(([k, v]) => !q || `${k}: ${v}`.toLowerCase().includes(q.toLowerCase()))
              .map(([k, v], i) => (
                <tr key={i}>
                  <td className="mono" style={{ color: "var(--ink-2)", width: "30%" }}>
                    {k}
                  </td>
                  <td className="mono" style={{ wordBreak: "break-all" }}>
                    {v}
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      ) : (
        <pre
          className="mono"
          data-testid="http-body"
          style={{
            margin: 0,
            padding: "var(--sp-3)",
            background: "var(--bg-sunken)",
            borderRadius: "var(--radius-sm)",
            fontSize: "var(--fs-xs)",
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
            maxHeight: 320,
            overflow: "auto",
          }}
        >
          {tab === "body" ? filterLines(bodyText) : filterLines(rawText)}
        </pre>
      )}
    </div>
  );
}
