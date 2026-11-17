# Snag 开发进度 PROGRESS

> 本文件由 **Claude（开发方）** 维护，实时记录开发进度。
> 每完成一天：把该行状态改为 `✅已完成`，填 commit 短哈希与日期，必要时写备注。
> 规则见 `README.md` 协作流程；详细任务见 `dev-plan/day-NN.md`。

## 状态图例
```
⬜未开始   🟨开发中   ✅已完成(已推送)   🧪测试中   🐞有bug待修   ✔️测试通过
```

## 当前位置

**Day 10 —— Event Journal + Outbox（提交后发布）✅（已提交 `c8cebaf`）**
下一步：**Day 11（dev-plan/day-11.md）**，等待 Owner 批准后开始。

Claude 开发前自检：`TESTING.md` 顶部「未解决 bug」= 无（Day 09 已测 T-035/T-036，Q-17 已处置 `9c2b285`、复测 T-037 关闭）。
✅ Day 10 已验证：`go test -race -tags sqlite_modernc ./internal/events/... ./internal/outbox/...` 全绿——
事务回滚则无事件/无消息；提交后经 Drain 才发布到 Bus；run 内 + 全局 sequence 单调无空洞；
publisher 失败指数退避重试不丢、成功后幂等不重发（§0.5「DB 提交后才发布」/§53.2 先存后发）。

---

## 进度总表

