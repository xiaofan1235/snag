// ResourcesPage —— Knowledge / Skills / Run Templates / Resource 库（UI 文档 §16.19/§16.20）。
// Knowledge 区分 VERIFIED_FACT/OPERATOR_NOTE/UNVERIFIED_REFERENCE；Skill 只是策略知识不能扩 Scope/关审批/取秘密。

import { Tabs } from "../../components/Tabs";
import { StatusBadge } from "../../components/StatusBadge";

export interface KnowledgeItem {
  id: string;
  level: "VERIFIED_FACT" | "OPERATOR_NOTE" | "UNVERIFIED_REFERENCE";
  text: string;
  source: string;
  techStack: string;
  ttl: string;
  lastHit: string;
  scope: "global" | "run";
}
export interface SkillItem {
  id: string;
  name: string;
  version: string;
  source: string;
  sha256: string;
  appliesTo: string;
  tools: string[];
  enabled: boolean;
}
export interface RunTemplate {
  id: string;
  name: string;
  scope: string;
  budget: string;
  tools: string;
}
export interface ResourcesModel {
  knowledge: KnowledgeItem[];
  skills: SkillItem[];
  templates: RunTemplate[];
}

const LEVEL_TONE: Record<KnowledgeItem["level"], string> = {
  VERIFIED_FACT: "ok", OPERATOR_NOTE: "info", UNVERIFIED_REFERENCE: "unknown",
};

export function ResourcesPage({ model }: { model: ResourcesModel }) {
  return (
    <Tabs
      items={[
        {
          id: "knowledge",
          label: "知识",
          content: (
            <div className="sn-table-wrap">
              <table className="sn-table">
                <thead><tr><th>可信级别</th><th>内容</th><th>来源</th><th>技术栈</th><th>TTL</th><th>分区</th></tr></thead>
                <tbody>
                  {model.knowledge.map((k) => (
                    <tr key={k.id}>
                      <td><span className="sn-badge" data-tone={LEVEL_TONE[k.level]}>{k.level}</span></td>
                      <td>{k.text}</td>
                      <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{k.source}</td>
                      <td>{k.techStack}</td>
                      <td>{k.ttl}</td>
                      <td>{k.scope === "global" ? "全局" : "Run 私有"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", padding: "var(--sp-2)" }}>跨 Run 只复用方法知识，不默认共享原始用户数据/Cookie/敏感 Evidence。</div>
            </div>
          ),
        },
        {
          id: "skills",
          label: "技能与模板",
          content: (
            <div className="sn-table-wrap">
              <table className="sn-table">
                <thead><tr><th>技能</th><th>版本</th><th>来源</th><th>清单哈希</th><th>适用作业</th><th>所需工具</th><th>状态</th></tr></thead>
                <tbody>
                  {model.skills.map((s) => (
                    <tr key={s.id}>
                      <td>{s.name}</td>
                      <td className="mono">{s.version}</td>
                      <td>{s.source}</td>
                      <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{s.sha256}</td>
                      <td>{s.appliesTo}</td>
                      <td>{s.tools.join(", ")}</td>
                      <td><StatusBadge status={s.enabled ? "RUNNING" : "STOPPED"} /></td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", padding: "var(--sp-2)" }}>Skill/Prompt 只是策略知识，不能声明扩大 Scope、关闭审批或获取秘密；首版只导入本地受审包。</div>
            </div>
          ),
        },
        {
          id: "templates",
          label: "运行模板",
          content: (
            <div className="sn-table-wrap">
              <table className="sn-table">
                <thead><tr><th>模板</th><th>范围</th><th>预算</th><th>工具集</th></tr></thead>
                <tbody>
                  {model.templates.map((t) => (
                    <tr key={t.id}><td>{t.name}</td><td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{t.scope}</td><td>{t.budget}</td><td>{t.tools}</td></tr>
                  ))}
                </tbody>
              </table>
              <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", padding: "var(--sp-2)" }}>Secret 只保存 SecretRef 不保存原值；克隆模板显示配置 Diff 并重新 Preflight。</div>
            </div>
          ),
        },
      ]}
    />
  );
}
