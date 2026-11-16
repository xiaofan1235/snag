# Day 28 —— Strategist + StrategySnapshot + PlanPatch

> 里程碑：M3 | 依赖：Day 25 | 产出：`internal/agent/strategist.go`
> 对照主文档：§6.1-6.3、§6.5 触发器、§32 编排

## 0. 今天做什么
Run Strategist 慢循环：吃不可变 StrategySnapshot（摘要），产出 PlanPatch（只提案不碰库），由触发器驱动。

## 1. 进度位置
第 28 天。这是 §32 的"统一指挥层"。

## 2. 模块架构
```
internal/agent/strategist.go   规划（只提案）
```

## 3. 技术选型
Provider（Day 23）+ 原生 tool-call 输出 PlanPatch。

## 4. 设计细节
- StrategySnapshot（§6.2）：run_id/snapshot_revision/policy_digest/asset_summary/.../coverage_summary/finding_summary/budget_summary/...；规划器不读整库、不读全部原始响应。
- PlanPatch（§6.3）：operations 限定 §6.3 清单（upsert_hypothesis/create_job/reprioritize_job/request_approval/close_epoch/...）。
- **不允许**模型改 Scope/账号/并发/定级/Evidence/直接 DONE/结束 Run（§6.3）。
- 触发器（§6.5）：run.started/surface.revision_changed/job.verified/finding.candidate/...；3-5s Debounce + 单飞 + Snapshot Revision 防过期。
- 允许按需拉原始证据（§1⑧/evidence.read_chunk）。

## 5. 分步开发
1. Snapshot 组装（摘要）。
2. Strategist 调模型产 PlanPatch。
3. 触发器 + Debounce + 单飞。

## 6. 完成效果（DoD）
- 给一个 Snapshot，Strategist 产出合法 PlanPatch（创建若干 Job）。

## 7. 怎么测试
- `tests/strategist_test.go`（Mock Provider）：产出 PlanPatch 合 schema；试图改 Scope 的 op 被拒；Debounce/单飞生效。
- 命令：`go test ./internal/agent/...`
- 对照验收：§6.3。
