-- 0004_outbox_events.sql —— Day 10 Event Journal + Outbox 所需的列补齐。
--
-- 【迁移不可变约定】Day 06 起迁移视为不可变，schema 演进新增迁移文件，不改 0001/0002/0003。
--
-- events.global_sequence（§15.6/§3151）：SSE 全局流排序键。§4.2 的 events 字段清单只列了
--   run 内 sequence，但 §3151「全局流使用 global_sequence」、§4891「统一 event_cursor/
--   global_sequence」明确要求全局单调序。属 spec 两节口径差，按 §15.6/§3151 向前补齐。
--   全局单调由单写者串行保证（MAX+1），UNIQUE 索引作完整性兜底。
-- outbox 退避列（§4 设计：重试指数退避）：attempts/next_attempt_at/last_error 让退避状态
--   持久化、跨重启存活，publisher 据 next_attempt_at<=now 决定何时重投。

-- +goose Up
ALTER TABLE events ADD COLUMN global_sequence INTEGER NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX idx_events_global_sequence ON events(global_sequence);

ALTER TABLE outbox ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE outbox ADD COLUMN next_attempt_at TEXT;
ALTER TABLE outbox ADD COLUMN last_error TEXT;
CREATE INDEX idx_outbox_pending ON outbox(published, next_attempt_at);

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_pending;
ALTER TABLE outbox DROP COLUMN last_error;
ALTER TABLE outbox DROP COLUMN next_attempt_at;
ALTER TABLE outbox DROP COLUMN attempts;
DROP INDEX IF EXISTS idx_events_global_sequence;
ALTER TABLE events DROP COLUMN global_sequence;
