# Day 27 —— 两阶段上下文压缩

> 里程碑：M3 | 依赖：Day 26 | 产出：`internal/agent/compaction.go`
> 对照主文档：§7.6 Context Compaction、§4.2 agent_context_revisions

## 0. 今天做什么
长 Run 的上下文受控压缩：阶段一（免费确定性丢弃过期块）、阶段二（付费 LLM 结构化摘要），保 exploit 连续性，原始 Revision 永不被改。

## 1. 进度位置
第 27 天。十几小时任务不压缩必溢出；但压缩有损，要保关键元素。

## 2. 模块架构
```
internal/agent/compaction.go   两阶段压缩 + CompactionRecord
```

## 3. 技术选型
- agent_context_revisions 链（FULL/COMPACTED）（§7.6）。
- 阶段二用独立 Summarizer profile。

## 4. 设计细节
- 阶段一（§7.6）：按块老化丢过期事件/已终结 Job 细节/超期 ToolResult，保留未决问题/活跃 Hypothesis/关键 Baseline 差异/IdentityRef/失败分类。
- 阶段二：仍超预算时 LLM 摘要，保 exploit 连续性（漏洞编号/关键参数名/差异结论/Evidence 引用/未验证假设）。
- 不变量（§7.6）：Journal/Evidence/原始 Revision 永不被压缩改；明文凭据绝不进摘要；空/膨胀摘要=FAILED 回退重试；Secret Canary 不进摘要。

## 5. 分步开发
1. 阶段一确定性丢弃。
2. 阶段二 LLM 摘要（调 Summarizer）。
3. CompactionRecord + 失败回退 + 崩溃恢复。

## 6. 完成效果（DoD）
- 上下文溢出前触发压缩，生成新 COMPACTED Revision，原始 Revision 不变；Secret Canary 不出现在摘要。

## 7. 怎么测试
- `tests/compaction_test.go`：溢出→压缩；空/膨胀摘要→FAILED 回退；Canary 不入摘要；旧 Evidence 引用仍可解析。
- 命令：`go test ./internal/agent/...`
- 对照验收：§7.6 验收。
