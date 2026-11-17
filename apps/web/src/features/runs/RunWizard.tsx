// RunWizard —— Run 创建向导（UI 文档 §16.2）。5 步：目标→规则→身份→执行环境→策略确认。
// 核心门禁：**只有 Preflight 全部通过才能 Start**。原始 Cookie/Token 不在普通表单回显（§16.2/§16.33）。

import { useMemo, useState } from "react";
import { Button } from "../../components/Button";
import { Input } from "../../components/Input";
import { Card } from "../../components/Card";
import { StatusBadge } from "../../components/StatusBadge";

export interface WizardTarget {
  name: string;
  workspace: string;
  startUrl: string;
  domain: string;
  includeSubdomains: boolean;
  forbiddenPaths: string;
  authorizationRef: string;
}
export interface WizardRules {
  rate: number;
  totalRequests: number;
  allowWriteOnOwnedTargets: boolean;
  idorQuota: number;
}
export interface WizardIdentity {
  mode: "anonymous" | "account_a" | "account_b";
  role: string;
  secretRef: string; // SecretRef，不是原始 Cookie
}
export interface WizardEnv {
  modelProfile: string;
  maxWorkers: number;
  costBudgetCents: number;
  tokenBudget: number;
  maxRuntimeMinutes: number;
}
export interface WizardState {
  target: WizardTarget;
  rules: WizardRules;
  identity: WizardIdentity;
  env: WizardEnv;
}

export interface PreflightCheck {
  id: string;
  label: string;
  ok: boolean;
  detail?: string;
}

export interface RunWizardProps {
  initial?: Partial<WizardState>;
  /** 由调用方（后端 Preflight）给出检查结果；全 ok 才允许 Start。 */
  preflight: PreflightCheck[];
  onStart: (state: WizardState) => void;
  onRunPreflight?: (state: WizardState) => void;
}

const STEPS = ["目标", "测试规则", "身份", "执行环境", "策略确认"];

const defaultState: WizardState = {
  target: { name: "", workspace: "", startUrl: "", domain: "", includeSubdomains: false, forbiddenPaths: "", authorizationRef: "" },
  rules: { rate: 1, totalRequests: 500, allowWriteOnOwnedTargets: false, idorQuota: 10 }, // 默认保守
  identity: { mode: "anonymous", role: "", secretRef: "" },
  env: { modelProfile: "default", maxWorkers: 2, costBudgetCents: 500, tokenBudget: 1_000_000, maxRuntimeMinutes: 120 },
};

