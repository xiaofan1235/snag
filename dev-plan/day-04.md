# Day 04 —— 四大契约冻结 + JCS 规范化（最关键的一天）

> 里程碑：M0 | 依赖：Day 01-03 | 产出：`api/schemas/*.json`、`internal/canon/`
> 对照主文档：§25 四大契约、§47.1 规范化总规、§9.1 Policy 示例、§6.3 PlanPatch、§6.7 ExperimentSpec

## 0. 今天做什么
冻结四大契约的 **Schema**：①Policy ②JobSpec/PlanPatch ③Action/Permit/ExecutionReceipt ④Evidence/Finding/Verification。并实现**统一规范化**（JCS）——所有 digest 的地基。

## 1. 进度位置
第 4 天，M0 的核心。**这四个定了，控制面/工具/AI/UI 才能并行**（§25）。之后改契约代价很大，所以今天要想清。

## 2. 模块架构
```
api/schemas/      policy.schema.json / planpatch.schema.json / action.schema.json /
                  evidence.schema.json / experiment.schema.json
internal/canon/   canonical.go: JCS 规范化 + SHA-256
```
canon 被所有要算 digest 的地方调用（Action/Policy/Snapshot/Request/Manifest）。

## 3. 技术选型
- JSON Schema 校验：`github.com/santhosh-tekuri/jsonschema`。
- **JCS = RFC 8785**（§47.1）：键序、数字、Unicode 归一化确定后再 SHA-256。可用现成 JCS 库或自实现（键排序 + 紧凑编码）。

## 4. 设计细节
- Policy Schema 照 §9.1 的 YAML 结构（target/authorization/methods/network/identities/limits/actions/ai/runtime）。
- PlanPatch 照 §6.3；支持的 op 限定 §6.3 清单。
- ExperimentSpec 照 §6.7；SequenceExperiment/RaceExperiment 照 §44。
- `canon.Digest(obj) string`：obj→JCS→SHA-256 hex。**全系统所有 digest 都走它**（§47.1 登记表）。
- 不变量：相同内容对象在任意两处 Digest 必须相等（这是红线/审批/去重的锚）。

## 5. 分步开发
1. 写五个 JSON Schema 文件（照上述章节）。
2. `canon/canonical.go`：实现 JCS + Digest；写 `ValidateAgainst(schema, obj)`。
3. 把 §47.1 的 digest 登记表写成常量注释，提醒每处用哪个。

## 6. 完成效果（DoD）
- 五个 Schema 能校验合法/非法样例（非法被拒）。
- `canon.Digest` 对同一对象的不同键序输入返回**相同**哈希。

## 7. 怎么测试
- `tests/canon_test.go`：同对象键序打乱 → Digest 相等；数字 1 vs 1.0 归一后相等。
- `tests/schema_test.go`：每个 Schema 喂合法+非法样例。
- 命令：`go test ./internal/canon/... ./api/...`
- 对照验收：§47.5"同内容对象在任意两处产生相同 digest"。
