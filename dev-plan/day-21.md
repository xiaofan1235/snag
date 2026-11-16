# Day 21 —— Receipt + 对账 + OUTCOME_UNKNOWN 闭环

> 里程碑：M2 | 依赖：Day 19,20 | 产出：`internal/execution/receipt.go outcome_reconciler.go`
> 对照主文档：§5.3 RECORDED 前置、§5.3 三种孤儿、§0.5

## 0. 今天做什么
ExecutionReceipt + RECORDED 的确定性前置条件 + 三种孤儿对账（Receipt-only/Evidence-only/孤儿 Blob），把执行→证据→记录的闭环收口。

## 1. 进度位置
第 21 天，M2 收口。做完后：整个"受控动作 + 完整留证 + 崩溃一致"闭环成立。

## 2. 模块架构
```
internal/execution/receipt.go            回执终结
internal/execution/outcome_reconciler.go  孤儿对账
```

## 3. 技术选型
纯 Go；用 Day 06 durable 连接保证提交持久性。

## 4. 设计细节
- RECORDED 前置（§5.3，全满足才进）：①Receipt 已终结（非仅已发出）②Proxy 流已关闭、HTTP Exchange 齐备 ③每个 Evidence 元数据与 Blob 哈希已持久化。
- 三种孤儿（§5.3）：Receipt-only / Evidence-only → OUTCOME_UNKNOWN/Quarantine 待配对；孤儿 Blob 定期回收。
- 非幂等写未对账前禁止同 Job 增代重发（§4.1）。

## 5. 分步开发
1. Receipt 终结写入（durable）。
2. RECORDED 前置检查器。
3. outcome_reconciler：扫孤儿、配对、Quarantine、回收。

## 6. 完成效果（DoD）
- 正常动作走到 RECORDED；制造 Evidence-only/Receipt-only → 进 OUTCOME_UNKNOWN 待配对，不自动成功。

## 7. 怎么测试
- `tests/outcome_test.go`：缺 Receipt/缺 Evidence/缺 Blob 各一种→不进 RECORDED；孤儿进 Quarantine。
- 命令：`go test ./internal/execution/...`
- 对照验收：§5.3；§22.3"after_network_send 崩溃后进 RECONCILING"。
