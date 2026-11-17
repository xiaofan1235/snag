-- 0007_model_steps.sql —— Day 25 Executor Loop：StepContext 冻结 + 效果幂等提交（§7.1/§7.2）。
--
-- 【迁移不可变约定】新增迁移，不改旧文件。
-- 既有 model_calls 记的是「一次模型调用的计量」，不含 §7.1 的 StepContext 请求级冻结字段，也不含
-- §7.2 的效果幂等账本（model_step_id + ordinal）。故新增：
--   model_steps        —— 每步的冻结快照 + 响应持久化（Model-visible⟺logged，可重建且 digest 一致）
--   model_step_effects —— 效果幂等账本：(model_step_id, ordinal) 为幂等键，崩溃后续提交不重复

-- +goose Up
CREATE TABLE model_steps (
    id                       TEXT PRIMARY KEY,                -- model_step_id
    run_id                   TEXT NOT NULL REFERENCES runs(id),
    attempt_id               TEXT,
    agent_session_id         TEXT,
    -- StepContext 冻结（§7.1，Generate 前持久化，请求期间不可变）
    attempt_generation       INTEGER NOT NULL DEFAULT 0,
    fencing_token            INTEGER NOT NULL DEFAULT 0,
    context_revision         INTEGER NOT NULL DEFAULT 0,
    policy_revision          INTEGER NOT NULL DEFAULT 0,
    tool_catalog_digest      TEXT,
    model_profile_digest     TEXT,
    provider_session_version INTEGER NOT NULL DEFAULT 0,
    identity_refs            TEXT,
    sandbox_profile          TEXT,
    sampled_through_sequence INTEGER NOT NULL DEFAULT 0,
    -- 响应持久化（§7.2，每次模型响应后至少保存）
    rendered_request_ref     TEXT,                            -- 实际请求（Redacted 规范化形式）引用
    request_digest           TEXT,                            -- 重建请求须与此一致
    response_stop_reason     TEXT,
    structured_output        TEXT,
    usage_json               TEXT,
    public_progress_summary  TEXT,
    provider_request_id      TEXT,
    response_kind            TEXT,                            -- ActionProposal/SpawnJob/Observation/CompletionClaim/Yield
    effects_total            INTEGER NOT NULL DEFAULT 0,
    effects_committed        INTEGER NOT NULL DEFAULT 0,
    state                    TEXT NOT NULL,                   -- STEP_STARTED/RESPONSE_COMMITTED/EFFECTS_COMMITTED/STALE
    created_at               TEXT NOT NULL,
    updated_at               TEXT NOT NULL
);
CREATE INDEX idx_model_steps_attempt ON model_steps(attempt_id, state);

CREATE TABLE model_step_effects (
    model_step_id TEXT NOT NULL REFERENCES model_steps(id),
    ordinal       INTEGER NOT NULL,                           -- (model_step_id, ordinal) = 幂等键（§7.2）
    kind          TEXT NOT NULL,
    payload       TEXT,
    committed     INTEGER NOT NULL DEFAULT 0,
    ref_id        TEXT,                                       -- 效果落地产生的实体 id（如 action id）
    created_at    TEXT NOT NULL,
    committed_at  TEXT,
    PRIMARY KEY (model_step_id, ordinal)
);

-- +goose Down
DROP TABLE IF EXISTS model_step_effects;
DROP INDEX IF EXISTS idx_model_steps_attempt;
DROP TABLE IF EXISTS model_steps;
