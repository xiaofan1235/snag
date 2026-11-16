# Day 22 —— 安全不变量熔断器 + 工具卡住(hang)处理

> 里程碑：M2 | 依赖：Day 18,19 | 产出：`internal/network/breaker.go`、`internal/execution/toolruntime.go`
> 对照主文档：§4.2(v2.4) 熔断、§9.7 Kill Switch、§51 卡住处理

## 0. 今天做什么
①对"应恒为 0"的安全计数（scope-violation/egress 绕过/Permit 重放/Fencing 失败）做自动熔断（触发 Kill Switch）；②工具/命令卡住处理（硬超时/看门狗/长命令自动转后台/外部调用超时）。

## 1. 进度位置
第 22 天，M2 收尾。这两块是"出事能自动刹车、卡住不拖死"的保险。

## 2. 模块架构
```
internal/network/breaker.go       安全计数熔断
internal/execution/toolruntime.go  工具超时/看门狗/后台 Job
```

## 3. 技术选型
- 熔断：内存计数 + 阈值 + Proxy 层 `revoke_all_target_egress`（§9.7）。
- 卡住：context 超时 + 心跳；长命令转后台（借鉴 dsh，§53/§51）。

## 4. 设计细节
- 熔断（§4.2 v2.4）：安全计数超阈 → Kill Switch + SYSTEM_DEGRADED + 诊断快照。
- 卡住（§51）：每工具 timeout_seconds；看门狗无心跳无进度即 kill；前台>5s 转后台 Job、后台 30 分钟上限；模型/Proxy/工具/MCP 每个外部调用都超时；非 TTY 不挂起。

## 5. 分步开发
1. breaker：计数 + 阈值 + 触发 Kill。
2. toolruntime：超时/看门狗/转后台/kill+诚实上报。
3. 外部调用统一超时包装。

## 6. 完成效果（DoD）
- 注入"scope 违例计数上升"→自动 Kill Switch。
- 一个挂死工具被看门狗在上限内 kill；长命令转后台不堵 Executor。

## 7. 怎么测试
- `tests/breaker_test.go`：违例计数超阈→egress 被撤。
- `tests/hang_test.go`：挂死工具→超时 kill；长命令→转后台。
- 命令：`go test ./internal/network/... ./internal/execution/...`
- 对照验收：§51.6；§4.2 熔断。
