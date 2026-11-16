-- 0001_core.sql —— Snag 核心聚合表（13 张）。对照主文档 §4.1。
-- goose 格式：装了 goose CLI 可 `goose -dir migrations sqlite3 db up`；
-- 未装 goose 时由 internal/store 的内置迁移器按同格式应用（Day 02 无网适配）。
--
-- 建表顺序已排好，避免「引用尚未创建的表」：被引用方先建。
-- 跨 Day 03 才出现的表（modules/endpoints/identities/agent_seats/approvals/
-- action_permits/execution_receipts/proxy_leases 等）此刻以普通 TEXT 列承载其外键，
-- 留到 Day 03 补齐表后再加 FK 约束（见 internal/store/README.md 的「待硬化」清单）。

-- +goose Up

-- 1) workspaces —— 授权工作目标的历史分组与记忆隔离边界（§4.1）。
CREATE TABLE workspaces (
    id                    TEXT PRIMARY KEY,
    name                  TEXT NOT NULL,
    memory_project_key    TEXT NOT NULL UNIQUE,        -- 服务端生成且唯一，不接受模型指定
    data_classification   TEXT,
    retention_policy_json TEXT,
    created_at            TEXT NOT NULL,
    archived_at           TEXT
);

-- 2) runs —— 一次完整测试任务（§4.1）。revision 乐观锁，单调递增。
CREATE TABLE runs (
    id                        TEXT PRIMARY KEY,
    workspace_id              TEXT NOT NULL REFERENCES workspaces(id),
    name                      TEXT NOT NULL,
    state                     TEXT NOT NULL,
    revision                  INTEGER NOT NULL DEFAULT 1,
    current_policy_revision   INTEGER NOT NULL DEFAULT 0,
    current_strategy_epoch    INTEGER NOT NULL DEFAULT 0,
    current_snapshot_revision INTEGER NOT NULL DEFAULT 0,
    strategy_dirty            INTEGER NOT NULL DEFAULT 0,  -- 布尔 0/1
    authorization_scope_digest TEXT NOT NULL,             -- 创建后不可变
    authorization_reference   TEXT,
    created_by                TEXT,
    created_at                TEXT NOT NULL,
    started_at                TEXT,
    ended_at                  TEXT,
    terminal_reason           TEXT,
    cancel_code               TEXT,
    cancel_actor              TEXT,
    cancel_detail             TEXT,
    last_error                TEXT
);
CREATE INDEX idx_runs_workspace ON runs(workspace_id);

-- 3) policy_revisions —— 策略不可原地修改，每次修改产生新 Revision（§4.1）。
CREATE TABLE policy_revisions (
    id                     TEXT PRIMARY KEY,
    run_id                 TEXT NOT NULL REFERENCES runs(id),
    revision               INTEGER NOT NULL,
    canonical_json         TEXT NOT NULL,
    digest                 TEXT NOT NULL,
    source_document_digest TEXT,
    approved_by            TEXT,
    created_at             TEXT NOT NULL,
    supersedes_revision    INTEGER NOT NULL DEFAULT 0,
    UNIQUE(run_id, revision)
);

-- 4) jobs —— Run 内工作单元（§4.1）。parent_job_id 自引用；claimed_by 指向 agent_sessions，
--    但 agent_sessions 在其后创建，为避免前向引用此处不加 FK（留普通 TEXT）。
CREATE TABLE jobs (
    id                  TEXT PRIMARY KEY,
    run_id              TEXT NOT NULL REFERENCES runs(id),
    parent_job_id       TEXT REFERENCES jobs(id),
    kind                TEXT NOT NULL,
    title               TEXT,
    spec_json           TEXT,
    spec_hash           TEXT,
    policy_revision     INTEGER NOT NULL DEFAULT 0,
    strategy_epoch      INTEGER NOT NULL DEFAULT 0,
    state               TEXT NOT NULL,
    priority            INTEGER NOT NULL DEFAULT 0,
    generation          INTEGER NOT NULL DEFAULT 1,
    revision            INTEGER NOT NULL DEFAULT 1,
    required_capability TEXT,
    claimed_by          TEXT,                 -- -> agent_sessions(id)，Day 03 硬化
    lease_until         TEXT,
    budget_json         TEXT,
    cancel_code         TEXT,
    cancel_actor        TEXT,
    cancel_detail       TEXT,
    cancel_requested_at TEXT,
    cancel_settled_at   TEXT,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);
