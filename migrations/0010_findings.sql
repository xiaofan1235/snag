-- 0010_findings.sql —— Day 32 模块聚类 + 技术栈指纹 + Discovery（§12.2/§12.3/§12.4）。
--
-- 【迁移不可变约定】新增迁移，不改旧文件。
-- discoveries（0002）无状态列，§12.4 要求线索状态机 OPEN/FOLLOWING/FOLLOWED/DISMISSED/PROMOTED/BLOCKED，
-- 且「退出前所有高价值 Discovery 必须有处置」——故加 state + high_value。
-- 无技术栈指纹表，§12.3 要求「技术栈推断必须有 EvidenceRef，不能把猜测升级成事实」——故新增
-- tech_fingerprints，evidence_ref 设 NOT NULL 在库层硬约束（无证据的指纹根本存不进来）。

-- +goose Up
ALTER TABLE discoveries ADD COLUMN state TEXT NOT NULL DEFAULT 'OPEN';
ALTER TABLE discoveries ADD COLUMN high_value INTEGER NOT NULL DEFAULT 0;

CREATE TABLE tech_fingerprints (
    id           TEXT PRIMARY KEY,
    run_id       TEXT NOT NULL REFERENCES runs(id),
    module_id    TEXT,
    category     TEXT NOT NULL,   -- framework/data_layer/auth_structure/param_binding/object_ownership/attack_surface/avoid_direction/unverified_inference
    value        TEXT NOT NULL,
    evidence_ref TEXT NOT NULL,   -- §12.3：技术栈推断必须有 EvidenceRef（库层硬约束，无证据存不进）
    confidence   TEXT,
    created_at   TEXT NOT NULL
);
CREATE INDEX idx_tech_fp_run ON tech_fingerprints(run_id, category);

-- +goose Down
DROP INDEX IF EXISTS idx_tech_fp_run;
DROP TABLE IF EXISTS tech_fingerprints;
-- 注：SQLite 不支持 DROP COLUMN（旧版本），discoveries 的 state/high_value 列保留（向后兼容无害）。
