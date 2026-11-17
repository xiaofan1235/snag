-- 0009_endpoint_unique.sql —— Day 31 攻击面发现：endpoints 去重唯一索引（§12.1 接口归一化 + 去重）。
--
-- 【迁移不可变约定】新增迁移，不改旧文件。
-- endpoints（0002）无唯一约束；攻击面发现会反复从 JS/规格/运行时发现同一接口，需按
-- (run_id, method, url_pattern) 去重（归一化 + 路径参数模板化后），配合 INSERT OR IGNORE 幂等入库，
-- 避免重复接口污染覆盖率并反复虚增 SurfaceRevision。

-- +goose Up
CREATE UNIQUE INDEX idx_endpoints_dedup ON endpoints(run_id, method, url_pattern);

-- +goose Down
DROP INDEX IF EXISTS idx_endpoints_dedup;