CREATE INDEX idx_jobs_run_state ON jobs(run_id, state);

-- 5) agent_sessions —— 逻辑执行上下文（§4.1）。attempt_id 指向其后创建的 attempts，留普通 TEXT。
CREATE TABLE agent_sessions (
    id                         TEXT PRIMARY KEY,
    run_id                     TEXT NOT NULL REFERENCES runs(id),
    role                       TEXT NOT NULL,
    job_id                     TEXT REFERENCES jobs(id),
    attempt_id                 TEXT,            -- -> attempts(id)，Day 03 硬化
    seat_id                    TEXT,            -- -> agent_seats(id)，Day 03 硬化
    occupant_generation        INTEGER NOT NULL DEFAULT 0,
    model_profile              TEXT,
    state                      TEXT NOT NULL,
    generation                 INTEGER NOT NULL DEFAULT 0,
    context_revision           INTEGER NOT NULL DEFAULT 0,
    provider_session_reference TEXT,
    created_at                 TEXT NOT NULL,
    last_active_at             TEXT NOT NULL
);
CREATE INDEX idx_sessions_run ON agent_sessions(run_id);

-- 6) attempts —— 对某 Job 的一次执行尝试（§4.1）。UNIQUE(job_id, generation)。
CREATE TABLE attempts (
    id               TEXT PRIMARY KEY,
    job_id           TEXT NOT NULL REFERENCES jobs(id),
    generation       INTEGER NOT NULL,
    agent_session_id TEXT REFERENCES agent_sessions(id),
    claim_token_hash TEXT,
    fencing_token    INTEGER NOT NULL DEFAULT 0,
    state            TEXT NOT NULL,
    progress_json    TEXT,
    result_json      TEXT,
    started_at       TEXT NOT NULL,
    ended_at         TEXT,
    failure_class    TEXT,
    failure_message  TEXT,
    UNIQUE(job_id, generation)
);

-- 7) actions —— 一次具体动作（§4.1）。identity_id/approval_id/permit_id/execution_receipt_id
--    指向 Day 03 的表，此刻为普通 TEXT。
CREATE TABLE actions (
    id                  TEXT PRIMARY KEY,
    run_id              TEXT NOT NULL REFERENCES runs(id),
    job_id              TEXT NOT NULL REFERENCES jobs(id),
    attempt_id          TEXT REFERENCES attempts(id),
    tool_name           TEXT NOT NULL,
    action_json         TEXT,
    canonical_digest    TEXT,
    risk_class          TEXT,
    idempotency_class   TEXT,
    state               TEXT NOT NULL,
    policy_revision     INTEGER NOT NULL DEFAULT 0,
    identity_id         TEXT NOT NULL,          -- -> identities(id)，Day 03 硬化
    approval_id         TEXT,
    permit_id           TEXT,
    permit_expires_at   TEXT,
    dispatched_at       TEXT,
    completed_at        TEXT,
    execution_receipt_id TEXT,
    failure_class       TEXT
);
CREATE INDEX idx_actions_run ON actions(run_id);
CREATE INDEX idx_actions_job ON actions(job_id);

-- 8) evidence —— 证据，内容不可修改（§4.1）。血缘三元组 identity_id/policy_revision/proxy_lease_id。
--    复合 UNIQUE(run_id, id) 供 finding_evidence 的复合外键引用，从 schema 禁止跨 Run 引用。
CREATE TABLE evidence (
    id                 TEXT PRIMARY KEY,
    run_id             TEXT NOT NULL REFERENCES runs(id),
    action_id          TEXT REFERENCES actions(id),
    kind               TEXT NOT NULL,
    identity_id        TEXT,
    policy_revision    INTEGER NOT NULL DEFAULT 0,
    proxy_lease_id     TEXT,
    plain_sha256       TEXT NOT NULL,
    blob_path          TEXT,
    redacted_blob_path TEXT,
    content_type       TEXT,
    size_bytes         INTEGER NOT NULL DEFAULT 0,
    captured_at        TEXT NOT NULL,
    capture_source     TEXT,
    metadata_json      TEXT,
    UNIQUE(run_id, id)
);
CREATE INDEX idx_evidence_run ON evidence(run_id);
CREATE INDEX idx_evidence_action ON evidence(action_id);

