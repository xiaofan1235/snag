# 开发路线图：按天拆解的施工计划（给较弱开发模型用）

> 配套：主文档《pentagent-dev-spec-v2.6.md》（权威参考）、《pentagent-ui-spec-v1.6.md》（UI 参考）。
> 本路线图把整个项目按架构拆成 **46 个工作日单元（Day 01–46）**，分 8 个阶段（M0–M7）。**每个 `day-NN.md` 是一个自包含的小开发文档**，含：进度位置、模块架构、技术选型、设计细节、分步开发、完成效果（DoD）、怎么测试。
> 目标：让开发模型**一次只看一天**，照着做、测过、再进下一天，而不必读 7000 行大文档。

## 怎么用（很重要）

```text
1. 一次只给模型一个 day-NN.md（加上它引用的主文档小节即可，别一次灌全书）
2. 严格按顺序：先做完一天的「完成效果」+「测试」全通过，再进下一天
3. 四大契约先冻结（Day 04）：Policy / JobSpec+PlanPatch / Action+Permit+Receipt / Evidence+Finding+Verification
   —— 这四个定了，后面才能并行，不然越改越乱
4. 每天结束：提交一次 git（一天一个可回滚的检查点）
5. 测试不能跳：弱模型最容易"看起来写完了其实不对"，用每天的验收命令兜底
6. 卡住就缩小：某天太大，就按它的「分步开发」拆成半天，别囫囵
```

## 现实预期（诚实）

```text
- 这是个大项目（分布式控制面 + AI 编排 + 安全代理 + 浏览器 + React UI）。
  "46 天"是工作单元数，不是日历承诺；弱模型 + 你，一个单元可能要 1~3 个实际工作日。
- 前 30 天（M0–M3）是脊椎，必须做对，别图快跳过测试。做完 M3 就有"能自主干活的核心"。
- M4–M7 是能力叠加（SRC 功能、验证报告、UI、加固），可按需裁剪先后。
- 想更快见效：先走通 M0→M1→M2 的「无 AI 也能受控发一个只读请求并完整留证」，
  这是整套安全模型的试金石。
```

## 阶段总览与"可演示里程碑"

| 阶段 | 天数 | 做完能演示什么 |
|---|---|---|
| **M0 契约与骨架** | Day 01–05 | 不用真实模型就能创建 Run/Job；四大契约冻结、代码生成通过 |
| **M1 持久化控制核心** | Day 06–13 | 杀掉核心进程后 Job 能恢复，旧 Attempt 写不进；CLI 能 pause/resume/stop |
| **M2 执行与证据核心** | Day 14–22 | 没有 AI，也能用 CLI 发起一个受控只读动作并完整留证；越域/越权被拦 |
| **M3 自主 Agent Runtime** | Day 23–30 | AI 能根据新证据动态创建后续 Job，而不是跑固定脚本 |
| **M4 SRC 功能包** | Day 31–35 | 本地靶场上连续自主推进多个模块；只有 Baseline 的格子不能标 TESTED |
| **M5 验证与报告** | Day 36–38 | 伪造结论进不了报告；零发现与未完成严格区分；反证自动降级 |
| **M6 Web UI（P0+P1）** | Day 39–43 | UI 不连内部库、重启浏览器不影响 Run；创建→审批→暂停→恢复→Finding→报告全链路 |
| **M7 加固与长时验收** | Day 44–46 | 12–24h Soak 通过；无越域、无直连、无伪造证据、无"未完成冒充完成" |

## 全量日表

