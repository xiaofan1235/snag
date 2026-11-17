-- 0008_context_blocks.sql —— Day 27 两阶段上下文压缩：每个 Context Revision 的块内容持久化（§7.6）。
--
-- 【迁移不可变约定】新增迁移，不改旧文件。
-- agent_context_revisions（0002）已存 Revision 链（FULL/COMPACTED）+ compaction_record_json，但没有
-- 每修订的「块」内容。压缩需要：读 parent 修订的块 → 阶段一确定性丢弃 → 阶段二 LLM 摘要 → 写入**新**
-- COMPACTED 修订的块（原修订块一行不改，§7.6 不变量）。故新增 agent_context_blocks 承载块内容；
-- evidence_refs 保证「旧 Evidence 引用压缩后仍可解析」。

-- +goose Up
CREATE TABLE agent_context_blocks (
    id                  TEXT PRIMARY KEY,
    context_revision_id TEXT NOT NULL REFERENCES agent_context_revisions(id),
    ordinal             INTEGER NOT NULL,
    kind                TEXT NOT NULL,   -- event/job_detail/tool_result/question/hypothesis/baseline_diff/identity_ref/secret_ref/failure_class/evidence_ref/summary
    source              TEXT,
    content             TEXT,
    tokens              INTEGER NOT NULL DEFAULT 0,
    aged                INTEGER NOT NULL DEFAULT 0,  -- 老化标记：阶段一确定性丢弃的输入（可丢弃类 && 已老化才丢）
    evidence_refs       TEXT,            -- JSON 数组：本块关联的 Evidence ID（压缩后仍可解析）
    created_at          TEXT NOT NULL
);
CREATE INDEX idx_ctx_blocks_rev ON agent_context_blocks(context_revision_id, ordinal);

-- +goose Down
DROP INDEX IF EXISTS idx_ctx_blocks_rev;
DROP TABLE IF EXISTS agent_context_blocks;
