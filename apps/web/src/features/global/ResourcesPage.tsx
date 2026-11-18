// 资源三分：知识库 / 技能 / 工具（原「资源库」单页三 Tab 拆为三个独立面板，移入左侧导航）。
// Knowledge 区分 VERIFIED_FACT/OPERATOR_NOTE/UNVERIFIED_REFERENCE；Skill 只是策略知识不能扩 Scope/关审批/取秘密。

import { useState } from "react";
import { StatusBadge } from "../../components/StatusBadge";
import { Markdown } from "../../components/Markdown";

export interface KnowledgeItem {
  id: string;
  level: "VERIFIED_FACT" | "OPERATOR_NOTE" | "UNVERIFIED_REFERENCE";
  text: string; // 摘要/一句话
  source: string;
  techStack: string;
  ttl: string;
  lastHit: string;
  scope: "global" | "run";
  title?: string; // 文档标题（列表展示；缺省回退 text）
  doc?: string; // 文档正文（默认 Markdown，缺省回退 text）
  format?: "md"; // 文档格式，默认 md
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
const LEVEL_LABEL: Record<KnowledgeItem["level"], string> = {
  VERIFIED_FACT: "已核实", OPERATOR_NOTE: "笔记", UNVERIFIED_REFERENCE: "未核实",
};

// 知识库：左侧文档列表 + 右侧文档阅读器（默认 Markdown 渲染）。
// 跨 Run 只复用方法知识，不默认共享原始用户数据/Cookie/敏感 Evidence。
export function KnowledgePanel({ items }: { items: KnowledgeItem[] }) {
  const [sel, setSel] = useState<KnowledgeItem | null>(items[0] ?? null);
  return (
    <div className="sn-kb">
      <div className="sn-kb-list" role="list" aria-label="知识文档">
        {items.map((k) => (
          <button
            key={k.id}
            role="listitem"
            className="sn-kb-item sn-focus"
            data-active={sel?.id === k.id || undefined}
            onClick={() => setSel(k)}
          >
            <span className="sn-kb-dot" data-tone={LEVEL_TONE[k.level]} title={k.level} />
            <span className="sn-kb-item-body">
              <span className="sn-kb-item-title">{k.title ?? k.text}</span>
              <span className="sn-kb-item-sub">{LEVEL_LABEL[k.level]} · {k.scope === "global" ? "全局" : "Run 私有"}</span>
            </span>
          </button>
        ))}
      </div>

      <div className="sn-kb-doc">
        {sel ? (
          <>
            <dl className="sn-kv sn-kb-meta">
              <dt>可信级别</dt><dd>{sel.level}</dd>
              <dt>来源</dt><dd className="mono" style={{ fontSize: "var(--fs-xs)" }}>{sel.source}</dd>
              <dt>技术栈</dt><dd>{sel.techStack}</dd>
              <dt>TTL</dt><dd>{sel.ttl}</dd>
              <dt>分区</dt><dd>{sel.scope === "global" ? "全局" : "Run 私有"}</dd>
              <dt>格式</dt><dd>{sel.format ?? "md"}</dd>
            </dl>
            <Markdown source={sel.doc ?? sel.text} />
            <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", marginTop: "var(--sp-4)" }}>
              跨 Run 只复用方法知识，不默认共享原始用户数据/Cookie/敏感 Evidence。
            </div>
          </>
        ) : (
          <div className="sn-empty">选择左侧知识文档查看</div>
        )}
      </div>
    </div>
  );
}

// 技能：Skill/Prompt 只是策略知识，不能声明扩大 Scope、关闭审批或获取秘密；首版只导入本地受审包。
export function SkillsPanel({ items }: { items: SkillItem[] }) {
  return (
    <div className="sn-table-wrap">
      <table className="sn-table">
        <thead><tr><th>技能</th><th>版本</th><th>来源</th><th>清单哈希</th><th>适用作业</th><th>所需工具</th><th>状态</th></tr></thead>
        <tbody>
          {items.map((s) => (
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
  );
}

// 工具（原「运行模板」）：Secret 只保存 SecretRef 不保存原值；克隆模板显示配置 Diff 并重新 Preflight。
export function ToolsPanel({ items }: { items: RunTemplate[] }) {
  return (
    <div className="sn-table-wrap">
      <table className="sn-table">
        <thead><tr><th>模板</th><th>范围</th><th>预算</th><th>工具集</th></tr></thead>
        <tbody>
          {items.map((t) => (
            <tr key={t.id}><td>{t.name}</td><td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{t.scope}</td><td>{t.budget}</td><td>{t.tools}</td></tr>
          ))}
        </tbody>
      </table>
      <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)", padding: "var(--sp-2)" }}>Secret 只保存 SecretRef 不保存原值；克隆模板显示配置 Diff 并重新 Preflight。</div>
    </div>
  );
}
