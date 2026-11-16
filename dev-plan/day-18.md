# Day 18 —— Policy Proxy + 进程内 epoch 缓存

> 里程碑：M2 | 依赖：Day 16,17 | 产出：`internal/network/policy_proxy.go`
> 对照主文档：§9.5 执行时序、§31 P1x（热路径缓存）、§13.2 留证、§9.0

## 0. 今天做什么
Policy Proxy：所有目标请求的咽喉——逐请求回查 Permit/Fencing/Scope/授权窗/撤销代际，**原始留证**；用进程内 epoch 缓存把热路径降为 O(1)。

## 1. 进度位置
第 18 天，M2 的核心。这是红线第二道闸门 + 证据产生点。

## 2. 模块架构
```
internal/network/
  policy_proxy.go   HTTP/CONNECT MITM 代理
  rate_limiter.go redirect.go dns_guard.go egress.go
```

## 3. 技术选型
- Go MITM 代理（goproxy 或自写）+ 每安装独立 CA（§11.3）。
- 进程内权威缓存：revocation_epoch + active_permits（§31 P1x）。

## 4. 设计细节
- 每请求：先读内存 epoch+permit（O(1)）；命中未撤销未过期→放行 + 原子递减 Capability；miss/epoch 不一致/过期→回落 DB（§31 P1x）。
- **不信任 Runner 持有的 Permit 快照**，每跳独立回查（§9.5）。
- 逐请求校验 Scope（Day 16）/授权时间窗/撤销代际/Fencing。
- 原始请求响应**直接在 Proxy 留证**（§13.2）——Evidence 来源。
- 重启必从 DB 重建缓存、重建前 fail-closed（§9.0.3）。
- TLS 上游指纹对齐留到 Day 31（§49.3）；今天先通。

## 5. 分步开发
1. MITM 代理 + CA 生成/安装到沙箱。
2. 每请求校验链（epoch 缓存→回落 DB）。
3. 捕获请求/响应为 Evidence（先写临时，Day 20 接 Blob）。
4. 重定向/DNS 守卫。

## 6. 完成效果（DoD）
- 经 Proxy 的合规请求放行并留证；越域/越权/过期 Permit 请求被拦。

## 7. 怎么测试
- `tests/proxy_test.go`（打 Mock Target）：合规放行+留证；越域重定向被拦；Permit 过期被拒；Capability 逐请求消耗。
- 命令：`go test ./internal/network/...`
- 对照验收：§9.0.5、§23 M2"子请求逐个消耗 Capability 且 Redirect 越域被重新授权拦截"。
