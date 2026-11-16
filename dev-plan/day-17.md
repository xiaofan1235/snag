# Day 17 —— 两层 Permit（Action Permit + Request Capability）

> 里程碑：M2 | 依赖：Day 15 | 产出：`internal/execution/permit.go`
> 对照主文档：§9.5 两层 Permit、§4.3 #4、§31 C1

## 0. 今天做什么
两层许可：Action Permit（一次性，"启动一次该动作"）+ Request Capability（逐请求，浏览器多子请求/重定向用）。单次消费 + 原子条件更新。

## 1. 进度位置
第 17 天。Permit 是"动作必须先授权才能执行"的凭证，Proxy（Day 18）会逐请求回查它。

## 2. 模块架构
```
internal/execution/permit.go   签发/消费 Permit 与 Request Capability
```

## 3. 技术选型
消费走 durable 连接的**单条条件更新**（§9.5）。

## 4. 设计细节
- Action Permit 绑定（§9.5）：Run/PolicyRevision/Job/AttemptGeneration/FencingToken/Tool/CanonicalActionDigest/Identity/ProxyLease/RiskClass/Expiry/Nonce。
- Request Capability：声明 max_subrequests（浏览器动作必须声明）；每线上请求原子消耗一个。
- 消费 = 单条条件更新（§9.5）：Permit 未用 ∧ Job 当前 generation ∧ Attempt 活跃 ∧ Lease 未过期 ∧ Run 可执行 ∧ PolicyRevision 有效。
- 每个 Permit 只能消费一次（§4.3 #4，与 DISPATCHED 同事务）。
- Attempt 租约过期 → 其未消费 Permit 全部撤销（接 Day 09）。

## 5. 分步开发
1. 签发 Action Permit（PDP allow 后）。
2. 消费：单条 CAS + 同事务写 DISPATCHED。
3. Request Capability 签发/原子消耗/超限中断。

## 6. 完成效果（DoD）
- 一个 Permit 只能消费一次（并发消费只 1 成功）；过期/越代消费被拒。

## 7. 怎么测试
- `tests/permit_test.go`：并发消费同 Permit→只 1 成功；旧 generation/过期 Lease 消费→拒绝；Capability 超 max_subrequests→中断。
- 命令：`go test -race ./internal/execution/...`
- 对照验收：§4.3 #4、§9.5、§22.4"Permit 重放"。
