-- 0002_full.sql —— 补齐 §43.3 权威表清单里 Day 02 之外的全部表（约 55 张）。
-- 对照主文档 §43.3（权威清单）、§4.2（关键表字段契约）、§4.3（数据不变量）。
--
-- 建表严格按**依赖顺序**（被引用表在前），因为迁移在 foreign_keys=ON 的连接上执行，
-- 前向引用的 FK 会在 CREATE 时失败。分组注释沿用 §43.3 分类，仅为可读。
--
-- 设计口径（§43.8：边角表实现期细化，不空想）：§4.2 给出字段契约的表照抄；其余按用途
-- 设计最小且合理的列（id/run_id/关键域字段/created_at/JSON 载荷），显式实现 spec 点名的约束。
-- 身份/许可等 ID 跨表引用，凡 Day 02 留作普通 TEXT 的（identity_id 等）此处保持一致、不加 FK，
-- 以免插入顺序耦合；真正的归属校验在 Store/应用层 + 运行期 foreign_keys（见 store/README「待硬化」）。
--
-- 注：identities 在 §43.3 清单中被漏列，但 §4.2 明确要求且 Day 02 多表引用其 id，故此处建出。

-- +goose Up

-- ========== 【身份 / 攻击面】 ==========

-- identities —— 测试身份注册表（§4.2；§43.3 漏列，据 §4.2 补）。
CREATE TABLE identities (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL REFERENCES runs(id),
    class         TEXT NOT NULL,              -- test_user_a / admin / anon ...
    label         TEXT,
    auth_material_ref TEXT,                   -- 指向密钥保管，不存明文凭据
    metadata_json TEXT,
    created_at    TEXT NOT NULL
);
CREATE INDEX idx_identities_run ON identities(run_id);

-- assets —— 授权范围内发现的资产（主机/域/IP 等）。
CREATE TABLE assets (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL REFERENCES runs(id),
    kind          TEXT NOT NULL,              -- host / domain / ip / service ...
    value         TEXT NOT NULL,
    source        TEXT,
    first_seen_at TEXT NOT NULL,
    metadata_json TEXT,
    UNIQUE(run_id, kind, value)
);
CREATE INDEX idx_assets_run ON assets(run_id);

-- modules —— 功能模块聚类（§4.2：id/run_id/module_key/title/summary）。
CREATE TABLE modules (
    id         TEXT PRIMARY KEY,
    run_id     TEXT NOT NULL REFERENCES runs(id),
    module_key TEXT NOT NULL,
    title      TEXT,
    summary    TEXT,
    created_at TEXT NOT NULL,
    UNIQUE(run_id, module_key)
);

