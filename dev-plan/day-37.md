# Day 37 —— Replay + Severity/CVSS + 反证自动降级

> 里程碑：M5 | 依赖：Day 36 | 产出：replay、severity、verdict 降级
> 对照主文档：§13.5 重放、§13.4 反证降级、§31 V7 CVSS

## 0. 今天做什么
重放（独立 Job + 新 Attempt + 新 Evidence 对比）+ 严重度/CVSS 确定性计算 + 反证自动降级（REPORTABLE 非一次性授予）。

## 1. 进度位置
第 37 天。让 Finding 可复核、可降级、定级有据。

## 2. 模块架构
```
internal/findings/severity.go
internal/evidence/verification.go（降级）
```

## 3. 技术选型
CVSS 向量确定性计算（§31 V7），AI 只写描述。

## 4. 设计细节
- 重放（§13.5）：独立 Verification Job→新 Attempt→重新 Policy→新 Evidence→对比→写 Verdict；写动作重放重新满足对象归属。
- 反证降级（§13.4）：新 Replay 反证→NEEDS_EVIDENCE/REJECTED；Evidence 哈希失败→降级+Quarantine；依赖 Verdict 被 supersede→重估。
- CVSS（§31 V7）：程序从结构化字段算 v3.1/v4.0 向量+justification。

## 5. 分步开发
1. Replay Job。
2. CVSS 计算。
3. 反证降级 + verdict revision。

## 6. 完成效果（DoD）
- REPORTABLE 发现被新反证→自动降级；重放产生新证据对比。

## 7. 怎么测试
- `tests/replay_test.go`：反证→降级；Evidence 哈希失败→降级+Quarantine。
- 命令：`go test ./internal/findings/... ./internal/evidence/...`
- 对照验收：§13.4 反证降级。
