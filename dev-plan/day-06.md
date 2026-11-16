# Day 06 —— 单写者 Store（durable/bulk 双连接）

> 里程碑：M1 持久化控制核心 | 依赖：Day 02 | 产出：`internal/store/writer.go`
> 对照主文档：§31 C1（每次提交持久性分级）、§31 C2（单写者）、§20.3

## 0. 今天做什么
把 Store 的**写路径**做成单写者串行 + 两个连接：`durable`（synchronous=FULL，承载 Permit/DISPATCHED/Receipt/Evidence 提交）和 `bulk`（synchronous=NORMAL，承载事件/度量，group-commit）。

## 1. 进度位置
第 6 天。这是"崩了不丢副作用一致性"的地基，必须现在做对（§31 C1 是 P0 坑）。

## 2. 模块架构
```
internal/store/
  writer.go   单写 goroutine + channel 投递；durable/bulk 两连接
  reader.go   只读连接池（WAL 允许并发读）
```

## 3. 技术选型
- WAL；PRAGMA：`busy_timeout=5000`、`foreign_keys=ON`、`mmap_size`、`temp_store=MEMORY`；durable 连接 `synchronous=FULL`，bulk 连接 `synchronous=NORMAL`。
- bulk writer 用微批（5–20ms 窗口合并提交）降低 fsync 次数。

## 4. 设计细节
- **凡是"提交先于外部副作用"的记录走 durable 连接**（Permit 消费+DISPATCHED、Receipt 终结、Evidence 元数据、Approval 消费、Policy 切换）。
- 事件/Outbox/度量走 bulk。
- 写路径单连接串行化（或 `SetMaxOpenConns(1)` 各一）；读走独立池。
- 后台定时 `wal_checkpoint(TRUNCATE)`。

## 5. 分步开发
1. `writer.go`：开两个 *sql.DB（不同 PRAGMA），各 MaxOpenConns(1)；一个写命令 channel。
2. `Tx(durable, fn)` / `Tx(bulk, fn)` 封装；bulk 加 group-commit。
3. reader 池：只读连接。
4. 把 Day 02/03 的写查询归类到 durable/bulk。

## 6. 完成效果（DoD）
- 3 个并发"Worker"写不同 Run 不报 SQLITE_BUSY（串行化生效）。
- durable 提交返回即已 fsync（断电测试点，Day 44 验）。

## 7. 怎么测试
- `tests/writer_concurrency_test.go`：3 goroutine 各写 1000 条事件（bulk）+ 偶发 durable 提交，无 BUSY、数据完整。
- 命令：`go test -race ./internal/store/...`
- 对照验收：§31 C1/C2；§30.2 #1 待验证项。
