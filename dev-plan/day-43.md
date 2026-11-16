# Day 43 —— UI：Report + 恢复预览 + 命令幂等/Resync（P0 收尾）

> 里程碑：M6 | 依赖：Day 41,42 | 产出：Report、recovery、幂等交互
> 对照 UI 文档：§16.22/16.12/16.33、§16.34 P0

## 0. 今天做什么
Report Center（阅读气质 + 导出门禁）、恢复预览、以及 **P0 验收依赖**：命令幂等 + Operation 查询 + 断线 Resync。

## 1. 进度位置
第 43 天，M6（P0）收口。做完后：UI P0 可演示全链路。

## 4. 设计细节
- Report（§16.22）：阅读区衬线/留白；只纳入 Reportable；被排除的单独列出；导出 Markdown/JSON。
- 恢复预览（§16.12）：未完成 Job/未知结果 Action/过期身份/失效代理/策略版本/Schema 兼容。
- 命令幂等/Resync（§16.33/§16.34 P0 不可后移）：202+operation_id；同 Idempotency-Key 查询/重试不生成第二份；stream.resync_required 重取。
- 不 optimistic 把 Run 标 Paused / Finding 标 Confirmed。

## 5. 分步开发
1. Report Center + 导出。
2. 恢复预览页。
3. CommandButton（幂等键+确认+结果跳转）+ Resync。

## 6. 完成效果（DoD）
- UI P0 验收清单全可演示：创建→审批→暂停→恢复→Finding→报告；断线重连无重复/倒退。

## 7. 怎么测试
- Playwright：完整链路 + 崩溃恢复 + 审批竞态两条链路（UI 验收清单 10）。
- 命令：`pnpm test:e2e`
- 对照：§16.34 P0 全部。
