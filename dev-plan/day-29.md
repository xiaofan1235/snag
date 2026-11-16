# Day 29 —— PlanCompiler（同事务 CAS）+ 战役计划

> 里程碑：M3 | 依赖：Day 28 | 产出：`internal/agent/plan_compiler.go`、campaign_plan
> 对照主文档：§6.4 Plan Compiler、§32.4 战役计划、§31 C/A5

## 0. 今天做什么
PlanCompiler 确定性程序：校验 PlanPatch（Schema/Snapshot 过期/引用存在/依赖成环/预算/深度/去重）→转数据库事务（同事务 CAS）。并引入战役计划。

## 1. 进度位置
第 29 天。这是"模型提案→安全落库"的闸门，拦幻觉计划。

## 2. 模块架构
```
internal/agent/plan_compiler.go   校验 + 落库
campaign_plan 读写（§32.4）
```

## 3. 技术选型
确定性 Go；CAS 条件更新（§6.4）。

## 4. 设计细节
- 12 步校验照 §6.4（JSON Schema/Snapshot Revision/引用存在/Job 类型/Policy Revision/依赖成环/父子权限收窄/预算/指纹去重/最大新增 Job/子 Agent 深度/转事务）。
- 原子性（§6.4）：Snapshot 检查与 Patch 落库**同事务 CAS**（`UPDATE runs SET current_snapshot_revision=+1 WHERE ...`，影响 0 即过期拒绝）。
- patch_id 幂等；过期拒绝置 strategy_dirty（§6.4）。
- 默认上限（§6.4）：单 Patch ≤8 Job、深度 ≤3、3 目标 Worker。
- 战役计划（§32.4）：跨 Epoch 稳定的方向树，Strategist 每 Epoch 精修不重建。

## 5. 分步开发
1. 12 步校验。
2. 同事务 CAS 落库 + patch 幂等。
3. campaign_plan 读写。

## 6. 完成效果（DoD）
- 合法 PlanPatch 落库建 Job；过期 Snapshot 的 Patch 被拒并置 dirty；重复 patch_id 幂等返回。

## 7. 怎么测试
- `tests/plancompiler_test.go`：成环依赖/超深度/超预算→拒；过期 Snapshot→CAS 影响 0；重复 patch 幂等。
- 命令：`go test ./internal/agent/...`
- 对照验收：§6.4；§22.3"after_exit_gate_pass 前并发 PlanPatch"。