-- 9) hypotheses —— 可证伪假设（§4.1）。module_id 指向 Day 03 的 modules，暂普通 TEXT。
CREATE TABLE hypotheses (
    id                  TEXT PRIMARY KEY,
    run_id              TEXT NOT NULL REFERENCES runs(id),
    module_id           TEXT,                   -- -> modules(id)，Day 03 硬化
    statement           TEXT NOT NULL,
    trust_boundary      TEXT,
    source_type         TEXT,
    source_evidence_ids TEXT,                   -- JSON 数组（展示冗余，权威引用经 Link 表）
    state               TEXT NOT NULL,
    expected_impact     TEXT,
    falsifier           TEXT,
    minimum_experiment  TEXT,
    stop_condition      TEXT,
    created_by_agent    TEXT,
    revision            INTEGER NOT NULL DEFAULT 1,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);
CREATE INDEX idx_hypotheses_run ON hypotheses(run_id);

-- 10) findings —— 从 Hypothesis 晋级的漏洞发现（§4.1）。
--     复合 UNIQUE(run_id, id) 供 finding_evidence 复合外键引用。
CREATE TABLE findings (
    id                   TEXT PRIMARY KEY,
    run_id               TEXT NOT NULL REFERENCES runs(id),
    hypothesis_id        TEXT REFERENCES hypotheses(id),
    title                TEXT,
    vulnerability_type   TEXT,
    severity             TEXT,
    state                TEXT NOT NULL,
    affected_assets_json TEXT,
    impact_json          TEXT,
    triage_result_json   TEXT,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL,
    UNIQUE(run_id, id)
);
CREATE INDEX idx_findings_run ON findings(run_id);

-- 11) finding_evidence —— Finding↔Evidence 链接（§4.1）。
--     复合外键 (run_id, evidence_id)->evidence(run_id,id)、(run_id, finding_id)->findings(run_id,id)，
--     从 schema 层禁止跨 Run 引用证据。
CREATE TABLE finding_evidence (
    run_id      TEXT NOT NULL,
    finding_id  TEXT NOT NULL,
    evidence_id TEXT NOT NULL,
    role        TEXT NOT NULL,
    PRIMARY KEY (finding_id, evidence_id, role),
    FOREIGN KEY (run_id, finding_id)  REFERENCES findings(run_id, id),
    FOREIGN KEY (run_id, evidence_id) REFERENCES evidence(run_id, id)
);

-- 12) coverage_cells —— 覆盖率「接口×参数×身份×攻击类型」四维（§4.1）。
CREATE TABLE coverage_cells (
    run_id           TEXT NOT NULL REFERENCES runs(id),
    module_id        TEXT NOT NULL,             -- -> modules(id)，Day 03 硬化
    endpoint_id      TEXT NOT NULL,             -- -> endpoints(id)，Day 03 硬化
    parameter_name   TEXT NOT NULL,
    identity_class   TEXT NOT NULL,
    attack_type      TEXT NOT NULL,
    state            TEXT NOT NULL,
    rationale        TEXT,
    evidence_ids     TEXT,                      -- JSON 数组（展示冗余）
    surface_revision INTEGER NOT NULL DEFAULT 0,
    updated_at       TEXT NOT NULL,
    PRIMARY KEY (run_id, module_id, endpoint_id, parameter_name, identity_class, attack_type)
);

-- 13) events —— Run 内事件日志（§4.1）。UNIQUE(run_id, sequence)，提交后由 Outbox 发布。
CREATE TABLE events (
    run_id         TEXT NOT NULL REFERENCES runs(id),
    sequence       INTEGER NOT NULL,
    event_type     TEXT NOT NULL,
    entity_type    TEXT,
    entity_id      TEXT,
    entity_version INTEGER NOT NULL DEFAULT 0,
    payload_json   TEXT,
    created_at     TEXT NOT NULL,
    PRIMARY KEY (run_id, sequence)
);

-- +goose Down
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS coverage_cells;
DROP TABLE IF EXISTS finding_evidence;
DROP TABLE IF EXISTS findings;
DROP TABLE IF EXISTS hypotheses;
DROP TABLE IF EXISTS evidence;
DROP TABLE IF EXISTS actions;
DROP TABLE IF EXISTS attempts;
DROP TABLE IF EXISTS agent_sessions;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS policy_revisions;
DROP TABLE IF EXISTS runs;
DROP TABLE IF EXISTS workspaces;
