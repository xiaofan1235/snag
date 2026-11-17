# Snag 控制台 Web UI（apps/web）

Snag 的本地控制台前端：React 18 + TypeScript + Vite，Vitest 测试，pnpm 管理。
仅用于**授权**渗透测试的可视化与控制。

## 环境要求

- Node.js **≥ 22**（pnpm 11 已弃用 Node 18–21）。
- pnpm **≥ 10.20**（10.x 与 11.x 均可，配置已跨版本兼容；见下）。

## 安装与常用命令

```bash
cd apps/web
pnpm install          # 安装依赖
pnpm dev              # 本地开发服务器
pnpm typecheck        # tsc 类型检查
pnpm test             # Vitest 单元/组件测试（jsdom）
pnpm build            # 生产构建（tsc -b && vite build）
```

## 关于 pnpm 配置（Q-25）

pnpm 11 起 `.npmrc` 只保留 auth/registry，pnpm 专属配置一律放 `pnpm-workspace.yaml`。本项目的
`pnpm-workspace.yaml` 已处理两个会让**新环境装不上**的点，无需手动加参数即可 `pnpm install`：

1. **供应链冷却**：pnpm 11 默认 `minimumReleaseAge=1440`（发布不满 1 天的版本不解析），会拒绝锁文件里
   较新的传递依赖。本仓锁文件已钉精确版本（安全边界由钉版 + 审阅保证），故显式设 `minimumReleaseAge: 0`
   以保证任何时间点可复现安装。
2. **构建脚本白名单**：`esbuild` 的 postinstall 需链接平台二进制。pnpm 10 读 `onlyBuiltDependencies`、
   pnpm 11 读 `allowBuilds`——两者都已写入，跨版本生效。

> 若仍遇到某包因 `minimumReleaseAge` 被拒，可在 `pnpm-workspace.yaml` 的 `minimumReleaseAgeExclude`
> 精确排除该包，而非全局放宽。

## 结构

```
src/
  api/         类型化 API client + 生成的契约类型（events.gen.ts / schema.gen.ts）+ TanStack Query hooks
  events/      SSE 快照一致性（consistency/stream/sse）+ 事件路由 + useRunStream
  components/  设计 Token（styles/tokens.css）+ 核心组件库（D2）+ 领域组件（D2.13）
  features/    P0 页面（dashboard/runs/overview/activity/findings/approvals/coverage/report/commands）
  app/         预览页（Gallery 组件库 + PagesDemo P0 页面）+ 主题
```

契约类型（`src/api/*.gen.ts`）由仓库根的 `go run ./cmd/gen-contracts` 从 `api/*.yaml` 生成，勿手改。
