# Day 38 —— 报告 + Exit Gate + 跨 Run 关联

> 里程碑：M5 | 依赖：Day 37 | 产出：`internal/report`、exit_gate、cross_run
> 对照主文档：§13.6 报告、§14.2 退出门禁、§47.3 跨 Run、§31 O2

## 0. 今天做什么
报告生成（程序填事实 + AI 写描述）+ 退出门禁（EXITING 冻结 + CAS）+ 类型化 PARTIAL 原因 + 跨 Run Finding 关联。

## 1. 进度位置
第 38 天，M5 收口。做完后：**伪造进不了报告、零发现与未完成严格区分**。

## 2. 模块架构
```
internal/report/   builder/markdown/json/export
internal/control/exit_gate.go
```

## 3. 技术选型
Go text/template 或 templ；报告 DRAFT/FINALIZED/STALE（§13.4）。

## 4. 设计细节
- 报告（§13.6）：程序负责事实/请求/响应/Evidence 引用；AI 只生成描述和修复建议；程序校验每个事实有来源；不引用非 REPORTABLE Finding；不自动外部提交。
- Exit Gate（§14.2）：进评估先 EXITING 冻结；全条件通过与 EXITING→COMPLETED 同事务 CAS（Run+Snapshot+Surface Revision）；任一变化放弃回 RUNNING。门禁条目照 §14.2 清单。
- 预算耗尽→PARTIAL（不绕门禁）；类型化 PARTIAL 原因 run_partial_reasons（§14.2）。
- 跨 Run 关联（§47.3）：同 Workspace 同 Scope Digest，REOPENED/FIXED/DUPLICATE。

## 5. 分步开发
1. 报告 builder（事实程序填）。
2. Exit Gate CAS。
3. 跨 Run 关联。

## 6. 完成效果（DoD）
- 生成 Markdown/JSON 报告，只含 REPORTABLE；零发现正常结项与 PARTIAL 区分；门禁活锁防护生效。

## 7. 怎么测试
- `tests/report_test.go`：非 REPORTABLE 不入报告；手改 Markdown 不进权威；门禁并发 PlanPatch→放弃完成。
- 命令：`go test ./internal/report/... ./internal/control/...`
- 对照验收：§23 M5；§14.2。
