# Day 03 —— 数据模型 Part2（补齐全部表）

> 里程碑：M0 | 依赖：Day 02 | 产出：`migrations/0002_*.sql`
> 对照主文档：§4.2、§43.3 主数据模型整合（权威表清单）

## 0. 今天做什么
把 §43.3 权威表清单里**剩余的表**全部建出来：调度执行、证据验证、规划、预算资源、命令事件、恢复、报告、记忆、知识技能、工具扩展，以及 v2.4+ 新增表。

## 1. 进度位置
第 3 天。数据模型到此完整。后面各模块直接用这些表，不再临时加表。

## 2. 模块架构
同 Day 02，继续在 `migrations/` + `store/`。一次性把表建全，避免后面边做边加导致不一致（§43.1 结构债教训）。

## 3. 技术选型
同 Day 02。JSON 列用 TEXT 存 canonical JSON（Day 04 定规范化）。

## 4. 设计细节
- 照 §43.3 清单逐张建：approvals/action_permits/request_capabilities/execution_receipts/
  strategy_epochs/strategy_snapshots/plan_patches/campaign_plan/budget_ledger/proxy_leases/
  sandbox_instances/resource_leases/model_calls/agent_context_revisions/commands/operations/
  user_directives/external_messages/outbox/recovery_operations/run_partial_reasons/
  reports/report_revisions/artifacts/quarantine/memory_*/knowledge_*/tool_registry/mcp_servers/
  sequence_experiments/race_experiments/oast_correlations/identity_auth_flows/
  cross_run_finding_links/audit_chain。
- 关键字段契约见主文档 §4.2（approvals/request_capabilities/commands/...）。
- `commands` 表 `UNIQUE(actor_id, idempotency_key)`。

## 5. 分步开发
1. `0002_full.sql`：按清单建表，带约束。分组写、加注释。
2. 补 sqlc query：每张表至少 get/insert。
3. `goose up` 应用。

## 6. 完成效果（DoD）
- 全部表存在（用 `.tables` 核对 §43.3 清单，不缺）。
- 复合外键、唯一约束生效（插重复 idempotency_key 失败）。

## 7. 怎么测试
- `tests/store_full_test.go`：建库后查询 `sqlite_master` 表数量≥清单数；插重复 commands key 期望报错。
- 命令：`go test ./internal/store/...`
- 对照验收：§43.3 清单逐项打勾。