-- discoveries —— 过程中的发现项（§4.2 名列）。
CREATE TABLE discoveries (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL REFERENCES runs(id),
    kind        TEXT NOT NULL,
    ref_type    TEXT,
    ref_id      TEXT,
    detail_json TEXT,
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_discoveries_run ON discoveries(run_id);

-- ========== 【调度执行】 ==========

-- agent_seats —— 稳定席位（§27.6）：Session 的血缘锚点。
CREATE TABLE agent_seats (
    id                   TEXT PRIMARY KEY,
    run_id               TEXT NOT NULL REFERENCES runs(id),
    role                 TEXT NOT NULL,
    seat_key             TEXT NOT NULL,
    state                TEXT NOT NULL,
    current_occupant_generation INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL,
    UNIQUE(run_id, seat_key)
);

-- job_dependencies —— Job 依赖图（§4.2 名列）。
CREATE TABLE job_dependencies (
    job_id            TEXT NOT NULL REFERENCES jobs(id),
    depends_on_job_id TEXT NOT NULL REFERENCES jobs(id),
    kind              TEXT NOT NULL DEFAULT 'finish',
    PRIMARY KEY (job_id, depends_on_job_id)
);

-- approvals —— 人工审批（§4.2 字段契约）。Approve/Deny 条件更新；一个 Approval 只放行一个 Action。
CREATE TABLE approvals (
    id                TEXT PRIMARY KEY,
    run_id            TEXT NOT NULL REFERENCES runs(id),
    action_id         TEXT NOT NULL REFERENCES actions(id),
    action_digest     TEXT NOT NULL,
    policy_revision   INTEGER NOT NULL DEFAULT 0,
    revision          INTEGER NOT NULL DEFAULT 1,   -- 乐观锁
    state             TEXT NOT NULL,                -- PENDING/APPROVED/DENIED/EXPIRED/CONSUMED
    expires_at        TEXT,
    consumed_action_id TEXT,
    resolved_by       TEXT,
    resolved_at       TEXT,
    created_at        TEXT NOT NULL
);
CREATE INDEX idx_approvals_run ON approvals(run_id);

-- action_permits —— 动作级许可（§17 两层 Permit 之上层；§4.3 #4 一次性消费）。
CREATE TABLE action_permits (
    id              TEXT PRIMARY KEY,
    run_id          TEXT NOT NULL REFERENCES runs(id),
    action_id       TEXT NOT NULL REFERENCES actions(id),
    attempt_id      TEXT REFERENCES attempts(id),
    fencing_token   INTEGER NOT NULL DEFAULT 0,
    policy_revision INTEGER NOT NULL DEFAULT 0,
    identity_id     TEXT,                      -- 应用层校验，见 README
    scope_json      TEXT,
    state           TEXT NOT NULL,             -- ISSUED/CONSUMED/REVOKED/EXPIRED
    issued_at       TEXT NOT NULL,
    consumed_at     TEXT,
    revoked_at      TEXT,
    expires_at      TEXT
);
CREATE INDEX idx_action_permits_run ON action_permits(run_id);
CREATE INDEX idx_action_permits_action ON action_permits(action_id);

-- request_capabilities —— 逐请求能力（§4.2 字段契约）。浏览器/多请求动作每个线上请求原子消耗一个。
CREATE TABLE request_capabilities (
    id              TEXT PRIMARY KEY,
    permit_id       TEXT NOT NULL REFERENCES action_permits(id),
    action_id       TEXT NOT NULL REFERENCES actions(id),
    run_id          TEXT NOT NULL REFERENCES runs(id),
    attempt_id      TEXT REFERENCES attempts(id),
    fencing_token   INTEGER NOT NULL DEFAULT 0,
    canonical_url   TEXT,
    identity_id     TEXT,
    policy_revision INTEGER NOT NULL DEFAULT 0,
    request_kind    TEXT,
    max_subrequests INTEGER NOT NULL DEFAULT 1,
    consumed_at     TEXT,
    revoked_at      TEXT
);
CREATE INDEX idx_request_caps_permit ON request_capabilities(permit_id);

-- execution_receipts —— 执行回执（§21）。血缘锚点，Evidence/Finding 据此校验合法执行路径。
CREATE TABLE execution_receipts (
    id              TEXT PRIMARY KEY,
    action_id       TEXT NOT NULL REFERENCES actions(id),
    run_id          TEXT NOT NULL REFERENCES runs(id),
    attempt_id      TEXT REFERENCES attempts(id),
    fencing_token   INTEGER NOT NULL DEFAULT 0,
    identity_id     TEXT,
    policy_revision INTEGER NOT NULL DEFAULT 0,
    proxy_lease_id  TEXT,
    status_code     INTEGER,
    outcome         TEXT,                      -- EXECUTED/OUTCOME_UNKNOWN/...
    request_digest  TEXT,
    response_digest TEXT,
    bytes           INTEGER NOT NULL DEFAULT 0,
    started_at      TEXT,
    completed_at    TEXT
);
CREATE INDEX idx_receipts_action ON execution_receipts(action_id);

-- tool_recovery_attempts —— 工具错误恢复（§4.2 字段契约）。每次恢复生成新 Action，不覆写原失败。
CREATE TABLE tool_recovery_attempts (
    id            TEXT PRIMARY KEY,
    action_id     TEXT NOT NULL REFERENCES actions(id),
    error_class   TEXT NOT NULL,
    strategy      TEXT NOT NULL,              -- retry/alternative/degraded
    new_action_id TEXT REFERENCES actions(id),
    created_at    TEXT NOT NULL
);

-- ========== 【证据验证】 ==========

-- evidence_verifications —— 独立 Verifier 的验证结果（§36；与 evidence 分表，证据内容不可变）。
CREATE TABLE evidence_verifications (
    id            TEXT PRIMARY KEY,
    evidence_id   TEXT NOT NULL REFERENCES evidence(id),
    run_id        TEXT NOT NULL REFERENCES runs(id),
    verifier_kind TEXT NOT NULL,
    verdict       TEXT NOT NULL,              -- CONFIRMED/REJECTED/UNABLE_TO_VERIFY
    model_profile TEXT,
    rationale_json TEXT,
    created_at    TEXT NOT NULL
);
CREATE INDEX idx_ev_verif_evidence ON evidence_verifications(evidence_id);

-- evidence_citations —— 逐字证据引用（§4.2 字段契约）：byte 区间 + 规范化 + 截断标记。
CREATE TABLE evidence_citations (
    id              TEXT PRIMARY KEY,
    finding_id      TEXT NOT NULL REFERENCES findings(id),
    evidence_id     TEXT NOT NULL REFERENCES evidence(id),
    receipt_sequence INTEGER,
    byte_start      INTEGER,
    byte_end        INTEGER,
    content_sha256  TEXT,
    normalization   TEXT,
    truncated       INTEGER NOT NULL DEFAULT 0,
    UNIQUE(finding_id, evidence_id, byte_start, byte_end)
);

-- coverage_revisions —— 覆盖面版本（surface_revision 的演进记录）。
CREATE TABLE coverage_revisions (
    id               TEXT PRIMARY KEY,
    run_id           TEXT NOT NULL REFERENCES runs(id),
    surface_revision INTEGER NOT NULL,
    reason           TEXT,
    created_at       TEXT NOT NULL,
    UNIQUE(run_id, surface_revision)
);

-- quarantine —— 不可信内容隔离（§26 不可信隔离）。
CREATE TABLE quarantine (
    id             TEXT PRIMARY KEY,
    run_id         TEXT NOT NULL REFERENCES runs(id),
    kind           TEXT NOT NULL,
    ref_type       TEXT,
    ref_id         TEXT,
    reason         TEXT,
    payload_digest TEXT,
    created_at     TEXT NOT NULL
);

-- ========== 【规划】 ==========

-- strategy_epochs —— 战略纪元（§4.2：epoch_id/run_id/reason/opened_at/closed_at）。
CREATE TABLE strategy_epochs (
    epoch_id  TEXT PRIMARY KEY,
    run_id    TEXT NOT NULL REFERENCES runs(id),
    reason    TEXT,
    opened_at TEXT NOT NULL,
    closed_at TEXT
);
CREATE INDEX idx_epochs_run ON strategy_epochs(run_id);

-- strategy_snapshots —— 战略快照（§4.2：snapshot_revision/snapshot_json/snapshot_digest）。
CREATE TABLE strategy_snapshots (
    id                TEXT PRIMARY KEY,
    run_id            TEXT NOT NULL REFERENCES runs(id),
    epoch_id          TEXT NOT NULL REFERENCES strategy_epochs(epoch_id),
    snapshot_revision INTEGER NOT NULL,
    snapshot_json     TEXT NOT NULL,
    snapshot_digest   TEXT NOT NULL,
    created_at        TEXT NOT NULL,
    UNIQUE(run_id, snapshot_revision)
);

-- plan_patches —— 计划增量（§4.2：patch_id/epoch_id/snapshot_revision/operations_json/state/applied_at）。
CREATE TABLE plan_patches (
    patch_id          TEXT PRIMARY KEY,
    run_id            TEXT NOT NULL REFERENCES runs(id),
    epoch_id          TEXT NOT NULL REFERENCES strategy_epochs(epoch_id),
    snapshot_revision INTEGER NOT NULL,
    operations_json   TEXT NOT NULL,
    state             TEXT NOT NULL,          -- APPLIED/REJECTED_STALE/REJECTED_INVALID
    applied_at        TEXT
);

-- campaign_plan —— 战役计划（§32）。
CREATE TABLE campaign_plan (
    id         TEXT PRIMARY KEY,
    run_id     TEXT NOT NULL REFERENCES runs(id),
    epoch_id   TEXT REFERENCES strategy_epochs(epoch_id),
    plan_json  TEXT NOT NULL,
    digest     TEXT,
    created_at TEXT NOT NULL
);

-- ========== 【预算资源】 ==========

-- budget_ledger —— 预算账本（§24 RESERVED→SETTLED）。
CREATE TABLE budget_ledger (
    id         TEXT PRIMARY KEY,
    run_id     TEXT NOT NULL REFERENCES runs(id),
    scope      TEXT NOT NULL,                 -- run/job/model...
    kind       TEXT NOT NULL,                 -- RESERVED/SETTLED/RELEASED
    amount     REAL NOT NULL DEFAULT 0,
    unit       TEXT,                          -- tokens/requests/seconds/usd
    ref_type   TEXT,
    ref_id     TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_budget_run ON budget_ledger(run_id);

-- proxy_leases —— 代理租约（§18/§38 出口）。
CREATE TABLE proxy_leases (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL REFERENCES runs(id),
    action_id   TEXT REFERENCES actions(id),
    attempt_id  TEXT REFERENCES attempts(id),
    identity_id TEXT,
    upstream    TEXT,
    egress_mode TEXT,                          -- proxy_pool/local
    state       TEXT NOT NULL,
    issued_at   TEXT NOT NULL,
    released_at TEXT
);
CREATE INDEX idx_proxy_leases_run ON proxy_leases(run_id);

-- sandbox_instances —— 沙箱实例。
CREATE TABLE sandbox_instances (
    id               TEXT PRIMARY KEY,
    run_id           TEXT NOT NULL REFERENCES runs(id),
    kind             TEXT NOT NULL,
    state            TEXT NOT NULL,
    resource_limits_json TEXT,
    created_at       TEXT NOT NULL,
    destroyed_at     TEXT
);

-- resource_leases —— 资源租约（§33 并发治理）。
CREATE TABLE resource_leases (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL REFERENCES runs(id),
    resource_kind TEXT NOT NULL,
    holder_ref    TEXT,
    state         TEXT NOT NULL,
    acquired_at   TEXT NOT NULL,
    released_at   TEXT,
    lease_until   TEXT
);

-- ========== 【模型】 ==========

-- model_calls —— 模型调用账本（§23；§4.3 #14 模型可见输入可重建）。
CREATE TABLE model_calls (
    id                TEXT PRIMARY KEY,
    run_id            TEXT NOT NULL REFERENCES runs(id),
    agent_session_id  TEXT REFERENCES agent_sessions(id),
    attempt_id        TEXT REFERENCES attempts(id),
    provider          TEXT,
    model             TEXT,
    request_digest    TEXT,
    response_digest   TEXT,
    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens      INTEGER NOT NULL DEFAULT 0,
    cost              REAL NOT NULL DEFAULT 0,
    state             TEXT NOT NULL,
    started_at        TEXT,
    completed_at      TEXT
);
CREATE INDEX idx_model_calls_run ON model_calls(run_id);

-- agent_context_revisions —— 上下文修订（§4.2 字段契约）。
CREATE TABLE agent_context_revisions (
    id                TEXT PRIMARY KEY,
    agent_session_id  TEXT NOT NULL REFERENCES agent_sessions(id),
    revision          INTEGER NOT NULL,
    parent_revision   INTEGER NOT NULL DEFAULT 0,
    kind              TEXT NOT NULL,          -- FULL/COMPACTED
    compaction_record_json TEXT,
    token_estimate    INTEGER NOT NULL DEFAULT 0,
    created_at        TEXT NOT NULL,
    UNIQUE(agent_session_id, revision)
);

-- ========== 【命令事件】 ==========

-- commands —— 幂等命令账本（§4.2 字段契约）。不保存原始秘密请求体；同 Key 重试返回原结果。
CREATE TABLE commands (
    actor_id        TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_digest  TEXT,
    state           TEXT NOT NULL,
    resource_refs   TEXT,
    created_at      TEXT NOT NULL,
    PRIMARY KEY (actor_id, idempotency_key)   -- §4.2 UNIQUE(actor_id, idempotency_key)
);

-- operations —— 异步操作（§4.2 字段契约）。
CREATE TABLE operations (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL REFERENCES runs(id),
    kind          TEXT NOT NULL,
    state         TEXT NOT NULL,              -- ACCEPTED/RUNNING/SUCCEEDED/FAILED
    resource_refs TEXT,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

-- user_directives —— 用户指令（§5 用户干预）。
CREATE TABLE user_directives (
    id                  TEXT PRIMARY KEY,
    run_id              TEXT NOT NULL REFERENCES runs(id),
    seat_id             TEXT,
    kind                TEXT NOT NULL,
    content_digest      TEXT,
    external_message_id TEXT,
    created_at          TEXT NOT NULL
);

-- external_messages —— 外部消息入口（§4.2 字段契约）。UNIQUE 保证 qitem 幂等。
CREATE TABLE external_messages (
    id                  TEXT PRIMARY KEY,
    source_kind         TEXT NOT NULL,
    external_message_id TEXT NOT NULL,
    run_id              TEXT REFERENCES runs(id),
    seat_id             TEXT,
    content_type        TEXT,
    payload_digest      TEXT,
    received_at         TEXT NOT NULL,
    directive_id        TEXT,
    UNIQUE(source_kind, external_message_id)
);

-- outbox —— 事件发布 outbox（§4.1：事务提交后由 Publisher 推送）。
CREATE TABLE outbox (
    id           TEXT PRIMARY KEY,
    run_id       TEXT NOT NULL REFERENCES runs(id),
    topic        TEXT NOT NULL,
    payload_json TEXT,
    published    INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL,
    published_at TEXT
);
CREATE INDEX idx_outbox_unpub ON outbox(published, run_id);

-- ========== 【恢复】 ==========

-- recovery_operations —— 恢复操作（§4.2 字段契约）。
CREATE TABLE recovery_operations (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL REFERENCES runs(id),
    seat_id     TEXT,
    kind        TEXT NOT NULL,
    outcome     TEXT NOT NULL,                -- RESUMED_EXACT/REBUILT_FROM_STATE/...（§43.5 恢复结果）
    detail_json TEXT,
    created_at  TEXT NOT NULL
);

-- run_partial_reasons —— Run 部分完成原因（§4.2 字段契约）。
CREATE TABLE run_partial_reasons (
    id                 TEXT PRIMARY KEY,
    run_id             TEXT NOT NULL REFERENCES runs(id),
    code               TEXT NOT NULL,
    entity_ref         TEXT,
    lost_coverage      TEXT,
    recovered_from_ref TEXT,
    created_at         TEXT NOT NULL
);

-- ========== 【报告产物】 ==========

-- reports —— 报告（§4.2：state/manifest_digest/revision）。
CREATE TABLE reports (
    id              TEXT PRIMARY KEY,
    run_id          TEXT NOT NULL REFERENCES runs(id),
    state           TEXT NOT NULL,            -- DRAFT/FINALIZED/STALE
    manifest_digest TEXT,
    revision        INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_reports_run ON reports(run_id);

-- report_revisions —— 报告修订（§4.2：finding_snapshot_json）。
CREATE TABLE report_revisions (
    id                   TEXT PRIMARY KEY,
    report_id            TEXT NOT NULL REFERENCES reports(id),
    run_id               TEXT NOT NULL REFERENCES runs(id),
    revision             INTEGER NOT NULL,
    finding_snapshot_json TEXT,
    manifest_digest      TEXT,
    created_at           TEXT NOT NULL,
    UNIQUE(report_id, revision)
);

-- artifacts —— 派生产物（§4.3 #10：MD/HTML/PDF 都是派生，非权威状态）。
CREATE TABLE artifacts (
    id             TEXT PRIMARY KEY,
    run_id         TEXT NOT NULL REFERENCES runs(id),
    kind           TEXT NOT NULL,
    blob_path      TEXT,
    content_sha256 TEXT,
    content_type   TEXT,
    size_bytes     INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT NOT NULL
);

-- ========== 【记忆】（§4.3 #15：Memory 表不得作 Job/Finding/Coverage 外键源或事实源） ==========

-- memory_sessions —— 记忆会话（§26 记忆隔离，按 Workspace + Scope Digest）。
CREATE TABLE memory_sessions (
    id             TEXT PRIMARY KEY,
    workspace_id   TEXT NOT NULL REFERENCES workspaces(id),
    scope_digest   TEXT NOT NULL,
    kind           TEXT,
    state          TEXT NOT NULL,
    created_at     TEXT NOT NULL,
    last_active_at TEXT NOT NULL
);

-- memory_outbox —— 记忆写出 outbox。
CREATE TABLE memory_outbox (
    id                TEXT PRIMARY KEY,
    memory_session_id TEXT NOT NULL REFERENCES memory_sessions(id),
    op                TEXT NOT NULL,
    payload_digest    TEXT,
    published         INTEGER NOT NULL DEFAULT 0,
    created_at        TEXT NOT NULL
);

-- memory_links —— 记忆关联（只做展示冗余，非权威引用）。
CREATE TABLE memory_links (
    id                TEXT PRIMARY KEY,
    memory_session_id TEXT NOT NULL REFERENCES memory_sessions(id),
    ref_type          TEXT NOT NULL,
    ref_id            TEXT NOT NULL,
    relation          TEXT,
    created_at        TEXT NOT NULL
);

-- memory_gaps —— 记忆缺口（哪些该记而未记/待补）。
CREATE TABLE memory_gaps (
    id                TEXT PRIMARY KEY,
    memory_session_id TEXT NOT NULL REFERENCES memory_sessions(id),
    kind              TEXT NOT NULL,
    detail_json       TEXT,
    created_at        TEXT NOT NULL
);

-- ========== 【知识技能】 ==========

-- knowledge_docs —— 知识文档（§41；trust_tier 见 §43.5 信任级）。
CREATE TABLE knowledge_docs (
    id             TEXT PRIMARY KEY,
    workspace_id   TEXT REFERENCES workspaces(id),
    source         TEXT,
    uri            TEXT,
    title          TEXT,
    content_sha256 TEXT,
    trust_tier     TEXT,                       -- VERIFIED_FACT/OPERATOR_NOTE/UNVERIFIED_REFERENCE
    created_at     TEXT NOT NULL
);

-- knowledge_chunks —— 知识分块（RAG 检索单元）。
CREATE TABLE knowledge_chunks (
    id            TEXT PRIMARY KEY,
    doc_id        TEXT NOT NULL REFERENCES knowledge_docs(id),
    ordinal       INTEGER NOT NULL,
    text          TEXT,
    embedding_ref TEXT,
    token_count   INTEGER NOT NULL DEFAULT 0,
    UNIQUE(doc_id, ordinal)
);

-- knowledge_hits —— 知识命中记录（§34；用于复盘检索有效性）。
CREATE TABLE knowledge_hits (
    id           TEXT PRIMARY KEY,
    run_id       TEXT NOT NULL REFERENCES runs(id),
    chunk_id     TEXT REFERENCES knowledge_chunks(id),
    query_digest TEXT,
    score        REAL,
    used         INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL
);

-- skills —— 技能包（§40）。
CREATE TABLE skills (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    version         TEXT,
    source          TEXT,
    manifest_digest TEXT,
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    UNIQUE(name, version)
);

-- ========== 【工具扩展】 ==========

-- tool_registry —— 工具注册表（§35：支持多语言工具）。
CREATE TABLE tool_registry (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    kind            TEXT,
    version         TEXT,
    language        TEXT,
    manifest_digest TEXT,
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    UNIQUE(name, version)
);

-- mcp_servers —— MCP 服务器（§35：开放接入、受控执行；trust_tier 分级）。
CREATE TABLE mcp_servers (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    url               TEXT,
    transport         TEXT,
    trust_tier        TEXT,
    enabled           INTEGER NOT NULL DEFAULT 1,
    capabilities_json TEXT,
    created_at        TEXT NOT NULL
);

-- ========== 【v2.4 新增：实验 / 关联 / 审计】 ==========

-- sequence_experiments —— 多步序列实验（§44）。
CREATE TABLE sequence_experiments (
    id                   TEXT PRIMARY KEY,
    run_id               TEXT NOT NULL REFERENCES runs(id),
    hypothesis_id        TEXT REFERENCES hypotheses(id),
    assertion            TEXT,
    falsifier            TEXT,
    required_evidence_json TEXT,
    budget_json          TEXT,
    state                TEXT NOT NULL,
    created_at           TEXT NOT NULL
);

-- sequence_steps —— 序列步骤（§44：步骤间结构化提取传值）。
CREATE TABLE sequence_steps (
    id                     TEXT PRIMARY KEY,
    sequence_experiment_id TEXT NOT NULL REFERENCES sequence_experiments(id),
    ordinal                INTEGER NOT NULL,
    identity_id            TEXT,
    request_template       TEXT,
    extract_json           TEXT,
    mutate_json            TEXT,
    created_at             TEXT NOT NULL,
    UNIQUE(sequence_experiment_id, ordinal)
);

-- race_experiments —— 受控竞态实验（§44）。
CREATE TABLE race_experiments (
    id                    TEXT PRIMARY KEY,
    run_id                TEXT NOT NULL REFERENCES runs(id),
    hypothesis_id         TEXT REFERENCES hypotheses(id),
    concurrency           INTEGER NOT NULL DEFAULT 2,
    request_template_json TEXT,
    assertion             TEXT,
    state                 TEXT NOT NULL,
    created_at            TEXT NOT NULL
);

-- oast_correlations —— 带外交互关联（§44 OAST）。
CREATE TABLE oast_correlations (
    id               TEXT PRIMARY KEY,
    run_id           TEXT NOT NULL REFERENCES runs(id),
    action_id        TEXT REFERENCES actions(id),
    token            TEXT NOT NULL,
    interaction_kind TEXT,
    received_at      TEXT,
    source_ip        TEXT,
    detail_json      TEXT,
    UNIQUE(run_id, token)
);

-- identity_auth_flows —— 录制的重认证流程（§45）。
CREATE TABLE identity_auth_flows (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL REFERENCES runs(id),
    identity_id TEXT REFERENCES identities(id),
    flow_json   TEXT,
    recorded_at TEXT,
    state       TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

-- cross_run_finding_links —— 跨 Run Finding 关联（§47；关联而非共享证据）。
CREATE TABLE cross_run_finding_links (
    id                TEXT PRIMARY KEY,
    finding_id        TEXT NOT NULL REFERENCES findings(id),
    run_id            TEXT NOT NULL REFERENCES runs(id),
    linked_finding_id TEXT NOT NULL,
    linked_run_id     TEXT NOT NULL,
    relation          TEXT,
    created_at        TEXT NOT NULL
);

-- audit_chain —— 哈希链审计（§47；篡改可证）。prev_hash 链接前一条。
CREATE TABLE audit_chain (
    run_id     TEXT NOT NULL REFERENCES runs(id),
    sequence   INTEGER NOT NULL,
    prev_hash  TEXT,
    entry_hash TEXT NOT NULL,
    entry_type TEXT NOT NULL,
    entry_ref  TEXT,
    created_at TEXT NOT NULL,
    PRIMARY KEY (run_id, sequence)
);

-- ========== 【攻击面：依赖上面表的子表，放最后】 ==========

-- endpoints —— 端点（依赖 assets）。
CREATE TABLE endpoints (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL REFERENCES runs(id),
    asset_id      TEXT REFERENCES assets(id),
    method        TEXT,
    url_pattern   TEXT,
    path          TEXT,
    discovered_via TEXT,
    first_seen_at TEXT NOT NULL,
    metadata_json TEXT
);
CREATE INDEX idx_endpoints_run ON endpoints(run_id);

-- endpoint_parameters —— 端点参数（依赖 endpoints）。
CREATE TABLE endpoint_parameters (
    id          TEXT PRIMARY KEY,
    endpoint_id TEXT NOT NULL REFERENCES endpoints(id),
    name        TEXT NOT NULL,
    location    TEXT,                          -- query/body/header/path/cookie
    value_type  TEXT,
    required    INTEGER NOT NULL DEFAULT 0,
    metadata_json TEXT,
    UNIQUE(endpoint_id, name, location)
);

-- endpoint_modules —— 端点↔模块归属（§4.2：endpoint_id + module_id + confidence）。
CREATE TABLE endpoint_modules (
    endpoint_id TEXT NOT NULL REFERENCES endpoints(id),
    module_id   TEXT NOT NULL REFERENCES modules(id),
    confidence  REAL NOT NULL DEFAULT 0,
    PRIMARY KEY (endpoint_id, module_id)
);

-- +goose Down
DROP TABLE IF EXISTS endpoint_modules;
DROP TABLE IF EXISTS endpoint_parameters;
DROP TABLE IF EXISTS endpoints;
DROP TABLE IF EXISTS audit_chain;
DROP TABLE IF EXISTS cross_run_finding_links;
DROP TABLE IF EXISTS identity_auth_flows;
DROP TABLE IF EXISTS oast_correlations;
DROP TABLE IF EXISTS race_experiments;
DROP TABLE IF EXISTS sequence_steps;
DROP TABLE IF EXISTS sequence_experiments;
DROP TABLE IF EXISTS mcp_servers;
DROP TABLE IF EXISTS tool_registry;
DROP TABLE IF EXISTS skills;
DROP TABLE IF EXISTS knowledge_hits;
DROP TABLE IF EXISTS knowledge_chunks;
DROP TABLE IF EXISTS knowledge_docs;
DROP TABLE IF EXISTS memory_gaps;
DROP TABLE IF EXISTS memory_links;
DROP TABLE IF EXISTS memory_outbox;
DROP TABLE IF EXISTS memory_sessions;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS report_revisions;
DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS run_partial_reasons;
DROP TABLE IF EXISTS recovery_operations;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS external_messages;
DROP TABLE IF EXISTS user_directives;
DROP TABLE IF EXISTS operations;
DROP TABLE IF EXISTS commands;
DROP TABLE IF EXISTS agent_context_revisions;
DROP TABLE IF EXISTS model_calls;
DROP TABLE IF EXISTS resource_leases;
DROP TABLE IF EXISTS sandbox_instances;
DROP TABLE IF EXISTS proxy_leases;
DROP TABLE IF EXISTS budget_ledger;
DROP TABLE IF EXISTS campaign_plan;
DROP TABLE IF EXISTS plan_patches;
DROP TABLE IF EXISTS strategy_snapshots;
DROP TABLE IF EXISTS strategy_epochs;
DROP TABLE IF EXISTS quarantine;
DROP TABLE IF EXISTS coverage_revisions;
DROP TABLE IF EXISTS evidence_citations;
DROP TABLE IF EXISTS evidence_verifications;
DROP TABLE IF EXISTS tool_recovery_attempts;
DROP TABLE IF EXISTS execution_receipts;
DROP TABLE IF EXISTS request_capabilities;
DROP TABLE IF EXISTS action_permits;
DROP TABLE IF EXISTS approvals;
DROP TABLE IF EXISTS job_dependencies;
DROP TABLE IF EXISTS agent_seats;
DROP TABLE IF EXISTS discoveries;
DROP TABLE IF EXISTS modules;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS identities;
