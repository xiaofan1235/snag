# Day 41 —— UI：Dashboard + Run 向导 + Overview + Activity

> 里程碑：M6 | 依赖：Day 40 | 产出：P0 页面组 1
> 对照 UI 文档：§16.2/16.3/16.12/16.13、D4 界面地图、D5 创新

## 0. 今天做什么
Dashboard（注意力台 + 离开期间叙事）、Run 创建向导（5 步）、Overview（真实进度分数）、Activity（事件卡流 + Agent Session Drawer）。

## 1. 进度位置
第 41 天。用户"创建 Run + 看它在干嘛"的主路径。

## 2. 模块架构
```
apps/web/src/features/ (dashboard/runs/overview/activity)
```

## 4. 设计细节
- Dashboard（§16.12/D6）："需要我处理"首屏第一区；终态/非终态用形状不只用色。
- Run 向导（§16.2）：目标→规则→身份→执行环境→策略确认；只有 Preflight 全过才 Start。
- Overview（§16.3）：不显示虚假 73%，显示 TESTED/ELIGIBLE、CONFIRMED/CANDIDATE、阻塞原因。
- Activity（§16.13）：Typed Card；Live Follow 可暂停；不展示原始隐藏思维链。

## 5. 分步开发
1. Dashboard 四区 + 叙事回放入口。
2. Run 向导 5 步 + Preflight 门禁。
3. Overview + Activity 流。

## 6. 完成效果（DoD）
- 能从 UI 创建 Run、Preflight、Start，并在 Activity 看到事件卡实时流。

## 7. 怎么测试
- Playwright：创建→Preflight→Start→Activity 有事件。
- 命令：`pnpm test:e2e`
- 对照：UI 文档 §16.34 P0。