| Day | 里程碑 | 模块 | 状态 | Commit | 日期 | 备注 |
|---|---|---|---|---|---|---|
| 01 | M0 | 项目骨架 + 领域类型（domain） | ✅已完成 | `860272b` | 2026-11-16 | go build/vet/test 全绿；8/8 测试；枚举对照 §43.5 齐全。待推送 |
| 02 | M0 | 数据模型 Part1 + Store 骨架 | ✅已完成 | `7461cb0` | 2026-11-16 | 13表+手写Store+内置迁移器；默认 build/vet/test 绿；**真库已验证**(go1.23+modernc v1.38.2，`-tags sqlite_modernc` 全 PASS) |
| 03 | M0 | 数据模型 Part2（补齐全部表） | ✅已完成 | `b27ed58` | 2026-11-16 | §43.3 全 68 表(补 55)；真库 8/8 PASS；唯一约束+复合外键跨Run拒绝生效 |
| 04 | M0 | 四大契约冻结 + JCS 规范化（最关键的一天） | ✅已完成 | `504ae08` | 2026-11-16 | 5契约Schema + canon(JCS RFC8785+SHA256)；键序/数字归一→同digest；合法非法样例全过 |
| 05 | M0 | 事件注册表 + OpenAPI 骨架 + Mock Target | ✅已完成 | `c7e93d6` | 2026-11-16 | 49事件注册表+OpenAPI(Run/Job)+自写生成器(Go/TS)+Mock靶场(越权语义)；契约测试全绿 |
| 06 | M1 | 单写者 Store（durable/bulk 双连接） | ✅已完成 | `461d8df` | 2026-11-16 | 单写goroutine串行化+durable(FULL)/bulk(NORMAL)双连接+group-commit(savepoint隔离)+只读池；-race 3并发无BUSY |
| 07 | M1 | Run 状态机（表驱动 + 代码生成 + 属性测试） | ✅已完成 | `1bb88e5` | 2026-11-16 | states.yaml→gen→run_sm.gen.go(34迁移)+控制器(durable条件更新)；rapid属性测试;任意组合确定结果 |
| 08 | M1 | Job 状态机 + RECONCILING + 取消语义 | ✅已完成 | `0e721aa` | 2026-11-16 | 生成器改多机通用(Run/Job)；job_states.yaml(13态/34迁移)→job_sm.gen.go；RECONCILING 进入/冻结Claim/对账超时→OUTCOME_UNRESOLVED终态+run_partial_reasons；迁移0003加reconcile_deadline；双模+race全绿 |
| 09 | M1 | Attempt + Claim/Lease/Fencing | ✅已完成 | `33e07f3` | 2026-11-17 | internal/scheduler/{claim,lease}.go；并发Claim恰1成功(SM守卫+单写者)；增代+全局单调fencing_token；旧Attempt写入被拒；租约过期→EXPIRED+lease_expired回READY重领；schema已就绪无需新迁移；-race全绿 |
| 10 | M1 | Event Journal + Outbox（提交后发布） | ✅已完成 | `c8cebaf` | 2026-11-17 | internal/events(Append:run+全局双序,事务内)/outbox(Enqueue+Publisher.Drain提交后发布+指数退避)；迁移0004加events.global_sequence+outbox退避列；回滚无事件/提交后才发布/序号无空洞/失败重试不丢；-race全绿 |
| 11 | M1 | commands/operations 幂等账本 | ⬜未开始 |  |  |  |
| 12 | M1 | Reconciler + 每阻塞态超时 | ⬜未开始 |  |  |  |
| 13 | M1 | CLI 骨架 + Pause/Resume/Stop + 故障注入 Hook | ⬜未开始 |  |  |  |
| 14 | M2 | Action 状态机 + ActionProposal | ⬜未开始 |  |  |  |
| 15 | M2 | Policy Engine（判定点 PDP） | ⬜未开始 |  |  |  |
| 16 | M2 | Scope 规范化器 + 差分模糊（红线的地基） | ⬜未开始 |  |  |  |
| 17 | M2 | 两层 Permit（Action Permit + Request Capability） | ⬜未开始 |  |  |  |
| 18 | M2 | Policy Proxy + 进程内 epoch 缓存 | ⬜未开始 |  |  |  |
| 19 | M2 | Sandbox + Egress + Fake Runner | ⬜未开始 |  |  |  |
| 20 | M2 | Evidence Store（CEBS + 内容寻址 + 加密） | ⬜未开始 |  |  |  |
| 21 | M2 | Receipt + 对账 + OUTCOME_UNKNOWN 闭环 | ⬜未开始 |  |  |  |
| 22 | M2 | 安全不变量熔断器 + 工具卡住(hang)处理 | ⬜未开始 |  |  |  |
| 23 | M3 | Provider 抽象 + 原生 tool-calling 结构化输出 | ⬜未开始 |  |  |  |
| 24 | M3 | 模型预算账本（RESERVED→SETTLED） | ⬜未开始 |  |  |  |
| 25 | M3 | Executor Loop（StepContext 冻结 + 效果幂等） | ⬜未开始 |  |  |  |
| 26 | M3 | Context Builder + 统一上下文预算 + 不可信隔离 | ⬜未开始 |  |  |  |
| 27 | M3 | 两阶段上下文压缩 | ⬜未开始 |  |  |  |
| 28 | M3 | Strategist + StrategySnapshot + PlanPatch | ⬜未开始 |  |  |  |
| 29 | M3 | PlanCompiler（同事务 CAS）+ 战役计划 | ⬜未开始 |  |  |  |
| 30 | M3 | Scheduler（评分 + 滞回 + ε 探索）+ 无进展/转向 | ⬜未开始 |  |  |  |
| 31 | M4 | 攻击面发现（JS AST + 浏览器 + 规格驱动） | ⬜未开始 |  |  |  |
| 32 | M4 | 模块聚类 + 技术栈指纹 + Discovery | ⬜未开始 |  |  |  |
| 33 | M4 | Hypothesis + Experiment + Coverage（TESTED 绑定） | ⬜未开始 |  |  |  |
| 34 | M4 | 序列/竞态实验 + OAST | ⬜未开始 |  |  |  |
| 35 | M4 | 工具错误恢复 + 知识库 + Skill + 联网检索 | ⬜未开始 |  |  |  |
| 36 | M5 | Candidate Finding + 逐字 EvidenceCitation + 独立 Verifier | ⬜未开始 |  |  |  |
| 37 | M5 | Replay + Severity/CVSS + 反证自动降级 | ⬜未开始 |  |  |  |
| 38 | M5 | 报告 + Exit Gate + 跨 Run 关联 | ⬜未开始 |  |  |  |
| 39 | M6 | UI：设计系统 Token + 核心组件 | ⬜未开始 |  |  |  |
| 40 | M6 | UI：API client + SSE + 领域组件 | ⬜未开始 |  |  |  |
| 41 | M6 | UI：Dashboard + Run 向导 + Overview + Activity | ⬜未开始 |  |  |  |
| 42 | M6 | UI：Finding + Evidence + Approval + Coverage | ⬜未开始 |  |  |  |
| 43 | M6 | UI：Report + 恢复预览 + 命令幂等/Resync（P0 收尾） | ⬜未开始 |  |  |  |
| 44 | M7 | 故障注入全套 + Policy Bypass Suite | ⬜未开始 |  |  |  |
| 45 | M7 | 浸泡 Soak + 资源泄漏 + Secret Canary | ⬜未开始 |  |  |  |
| 46 | M7 | 报告证据审计 + 验收清单（可连续自主运行版本） | ⬜未开始 |  |  |  |

