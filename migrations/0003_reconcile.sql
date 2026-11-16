-- 0003_reconcile.sql —— 给 jobs 增加对账截止时间列（§5.2 RECONCILING + §31 C3 对账超时）。
--
-- 【迁移不可变约定】Day 06 起已推送迁移视为不可变；schema 演进一律新增迁移文件，
-- 不改 0001/0002。本迁移只做一件事：ALTER TABLE ADD COLUMN（SQLite 原生支持、无表重建）。
--
-- reconcile_deadline：Job 进入 RECONCILING 时写入的墙钟截止时刻（RFC3339，UTC 'Z'）。
--   到点未完成对账 → 自动 OUTCOME_UNRESOLVED 终态（见 internal/control/job_controller.go）。
--   选墙钟而非 monotonic：截止时刻需跨进程/重启存活，monotonic 不可持久化（§31 L5 的
--   monotonic 约束针对进程内定时器调度；权威到期判定 = deadline vs 墙钟 now）。

-- +goose Up
ALTER TABLE jobs ADD COLUMN reconcile_deadline TEXT;

-- +goose Down
ALTER TABLE jobs DROP COLUMN reconcile_deadline;
