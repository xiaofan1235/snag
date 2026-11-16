# Day 15 —— Policy Engine（判定点 PDP）

> 里程碑：M2 | 依赖：Day 04,14 | 产出：`internal/policy/engine.go`
> 对照主文档：§9.0 红线总纲、§9.2 判定输入、§9.1 Policy、§6.3 不允许的操作

## 0. 今天做什么
Policy Decision Point：对每个 ActionProposal 做**确定性判定**（Scope/Method/Identity/Ownership/Rate/Quota/Budget/Risk/红线动作类）。

## 1. 进度位置
第 15 天。这是红线的第一道执法闸门（§9.0 三闸门之一）。Scope 规范化明天单独做（最关键）。

## 2. 模块架构
```
internal/policy/
  engine.go     判定主逻辑
  methods.go identities.go quota.go   各维度
```

## 3. 技术选型
纯 Go 确定性判定；读 policy_revisions（canonical JSON）。

## 4. 设计细节
- 判定输入照 §9.2（不止 URL+Method：Run/PolicyRevision/Job/Attempt/Identity/Tenant/EndpointVerificationState/ObjectOwnership/CanonicalURL/Method/BodyDigest/Tool/RiskClass/...）。
- 动作类红线照 §9.1 actions（write/real_user_data/financial/persistence/internal_network 默认 deny）。
- **模型无法通过提案改 Scope/定级/结束 Run**（§6.3）——PDP 拒绝这些。
- 输出：Allow + 需审批标记 / POLICY_DENIED + 原因。

## 5. 分步开发
1. `Decide(ctx PolicyInput) Decision`：逐维度检查。
2. 方法/身份/配额/预算子检查。
3. 命中红线动作类 → deny 或 需审批。

## 6. 完成效果（DoD）
- 越权 Method/身份/写动作被 deny；合规只读被 allow。

## 7. 怎么测试
- `tests/policy_engine_test.go`：构造各违规输入→期望 deny；合规→allow。
- 命令：`go test ./internal/policy/...`
- 对照验收：§9.2、§9.6。
