# Day 33 —— Hypothesis + Experiment + Coverage（TESTED 绑定）

> 里程碑：M4 | 依赖：Day 30,32 | 产出：`internal/coverage`、hypotheses、experiments
> 对照主文档：§6.7 ExperimentSpec、§14.1 覆盖率、§31 V2/V5

## 0. 今天做什么
假设（Hypothesis）+ 单变量实验（Experiment）+ 覆盖率矩阵（TESTED 绑定已验证 Experiment + Falsifier 结果）。

## 1. 进度位置
第 33 天。把"自由发挥"变成"可证伪的实验"，并如实记覆盖。

## 2. 模块架构
```
internal/coverage/   matrix/evaluator/freshness
hypotheses/experiments 读写
```

## 3. 技术选型
确定性 Go；覆盖率风险采样（§31 V5，非穷举）。

## 4. 设计细节
- ExperimentSpec 照 §6.7（hypothesis/module/endpoint/identity_refs/baseline/variable/expected/falsifier/stop_condition/required_evidence/tool_allowlist/budget）。
- Executor 不能自改身份上限/对象归属/预算/范围/风险等级（§6.7）。
- Coverage TESTED 要求（§14.1）：真实 Action + Evidence + 参数一致 + 正确身份 + 结果记录 + **绑定已验证 Experiment Job**（cell_id+spec_hash+attack_type+identity+parameter+Falsifier 结果）。
- **只有 Baseline GET 的格子不能标 TESTED**（§14.1）。
- Falsifier 具体可检 + 确定性校验其被真实评估（§31 V2）。
- 覆盖率风险采样（§31 V5）。

## 5. 分步开发
1. Hypothesis CRUD。
2. Experiment 执行（经执行面）。
3. Coverage 矩阵 + TESTED 绑定校验 + Falsifier 检查。

## 6. 完成效果（DoD）
- 一个 IDOR 实验在 Mock Target 上跑：baseline/variant/ownership 证据齐→格子标 TESTED；只有 baseline 的格子标不了。

## 7. 怎么测试
- `tests/coverage_test.go`：只 baseline→不能 TESTED；Falsifier 未评估→不计；风险采样生效。
- 命令：`go test ./internal/coverage/...`
- 对照验收：§23 M4"只有 Baseline GET 的单元格不能标 TESTED"。
