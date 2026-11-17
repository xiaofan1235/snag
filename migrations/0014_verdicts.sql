-- 0014_verdicts.sql —— Day 37 重放 + 反证自动降级（§13.4/§13.5）。
--
-- 【迁移不可变约定】新增迁移，不改旧文件。
-- 无验证结论表。§13.4「REPORTABLE 非一次性授予」：VerificationVerdict 带 revision、输入 Evidence Manifest、
-- 适用 Finding Revision；反证/哈希失败/Evidence 失效/依赖 supersede 时自动降级。故新增 verification_verdicts，
-- 并给 evidence 加 quarantined（哈希校验失败的证据进隔离）。

-- +goose Up
CREATE TABLE verification_verdicts (
    id                     TEXT PRIMARY KEY,
    finding_id             TEXT NOT NULL REFERENCES findings(id),
    run_id                 TEXT NOT NULL REFERENCES runs(id),
    revision               INTEGER NOT NULL,
    verdict                TEXT NOT NULL,   -- CONFIRMED/REJECTED/UNABLE_TO_VERIFY/NEEDS_EVIDENCE
    finding_revision       INTEGER NOT NULL DEFAULT 1,
    evidence_manifest_json TEXT,
    superseded             INTEGER NOT NULL DEFAULT 0,
    reason                 TEXT,
    created_at             TEXT NOT NULL,
    UNIQUE(finding_id, revision)
);
CREATE INDEX idx_verdicts_finding ON verification_verdicts(finding_id, superseded);

ALTER TABLE evidence ADD COLUMN quarantined INTEGER NOT NULL DEFAULT 0;

-- +goose Down
DROP INDEX IF EXISTS idx_verdicts_finding;
DROP TABLE IF EXISTS verification_verdicts;
