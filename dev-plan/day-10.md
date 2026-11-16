# Day 10 —— Event Journal + Outbox（提交后发布）

> 里程碑：M1 | 依赖：Day 06 | 产出：`internal/events/`、`internal/outbox/`
> 对照主文档：§4.2 events、§0.5 崩溃一致性、§31 C1、§53.2（Outbox 印证）

## 0. 今天做什么
事件日志（全局单调 sequence）+ Outbox Publisher：**事件在事务提交后才发布**，决不在 DB 提交前发给 UI。

## 1. 进度位置
第 10 天。这是 SSE、审计、对账的事实来源。

## 2. 模块架构
```
internal/events/  写事件（事务内，bulk 连接）、全局 sequence
internal/outbox/  publisher：提交后扫 outbox 表、推 SSE、标记已发
```

## 3. 技术选型
- 事件写 bulk 连接 + group-commit（Day 06）。
- Outbox 模式：命令在事务内 enqueue，提交后由 publisher 执行（§53.2 的"先存后发"）。

## 4. 设计细节
- `events` `UNIQUE(run_id, sequence)`；全局 `global_sequence`（§15.6）。
- `entity_version` 直接用聚合的 revision（§4.3 #11）。
- 事件**事务提交后**由 Outbox Publisher 推送（§4.2，不能提交前发）。
- Outbox 重试指数退避；失败不回滚领域事务（§26.9 原则）。

## 5. 分步开发
1. `events.Append(tx, evt)`：事务内写，带 run sequence + global sequence。
2. `outbox` 表 enqueue（事务内）。
3. `publisher`：轮询 due、推送（先推到内存 bus，Day 40 接 SSE）、标记、退避重试。

## 6. 完成效果（DoD）
- 写一个状态变更，事件在**提交后**出现在 bus；提交失败则无事件。
- 事件 sequence 无空洞、单调。

## 7. 怎么测试
- `tests/outbox_test.go`：事务回滚 → 无事件；提交 → 有事件且 sequence 连续；publisher 失败重试不丢。
- 命令：`go test ./internal/events/... ./internal/outbox/...`
- 对照验收：§0.5"数据库提交后才发布 SSE"。
