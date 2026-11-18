// OperationsPage —— Providers/Models/Budget + Identity/Vault + Proxy/Egress + Sandbox/Runner/Tools + Diagnostics
// （UI 文档 §16.24/§16.25/§16.26/§16.27/§16.29）。密钥只写不回显；无「临时直连」选项；审计只读。

import { Tabs } from "../../components/Tabs";
import { StatusBadge } from "../../components/StatusBadge";
import { Button } from "../../components/Button";
import { FractionProgress } from "../../components/Progress";

export interface ProviderItem { id: string; type: string; baseUrl: string; enabled: boolean; models: string[]; checkResult: string; }
export interface ModelProfile { slot: string; model: string; fallback: string; temperature: number; maxOutput: number; }
export interface IdentityItem { id: string; alias: string; role: string; tenant: string; auth: string; expires: string; lastValidated: string; }
export interface EgressPool { id: string; provider: string; region: string; quota: string; currentLease: string; health: string; boundRun: string; }
export interface SandboxItem { id: string; run: string; imageDigest: string; state: string; cpu: string; mem: string; lastHeartbeat: string; }
export interface ToolReg { id: string; name: string; version: string; manifestHash: string; sideEffect: string; successRate: string; enabled: boolean; }
export interface DiagnosticsItem { label: string; value: string; ok: boolean; }
export interface OperationsModel {
  providers: ProviderItem[];
  profiles: ModelProfile[];
  identities: IdentityItem[];
  egress: EgressPool[];
  sandboxes: SandboxItem[];
  tools: ToolReg[];
  diagnostics: DiagnosticsItem[];
  budget: { requestsUsed: number; requestsMax: number; costUsedCents: number; costMaxCents: number };
}

export function OperationsPage({ model, onCheck, onRotate }: {
  model: OperationsModel;
  onCheck?: (providerId: string) => void;
  onRotate?: (poolId: string) => void;
}) {
  return (
    <Tabs
      items={[
        {
          id: "providers",
          label: "模型供应商 / 模型 / 预算",
          content: (
            <div style={{ display: "grid", gap: "var(--sp-4)" }}>
              <div className="sn-table-wrap">
                <table className="sn-table">
                  <thead><tr><th>供应商</th><th>BaseURL</th><th>模型</th><th>连接检查</th><th>状态</th><th></th></tr></thead>
                  <tbody>
                    {model.providers.map((p) => (
                      <tr key={p.id}>
                        <td>{p.type}</td>
                        <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{p.baseUrl}</td>
                        <td>{p.models.join(", ")}</td>
                        <td>{p.checkResult}</td>
                        <td><StatusBadge status={p.enabled ? "RUNNING" : "STOPPED"} /></td>
                        <td><Button size="sm" variant="ghost" onClick={() => onCheck?.(p.id)}>连接检查</Button></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", padding: "var(--sp-2)" }}>密钥只在写入时提交，读取只返回掩码和 SecretRef。</div>
              </div>
              <div>
                <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-1)" }}>Model Profile（槽位）</div>
                <table className="sn-table">
                  <thead><tr><th>槽位</th><th>主模型</th><th>备用模型</th><th>温度</th><th>输出上限</th></tr></thead>
                  <tbody>{model.profiles.map((m) => (<tr key={m.slot}><td>{m.slot}</td><td>{m.model}</td><td>{m.fallback}</td><td>{m.temperature}</td><td>{m.maxOutput}</td></tr>))}</tbody>
                </table>
              </div>
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--sp-4)" }}>
                <div><div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>请求预算</div><FractionProgress done={model.budget.requestsUsed} total={model.budget.requestsMax} /></div>
                <div><div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>费用预算（分）</div><FractionProgress done={model.budget.costUsedCents} total={model.budget.costMaxCents} /></div>
              </div>
            </div>
          ),
        },
        {
          id: "identity",
          label: "身份 / 密钥保险库",
          content: (
            <div className="sn-table-wrap">
              <table className="sn-table">
                <thead><tr><th>别名</th><th>角色</th><th>租户</th><th>认证方式</th><th>有效期</th><th>最后验证</th></tr></thead>
                <tbody>{model.identities.map((i) => (<tr key={i.id}><td>{i.alias}</td><td>{i.role}</td><td>{i.tenant}</td><td>{i.auth}</td><td>{i.expires}</td><td>{i.lastValidated}</td></tr>))}</tbody>
              </table>
              <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", padding: "var(--sp-2)" }}>新建/轮换秘密写后即不可见；Secret Reveal 默认不提供；不允许把 Secret 值复制进指令/评论/知识库/报告。</div>
            </div>
          ),
        },
        {
          id: "egress",
          label: "代理 / 出口",
          content: (
            <div className="sn-table-wrap">
              <table className="sn-table">
                <thead><tr><th>出口池</th><th>区域</th><th>额度</th><th>当前租约</th><th>健康</th><th>绑定运行</th><th></th></tr></thead>
                <tbody>
                  {model.egress.map((p) => (
                    <tr key={p.id}>
                      <td>{p.provider}</td><td>{p.region}</td><td>{p.quota}</td>
                      <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{p.currentLease}</td>
                      <td><StatusBadge status={p.health} /></td><td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{p.boundRun}</td>
                      <td><Button size="sm" variant="ghost" onClick={() => onRotate?.(p.id)}>轮换</Button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", padding: "var(--sp-2)" }}>目标流量必须显示绑定出口，不允许 DIRECT 假象；页面没有「代理失败时临时直连」选项。</div>
            </div>
          ),
        },
        {
          id: "sandbox",
          label: "沙箱 / 执行器 / 工具",
          content: (
            <div style={{ display: "grid", gap: "var(--sp-4)" }}>
              <table className="sn-table">
                <thead><tr><th>实例</th><th>运行</th><th>镜像摘要</th><th>状态</th><th>CPU/内存</th><th>最后心跳</th></tr></thead>
                <tbody>{model.sandboxes.map((s) => (<tr key={s.id}><td className="mono">{s.id}</td><td className="mono">{s.run}</td><td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{s.imageDigest}</td><td><StatusBadge status={s.state} /></td><td>{s.cpu}/{s.mem}</td><td>{s.lastHeartbeat}</td></tr>))}</tbody>
              </table>
              <div>
                <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginBottom: "var(--sp-1)" }}>工具注册表</div>
                <table className="sn-table">
                  <thead><tr><th>工具</th><th>版本</th><th>清单哈希</th><th>副作用</th><th>成功率</th><th>状态</th></tr></thead>
                  <tbody>{model.tools.map((t) => (<tr key={t.id}><td className="mono">{t.name}</td><td>{t.version}</td><td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{t.manifestHash}</td><td>{t.sideEffect}</td><td>{t.successRate}</td><td><StatusBadge status={t.enabled ? "RUNNING" : "STOPPED"} /></td></tr>))}</tbody>
                </table>
              </div>
            </div>
          ),
        },
        {
          id: "diagnostics",
          label: "诊断",
          content: (
            <dl className="sn-kv" style={{ gridTemplateColumns: "auto auto 1fr", rowGap: "var(--sp-2)" }}>
              {model.diagnostics.map((d) => (
                <div key={d.label} style={{ display: "contents" }}>
                  <dt>{d.label}</dt>
                  <dd><StatusBadge status={d.ok ? "COMPLETED" : "FAILED"} /></dd>
                  <dd className="mono" style={{ fontSize: "var(--fs-xs)" }}>{d.value}</dd>
                </div>
              ))}
            </dl>
          ),
        },
      ]}
    />
  );
}
