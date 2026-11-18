// BrowserPage —— Browser Observatory（UI 文档 §16.17）。会话列表 + 截图/DOM/Network/Console/Trace/Cookie。
// 默认只读实时画面，不开放任意 CDP；「人工接管」需新 Approval（绑定 Identity/Run/PolicyRevision/时限）。

import { useState } from "react";
import { Card } from "../../components/Card";
import { Button } from "../../components/Button";
import { StatusBadge } from "../../components/StatusBadge";
import { Tabs } from "../../components/Tabs";
import { Modal } from "../../components/Modal";

export interface BrowserSession {
  id: string;
  identity: string;
  container: string;
  state: string;
  currentPage: string;
  createdAt: string;
  lastAction: string;
  screenshots: Array<{ actionId: string; caption: string }>;
  domSnapshot: string;
  network: Array<{ method: string; url: string; status: number }>;
  console: string[];
  cookies: Array<{ name: string; scope: string; expires: string; redactedSummary: string }>;
}
export interface BrowserModel {
  sessions: BrowserSession[];
}

export function BrowserPage({ model, onTakeover }: { model: BrowserModel; onTakeover?: (id: string) => void }) {
  const [sel, setSel] = useState<BrowserSession | null>(model.sessions[0] ?? null);
  const [confirming, setConfirming] = useState(false);

  return (
    <div className="sn-three-col" style={{ gridTemplateColumns: "240px 1fr" }}>
      <div className="sn-col-scroll">
        <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-1)" }}>浏览器会话</div>
        {model.sessions.map((s) => (
          <button key={s.id} className="sn-btn sn-focus" data-variant={sel?.id === s.id ? "secondary" : "ghost"} data-size="sm"
            style={{ width: "100%", justifyContent: "space-between", marginBottom: 2 }} onClick={() => setSel(s)}>
            <span className="mono" style={{ fontSize: "var(--fs-xs)" }}>{s.identity}</span>
            <StatusBadge status={s.state} />
          </button>
        ))}
      </div>
      <div>
        {sel ? (
          <>
            <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-2)" }}>
              <span>容器 {sel.container} · 当前页 <span className="mono">{sel.currentPage}</span></span>
              <Button size="sm" variant="danger" style={{ marginLeft: "auto" }} onClick={() => setConfirming(true)}>人工接管（需审批）</Button>
            </div>
            <Tabs
              items={[
                { id: "shots", label: "截图", content: (
                  <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill,minmax(160px,1fr))", gap: "var(--sp-2)" }}>
                    {sel.screenshots.map((sh, i) => (
                      <Card key={i}>
                        <div style={{ height: 80, background: "var(--bg-sunken)", borderRadius: "var(--radius-sm)", display: "grid", placeItems: "center", color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>截图（需授权查看原图）</div>
                        <div style={{ fontSize: "var(--fs-xs)", color: "var(--ink-3)", marginTop: 4 }}>{sh.caption}（{sh.actionId}）</div>
                      </Card>
                    ))}
                  </div>
                ) },
                { id: "dom", label: "DOM 快照", content: <pre className="mono sn-terminal" style={{ background: "var(--bg-sunken)", color: "var(--ink-1)" }}>{sel.domSnapshot}</pre> },
                { id: "net", label: "网络（经代理留证）", content: (
                  <table className="sn-table"><thead><tr><th>Method</th><th>URL</th><th>Status</th></tr></thead>
                    <tbody>{sel.network.map((n, i) => (<tr key={i}><td>{n.method}</td><td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{n.url}</td><td>{n.status}</td></tr>))}</tbody>
                  </table>
                ) },
                { id: "console", label: "控制台", content: <pre className="sn-terminal">{sel.console.join("\n")}</pre> },
                { id: "cookie", label: "Cookie / 存储", content: (
                  <table className="sn-table"><thead><tr><th>名称</th><th>作用域</th><th>过期</th><th>脱敏摘要</th></tr></thead>
                    <tbody>{sel.cookies.map((c, i) => (<tr key={i}><td className="mono">{c.name}</td><td>{c.scope}</td><td>{c.expires}</td><td className="mono">{c.redactedSummary}</td></tr>))}</tbody>
                  </table>
                ) },
              ]}
            />
          </>
        ) : (
          <div className="sn-empty">无浏览器会话</div>
        )}
      </div>

      <Modal open={confirming} onClose={() => setConfirming(false)} title="请求人工接管？" danger confirmText="发起接管审批"
        onConfirm={() => { if (sel) onTakeover?.(sel.id); setConfirming(false); }}>
        人工接管需新 Approval，绑定 Identity/Run/PolicyRevision 与时限；接管期间每次导航/输入/点击/上传均形成 Action 并经同一 Scope/Policy 检查。
      </Modal>
    </div>
  );
}
