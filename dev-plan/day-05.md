# Day 05 —— 事件注册表 + OpenAPI 骨架 + Mock Target

> 里程碑：M0 | 依赖：Day 04 | 产出：`api/events.yaml`、`api/openapi.yaml`、`cmd/mocktarget/`
> 对照主文档：§15.6 SSE 事件、§15 API、§22.5 本地靶场

## 0. 今天做什么
① `events.yaml` 唯一事件注册表（前后端类型从它生成）；② OpenAPI 骨架（Run/Job/Finding 等路由）；③ 一个**假目标服务**（Mock Target），后面没有真实目标也能测。

## 1. 进度位置
第 5 天，M0 收尾。做完后：**不用真实模型、不用真实目标**就能跑契约测试和后续开发。

## 2. 模块架构
```
api/events.yaml   所有 SSE 事件类型（§15.6 清单）—— 唯一来源
api/openapi.yaml  REST 路由契约（§15.1-15.8）
cmd/mocktarget/   一个本地 HTTP 服务：双账号、对象归属、跨身份差异、限流、重定向（§22.5）
```

## 3. 技术选型
- OpenAPI 3.1 + `oapi-codegen` 生成 Go 类型（§2.1）。
- events.yaml 自定义 schema → 生成 Go + TS 事件类型。
- Mock Target：标准库 net/http 即可。

## 4. 设计细节
- events.yaml 照 §15.6 全清单（run.updated / job.* / action.updated / evidence.created / ... memory.* / seat.*）。
- OpenAPI 先写 §15.1 Run API + §15.2 Job API 的路由与 Schema。
- Mock Target 实现 §22.5 的能力：匿名/双账号、对象归属（/orders/{id} 属于某账号）、跨身份读取差异、参数差分、限流、重定向、清理接口。**它是你测越权/IDOR 的靶子**。

## 5. 分步开发
1. 写 events.yaml（照 §15.6）。
2. 写 openapi.yaml 的 Run/Job 路由；`oapi-codegen` 生成。
3. `cmd/mocktarget/main.go`：实现上述端点，内存存订单/用户。

## 6. 完成效果（DoD）
- 从 events.yaml 和 openapi.yaml 能生成可编译的 Go 类型。
- `go run ./cmd/mocktarget` 起服务；curl 两个账号读同一订单，一个 200 一个 403（可测越权）。

## 7. 怎么测试
- `tests/mocktarget_test.go`：账号 A 读自己订单 200、读 B 订单 403；无身份读 401。
- 契约测试：断言 events.yaml 每个事件、openapi 每个路由都有 Schema（§23 M0"文档注册的每个路由/事件都有 Schema"）。
- 命令：`go test ./api/... ./cmd/mocktarget/...`
