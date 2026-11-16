# Day 25 —— Executor Loop（StepContext 冻结 + 效果幂等）

> 里程碑：M3 | 依赖：Day 23,24 | 产出：`internal/agent/executor.go runtime.go`
> 对照主文档：§7.1 Agent Loop、§7.2 持久化、§31 A?（StepContext）

## 0. 今天做什么
Executor 快循环：每步先持久化 StepContext（请求级不可变快照），模型响应效果**幂等提交**（model_step_id + ordinal），崩溃后继续提交不重生成。

## 1. 进度位置
第 25 天。这是"子 Agent"的执行体，是 §32 编排模型里的快闭环。

## 2. 模块架构
```
internal/agent/
  executor.go   Generate→commit→dispatch 循环
  runtime.go    Session 管理
```

## 3. 技术选型
- 每步从持久 Step 继续（§7.1）；ctx 贯穿取消（§46）。

## 4. 设计细节
- 循环（§7.1）：LoadAttemptSnapshot→Build context→Generate→CommitModelResponse→按 Kind 分发（ActionProposal/SpawnJob/Observation/CompletionClaim/Yield）。
- StepContext 冻结（§7.1）：model_step_id/attempt_generation/fencing_token/context_revision/policy_revision/tool_catalog_digest/...；请求开始后不可变。
- **效果幂等提交**（§7.2）：CommitModelResponse 先持久化响应，created_action_ids/effects_committed 同事务；每效果 model_step_id+ordinal 幂等键，崩溃后继续提交剩余，不重新调模型。
- 流式期间工具目录/Policy/Identity 变化 → 旧响应整体失效重规划（§7.1）。
- Model-visible ⟺ logged（§7.2）：rendered_request_ref 保存实际请求，可重建且 digest 一致。

## 5. 分步开发
1. StepContext 持久化。
2. 循环主体 + CommitModelResponse 幂等。
3. 崩溃恢复：按 effects_committed 续提交。

## 6. 完成效果（DoD）
- 用 Mock Provider 跑一个 Job：提案→（经执行面）→观察→下一步；CommitModelResponse 后注入崩溃→恢复续提交不重复。

## 7. 怎么测试
- `tests/executor_test.go`：效果幂等（崩溃后不重复 Action）；流式期间 Policy 变→旧响应失效。
- 命令：`go test ./internal/agent/...`
- 对照验收：§7.2；§22.3"after_model_response_commit 崩溃"。
