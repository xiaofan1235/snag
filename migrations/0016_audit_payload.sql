-- 0016_audit_payload.sql —— Day 46 篡改可证审计哈希链（§47.2）。
--
-- 【迁移不可变约定】只 ALTER/新增，不改旧文件。
-- audit_chain 在 0002_full 已建骨架（run_id/sequence/prev_hash/entry_hash/entry_type/entry_ref），
-- 但缺「关键字段的规范化 payload」——而 §47.2 的 entry_hash = SHA256(JCS(关键字段) || prev_hash)，
-- 要让第三方**离线重算验链**，导出的链必须带关键字段本身。故补 payload_json（已 JCS 规范化的关键字段）。
-- SQLite 旧版不支持 DROP COLUMN，Down 不回删（与既有约定一致）。

-- +goose Up
ALTER TABLE audit_chain ADD COLUMN payload_json TEXT;

-- +goose Down
-- ALTER 新增列不回删（SQLite 旧版不支持 DROP COLUMN；迁移不可变约定）。