| Day | 阶段 | 模块 | 依赖 | 核心产出 |
|---|---|---|---|---|
| 01 | M0 | 项目骨架 + domain 类型 | — | monorepo、internal/domain、errors |
| 02 | M0 | 数据模型 Part1 + Store 骨架 | 01 | 核心聚合表、migrations、sqlc |
| 03 | M0 | 数据模型 Part2（全表） | 02 | 执行/证据/规划/知识等全部表 |
| 04 | M0 | **四大契约冻结** + JCS | 01-03 | Policy/JobSpec/PlanPatch/Action/Permit/Evidence Schema |
| 05 | M0 | 事件注册表 + OpenAPI + Mock Target | 04 | events.yaml、代码生成、假目标 |
| 06 | M1 | 单写者 Store + 双写连接 | 02 | durable/bulk 双连接、PRAGMA、事务封装 |
| 07 | M1 | Run 状态机（表驱动+代码生成） | 06 | Run 迁移表、属性测试 |
| 08 | M1 | Job 状态机 + RECONCILING | 07 | Job 迁移表、取消语义 |
| 09 | M1 | Attempt + Claim/Lease/Fencing | 08 | 单调 fencing_token |
| 10 | M1 | Event Journal + Outbox | 06 | 全局 sequence、提交后发布 |
| 11 | M1 | commands/operations 幂等 | 10 | 幂等账本 |
| 12 | M1 | Reconciler + 阻塞态超时 | 09-11 | 启动恢复顺序 |
| 13 | M1 | CLI 骨架 + Pause/Resume/Stop | 12 | Cobra、故障注入 Hook 接口 |
| 14 | M2 | Action 状态机 + ActionProposal | 04 | idempotency_class |
| 15 | M2 | Policy Engine 判定 | 04,14 | 判定输入、动作类红线 |
| 16 | M2 | **Scope 规范化器 + fuzz** | 15 | 差分模糊、属性测试（红线地基） |
| 17 | M2 | 两层 Permit | 15 | Action Permit + Request Capability |
| 18 | M2 | Policy Proxy + epoch 缓存 | 16,17 | MITM、逐请求校验、留证 |
| 19 | M2 | Sandbox + Egress + Fake Runner | 18 | 分级沙箱、attestation |
| 20 | M2 | Evidence Store（CEBS） | 18 | 内容寻址、zstd+AEAD、血缘三元组 |
| 21 | M2 | Receipt + 对账 + OUTCOME_UNKNOWN | 19,20 | RECORDED 前置、孤儿对账 |
| 22 | M2 | 熔断器 + 工具卡住处理 | 19 | 安全不变量熔断、hang 处理 |
| 23 | M3 | Provider 抽象 + tool-calling | 04 | 结构化输出 |
| 24 | M3 | 模型预算账本 | 23 | RESERVED→SETTLED、多维计费 |
| 25 | M3 | Executor Loop | 23,24 | StepContext 冻结、效果幂等 |
| 26 | M3 | Context Builder + 不可信隔离 | 25 | 统一上下文预算 |
| 27 | M3 | 两阶段上下文压缩 | 26 | 受控压缩 |
| 28 | M3 | Strategist + PlanPatch | 25 | StrategySnapshot |
| 29 | M3 | PlanCompiler + 战役计划 | 28 | 同事务 CAS |
| 30 | M3 | Scheduler + 无进展/转向 | 29 | 评分、滞回、ε 探索 |
| 31 | M4 | 攻击面发现 | 19,30 | JS AST、浏览器、规格驱动 |
| 32 | M4 | 模块聚类 + 指纹 + Discovery | 31 | tech_stack、Discovery |
| 33 | M4 | Hypothesis + Experiment + Coverage | 30,32 | TESTED 绑定、Falsifier |
| 34 | M4 | 序列/竞态实验 + OAST | 33 | 业务逻辑、受控竞态 |
| 35 | M4 | 工具恢复 + 知识库 + Skill + 联网 | 32 | RAG 检索 |
| 36 | M5 | Candidate Finding + Verifier | 20,33 | 逐字 EvidenceCitation |
| 37 | M5 | Replay + Severity/CVSS + 降级 | 36 | 反证自动降级 |
| 38 | M5 | 报告 + Exit Gate + 跨 Run 关联 | 37 | PARTIAL 原因、门禁 |
| 39 | M6 | 设计系统 + 核心组件 | — | Token、Button/Select/Progress/Table |
| 40 | M6 | API client + SSE + 领域组件 | 39 | ActionCard、EvidenceViewer |
| 41 | M6 | Dashboard + 向导 + Overview + Activity | 40 | P0 页面 1 |
| 42 | M6 | Finding + Evidence + Approval + Coverage | 40 | P0 页面 2 |
| 43 | M6 | Report + 恢复预览 + 幂等/Resync | 40 | P0 收尾 |
| 44 | M7 | 故障注入全套 + Policy Bypass | 全部 | 绕过测试套件 |
| 45 | M7 | Soak + 资源泄漏 + Secret Canary | 44 | 12–24h 浸泡 |
| 46 | M7 | 证据审计 + 验收清单 | 45 | 可连续自主运行版本 |

## 每天文档的固定结构

```text
0. 今天做什么（一句话）
1. 进度位置（第几天、属哪个里程碑、前置、做完能干什么）
2. 模块架构（在系统里的位置、和谁交互、数据流）
3. 技术选型（用什么、为什么）
4. 设计细节（数据结构、接口、关键不变量、边界）——引用主文档对应章节
5. 分步开发（给弱模型的具体步骤：建什么文件、写什么函数、关键逻辑）
6. 完成效果 DoD（做完后能演示的具体行为）
7. 怎么测试（单元测试 + 可跑的验收命令 + 对照主文档验收条目）
```

## 测试纪律（四层，越往后越全）

```text
单元测试      每天的模块自测（状态迁移、规范化、Permit 单次消费…）
契约测试      OpenAPI/Proto/Schema 一致（M0 起）
故障注入      在 Hook 点模拟崩溃，验证恢复（M1 起随实现同步）
浸泡 Soak     12–24h 随机杀进程/过期/断网（M7）
```

**一句话**：别把这 46 天当成"读完"的材料，当成"一天干一件、干完测过再走"的施工单。
