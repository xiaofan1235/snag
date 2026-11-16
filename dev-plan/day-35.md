# Day 35 —— 工具错误恢复 + 知识库 + Skill + 联网检索

> 里程碑：M4 | 依赖：Day 32 | 产出：tool_recovery、knowledge、skills、web
> 对照主文档：§8.5 工具恢复、§34 知识库、§40 Skill、§36 联网、§41 检索

## 0. 今天做什么
工具错误恢复协议（分类→重试→替代→降级）+ 知识库（md+RAG 检索）+ Skill 导入 + 联网知识 + 卡住即检索。

## 1. 进度位置
第 35 天，M4 收口。让 AI 有"方法脑"可查。

## 2. 模块架构
```
internal/knowledge/   knowledge/skills/search（FTS5+sqlite-vec）
internal/tools/recovery.go
```

## 3. 技术选型
- 检索：FTS5 + **sqlite-vec** 混合（§41/§31 O6）。
- 联网走 utility 通道（§36.1，非目标 Proxy）。

## 4. 设计细节
- 工具恢复（§8.5）：每工具 ToolRecoveryPolicy（error_class→retry/alternative/degradation）；替代/降级=新 Digest+新 Permit，仍走 Sandbox/Proxy；OUTCOME_UNKNOWN 不自动换工具。
- 知识库（§34）：md+frontmatter；入库校验（schema/Secret Canary/去重）；三段式检索（§41：分类粗筛→混合索引精排→取详情，预算内）。
- Skill 导入（§40）：本地受审包；不声明越权；按 applies_to 加载。
- 联网（§36）：knowledge.web_search 走 utility；结果不可信、标参考；卡住即检索（§36.2）。

## 5. 分步开发
1. ToolRecoveryPolicy 执行器。
2. 知识库入库 + 三段式检索。
3. Skill 导入 + 联网检索 + 触发点。

## 6. 完成效果（DoD）
- 工具失败按策略恢复；给情境能检索到相关知识；卡住时自动检索注入。

## 7. 怎么测试
- `tests/knowledge_test.go`：五类工具错误各走预期；检索按 applies_to 命中；两 Workspace 不互查；Canary 拦截。
- 命令：`go test ./internal/knowledge/... ./internal/tools/...`
- 对照验收：§8.5、§34.9、§41.7。
