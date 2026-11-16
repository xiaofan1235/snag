# internal/store —— 数据访问层

Store 是**唯一**直接碰 SQLite 的地方（§0.2 单向约束）。上层只调 `*store.Store` 的方法，不碰 `database/sql`。

## 相对开发文档（Day 02）的三处有意偏离

> 原因：当前开发会话 **Bash 网络关闭**，无法 `go get` / 装 `sqlc` / 装 `goose`；且 domain/store 层本就应尽量少外部依赖、在受限环境可构建。偏离均记录在此与对应 commit。

| 文档原方案 | 实际做法 | 为什么 |
|---|---|---|
| `sqlc` 生成查询 | **手写**查询层（store.go / store_jobs.go / store_workspace.go） | 少一个代码生成工具依赖，更可控；生产项目常见做法。SQL 就在代码里，可读可审。 |
| `goose` CLI 跑迁移 | **内置极简迁移器**（migrate.go），迁移文件仍按 **goose 格式** | 无 goose 也能 `MigrateUp()`；装了 goose CLI 时 `goose -dir migrations sqlite3 db up` 同样能用，不锁死。 |
| `modernc.org/sqlite` 直接依赖 | 驱动设为 **build tag 可选接入**（sqlite/driver_modernc.go） | 默认 `go build ./...` 零外部依赖、无网可编译；接入驱动后真连库。 |

## 接入真实数据库

驱动 `modernc.org/sqlite v1.38.2`（纯 Go）已在 `go.mod`（因其要求 go 1.23，模块 go 指令随之定为 1.23）。默认构建不编译它；加 build tag 即真连库：

```bash
go test  -tags sqlite_modernc ./internal/store/...   # 真跑迁移 + 乐观锁 + Job 读写
go build -tags sqlite_modernc ./...                   # 运行时带真驱动
```

不加 tag 时：`go build ./...` / `go test ./...` 全绿，且**不编译 modernc**（驱动文件受 `//go:build sqlite_modernc` 保护）；需要真库的测试**自动 SKIP**（见 `store_test.go` 的 `sqlite.Registered()` 判断）。

> ✅ 已验证（2026-11-16，go 1.23 + modernc v1.38.2）：`go test -tags sqlite_modernc ./internal/store/...` 全 PASS——13 表迁移、乐观锁（旧 revision→ErrConflict）、NotFound、Job 读写均在真实 SQLite 上通过。

## 待硬化（Day 03 补齐表后加外键）

0001_core.sql 中以下列当前是普通 TEXT（指向 Day 03 才创建的表），Day 03 要补 FK：

- `jobs.claimed_by`、`agent_sessions.{attempt_id,seat_id}`
- `actions.{identity_id,approval_id,permit_id,execution_receipt_id}`
- `evidence.proxy_lease_id`
- `hypotheses.module_id`、`coverage_cells.{module_id,endpoint_id}`

## 现有方法

- Workspace：`InsertWorkspace` / `GetWorkspace`
- Run：`InsertRun` / `GetRun` / `UpdateRunState`（乐观锁，冲突返回 `domain.ErrConflict`）
- Job：`InsertJob` / `GetJob`
- 迁移：`Migrate(db)` / `(*Store).MigrateUp()`

更多表的读写随后续开发天补齐。
