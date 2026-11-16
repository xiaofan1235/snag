# Day 02 —— 数据模型 Part1 + Store 骨架

> 里程碑：M0 | 依赖：Day 01 | 产出：`migrations/0001_*.sql`、`internal/store/`
> 对照主文档：§4.1、§43.3 主数据模型整合、§2.1 技术栈

## 0. 今天做什么
建 SQLite 库与**核心聚合表**（workspaces/runs/policy_revisions/agent_sessions/jobs/attempts/actions/evidence/hypotheses/findings/finding_evidence/coverage_cells/events），接上 sqlc + goose。

## 1. 进度位置
第 2 天。Day 03 补齐其余表。做完后：能 migrate 出库、sqlc 生成类型安全的查询。

## 2. 模块架构
```
migrations/        goose 迁移文件（0001 起）
internal/store/    sqlc 生成的查询 + Store 封装
  sqlite/          driver、连接管理
```
Store 是**唯一碰 SQLite 的地方**；上层只调 Store 接口（§0.2 单向约束）。

## 3. 技术选型
- SQLite **WAL**；驱动先用纯 Go `modernc.org/sqlite`（§2.1；Day 06 再并测 CGO）。
- **sqlc** 生成查询、**goose** 迁移（§2.1）。
- 迁移里开 `PRAGMA foreign_keys=ON`。

## 4. 设计细节
- 表字段照 §4.1；**复合外键** `finding_evidence(run_id, evidence_id) -> evidence(run_id, id)`（§4.1，禁跨 Run 引用）。
- 所有"可变聚合"带 `revision` 列（§4.3 不变量 #11）。
- `events` 表 `UNIQUE(run_id, sequence)`（§4.2）。
- 先不建 §43.3 的全部表，只建核心聚合（13 张）。

## 5. 分步开发
1. 写 `0001_core.sql`：上面 13 张表的 CREATE，带主键/外键/唯一约束/revision 列。
2. 配 `sqlc.yaml` 指向 migrations + queries 目录。
3. 写几条基础 query（runs: insert/get/update-by-revision；jobs: insert/get）。
4. `sqlc generate`；`goose up` 建库。

## 6. 完成效果（DoD）
- `goose up` 建出库，13 张表存在。
- sqlc 生成的 Go 代码编译过。
- 能插入一个 Run、按 `WHERE id=? AND revision=?` 条件更新（影响 0 行即冲突）。

## 7. 怎么测试
- `tests/store_core_test.go`：建临时库→插 Run→条件更新成功→用旧 revision 再更新影响 0 行（验证乐观锁）。
- 命令：`goose -dir migrations sqlite3 /tmp/t.db up && go test ./internal/store/...`
- 对照验收：§23 M0"SQL Migration"。
