# Day 36 —— Candidate Finding + 逐字 EvidenceCitation + 独立 Verifier

> 里程碑：M5 验证与报告 | 依赖：Day 20,33 | 产出：`internal/findings`、verifier
> 对照主文档：§13.3 Finding 晋级、§13.4 Verifier、§31 V4

## 0. 今天做什么
候选 Finding + **逐字证据引用**（防释义冒充）+ 独立验证器（确定性层 + AI 语义层，读独立投影 + 主动反证）。

## 1. 进度位置
第 36 天，M5 开端。这是"AI 不能自己宣布漏洞"的核心。

## 2. 模块架构
```
internal/findings/   service/triage/severity
internal/evidence/verification.go
```

## 3. 技术选型
- 确定性验证 + 异质模型语义验证（§31 A3 多样性）。

## 4. 设计细节
- Finding 晋级（§13.3）：Hypothesis→CANDIDATE→NEEDS_EVIDENCE/REJECTED/CONFIRMED→TRIAGED→REPORTABLE；Agent 自报不能直接 REPORTABLE（§4.3 #7）。
- 逐字引用（§13.3）：每条事实挂 EvidenceCitation{evidence_id,receipt_sequence,byte_start,byte_end,content_sha256,normalization}，映射 CEBS 连续片段；释义不符即拒晋级；跨 Run/伪造 ID 编译期拒。
- 确定性验证器（§13.4）：哈希/动作真实执行/请求响应同 Action/身份匹配/角色齐全/相反证据。
- AI 语义验证器（§13.4）：数据是否真敏感/是否真跨身份/差异是否支持结论/其他解释/定级是否过高；读独立投影不读作者话术（§31 V4）；主动搜寻反证。

## 5. 分步开发
1. Finding 晋级状态机。
2. EvidenceCitation 校验（映射 CEBS + 内容核对）。
3. 确定性 + AI 两层验证器。

## 6. 完成效果（DoD）
- 一个真实 IDOR 候选挂逐字引用→确定性+语义验证通过→CONFIRMED；释义不符的被拒。

## 7. 怎么测试
- `tests/finding_test.go`：伪造 Evidence ID 编译期拒；释义冒充拒晋级；Agent 自报不能直接 REPORTABLE。
- 命令：`go test ./internal/findings/... ./internal/evidence/...`
- 对照验收：§13.3、§22.4"释义冒充逐字引用"。
