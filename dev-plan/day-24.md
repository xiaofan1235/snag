# Day 24 —— 模型预算账本（RESERVED→SETTLED）

> 里程碑：M3 | 依赖：Day 23 | 产出：`internal/agent/budget.go`
> 对照主文档：§7.5 模型预算、§31 A4 多维计费

## 0. 今天做什么
预算"预留+结算"状态机 + 多维计费（input/output/cache-write/cache-read/reasoning 分列）+ finalization_reserve（收尾额度）。

## 1. 进度位置
第 24 天。防多 Agent 并发突破预算、崩溃后泄漏/重复扣费。

## 2. 模块架构
```
internal/agent/budget.go   budget_ledger 读写，预留/结算
```

## 3. 技术选型
- 预留状态机：RESERVED→IN_FLIGHT→SUCCEEDED→SETTLED（/UNKNOWN→SETTLED）（§7.5）。
- 金额用**整数 micro-USD**、Token 整数（§31 C7），不碰 float。

## 4. 设计细节
- 调用前按 input+max_output 预留，保存 provider_request_id + 最坏成本（§7.5）。
- 调用后按真实 Usage 结算；崩溃后 IN_FLIGHT/UNKNOWN 不自动释放，按最坏保留至对账。
- 预算维度照 §7.5（Run 总费用/Token、单 Job、单调用输出上限、并发数、每日、finalization_reserve）。
- pricing 表可配置带版本（§31 A4）；无价显示"估算/未知"。

## 5. 分步开发
1. budget_ledger 预留/结算（durable）。
2. pricing 表 + 多维 token 计费。
3. finalization_reserve 划分。

## 6. 完成效果（DoD）
- 并发预留不突破上限；崩溃后 UNKNOWN 预留不泄漏（对账后释放）；收尾额度可单独产 PARTIAL 报告。

## 7. 怎么测试
- `tests/budget_test.go`：并发预留到上限→拒绝；UNKNOWN 不自动释放；结算率 100%。
- 命令：`go test -race ./internal/agent/...`
- 对照验收：§7.5；§22.6"无 UNKNOWN 预算预留泄漏"。