export function RunWizard({ initial, preflight, onStart, onRunPreflight }: RunWizardProps) {
  const [step, setStep] = useState(0);
  const [state, setState] = useState<WizardState>({ ...defaultState, ...initial } as WizardState);

  const normalizedScope = useMemo(() => {
    const t = state.target;
    const sub = t.includeSubdomains ? "*." : "";
    return t.domain ? `${sub}${t.domain}${t.forbiddenPaths ? ` (禁: ${t.forbiddenPaths})` : ""}` : "（未设定）";
  }, [state.target]);

  const preflightPassed = preflight.length > 0 && preflight.every((c) => c.ok);

  function patchTarget(p: Partial<WizardTarget>) {
    setState((s) => ({ ...s, target: { ...s.target, ...p } }));
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-4)" }}>
      {/* 步骤指示 */}
      <ol role="list" style={{ display: "flex", gap: "var(--sp-2)", listStyle: "none", padding: 0, margin: 0 }}>
        {STEPS.map((label, i) => (
          <li
            key={label}
            aria-current={i === step ? "step" : undefined}
            style={{
              padding: "var(--sp-1) var(--sp-3)",
              borderRadius: "var(--radius-sm)",
              background: i === step ? "var(--accent-weak)" : "var(--bg-sunken)",
              color: i === step ? "var(--accent)" : "var(--ink-3)",
              fontSize: "var(--fs-sm)",
            }}
          >
            {i + 1}. {label}
          </li>
        ))}
      </ol>

      <Card>
        {step === 0 && (
          <div style={{ display: "grid", gap: "var(--sp-3)" }}>
            <Input label="Run 名称" value={state.target.name} onChange={(e) => patchTarget({ name: e.target.value })} />
            <Input label="起始 URL" value={state.target.startUrl} onChange={(e) => patchTarget({ startUrl: e.target.value })} />
            <Input label="精确域名" value={state.target.domain} onChange={(e) => patchTarget({ domain: e.target.value })} />
            <Input label="禁止路径" value={state.target.forbiddenPaths} onChange={(e) => patchTarget({ forbiddenPaths: e.target.value })} />
            <Input label="授权材料引用" value={state.target.authorizationRef} onChange={(e) => patchTarget({ authorizationRef: e.target.value })} />
            <div style={{ color: "var(--ink-2)", fontSize: "var(--fs-sm)" }}>
              规范化 Scope：<span className="mono">{normalizedScope}</span>
            </div>
          </div>
        )}
        {step === 1 && (
          <div style={{ display: "grid", gap: "var(--sp-3)" }}>
            <Input
              label="速率（req/s）"
              type="number"
              value={state.rules.rate}
              onChange={(e) => setState((s) => ({ ...s, rules: { ...s.rules, rate: Number(e.target.value) } }))}
            />
            <Input
              label="总请求数上限"
              type="number"
              value={state.rules.totalRequests}
              onChange={(e) => setState((s) => ({ ...s, rules: { ...s.rules, totalRequests: Number(e.target.value) } }))}
            />
            <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>默认全部保守（写操作默认关闭，配额最小）。</div>
          </div>
        )}
        {step === 2 && (
          <div style={{ display: "grid", gap: "var(--sp-3)" }}>
            <Input label="角色" value={state.identity.role} onChange={(e) => setState((s) => ({ ...s, identity: { ...s.identity, role: e.target.value } }))} />
            <Input
              label="Cookie/Token SecretRef"
              placeholder="secret://…"
              value={state.identity.secretRef}
              onChange={(e) => setState((s) => ({ ...s, identity: { ...s.identity, secretRef: e.target.value } }))}
              hint="只填引用，原始 Cookie 不在此回显"
            />
          </div>
        )}
        {step === 3 && (
          <div style={{ display: "grid", gap: "var(--sp-3)" }}>
            <Input label="模型 Profile" value={state.env.modelProfile} onChange={(e) => setState((s) => ({ ...s, env: { ...s.env, modelProfile: e.target.value } }))} />
            <Input
              label="费用预算（分）"
              type="number"
              value={state.env.costBudgetCents}
              onChange={(e) => setState((s) => ({ ...s, env: { ...s.env, costBudgetCents: Number(e.target.value) } }))}
            />
            <Input
              label="最大运行时长（分钟）"
              type="number"
              value={state.env.maxRuntimeMinutes}
              onChange={(e) => setState((s) => ({ ...s, env: { ...s.env, maxRuntimeMinutes: Number(e.target.value) } }))}
            />
          </div>
        )}
        {step === 4 && (
          <div>
            <h3 style={{ marginTop: 0, fontSize: "var(--fs-lg)" }}>策略确认</h3>
            <ul style={{ color: "var(--ink-2)", fontSize: "var(--fs-sm)" }}>
              <li>允许访问：<span className="mono">{normalizedScope}</span></li>
              <li>允许执行：速率 {state.rules.rate}/s，总请求 {state.rules.totalRequests}，写操作 {state.rules.allowWriteOnOwnedTargets ? "开" : "关"}</li>
              <li>预算：{state.env.costBudgetCents} 分 / {state.env.maxRuntimeMinutes} 分钟</li>
            </ul>
            <div style={{ marginTop: "var(--sp-3)" }}>
              <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-2)" }}>
                <strong style={{ fontSize: "var(--fs-sm)" }}>Preflight</strong>
                {onRunPreflight && (
                  <Button size="sm" variant="secondary" onClick={() => onRunPreflight(state)}>
                    运行 Preflight
                  </Button>
                )}
              </div>
              {preflight.length === 0 ? (
                <div className="sn-empty" style={{ padding: "var(--sp-3)" }}>尚未运行 Preflight</div>
              ) : (
                <ul role="list" style={{ listStyle: "none", padding: 0, margin: 0, display: "grid", gap: "var(--sp-1)" }}>
                  {preflight.map((c) => (
                    <li key={c.id} style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
                      <StatusBadge status={c.ok ? "COMPLETED" : "FAILED"} />
                      <span style={{ fontSize: "var(--fs-sm)" }}>{c.label}</span>
                      {c.detail && <span style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>{c.detail}</span>}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        )}
      </Card>

      <div style={{ display: "flex", gap: "var(--sp-2)", justifyContent: "space-between" }}>
        <Button variant="ghost" disabled={step === 0} onClick={() => setStep((s) => Math.max(0, s - 1))}>
          上一步
        </Button>
        {step < STEPS.length - 1 ? (
          <Button variant="primary" onClick={() => setStep((s) => Math.min(STEPS.length - 1, s + 1))}>
            下一步
          </Button>
        ) : (
          <Button
            variant="primary"
            disabled={!preflightPassed}
            title={preflightPassed ? undefined : "Preflight 未全部通过，不能 Start"}
            onClick={() => onStart(state)}
          >
            Start
          </Button>
        )}
      </div>
    </div>
  );
}
