-- 0006_budget_reservations.sql —— Day 24 模型预算账本：预留状态机（§7.5）。
--
-- 【迁移不可变约定】新增迁移，不改旧文件。
-- 既有 budget_ledger 是扁平流水（RESERVED/SETTLED 条目），表达不了 §7.5 的「预留本身是状态机」：
-- RESERVED→IN_FLIGHT→SUCCEEDED/UNKNOWN→SETTLED，需逐预留记状态 + provider_request_id（幂等键）
-- + 最坏预留成本 + 真实多维结算。故新增 budget_reservations 承载状态机，budget_ledger 仍作审计流水。
-- 金额一律整数 micro_usd、Token 整数（§31 C7，不碰 float）。

-- +goose Up
CREATE TABLE budget_reservations (
    id                  TEXT PRIMARY KEY,
    run_id              TEXT NOT NULL REFERENCES runs(id),
    job_id              TEXT,                              -- 可空：单 Job 维度
    provider_request_id TEXT,                              -- 幂等键（§7.5：Provider 支持幂等键时优先用）
    state               TEXT NOT NULL,                     -- RESERVED/IN_FLIGHT/SUCCEEDED/UNKNOWN/SETTLED
    settle_reason       TEXT,                              -- succeeded/unknown_released/unknown_billed
    worst_micro_usd     INTEGER NOT NULL DEFAULT 0,        -- 预留最坏成本（input+max_output 估算）
    reserved_in_tokens  INTEGER NOT NULL DEFAULT 0,
    reserved_out_tokens INTEGER NOT NULL DEFAULT 0,        -- = max_output（§7.5 调用前按 max_output 预留）
    finalization        INTEGER NOT NULL DEFAULT 0,        -- 1=动用收尾预留额度（§7.5 finalization_reserve）
    actual_in_tokens    INTEGER NOT NULL DEFAULT 0,        -- 结算：真实多维 Token（§31 A4）
    actual_out_tokens   INTEGER NOT NULL DEFAULT 0,
    actual_cache_write  INTEGER NOT NULL DEFAULT 0,
    actual_cache_read   INTEGER NOT NULL DEFAULT 0,
    actual_reasoning    INTEGER NOT NULL DEFAULT 0,
    settled_micro_usd   INTEGER NOT NULL DEFAULT 0,        -- 结算真实成本
    pricing_version     TEXT,                              -- 用的 pricing 表版本（§31 A4；空=估算/未知）
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);
CREATE INDEX idx_budget_res_run ON budget_reservations(run_id, state);
-- 幂等键唯一（仅当非空）：同一 provider_request_id 不重复预留，防崩溃重试重复扣费。
CREATE UNIQUE INDEX idx_budget_res_prid ON budget_reservations(provider_request_id)
    WHERE provider_request_id IS NOT NULL AND provider_request_id != '';

-- +goose Down
DROP INDEX IF EXISTS idx_budget_res_prid;
DROP INDEX IF EXISTS idx_budget_res_run;
DROP TABLE IF EXISTS budget_reservations;
