# Day 08 —— Job 状态机 + RECONCILING + 取消语义

> 里程碑：M1 | 依赖：Day 07 | 产出：`internal/control/job_sm.*`
> 对照主文档：§5.2 Job 状态、§5.3、§4.1 取消语义、§31 C3

## 0. 今天做什么
Job 状态机（含关键的 `RECONCILING`）+ 具名取消码 + **RECONCILING 对账超时**（§31 C3，防永久卡死连坐全 Run）。

## 1. 进度位置
第 8 天。RECONCILING 是整套"崩溃一致性"的枢纽，别省。

## 2. 模块架构
同 Day 07 的 states.yaml/代码生成，新增 Job 的迁移表；取消码在 domain。

## 3. 技术选型
同 Day 07。超时用 monotonic clock（§31 L5）。

## 4. 设计细节
- 状态照 §5.2：PENDING/BLOCKED/READY/CLAIMED/RUNNING/RECONCILING/WAITING_APPROVAL/SUBMITTED/VERIFYING/DONE/RETRYING/CANCELLED/FAILED。
- `RECONCILING`（§5.2）：有未决 OUTCOME_UNKNOWN 动作时先进它，冻结新 Claim/Permit/依赖 Job；对账完成后 →RETRYING 或携 `cancel_code=OUTCOME_UNRESOLVED` 终态。
- **对账超时**（§31 C3）：每个 RECONCILING 带截止时间，到点自动 OUTCOME_UNRESOLVED 终态 + 记 run_partial_reasons。
- cancel_code 枚举照 §4.1。

## 5. 分步开发
1. Job states.yaml + 生成。
2. 取消：`cancel_code/cancel_actor/cancel_detail` 写入；取消只影响未来动作。
3. RECONCILING 进入/退出逻辑 + 超时字段 + 到期处理。

## 6. 完成效果（DoD）
- 一个带未决 OUTCOME_UNKNOWN 的 Job 进 RECONCILING，冻结新 Claim；对账超时后自动终态，不永久卡。

## 7. 怎么测试
- `tests/job_sm_test.go`：造未决动作→Job 进 RECONCILING→超时→终态 OUTCOME_UNRESOLVED + run_partial_reasons 有记录。
- 属性测试：随机事件不产生未定义转移。
- 命令：`go test ./internal/control/...`
- 对照验收：§5.2、§31 C3、§1②。
