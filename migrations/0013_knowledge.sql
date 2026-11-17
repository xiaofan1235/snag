-- 0013_knowledge.sql —— Day 35 知识库 + Skill（§34/§40/§41）。
--
-- 【迁移不可变约定】新增迁移，不改旧文件。
-- knowledge_docs（0002）只有元数据，无正文与 applies_to；检索需正文 + 分类。
-- skills（0002）只有 name/version/manifest_digest，缺 §40 的 workspace 隔离、applies_to 分类加载、
-- 声明权限（用于「不声明越权则拒」）与正文——故 ALTER 既有 skills 表补齐（不新建，避免表名冲突）。

-- +goose Up
ALTER TABLE knowledge_docs ADD COLUMN content TEXT;
ALTER TABLE knowledge_docs ADD COLUMN applies_to TEXT;  -- 逗号分隔的攻击类型/模块（分类粗筛，§41）

ALTER TABLE skills ADD COLUMN workspace_id TEXT;         -- §40 Workspace 隔离
ALTER TABLE skills ADD COLUMN applies_to TEXT;           -- 逗号分隔，按 applies_to 加载
ALTER TABLE skills ADD COLUMN declared_perms TEXT;       -- 逗号分隔，声明的能力（不得含越权）
ALTER TABLE skills ADD COLUMN content TEXT;
ALTER TABLE skills ADD COLUMN content_sha256 TEXT;

-- +goose Down
-- SQLite 旧版不支持 DROP COLUMN；新增列向后兼容无害，保留。