---

## 开发日志

> 每天追加一条：做了什么、遇到什么、下一步。

- **2026-11-16** — Day 0：创建仓库 `xiaofan1235/snag`（private），推送两份大文档 + 46 个日计划 + 路线图 + PROGRESS/TESTING 骨架。等待批准开始 Day 01。
- **2026-11-16** — Day 01 完成（提交 `860272b`）：`go mod init pentagent` + §3 目录骨架；`internal/domain` 全套零依赖领域类型（states/ids/errors + 12 个聚合照 §4.1）；UUIDv7 自实现（时间有序+同毫秒单调），避免引入外部库以适配无网络构建；8 组单元测试全绿，枚举对照 §43.5 逐一核对无缺。下一步：Day 02。（已推送）
- **2026-11-16** — Owner 将 `snag` 设为公开并授权推送；`860272b`+进度提交已成功 push 到 GitHub（main）。「Claude 开发并推送」流程打通。Owner 授权：后续如有更优实现可偏离开发文档，但须在 commit/PROGRESS/TESTING 留痕以便本地测试模型对照。
- **2026-11-16** — Day 02 完成（提交 `7461cb0`）：13 张核心表(§4.1) + 手写 Store 查询层 + 内置 goose 格式迁移器。**三处有意偏离**(无网适配，已记 README/commit)：sqlc→手写查询；goose CLI→内置迁移器；sqlite 驱动→build tag 可选接入(默认零外部依赖)。build/vet/test 默认全绿，真库测试 SKIP。**未验证项**：连真实 SQLite 的迁移+乐观锁用例，需联网 `go get modernc.org/sqlite` 后 `-tags sqlite_modernc` 跑，待本地模型或联网补验。删除空占位目录 api//tests/。下一步：Day 03。
- **2026-11-16** — 修 BUG-001（本地测试模型报，高）：`.gitignore` 的 `coverage.*` 规则误把 `internal/domain/coverage.go` 源码一起忽略，Day 01 推送时该文件漏提交，全新 clone 下 domain 测试编译失败——即 Day 01 PROGRESS 记的「8/8 全绿」在**仓库实况里并不成立**（开发本地工作区有该文件才没暴露，特此更正）。修复 `d9d9c62`：改 `.gitignore` 为具体产物名、补提交 coverage.go，并采纳测试模型 T-005 建议加 `.gitattributes`(行尾 LF，根除 Windows 测试机 CRLF 假报)。已在全新 clone 复验 build/vet/test 全绿；待本地模型复测 T-002/T-003。
- **2026-11-16** — Owner 开通网络权限（本会话即时生效）。补做 Day 02 真库验证：`go get modernc.org/sqlite@v1.38.2`。踩坑记录：默认装到 v1.62.0 会把 go 指令顶到 1.26.0（libc/x-sys 要求），故**把驱动钉在 v1.38.2（+libc v1.66.3），go 指令定为 1.23**——满足纯 Go 驱动的最低要求又不强依赖刚发布的 1.26；清理了 toolchain 固定行。`go test -tags sqlite_modernc ./internal/store/...` 全 PASS，默认构建仍零驱动。提交 go.mod/go.sum。下一步：等 Owner 批准后开 Day 03。
- **2026-11-16** — Day 03 完成（提交 `b27ed58`）：照 §43.3 权威清单补齐其余 55 张表（共 68）。§4.2 给字段契约的照抄，其余按用途设计最小合理 schema（§43.8）；按依赖顺序建表避免 FK 前向引用。补建 §43.3 漏列、§4.2 要求的 `identities`。决定：Day 02 遗留的跨表引用列（actions.identity_id 等）**不做表重建加 FK**（SQLite 限制 + 高风险低收益），改应用层校验，已记 README。真库 `-tags sqlite_modernc` 8/8 PASS：68 表齐、commands/external_messages 唯一约束、复合外键跨 Run 拒绝。下一步：Day 04（四大契约冻结，最关键）。
- **2026-11-16** — 按本地测试模型首轮代码质量审计（T-013，功能 bug 0）加固（提交 `c2635d6`）。Q-1~Q-6 逐条核对后全部认同采纳，趁 Day 04 契约冻结前修完：Q-1 cross_run_finding_links 补复合外键(确为疏漏)；Q-2 迁移器剥行内尾注+缺标记报错+内部测试；Q-3 预算/成本去浮点改整数子单位；Q-4 补 FK/热点索引；Q-5 README 记 UTC 时间不变量；Q-6 强化跨 Run 测试+foreign_key_check。真库 11/11 PASS。TESTING 已回写开发方回应，待本地模型复测。下一步仍为 Day 04，等 Owner 批准。
- **2026-11-16** — Day 04 完成（提交 `504ae08`），M0 最关键一天：冻结四大契约 JSON Schema（policy/planpatch/action/experiment/evidence，照 §9.1/§6.3/§6.7/§4.1）+ 实现 `internal/canon`（JCS RFC 8785 规范化 + SHA-256 Digest + Schema 校验）。规范化**用成熟库 gowebpki/jcs**（数字归一最易搓错，不自实现）+ santhosh-tekuri/jsonschema v6；两库均 go 1.21，基线仍 1.23。验证 §47.5 立身不变量：键序打乱/数字 1vs1.0 → 同 digest；5 Schema 合法通过、非法(version≠1/match=regex/op越权/缺字段/伪造state)被拒。digest 登记表(§47.1)写成常量注释。**契约已冻结**。下一步：Day 05。
- **2026-11-16** — Day 05 完成（提交 `c7e93d6`），**M0 收尾**：api/events.yaml(§15.6 全 49 事件注册表) + api/openapi.yaml(OpenAPI 3.1，Run/Job 路由) + cmd/gen-contracts(自写生成器→Go 事件常量/payload 结构体 + TS + OpenAPI 组件结构体/路由注册表) + cmd/mocktarget(本地假靶场，§22.5 双账号/对象归属/跨身份越权)。**有意偏离**:不用 oapi-codegen 改自写生成器(同 sqlc→手写的理由,契约扁平+避免 3.1 支持不全与大依赖),已记注释。契约测试断言生成物与 YAML 同步(忘重跑即 FAIL)。真起服务 curl A→200/读B→403/无身份→401。双模全绿。**至此 M0 五天全部完成**:数据模型(68表)+四大契约冻结+JCS+事件/API 骨架+靶场。下一步:Day 06 进 M1 控制核心。
- **2026-11-16** — Day 06 完成（提交 `461d8df`），**M1 起步**：单写者 Store——internal/store/writer.go（单写 goroutine 串行化所有写，消除 SQLITE_BUSY；durable(synchronous=FULL)/bulk(NORMAL) 两条写连接各 MaxOpenConns(1)；bulk group-commit 用 SAVEPOINT 逐命令错误隔离；后台 wal_checkpoint）+ reader.go（只读池 query_only）。踩坑：submit 初版 select 在 Close 后可能把命令孤立于缓冲通道致 <-res 死锁，已修为关闭优先+等结果时监听关闭；-race 首编 modernc/libc 很慢非死锁。验证 go test -race：3 并发 Worker 各写 1000 事件无 BUSY、revision 累加一致、关闭不死锁。下一步：Day 07。
- **2026-11-16** — Day 06 单写者审计 Q-13/Q-14 处置（提交 `25affd7`）。Q-13：Migrate()/DurableDB() 无运行期防护（Start() 后用会与 writer goroutine 争用 durable 连接）——加 started(atomic) 守卫，Migrate 启动后报错、DurableDB 改名 DurableDBForTest 且启动后 panic + 守卫测试。Q-14：reader 移除冗余 journal_mode(WAL)、checkpoint best-effort 加注释。环境说明：测试机无 C 编译器跑不了 -race，本项目 -race 在 Linux+gcc 上通过，测试方以行为探针补位，建议后续 Linux CI 跑 -race。双模全绿。
- **2026-11-16** — Day 07 完成（提交 `1bb88e5`）：Run 状态机表驱动 + 代码生成 + 属性测试。states.yaml（14 状态/4 终态/34 迁移，§5.1）→ gen.go（go:generate）→ run_sm.gen.go（Next 对任意(状态,事件)有确定结果，未登记显式拒绝不 panic）+ run_states.md（mermaid 状态图）。状态机纯函数化，Guard 由 controller 决定发哪个事件（保持可穷举）。run_controller.go：Apply 在 durable 事务内读态+条件更新（WHERE revision=?）。踩坑：生成器 titleWord 首字母未大写致 Eventpreflight（states 碰巧首字母已大写没暴露），已修。rapid v1.2.0(go1.18) 属性测试 100 组：随机序列永不进未知态、拒绝保持原态、终态吸收；控制器 revision 单调、非法/不存在被拒。go generate 可复现无 diff，基线仍 1.23。下一步：Day 08。
- **2026-11-16** — Q-15 修复（提交 `68e8f27`）：测试模型迁移表对账发现 states.yaml 缺 §5.1/§9.7 规定转移（属性测试检不出「表是否符合 spec」）。核对原文后全部确认并修（迁移 34→45）：① pause 扩至所有非终态（§9.7 L2458），有意例外 STOPPING（stop 不可被 pause 反悔，已标注）；② partial 补 RUNNING 直达（图 L1340）+ BLOCKED（L1348）；③ 新增 reopen EXITING→RUNNING（L1336 门禁发现新工作，此前缺失是功能缺口）；④ RUNNING→COMPLETED 按图保持经 EXITING 单路径（L1351「同一事务」= controller 原子性）。+ 测试锚点与 reopen 控制器测试。双模全绿。
- **2026-11-16** — M0 收尾审计 Q-11/Q-12 处置（提交 `fff046f`）。Q-11：spec §27.5 引用了 §15.6 未登记的 external_message.updated（实现忠实跟随 §15.6、五方对账无误，属 spec 两节打架）——向前补齐，events.yaml 增该事件成为第 50 个，重跑 gen-contracts（若 Owner 更想改 §27.5 可回退）。Q-12：生成器类型映射 default 静默回退 string（同 Q-7 静默吞错）——改为未知类型报错并传播 + 负向测试。双模全绿，五方对账 50/50。
- **2026-11-16** — 按契约冻结日对抗审计（T-017/T-018）收口 Q-7~Q-10（提交 `dcafa37`）。JCS 本体 RFC 8785 全硬边界经测试方独立探针验证合规；Schema 收口三项采纳：Q-7 policy.actions 改 additionalProperties=false（红线开关拼错响亮失败）、Q-8 ValidateAgainst 开启 format 断言（垃圾授权日期被拒）、Q-9 canon 文档记 digest 数值 <2^53 约束。Q-10 op 载荷有意保留宽松（M0 留白，审查认可），加说明待实现各 op 时收口。新增 TestPolicyHardening。趁冻结日廉价窗口改完，未留隐患进契约。下一步：Day 05。
- **2026-11-16** — Day 08 完成（提交 `0e721aa`），**RECONCILING 崩溃一致性枢纽**：Job 状态机 + §4.1 取消 + §31 C3 对账超时。**生成器改造**：gen.go 从单机改**多机通用**（machine 描述符 + 命名前缀参数化），一次驱动 Run/Job 两台机——Run 保持历史无前缀导出 API（Event/Next/AllEvents/AllStates/IsTerminal），Job 用 Job 前缀；内部 map/struct 改名 run*/job* 避免同包冲突（Run 产物纯改名、零语义变化）。**有意偏离/设计决策**：§5.2 只给状态清单 + RECONCILING 文字语义、**没给显式边图**（不同于 §5.1 Run），故 job_states.yaml 的 34 条边按 Job 生命周期语义设计，锚点与理由全写进 yaml 头注供测试模型交叉核对。关键决策：① reconcile_unresolved 落 **CANCELLED**（OUTCOME_UNRESOLVED 是 cancel_code，属取消语义，与其它 cancel_code 一致），真·引擎故障走 fail→FAILED；② cancel 从所有非终态进入**但有意排除 RECONCILING**（对账未完成不得被取消抢先终态，§5.2「对账完成后才能终态化」，与 Run 表 STOPPING 排除 pause 同构）；③ RECONCILING 三条出边（reconcile_retry/reconcile_unresolved/fail）保证非陷阱。**控制器** job_controller.go：Apply（通用事件，durable 条件更新 WHERE revision=?）；RequestCancel（§4.1 路由：有未决 OUTCOME_UNKNOWN 动作→先进 RECONCILING 设截止时刻，否则→CANCELLED 写 cancel_code/actor/detail）；Reconcile（显式进入，无未决则 ErrInvalid）；ExpireReconciliations（注入 now，扫过期 RECONCILING→CANCELLED+cancel_code=OUTCOME_UNRESOLVED+记 run_partial_reasons，单 durable 事务快照一致）。**迁移 0003**（遵守 Day06 起迁移不可变约定，ALTER ADD COLUMN reconcile_deadline，墙钟 RFC3339——跨重启存活，monotonic 不可持久；§31 L5 monotonic 约束针对进程内定时器）。属性测试（rapid）：随机序列永不进未知态；DB 测试覆盖 DoD——取消进 RECONCILING+冻结 Claim（ErrInvalid）、未到期不动/到期终态、run_partial_reasons 有记录、重复扫描幂等为 0。双模 + `-race` 全绿，基线仍 1.23。下一步：Day 09。
- **2026-11-17** — Q-16 处置（提交 `4941a04`）。测试模型 Day 08 审计发现 WAITING_APPROVAL 审批**被拒/过期**出口未定义（只有 approve→RUNNING，余靠 cancel/fail 兜底，spec 侧亦空白）。核对 §5.2 确认是真实空白，分两半：**被拒** 加 `deny: WAITING_APPROVAL→RUNNING`（架构被迫非猜测——被动等待态不能自决终结，审批被拒=该红线动作不可行，Job 去留交规划器在 RUNNING 侧重排，不把「被拒=死」冻进 SM）；**过期** 有意留白延后 Day 15（重发 vs 放弃耦合 Policy Engine/两层 Permit，此刻定边必猜，且无活性缺陷）。与测试方建议 denied→CANCELLED 口径不同已在 TESTING 说明可改。迁移 34→35，补 deny 测试锚点，双模全绿 + generate 幂等。下一步：Day 09。
- **2026-11-17** — Day 09 完成（提交 `33e07f3`），**崩溃安全恢复机制核心**：Claim + Lease + 单调 Fencing（§4.1/§4.3 #2/§31）。新增 internal/scheduler/{claim,lease}.go；attempts 表 schema 已就绪（UNIQUE(job_id,generation)+fencing_token+generation）**无需新迁移**。**Claim**：SM 守卫 READY→CLAIMED + 单写者串行 + revision 条件更新，并发认领同一 Job 恰 1 成功（后到者读到 CLAIMED 被 SM 拒 ErrConflict，UNIQUE 末端兜底）；每次认领增 per-job generation（MAX+1）+ 发放**全局单调** fencing_token（MAX+1），写 Attempt。**设计决策**：fencing_token 选全局单调而非每 Job 计数——既满足「随 generation 单调递增」，又可直接服务 Day 17 Permit/Proxy 等共享资源的陈旧写者拒绝、无需返工；单调性由单写者串行保证，读自持久 attempts 表故跨重启存活（attempts 追加式从不删除=单调前提）。**Fencing 校验**：guardFencingTx/CheckFencing/WriteAttemptResult——写前校验令牌为当代有效值，旧 Attempt（小令牌、非当代 generation）即使进程复活也写不进（§22.4 过期提交被拒）。**Lease**：Renew（过 fencing 校验后延长，陈旧令牌拒续）；ExpireLeases（注入 now 扫过期 CLAIMED：标当代 Attempt EXPIRED + SM lease_expired 回 READY 重领）。**Job 状态机加 lease_expired: CLAIMED→READY**（认领后未执行即失联→安全重领；RUNNING 态过期牵涉已派发动作对账，耦合 Day 12 Reconciler，有意不在此处理）。domain：AttemptState 落地 RUNNING/SUBMITTED/EXPIRED 子集（§43.8）。验证：-race 全绿（16 并发 Claim 恰 1 成功 / 旧 fencing 写入被拒 / 过期增代重领 / 续租校验）。双模全绿，基线仍 1.23。下一步：Day 10。
- **2026-11-17** — Q-17 处置（提交 `9c2b285`）。测试模型 Day 09 审计（info）：guardFencingTx 只校验「令牌=当代」不查 Job 状态，终态 Job 的当代令牌仍能过 fencing 守卫（T-036 P4）。当前无害但 Day 17 Permit/Day 18 Proxy 复用会放行终态 Job。**立即 fail-closed 硬化**（普适不变量、不预设 Day 17 设计、仅几行）：currentFencingTx 顺带读 state，guardFencingTx 先拒终态 Job（ErrConflict）。+ TestFencingRejectsTerminalJob。-race + 双模全绿。下一步：Day 10。
- **2026-11-17** — Day 10 完成（提交 `c8cebaf`），**SSE/审计/对账的事实来源**：Event Journal + Outbox（提交后发布）。新增 internal/events/、internal/outbox/。**events.Append(tx, evt)**：事务内写事件，run 内 sequence=MAX+1 + 全局 global_sequence=MAX+1；tx 由调用方给定——与领域状态变更同事务传入即「提交失败则无事件」（§0.5）；两级序号由单写者串行（durable/bulk 同一 goroutine）保证无空洞。**outbox**：Enqueue(tx) 事务内投递，Publisher.Drain 提交后扫表→Bus→标 published、失败指数退避；bus.Publish 在任何 DB 事务之外（不占单写者、失败不回滚领域事务 §26.9）；至少一次 + published=0 守卫幂等；MemBus 内存实现（Day 40 换 SSE）。**迁移 0004**：events.global_sequence+UNIQUE（§4.2 字段清单只列 run 内 sequence，但 §3151/§4891 明确要全局单调序，属 spec 两节口径差，按 §15.6/§3151 向前补齐）+ outbox 退避列（attempts/next_attempt_at/last_error 持久化跨重启存活）。**连带**：Day 06 并发写探针不走 Append 直插 events，手工分配 per-worker 不相交 global_sequence 以满足新 UNIQUE（仅测试适配 schema 演进）。验证：-race 全绿（回滚无事件/提交后才发布/序号单调无空洞/失败退避不丢/幂等不重发）。双模全绿，基线仍 1.23。下一步：Day 11。
