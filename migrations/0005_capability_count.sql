-- 0005_capability_count.sql —— Day 17 两层 Permit：给 request_capabilities 加消费计数。
--
-- 【迁移不可变约定】新增迁移，不改旧文件。
-- request_capabilities 原有 consumed_at（单次标记）不足以表达「预算 max_subrequests、逐请求消耗」，
-- 加 consumed_count 以做原子预算消耗：UPDATE ... SET consumed_count=consumed_count+1
-- WHERE consumed_count < max_subrequests（超限即中断，§9.5 浏览器多子请求）。

-- +goose Up
ALTER TABLE request_capabilities ADD COLUMN consumed_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE request_capabilities DROP COLUMN consumed_count;
