# Day 34 —— 序列/竞态实验 + OAST

> 里程碑：M4 | 依赖：Day 33 | 产出：sequence/race experiments、oast
> 对照主文档：§44 业务逻辑/竞态/OAST

## 0. 今天做什么
多步序列实验（步骤间确定性传值）+ 受控竞态实验（显式授权、只豁免串行调度）+ OAST 带外验证（盲类）。

## 1. 进度位置
第 34 天。SRC 最值钱的业务逻辑/竞态/盲洞靠这天。

## 2. 模块架构
```
internal/experiments/sequence.go race.go
internal/oast/   关联生命周期
```

## 3. 技术选型
- 竞态同步：last_byte/batch（§44.2）。
- OAST：自建服务（interactsh 式，用户配置，§44.3）。

## 4. 设计细节
- SequenceExperiment（§44.1）：steps + 结构化提取（JSONPath）+ 断言 + Falsifier；每步独立 Policy/Permit/留证。
- RaceExperiment（§44.2）：risk_class=concurrent_write，默认 deny + 人工审批；仅单端点、有界并发；**只豁免"同端点写串行"，每并发请求仍过 Proxy/Scope/Fencing**；副作用进 RECONCILING。
- OAST（§44.3）：create_correlation→嵌 payload→异步回连→按关联 ID 配回证据；超时=UNABLE_TO_VERIFY；回调内容不可信。

## 5. 分步开发
1. 序列实验（传值+断言）。
2. 竞态实验（审批+有界并发+豁免串行但不豁免执法）。
3. OAST 关联生命周期。

## 6. 完成效果（DoD）
- 序列实验步骤间传值；竞态实验经审批并发、每请求仍过 Proxy；OAST 回调配回证据。

## 7. 怎么测试
- `tests/sequence_race_test.go`：竞态只豁免串行、越权并发仍拒；OAST 超时→UNABLE_TO_VERIFY。
- 命令：`go test ./internal/experiments/... ./internal/oast/...`
- 对照验收：§44.5。
