# Day 40 —— UI：API client + SSE + 领域组件

> 里程碑：M6 | 依赖：Day 39 | 产出：api client、SSE、ActionCard/EvidenceViewer
> 对照 UI 文档：§16.32-16.33、D2.13 领域组件

## 0. 今天做什么
OpenAPI 生成的 API client + fetch-based SSE（重放/resync）+ 领域组件（ActionCard/EvidenceViewer/HTTPViewer/DiffViewer/Inspector）。

## 1. 进度位置
第 40 天。打通"快照+SSE 一致性"的数据层，页面才能实时。

## 2. 模块架构
```
apps/web/src/api/ (generated client, queries, mutations)
apps/web/src/events/ (event-source, event-router, resync)
```

## 3. 技术选型
OpenAPI 生成类型/client；fetch-based SSE（原生 EventSource 无法设首帧头，§15.6）。

## 4. 设计细节
- 快照+SSE 一致性（§16.33）：先取快照记 event_cursor→建流从 cursor+1 补发→stream.resync_required 停增量重取。
- 收到比快照旧的事件丢弃；LIVE/RECONNECTING/STALE/RESYNCING 状态。
- 领域组件照 D2.13：EvidenceViewer（默认脱敏、Raw 需 Reveal）、DiffViewer（Web Worker）。
- Secret 不进 LocalStorage/URL（§16.33）。

## 5. 分步开发
1. 生成 API client + queries/mutations。
2. SSE event-source + router + resync。
3. 领域组件。

## 6. 完成效果（DoD）
- 页面能取快照 + 接 SSE 增量；断线重连补发无空洞；EvidenceViewer 默认脱敏。

## 7. 怎么测试
- Vitest：SSE 重放无丢事件；旧事件丢弃；resync 重取。
- 命令：`pnpm test`
- 对照：§16.33、UI 验收清单 1。
