# Day 31 —— 攻击面发现（JS AST + 浏览器 + 规格驱动）

> 里程碑：M4 SRC 功能包 | 依赖：Day 19,30 | 产出：`internal/knowledge/surface`、工具 crawl/js.*
> 对照主文档：§12.1 资产接口发现、§31 E3 规格驱动、§49 浏览器

## 0. 今天做什么
攻击面发现工具集：同源爬取、JS Chunk 下载、JS AST 提取端点、浏览器运行时 XHR/Fetch 捕获、Swagger/OpenAPI/GraphQL 规格驱动生成。

## 1. 进度位置
第 31 天，M4 开端。有了攻击面，AI 才有东西可测。

## 2. 模块架构
```
runners/js/      JS 下载/AST（tree-sitter）/端点提取/secrets
runners/browser/ 浏览器运行时捕获（CDP）—— 经 Policy Proxy
```

## 3. 技术选型
- JS AST：**tree-sitter**（有 Go 绑定，§12.1/§31 L2）。
- 浏览器：Playwright sidecar 或 Go rod（§31 L2，先评估够不够）。
- TLS 指纹上游对齐（§49.3）在 Proxy 侧配。

## 4. 设计细节
- 功能照 §12.1（首页/同源遍历、JS Chunk、Source Map、静态/动态接口、运行时 XHR/Fetch、路由菜单、参数、WebSocket、Swagger/OpenAPI/GraphQL）。
- 接口归一化、路径参数模板化、来源标记 static/runtime/spec/inferred/user（§12.1）。
- 规格驱动（§31 E3）：发现规格→确定性生成端点×参数矩阵，填 endpoints/endpoint_parameters。
- 注意点（§12.1）：不无限目录爆破；推测接口仍过 Policy；第三方域只记录不访问；浏览器禁 Service Worker 保归因（§42）。

## 5. 分步开发
1. crawl.same_origin + js.fetch/parse_ast/extract_endpoints。
2. 浏览器运行时捕获（经 Proxy）。
3. 规格解析 → 生成矩阵。

## 6. 完成效果（DoD）
- 对 Mock Target（给个带 JS 的页面）跑发现，产出 endpoints/参数，来源标记正确。

## 7. 怎么测试
- `tests/surface_test.go`：JS 里的接口被 AST 提取；规格驱动生成矩阵；SurfaceRevision 提升。
- 命令：`go test ./runners/... ./internal/knowledge/...`
- 对照验收：§12.1。
