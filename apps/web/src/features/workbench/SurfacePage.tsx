// SurfacePage —— 攻击面页面（UI 文档 §16.5 + §16.15）。Endpoint 表（多维筛选）+ Discovery Inbox +
// 资产聚合（按 Application/模块）。范围外资产只记录、无测试/重放/扫描按钮（§16.15 红线）。

import { useMemo, useState } from "react";
import { Card } from "../../components/Card";
import { Select } from "../../components/Select";
import { StatusBadge } from "../../components/StatusBadge";
import { Tabs } from "../../components/Tabs";
import { Button } from "../../components/Button";

export interface Endpoint {
  id: string;
  module: string;
  method: string;
  url: string;
  authType: string;
  statusCode: number;
  source: string;
  hasParams: boolean;
  hasFinding: boolean;
  covered: boolean;
  inScope: boolean;
}
export interface Discovery {
  id: string;
  kind: string; // 新参数/异常响应/JS 引用/身份差异/链候选/被策略阻止
  detail: string;
  source: string;
  confidence: string;
  stale: boolean;
  followState: string;
}
export interface SurfaceModel {
  surfaceRevision: number;
  endpoints: Endpoint[];
  discoveries: Discovery[];
}

export interface SurfacePageProps {
  model: SurfaceModel;
  onEndpointClick?: (ep: Endpoint) => void;
  onDiscoveryDirective?: (id: string, action: string) => void;
}

export function SurfacePage({ model, onEndpointClick, onDiscoveryDirective }: SurfacePageProps) {
  const [module, setModule] = useState("");
  const [method, setMethod] = useState("");
  const modules = useMemo(() => Array.from(new Set(model.endpoints.map((e) => e.module))), [model.endpoints]);
  const methods = useMemo(() => Array.from(new Set(model.endpoints.map((e) => e.method))), [model.endpoints]);
  const filtered = model.endpoints.filter((e) => (!module || e.module === module) && (!method || e.method === method));

  const endpointsTab = (
    <>
      <div style={{ display: "flex", gap: "var(--sp-3)", marginBottom: "var(--sp-3)", alignItems: "center" }}>
        <span className="mono" style={{ color: "var(--ink-3)", fontSize: "var(--fs-sm)" }}>SurfaceRevision rev {model.surfaceRevision}</span>
        <Select options={[{ value: "", label: "全部模块" }, ...modules.map((m) => ({ value: m, label: m }))]} value={module} onChange={setModule} aria-label="按模块" />
        <Select options={[{ value: "", label: "全部方法" }, ...methods.map((m) => ({ value: m, label: m }))]} value={method} onChange={setMethod} aria-label="按方法" />
      </div>
      <div className="sn-table-wrap">
        <table className="sn-table">
          <thead>
            <tr><th>模块</th><th>方法</th><th>URL</th><th>认证</th><th>状态码</th><th>参数</th><th>发现</th><th>覆盖</th><th>范围</th></tr>
          </thead>
          <tbody>
            {filtered.map((e) => (
              <tr key={e.id} style={{ cursor: "pointer" }} onClick={() => onEndpointClick?.(e)}>
                <td className="mono">{e.module}</td>
                <td>{e.method}</td>
                <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{e.url}</td>
                <td>{e.authType}</td>
                <td>{e.statusCode}</td>
                <td>{e.hasParams ? "有" : "—"}</td>
                <td>{e.hasFinding ? <StatusBadge status="REPORTABLE" /> : "—"}</td>
                <td>{e.covered ? <StatusBadge status="COMPLETED" /> : <span className="sn-badge" data-tone="ink-3">未覆盖</span>}</td>
                <td>{e.inScope ? <span className="sn-badge" data-tone="ok">范围内</span> : <span className="sn-badge" data-tone="unknown">范围外</span>}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );

  const discoveryTab = (
    <div style={{ display: "grid", gap: "var(--sp-2)" }}>
      {model.discoveries.length === 0 && <div className="sn-empty">Discovery Inbox 为空</div>}
      {model.discoveries.map((d) => (
        <Card key={d.id}>
          <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-1)" }}>
            <span className="sn-badge" data-tone="info">{d.kind}</span>
            <span className="sn-badge" data-tone={d.confidence === "high" ? "ok" : "ink-2"}>{d.confidence}</span>
            {d.stale && <span className="sn-badge" data-tone="unknown">已过期</span>}
            <span style={{ marginLeft: "auto", color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>来源 {d.source} · {d.followState}</span>
          </div>
          <div style={{ fontSize: "var(--fs-sm)" }}>{d.detail}</div>
          <div style={{ display: "flex", gap: "var(--sp-2)", marginTop: "var(--sp-2)" }}>
            <Button size="sm" variant="secondary" onClick={() => onDiscoveryDirective?.(d.id, "evaluate")}>交给 Strategist 评估</Button>
            <Button size="sm" variant="ghost" onClick={() => onDiscoveryDirective?.(d.id, "duplicate")}>标记重复</Button>
            <Button size="sm" variant="ghost" onClick={() => onDiscoveryDirective?.(d.id, "dismiss")}>忽略</Button>
          </div>
        </Card>
      ))}
    </div>
  );

  return (
    <Tabs
      items={[
        { id: "endpoints", label: "端点", content: endpointsTab },
        { id: "discovery", label: "发现收件箱", badge: model.discoveries.filter((d) => d.followState !== "dismissed").length, content: discoveryTab },
      ]}
    />
  );
}
