// SettingsPage —— Diagnostics / Preflight / Settings（UI 文档 §16.29）。
// Preflight 分组展示 PASS/WARN/FAIL/NOT_APPLICABLE + Recheck；FAIL 阻断项不能前端勾选忽略。
// Settings：通用/安全/数据/API（界面偏好存本地，安全默认值与权威配置存后端）。

import { Tabs } from "../../components/Tabs";
import { StatusBadge } from "../../components/StatusBadge";
import { Button } from "../../components/Button";

export type PreflightStatus = "PASS" | "WARN" | "FAIL" | "NOT_APPLICABLE";
export interface PreflightCheckRow { group: string; item: string; status: PreflightStatus; detail: string; fix?: string; }
export interface SettingsModel { preflight: PreflightCheckRow[]; }

const PF_TONE: Record<PreflightStatus, string> = { PASS: "ok", WARN: "warn", FAIL: "danger", NOT_APPLICABLE: "ink-3" };
const PF_STATUS: Record<PreflightStatus, string> = { PASS: "COMPLETED", WARN: "BLOCKED", FAIL: "FAILED", NOT_APPLICABLE: "STOPPED" };

export function SettingsPage({ model, onRecheck }: { model: SettingsModel; onRecheck?: (item: string) => void }) {
  const groups = Array.from(new Set(model.preflight.map((p) => p.group)));
  return (
    <Tabs
      items={[
        {
          id: "preflight",
          label: "预检中心",
          content: (
            <div style={{ display: "grid", gap: "var(--sp-4)" }}>
              {groups.map((g) => (
                <div key={g}>
                  <div style={{ fontWeight: 600, marginBottom: "var(--sp-2)" }}>{g}</div>
                  <div className="sn-table-wrap">
                    <table className="sn-table">
                      <tbody>
                        {model.preflight.filter((p) => p.group === g).map((p) => (
                          <tr key={p.item}>
                            <td style={{ width: 120 }}><StatusBadge status={PF_STATUS[p.status]} /><span className="sn-badge" data-tone={PF_TONE[p.status]} style={{ marginLeft: 4 }}>{p.status}</span></td>
                            <td>{p.item}</td>
                            <td style={{ color: "var(--ink-2)", fontSize: "var(--fs-xs)" }}>{p.detail}{p.fix && <div style={{ color: "var(--warn)" }}>修复：{p.fix}</div>}</td>
                            <td style={{ width: 80 }}><Button size="sm" variant="ghost" onClick={() => onRecheck?.(p.item)}>重检</Button></td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              ))}
              <div style={{ color: "var(--ink-3)", fontSize: "var(--fs-xs)" }}>FAIL 的阻断项不能由前端勾选忽略；确需人工接受的风险必须形成带期限的 Policy/Exception 审批。</div>
            </div>
          ),
        },
        {
          id: "general",
          label: "通用",
          content: (
            <dl className="sn-kv">
              <dt>语言</dt><dd>简体中文</dd>
              <dt>时区</dt><dd>Europe/London</dd>
              <dt>主题</dt><dd>跟随状态条切换</dd>
              <dt>密度</dt><dd>comfortable（阅读区）/ compact（监控区）</dd>
              <dt>默认页面</dt><dd>值班台</dd>
            </dl>
          ),
        },
        {
          id: "security",
          label: "安全",
          content: (
            <dl className="sn-kv">
              <dt>默认脱敏</dt><dd>开（服务端 Redacted Projection + 前端额外防线）</dd>
              <dt>Raw Reveal 时限</dt><dd>5 分钟（短时授权 + 审计）</dd>
              <dt>外部内容</dt><dd>HTML/SVG/Markdown 视为不可信；CSP 禁外部脚本/自动外链图片</dd>
              <dt>下载确认</dt><dd>安全 Content-Disposition</dd>
              <dt>本地会话锁定</dt><dd>同源 HttpOnly + SameSite=Strict；Token 不入 URL/LocalStorage</dd>
            </dl>
          ),
        },
        {
          id: "api",
          label: "API",
          content: (
            <dl className="sn-kv">
              <dt>本地监听地址</dt><dd className="mono">127.0.0.1:8787（非 Loopback 默认禁用，需单独加固）</dd>
              <dt>Token 轮换</dt><dd>支持</dd>
              <dt>允许 Origin</dt><dd>同源</dd>
            </dl>
          ),
        },
      ]}
    />
  );
}
