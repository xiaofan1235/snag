# Day 23 —— Provider 抽象 + 原生 tool-calling 结构化输出

> 里程碑：M3 自主 Agent Runtime | 依赖：Day 04 | 产出：`internal/providers/`
> 对照主文档：§7.4 Provider 抽象、§31 A1、§6.3 PlanPatch schema

## 0. 今天做什么
模型 Provider 抽象（Generate/Stream/CountTokens/Capabilities/Cancel）+ 每个角色经**一次原生 tool-call** 输出结构化结果（schema 即工具入参）。

## 1. 进度位置
第 23 天，M3 开端。这是"让模型只提案、且提案一定合 schema"的接口。

## 2. 模块架构
```
internal/providers/
  provider.go registry.go usage.go
  adapters/   各家 Provider 适配
```

## 3. 技术选型
- 统一接口 + Adapter；原生 tool-calling（§31 A1）；自托管模型用约束解码（GBNF/outlines）。

## 4. 设计细节
- 接口照 §7.4：Generate/StreamGenerate/CountTokens/GetCapabilities/Cancel；Capability 含 tool_calling/structured_output/streaming/prompt_caching/max_context/...
- 每角色一个"emit_X"工具，input_schema = PlanPatch/ActionProposal/Verdict 的 schema（Day 04）。
- 故障分类照 §7.4（网络/429/5xx/上下文超限/无效 Schema/截断/配额/安全拒绝/模型切换），各自行为。
- 安全拒绝不自动换 Provider 绕过（§7.4）。

## 5. 分步开发
1. provider.go 接口 + registry。
2. 一个真实 Adapter（你用的模型）+ 一个 Mock Provider（测试用，返回预设结构化输出）。
3. 故障分类 + 有界重试退避。

## 6. 完成效果（DoD）
- Mock Provider 能按 schema 返回一个合法 PlanPatch；非法输出触发"最多一次修复"。

## 7. 怎么测试
- `tests/provider_test.go`：Mock 返回合法/非法结构化输出；429→退避；安全拒绝→不换 Provider。
- 命令：`go test ./internal/providers/...`
- 对照验收：§7.4、§31 A1。
