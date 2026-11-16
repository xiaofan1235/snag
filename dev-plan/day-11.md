# Day 11 —— commands/operations 幂等账本

> 里程碑：M1 | 依赖：Day 10 | 产出：`internal/control/commands.go`
> 对照主文档：§15.7、§4.2 commands/operations、§16.33

## 0. 今天做什么
幂等命令账本：同一 `Idempotency-Key` 重试返回原 Job/Approval/Operation，不创建第二份；异步命令返回 `202 + operation_id`。

## 1. 进度位置
第 11 天。这是"网络超时重试不产生重复对象"的保证，CLI/UI 都依赖它。

## 2. 模块架构
```
internal/control/commands.go   命令受理 + 幂等查重 + operation 跟踪
```

## 3. 技术选型
- `commands` 表 `UNIQUE(actor_id, idempotency_key)`（§4.2）。
- 不保存原始秘密请求体（§4.2），只存 request_digest。

## 4. 设计细节
- 收命令：先查 (actor_id, idempotency_key)；命中则返回原结果（原 Job/Approval/Operation ref）。
- 基于既有对象变更支持 `expected_revision`（§15.7），冲突返回稳定错误。
- 异步命令 → 建 operation（ACCEPTED/RUNNING/SUCCEEDED/FAILED），返回 operation_id。

## 5. 分步开发
1. `Receive(actorID, idemKey, reqDigest, fn)`：查重→执行→记账，同事务。
2. operations 表 CRUD + `GET /v1/operations/{id}` 查询。
3. `expected_revision` 冲突处理。

## 6. 完成效果（DoD）
- 同 key 重试返回同一对象（不新建）。
- 异步命令返回 operation_id，可查终态。

## 7. 怎么测试
- `tests/idempotency_test.go`：同 key 调两次→同一 Job id；并发同 key→只建一个；expected_revision 冲突→稳定错误码。
- 命令：`go test -race ./internal/control/...`
- 对照验收：§15.7；§23 M1"同 Idempotency-Key 重试不产生第二份对象"。
