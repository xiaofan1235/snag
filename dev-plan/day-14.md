# Day 14 —— Action 状态机 + ActionProposal

> 里程碑：M2 执行与证据核心 | 依赖：Day 04 | 产出：`internal/execution/action_sm.*`
> 对照主文档：§5.3 Action 状态、§1.1 核心实体 actions、§4.1 约束

## 0. 今天做什么
Action 状态机（PROPOSED→...→RECORDED / OUTCOME_UNKNOWN）+ ActionProposal 结构 + `idempotency_class`（决定对账方式）。

## 1. 进度位置
第 14 天，M2 开端。Action 是"真正影响外部的动作"的载体，后面 Permit/Proxy/Evidence 都绕它。

## 2. 模块架构
```
internal/execution/action_sm.*   Action 迁移表（同 Day07 代码生成套路）
internal/domain/action.go        ActionProposal / 字段（Day01 已建，今天补方法）
```

## 3. 技术选型
同 Day 07 代码生成；digest 用 Day 04 的 canon。

## 4. 设计细节
- 状态照 §5.3：PROPOSED/POLICY_DENIED/WAITING_APPROVAL/AUTHORIZED/DISPATCHED/EXECUTED/RECORDED/OUTCOME_UNKNOWN/CANCELLED。
- `canonical_digest` = canon.Digest(action_json)（红线/审批/去重的锚）。
- `idempotency_class`（§4.1）：非幂等写未对账前禁止同 Job 增代重发；幂等重试沿用目标侧 key。
- **最关键恢复规则**（§5.3）：DISPATCHED 后崩溃 → OUTCOME_UNKNOWN，不自动失败/重试/成功。

## 5. 分步开发
1. Action states.yaml + 生成。
2. ActionProposal 结构（tool_name/action_json/risk_class/identity_id/idempotency_class）。
3. RECORDED 前置条件先留空接口（Day 21 填）。

## 6. 完成效果（DoD）
- Action 能从 PROPOSED 走到 AUTHORIZED；DISPATCHED 后注入崩溃 → OUTCOME_UNKNOWN。

## 7. 怎么测试
- `tests/action_sm_test.go`：逐迁移；DISPATCHED+崩溃→OUTCOME_UNKNOWN；非幂等写未对账不许增代重发。
- 命令：`go test ./internal/execution/...`
- 对照验收：§5.3。
