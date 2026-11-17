-- 0012_race_oast.sql —— Day 34 受控竞态实验 + OAST（§44.2/§44.3）。
--
-- 【迁移不可变约定】新增迁移，不改旧文件。
-- race_experiments（0002）缺 §44.2 的 risk_class/sync_strategy/审批/成功计数；oast_correlations 缺
-- §44.3 的状态机（PENDING/RESOLVED/TIMEOUT）与 experiment 绑定。补齐。

-- +goose Up
ALTER TABLE race_experiments ADD COLUMN endpoint_id TEXT;
ALTER TABLE race_experiments ADD COLUMN identity_id TEXT;
ALTER TABLE race_experiments ADD COLUMN risk_class TEXT NOT NULL DEFAULT 'concurrent_write';
ALTER TABLE race_experiments ADD COLUMN sync_strategy TEXT NOT NULL DEFAULT 'batch';
ALTER TABLE race_experiments ADD COLUMN max_total_requests INTEGER NOT NULL DEFAULT 20;
ALTER TABLE race_experiments ADD COLUMN requires_approval INTEGER NOT NULL DEFAULT 1;
ALTER TABLE race_experiments ADD COLUMN approval_state TEXT NOT NULL DEFAULT 'PENDING';
ALTER TABLE race_experiments ADD COLUMN success_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE race_experiments ADD COLUMN updated_at TEXT;

ALTER TABLE oast_correlations ADD COLUMN experiment_id TEXT;
ALTER TABLE oast_correlations ADD COLUMN state TEXT NOT NULL DEFAULT 'PENDING';
ALTER TABLE oast_correlations ADD COLUMN created_at TEXT;
ALTER TABLE oast_correlations ADD COLUMN oast_event_evidence_id TEXT;

-- +goose Down
-- SQLite 旧版不支持 DROP COLUMN；新增列向后兼容无害，保留。
