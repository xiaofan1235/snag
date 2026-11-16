# Day 09 —— Attempt + Claim/Lease/Fencing

> 里程碑：M1 | 依赖：Day 08 | 产出：`internal/scheduler/claim.go lease.go`
> 对照主文档：§4.1 attempts、§4.3 不变量 #2、§31（fencing）

## 0. 今天做什么
Job 认领（Claim）+ 租约（Lease）+ **单调 fencing_token**：旧 Attempt 即使进程复活也写不进新 Attempt 的东西。

## 1. 进度位置
第 9 天。这是"杀进程后能安全恢复、不重复执行"的机制核心。

## 2. 模块架构
```
internal/scheduler/
  claim.go   原子认领：UNIQUE(job_id, generation) + 生成 fencing_token
  lease.go   租约续期/过期
```

## 3. 技术选型
- 认领 = 一条条件 UPDATE（CAS），走 durable 连接。
- fencing_token 随 Job generation 单调递增。

## 4. 设计细节
- `attempts` 唯一约束 `UNIQUE(job_id, generation)`（§4.1）。
- Claim 事务内：增 generation、生成单调 fencing_token、写 Attempt。
- fencing_token 要写入 Attempt/Permit/Receipt/Proxy 校验上下文（§4.1）——今天先写 Attempt，Permit 在 Day 17 接。
- 租约过期：其未消费 Permit 全部撤销（Day 17 接）；现在先标记过期 Attempt。
- 不变量 #2：每次重新认领增 generation + 新 fencing_token。

## 5. 分步开发
1. `Claim(jobID)`：CAS 增代、生成 fencing_token、插 Attempt；失败即冲突。
2. `Lease`：lease_until 续期；扫描过期。
3. 旧 generation 的写入一律校验 fencing_token，不匹配拒绝。

## 6. 完成效果（DoD）
- 同一 Job 并发 Claim 只有一个成功。
- 旧 Attempt（小 fencing_token）的写入被拒。

## 7. 怎么测试
- `tests/fencing_test.go`：并发 Claim 只 1 成功；用旧 fencing_token 写入期望拒绝；租约过期后新 Claim 增代。
- 命令：`go test -race ./internal/scheduler/...`
- 对照验收：§4.3 #2；§22.4"过期 Attempt 提交"。
