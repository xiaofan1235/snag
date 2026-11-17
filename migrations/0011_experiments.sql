-- 0011_experiments.sql —— Day 33 Experiment（绑定 Coverage TESTED，§6.7/§14.1）。
--
-- 【迁移不可变约定】新增迁移，不改旧文件。
-- 无 experiments 表。§6.7 ExperimentSpec 要持久化（含 spec_hash 做 Coverage TESTED 绑定），§14.1 要求
-- TESTED 绑定「已验证 Experiment Job + Falsifier 结果」，§31 V2 要求 Falsifier 具体可检且确定性校验其
-- 被真实评估——故新增 experiments，falsifier NOT NULL、falsifier_result 记 held/refuted/not_evaluated，
-- 只有 VERIFIED 且 falsifier_result≠not_evaluated 的实验才能让格子标 TESTED。

-- +goose Up
CREATE TABLE experiments (
    id                   TEXT PRIMARY KEY,
    run_id               TEXT NOT NULL REFERENCES runs(id),
    hypothesis_id        TEXT,
    module_id            TEXT,
    endpoint_id          TEXT,
    spec_hash            TEXT NOT NULL,        -- ExperimentSpec 的 JCS 摘要（TESTED 绑定键之一）
    attack_type          TEXT NOT NULL,
    identity_class       TEXT,
    parameter_name       TEXT,
    variable_json        TEXT,                 -- {location,name,from,to}（单变量，§6.7）
    expected_observation TEXT,
    falsifier            TEXT NOT NULL,        -- 结构化 Falsifier（§31 V2，不接受空洞文本）
    required_evidence    TEXT,                 -- JSON 数组：baseline/variant/ownership
    collected_evidence   TEXT,                 -- JSON 数组：实际已采集的证据类型
    state                TEXT NOT NULL,        -- PROPOSED/RUNNING/VERIFIED/FAILED
    falsifier_result     TEXT NOT NULL DEFAULT 'not_evaluated', -- held/refuted/not_evaluated
    termination_reason   TEXT,
    -- §6.7 Executor 不可自改的锁定字段（创建后冻结）：身份上限/对象归属/预算/范围/风险等级
    locked_identity_cap  INTEGER NOT NULL DEFAULT 0,
    locked_budget_reqs   INTEGER NOT NULL DEFAULT 0,
    locked_risk_level    TEXT,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);
CREATE INDEX idx_experiments_run ON experiments(run_id, state);

-- +goose Down
DROP INDEX IF EXISTS idx_experiments_run;
DROP TABLE IF EXISTS experiments;
