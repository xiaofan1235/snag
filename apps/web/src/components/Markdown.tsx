// Markdown —— 轻量 Markdown 渲染（手写，不引三方依赖，与本仓「手写优于大依赖」一致）。
// 直接构造 React 元素而非 dangerouslySetInnerHTML，天然防 XSS（知识库可能含未核实引用，安全优先）。
// 支持：标题 #..###### / 段落 / 粗体 ** / 斜体 * / 行内代码 ` / 围栏代码块 ``` / 无序列表 -*+ /
// 有序列表 1. / 引用 > / 分隔线 --- / 链接 [文本](url)。链接仅放行 http(s)/相对/锚点/mailto，余者退化为纯文本。

import { createElement, type ReactNode } from "react";

function safeHref(url: string): { href: string; external: boolean } | null {
  const u = url.trim();
  if (/^https?:\/\//i.test(u)) return { href: u, external: true };
  if (/^\/\//.test(u)) return { href: "https:" + u, external: true };
  if (/^(\/|\.\/|\.\.\/|#)/.test(u)) return { href: u, external: false };
  if (/^mailto:/i.test(u)) return { href: u, external: true };
  return null; // javascript:/data: 等一律拒绝
}

// 行内解析：代码 > 粗体 > 斜体 > 链接。返回 ReactNode[]。
function parseInline(text: string, kp: string): ReactNode[] {
  const out: ReactNode[] = [];
  const re = /(`([^`]+)`)|(\*\*([^*]+)\*\*)|(\*([^*]+)\*)|(\[([^\]]+)\]\(([^)\s]+)\))/;
  let rest = text;
  let i = 0;
  while (rest.length) {
    const m = re.exec(rest);
    if (!m) { out.push(rest); break; }
    if (m.index > 0) out.push(rest.slice(0, m.index));
    const k = `${kp}-${i++}`;
    if (m[1]) {
      out.push(<code key={k}>{m[2]}</code>);
    } else if (m[3]) {
      out.push(<strong key={k}>{parseInline(m[4], k)}</strong>);
    } else if (m[5]) {
      out.push(<em key={k}>{parseInline(m[6], k)}</em>);
    } else {
      const safe = safeHref(m[9]);
      out.push(
        safe ? (
          <a key={k} href={safe.href} {...(safe.external ? { target: "_blank", rel: "noopener noreferrer nofollow" } : {})}>
            {m[8]}
          </a>
        ) : (
          <span key={k}>{m[8]}</span>
        )
      );
    }
    rest = rest.slice(m.index + m[0].length);
  }
  return out;
}

const BLOCK_START = /^(```|#{1,6}\s|>\s?|\s*[-*+]\s+|\s*\d+\.\s+|---+\s*$|\*\*\*+\s*$|___+\s*$)/;

export interface MarkdownProps {
  source: string;
  className?: string;
}

export function Markdown({ source, className }: MarkdownProps) {
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const blocks: ReactNode[] = [];
  let i = 0;
  let key = 0;
  const K = () => `b${key++}`;

  while (i < lines.length) {
    const line = lines[i];

    // 围栏代码块
    if (/^```/.test(line)) {
      const buf: string[] = [];
      i++;
      while (i < lines.length && !/^```\s*$/.test(lines[i])) { buf.push(lines[i]); i++; }
      i++; // 跳过闭合围栏
      blocks.push(<pre key={K()} className="sn-md-pre"><code>{buf.join("\n")}</code></pre>);
      continue;
    }
    // 空行
    if (/^\s*$/.test(line)) { i++; continue; }
    // 分隔线
    if (/^(---+|\*\*\*+|___+)\s*$/.test(line)) { blocks.push(<hr key={K()} className="sn-md-hr" />); i++; continue; }
    // 标题
    const h = line.match(/^(#{1,6})\s+(.*)$/);
    if (h) {
      blocks.push(createElement(`h${h[1].length}`, { key: K() }, parseInline(h[2], K())));
      i++;
      continue;
    }
    // 引用
    if (/^>\s?/.test(line)) {
      const buf: string[] = [];
      while (i < lines.length && /^>\s?/.test(lines[i])) { buf.push(lines[i].replace(/^>\s?/, "")); i++; }
      blocks.push(<blockquote key={K()}>{parseInline(buf.join(" "), K())}</blockquote>);
      continue;
    }
    // 无序列表
    if (/^\s*[-*+]\s+/.test(line)) {
      const items: ReactNode[] = [];
      while (i < lines.length && /^\s*[-*+]\s+/.test(lines[i])) {
        items.push(<li key={K()}>{parseInline(lines[i].replace(/^\s*[-*+]\s+/, ""), K())}</li>);
        i++;
      }
      blocks.push(<ul key={K()} className="sn-md-ul">{items}</ul>);
      continue;
    }
    // 有序列表
    if (/^\s*\d+\.\s+/.test(line)) {
      const items: ReactNode[] = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        items.push(<li key={K()}>{parseInline(lines[i].replace(/^\s*\d+\.\s+/, ""), K())}</li>);
        i++;
      }
      blocks.push(<ol key={K()} className="sn-md-ol">{items}</ol>);
      continue;
    }
    // 段落：合并连续、非块起始的行
    const buf: string[] = [];
    while (i < lines.length && !/^\s*$/.test(lines[i]) && !BLOCK_START.test(lines[i])) {
      buf.push(lines[i]); i++;
    }
    blocks.push(<p key={K()}>{parseInline(buf.join(" "), K())}</p>);
  }

  return <div className={className ? `sn-md ${className}` : "sn-md"}>{blocks}</div>;
}
