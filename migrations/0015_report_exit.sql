-- 0015_report_exit.sql —— Day 38 报告 + Exit Gate + 跨 Run 关联（§13.6/§14.2/§47.3）。
--
-- 【迁移不可变约定】只新增/ALTER，不改旧文件。
-- reports / cross_run_finding_links 在 0002_full 已建空壳（仅骨架列），本迁移按 Day 38 需要补列：
--   - reports：报告 = 程序填事实 + AI 写描述；需存渲染体（Markdown/JSON）、Evidence Manifest、
--     以及把本次报告**绑定到同一 Finding/Verification 快照**所需的 run/snapshot/surface 修订号
--     与 finding_snapshot_json（Finalize 时据此重校验依赖，失效则标 STALE，§13.4/§14.2）。
--   - cross_run_finding_links：§47.3 关联依据（确定性指纹 basis + confidence）与红线前提
--     （仅同 Workspace 同 Scope Digest）所需的 workspace_id/scope_digest。
-- SQLite 旧版不支持 DROP COLUMN，故 Down 不回删 ALTER 列（与既有约定一致）。

-- +goose Up
ALTER TABLE reports ADD COLUMN format                 TEXT;              -- 主渲染格式标记（markdown|json；双体均存）
ALTER TABLE reports ADD COLUMN markdown_body          TEXT;              -- 程序渲染的 Markdown 全文
ALTER TABLE reports ADD COLUMN json_body              TEXT;              -- 程序渲染的 JSON 全文
ALTER TABLE reports ADD COLUMN evidence_manifest_json TEXT;              -- Evidence Manifest（引用，不搬运）
ALTER TABLE reports ADD COLUMN finding_snapshot_json  TEXT;              -- 绑定快照：finding_id+revision+verdict_revision 列表
ALTER TABLE reports ADD COLUMN run_revision           INTEGER NOT NULL DEFAULT 0;
ALTER TABLE reports ADD COLUMN snapshot_revision      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE reports ADD COLUMN surface_revision       INTEGER NOT NULL DEFAULT 0;
ALTER TABLE reports ADD COLUMN finalized_at           TEXT;
ALTER TABLE reports ADD COLUMN stale_reason           TEXT;

ALTER TABLE cross_run_finding_links ADD COLUMN workspace_id TEXT;        -- 红线：仅同 Workspace
ALTER TABLE cross_run_finding_links ADD COLUMN scope_digest TEXT;        -- 红线：仅同 Scope Digest（§4.0）
ALTER TABLE cross_run_finding_links ADD COLUMN basis        TEXT;        -- 关联依据（端点+参数+攻击类型+对象归属 指纹）
ALTER TABLE cross_run_finding_links ADD COLUMN confidence   TEXT;        -- high|medium|low

CREATE INDEX IF NOT EXISTS idx_reports_run         ON reports(run_id, state);
CREATE INDEX IF NOT EXISTS idx_crossrun_finding    ON cross_run_finding_links(finding_id);
CREATE INDEX IF NOT EXISTS idx_crossrun_ws_scope   ON cross_run_finding_links(workspace_id, scope_digest);

-- +goose Down
DROP INDEX IF EXISTS idx_reports_run;
DROP INDEX IF EXISTS idx_crossrun_finding;
DROP INDEX IF EXISTS idx_crossrun_ws_scope;
-- ALTER 新增列不回删（SQLite 旧版不支持 DROP COLUMN；迁移不可变约定）。
