// FilesPage —— Files / Artifacts / Resources（UI 文档 §16.19）。产物树 + Resource 库。
// 文件内容是不可信数据；Secret 进 Vault 不作普通文本；压缩包先列目录不自动解压执行。

import { useState } from "react";
import { Card } from "../../components/Card";
import { Button } from "../../components/Button";
import { Tabs } from "../../components/Tabs";

export interface ArtifactFile {
  path: string;
  type: string;
  sizeBytes: number;
  sha256: string;
  sourceJob: string;
  createdAt: string;
  retention: string;
  sensitive: boolean;
}
export interface ResourceItem {
  id: string;
  name: string;
  scope: string;
  tags: string[];
  sha256: string;
  uploader: string;
  expires: string;
  isSecret: boolean;
}
export interface FilesModel {
  files: ArtifactFile[];
  resources: ResourceItem[];
}

function humanSize(n: number): string {
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
  return (n / 1024 / 1024).toFixed(1) + " MB";
}

export function FilesPage({ model, onPreview }: { model: FilesModel; onPreview?: (path: string) => void }) {
  const [sel, setSel] = useState<ArtifactFile | null>(null);
  return (
    <Tabs
      items={[
        {
          id: "files",
          label: "运行文件",
          content: (
            <div className="sn-three-col" style={{ gridTemplateColumns: "1fr 300px" }}>
              <div className="sn-table-wrap">
                <table className="sn-table">
                  <thead><tr><th>路径</th><th>类型</th><th>大小</th><th>来源 Job</th><th>保留</th><th>敏感</th></tr></thead>
                  <tbody>
                    {model.files.map((f) => (
                      <tr key={f.path} style={{ cursor: "pointer" }} onClick={() => setSel(f)}>
                        <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{f.path}</td>
                        <td>{f.type}</td>
                        <td>{humanSize(f.sizeBytes)}</td>
                        <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{f.sourceJob}</td>
                        <td>{f.retention}</td>
                        <td>{f.sensitive ? <span className="sn-badge" data-tone="warn">敏感</span> : "—"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div>
                {sel ? (
                  <Card>
                    <div style={{ fontWeight: 600, marginBottom: "var(--sp-2)", wordBreak: "break-all" }}>{sel.path}</div>
                    <dl className="sn-kv">
                      <dt>SHA-256</dt><dd className="mono" style={{ fontSize: "var(--fs-xs)" }}>{sel.sha256}</dd>
                      <dt>创建</dt><dd>{sel.createdAt}</dd>
                    </dl>
                    <div style={{ display: "flex", gap: "var(--sp-2)", marginTop: "var(--sp-3)" }}>
                      <Button size="sm" variant="secondary" onClick={() => onPreview?.(sel.path)}>安全预览</Button>
                      <Button size="sm" variant="ghost">下载</Button>
                      <Button size="sm" variant="ghost">加入证据包</Button>
                    </div>
                  </Card>
                ) : (
                  <div className="sn-empty">选择文件查看详情</div>
                )}
              </div>
            </div>
          ),
        },
        {
          id: "resources",
          label: "用户 Resource 库",
          content: (
            <div className="sn-table-wrap">
              <table className="sn-table">
                <thead><tr><th>名称</th><th>范围</th><th>标签</th><th>上传者</th><th>有效期</th><th>类型</th></tr></thead>
                <tbody>
                  {model.resources.map((r) => (
                    <tr key={r.id}>
                      <td>{r.name}</td>
                      <td className="mono" style={{ fontSize: "var(--fs-xs)" }}>{r.scope}</td>
                      <td>{r.tags.join(", ")}</td>
                      <td>{r.uploader}</td>
                      <td>{r.expires}</td>
                      <td>{r.isSecret ? <span className="sn-badge" data-tone="danger">Secret（入 Vault）</span> : "普通"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ),
        },
      ]}
    />
  );
}
