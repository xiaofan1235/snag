# Day 39 —— UI：设计系统 Token + 核心组件

> 里程碑：M6 Web UI | 依赖：Day 05(API) | 产出：`apps/web/` 组件库
> 对照 UI 文档：第二部分 D1 Token、D2 组件库、D7 性能

## 0. 今天做什么
前端工程骨架 + 设计 Token（亮/暗）+ 核心标准组件（Button/Input/Select/Progress/StatusBadge/Table/Card/Modal/Drawer/Toast）。

## 1. 进度位置
第 39 天，M6 开端。**先定气质再铺页面**（UI 文档 D9）。

## 2. 模块架构
```
apps/web/src/
  components/ (data-table/action-card/status-badge/...)
  app/ api/ events/ stores/ workers/
```

## 3. 技术选型
React+TS+Vite、TanStack Query/Table/Virtual、CodeMirror、ECharts、原生 EventSource（UI 文档第一部分 §16.32）。

## 4. 设计细节
- Token 照 UI 文档 D1（暖纸墨底/静默语义色/暗色暖黑）CSS 变量。
- 组件照 D2：每个组件变体/尺寸/状态完整（default/hover/active/focus/disabled/loading）、只用 Token、无障碍、性能（Select/Table 虚拟化）。
- StatusBadge 全站统一状态映射（D2.6）、颜色+图标+文字。

## 5. 分步开发
1. Vite 工程 + Token CSS 变量。
2. 10 个核心组件，每个含全部状态 + Storybook/示例。
3. 性能约束（Select>50 虚拟化、Table>100 虚拟化）。

## 6. 完成效果（DoD）
- 组件库可独立预览；亮/暗切换；StatusBadge 覆盖所有状态枚举。

## 7. 怎么测试
- Vitest + Testing Library：每组件各状态渲染；无障碍（focus-visible/aria）。
- 命令：`pnpm test`（apps/web）
- 对照：UI 文档 D2/D7。
