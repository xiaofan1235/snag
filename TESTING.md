# Snag 测试记录 TESTING

> 本文件由 **本地测试模型** 维护。职责：对每个已完成的开发天（见 `PROGRESS.md` 里标 ✅ 的 Day），
> 按该天 `dev-plan/day-NN.md` 的「## 7. 怎么测试」逐条跑测试，把结果**追加**到下面「测试日志」。
> **Claude 每次开发前会先读本文件顶部的「未解决 bug」**——所以发现的 bug 一定要记在那里，Claude 会优先修。

## 怎么记录（给测试模型的说明）

```
1. 每跑一条测试 = 测试日志里追加一条，字段必须齐全（见下方格式与实例）。
2. 每条必须有：时间戳、对应开发天数 + 文档链接、测试命令、预期、实际、结果(PASS/FAIL)。
3. 结果 FAIL 的，在「未解决 bug」区登记一条（给 bug 编号），Claude 修复后你复测，
   通过了就把该 bug 标「已修复」并补一条复测记录。
4. 不要删历史记录，只追加；bug 状态就地更新。
```

测试条目字段格式：

```
### T-<递增编号>  [PASS|FAIL]
- 时间戳: 2026-11-16T09:20:00Z
- 对应: Day 01（dev-plan/day-01.md） / 里程碑 M0
- 测试项: 一句话说测什么
- 命令: 实际跑的命令
- 预期: 应该出现什么
- 实际: 真实出现什么
- 结果: PASS 或 FAIL
- 关联 bug: 无 / BUG-<编号>
```

---

## 未解决 bug（Claude 开发前必看）

> 有 bug 就在这里登记；Claude 下次开发**先修这里的，再做新功能**。修复后移到最下方「已修复」。

| Bug 编号 | 发现时间 | 对应 Day | 现象 | 严重度 | 状态 |
|---|---|---|---|---|---|
| —— | —— | —— | *(暂无)* | —— | —— |

---

## 测试日志（只追加）

### T-001  [PASS]
- 时间戳: 2026-11-16T08:52:00Z
- 对应: Day 01（dev-plan/day-01.md §7） / 里程碑 M0
- 测试项: `go build ./...` 全仓库编译通过（DoD 第 1 条）
- 命令: `go build ./...`
- 预期: 无错误退出，cmd/ + internal/ 全部包编译通过
- 实际: 退出码 0，无输出（BUILD_OK）
- 结果: PASS
- 关联 bug: 无
- 备注: 本机 Go 1.26.2 windows/amd64。注意 `go build` 不编译 `_test.go`，见 T-003

### T-002  [FAIL]
- 时间戳: 2026-11-16T08:52:30Z
- 对应: Day 01（dev-plan/day-01.md §6 DoD） / 里程碑 M0
- 测试项: `go vet ./...` 无告警
- 命令: `go vet ./...`
- 预期: 干净通过
- 实际: `vet.exe: internal\domain\domain_test.go:219:10: undefined: domain.CoverageCell`，编译失败
- 结果: FAIL
- 关联 bug: BUG-001

### T-003  [FAIL]
- 时间戳: 2026-11-16T08:52:31Z
- 对应: Day 01（dev-plan/day-01.md §7） / 里程碑 M0
- 测试项: `go test ./internal/domain/...` 全绿（Day 01 核心验收命令）
- 命令: `go test ./internal/domain/... -v -count=1`
- 预期: 8 个测试函数全部通过
- 实际: `undefined: domain.CoverageCell` → `FAIL pentagent/internal/domain [build failed]`，0 个用例被执行
- 结果: FAIL
- 关联 bug: BUG-001
- 备注: 全部 8 个用例（枚举值/ID格式/ID唯一单调/Key确定性/构造器/零值不panic/JSON蛇形/领域错误）被编译错误阻断。BUG-001 修复后需复测本条

### T-004  [PASS]
- 时间戳: 2026-11-16T08:58:00Z
- 对应: Day 01（dev-plan/day-01.md §6 DoD「枚举齐全：对照 §43.5 逐一核对，不缺」） / 里程碑 M0
- 测试项: `internal/domain/states.go` 全部枚举对照主文档 §43.5/§43.6/§4.1 人工逐项核对（测试代码本身核对的是常量↔字符串，本条核对的是代码↔规格——测试者独立复核）
- 命令: 静态比对 `states.go` vs `docs/pentagent-dev-spec-v2.6.md` §43.5(L6317-6335)、§43.6(L6339-6345)、§4.1(L1040-1051)
- 预期: 每组枚举不多不少与规格一致
- 实际: 12 组全对齐——RunState 14/14(含 QUEUED 注 §33)、JobState 13/13、ActionState 9/9、FindingState 6/6、VerificationVerdict 3/3、CancelCode 10/10、RecoveryResult 7/7、KnowledgeTrust 3/3、EgressMode 2/2、EnforcementCompleteness 4/4、EgressChannel 4/4(§43.6)、FindingEvidenceRole 8/8(§4.1 finding_evidence.role)。AttemptState/SessionState/HypothesisState/RiskClass/IdempotencyClass/DataClass 按 §43.5 未登记取值集而声明为开放字符串类型，符合「不预埋可能漂移的常量」原则，无枚举漂移
- 结果: PASS
- 关联 bug: 无

### T-005  [PASS]
- 时间戳: 2026-11-16T09:00:00Z
- 对应: Day 01（dev-plan/day-01.md） / 里程碑 M0
- 测试项: 代码格式 gofmt 合规性
- 命令: `gofmt -l .`
- 预期: 无文件列出
- 实际: 列出全部 17 个 .go 文件——经 `git ls-files --eol` 核查为 `i/lf w/crlf`：仓库提交的是 LF，本测试机 `core.autocrlf=true` 检出转 CRLF 的假象，gofmt diff 为整文件纯行尾差异，非真实格式问题
- 结果: PASS（判定为测试环境假象，不算 bug）
- 关联 bug: 无
- 备注: 建议（非 bug）：仓库根加 `.gitattributes`（`*.go text eol=lf`），可消除所有 Windows 检出机器上的该假象，也防止未来有人在 Windows 上回推 CRLF。（已在 d9d9c62 采纳落地）

### T-006  [PASS]
- 时间戳: 2026-11-16T09:30:00Z
- 对应: Day 01（BUG-001 修复 d9d9c62 复测，原 T-002） / 里程碑 M0
- 测试项: `go vet ./...` 复测
- 命令: `go vet ./...`
- 预期: 干净通过（CoverageCell 已定义）
- 实际: 干净通过（VET_OK）
- 结果: PASS
- 关联 bug: BUG-001（关闭）

### T-007  [PASS]
- 时间戳: 2026-11-16T09:30:30Z
- 对应: Day 01（BUG-001 修复 d9d9c62 复测，原 T-003） / 里程碑 M0
- 测试项: `go test ./internal/domain/...` 全绿复测——Day 01 核心验收恢复
- 命令: `go test ./internal/domain/... -count=1`
- 预期: 8 个测试函数全部通过（此前 0 个可执行）
- 实际: `ok pentagent/internal/domain 0.710s`，8/8 通过
- 结果: PASS
- 关联 bug: BUG-001（关闭）
- 备注: 补充核对修复文件质量：`coverage.go` 的 CoverageCell 字段与 §4.1 `coverage_cells`（L1053-1068）11/11 逐字段一致

### T-008  [PASS]
- 时间戳: 2026-11-16T09:31:00Z
- 对应: Day 02（dev-plan/day-02.md §7，提交 9c108c7/7461cb0） / 里程碑 M0
- 测试项: Day 02 默认套件：全仓库 build + vet + test（零外部依赖路径）
- 命令: `go build ./... && go vet ./... && go test ./... -count=1`
- 预期: 全部通过；store 真库用例在无驱动时按设计 SKIP
- 实际: cmd/pentagent、cmd/pentagentd 无测试文件；`ok pentagent/internal/domain`；`ok pentagent/internal/store`（真库用例按设计跳过，见 T-009）
- 结果: PASS
- 关联 bug: 无
- 备注: day-02 §7 原命令 `goose -dir migrations ...` 因开发方无网适配改为内置迁移器（README 已记录三处有意偏离），测试口径相应适配为 T-009

### T-009  [PASS]
- 时间戳: 2026-11-16T09:33:00Z
- 对应: Day 02（dev-plan/day-02.md §7 核心验收 + PROGRESS「未验证项：连真实 SQLite 的迁移+乐观锁用例」） / 里程碑 M0
- 测试项: 真库补验——联网安装 sqlite 驱动跑 `-tags sqlite_modernc` 真连库测试（开发会话无网未验，测试机补验）
- 命令: `go get modernc.org/sqlite && go test -tags sqlite_modernc ./internal/store/... -count=1 -v`
- 预期: 迁移建表成功；乐观锁语义成立——用旧 revision 条件更新影响 0 行（day-02 §7 核心断言）
- 实际: modernc.org/sqlite v1.62.0 安装成功；TestMigrateCreatesTables PASS、TestRunOptimisticLock PASS、TestGetRunNotFound PASS、TestJobInsertGet PASS，4/4 全绿
- 结果: PASS
- 关联 bug: 无
- 备注: 测试后已还原 go.mod / 删除 go.sum，仓库零足迹；驱动留存于本机模块缓存，下次复跑直接 `go get` 即可

### T-010  [PASS]
- 时间戳: 2026-11-16T09:52:00Z
- 对应: Day 03（dev-plan/day-03.md §7，提交 b27ed58） / 里程碑 M0
- 测试项: Day 03 默认套件（零驱动路径）
- 命令: `go build ./... && go vet ./... && go test ./... -count=1`
- 预期: 全绿；真库用例无驱动时按设计 SKIP
- 实际: BUILD_OK / VET_OK / domain ok / store ok
- 结果: PASS
- 关联 bug: 无

### T-011  [PASS]
- 时间戳: 2026-11-16T09:53:00Z
- 对应: Day 03（dev-plan/day-03.md §7 核心验收） / 里程碑 M0
- 测试项: 真库全套：§43.3 全部表建出 + commands 幂等唯一 + external_messages 唯一 + 复合外键跨 Run 拒绝 + Day 02 回归
- 命令: `go test -tags sqlite_modernc ./internal/store/... -count=1 -v`（go.mod 已由 b278425 正式接入 modernc.org/sqlite，本机直跑）
- 预期: 8/8 PASS（开发方自报口径，独立复跑验证）
- 实际: TestAllTablesExist / TestCommandsIdempotencyUnique / TestExternalMessagesUnique / TestFindingEvidenceCrossRunRejected + Day 02 四项回归，8/8 PASS，0.913s
- 结果: PASS
- 关联 bug: 无
- 备注: 复跑不采信自报——Day 03 验收以本机实跑为准

### T-012  [PASS]
- 时间戳: 2026-11-16T09:55:00Z
- 对应: Day 03（§43.3 清单逐项打勾） / 里程碑 M0
- 测试项: 68 表声明独立复核：spec §43.3 权威清单 vs 迁移实际建表 vs 测试 allTables 三方一致
- 命令: 静态比对 docs §43.3(L6273-6305) × migrations/0001+0002 × store_full_test.go allTables
- 预期: 三方数量与名称一致
- 实际: §43.3 清单 67 张 + identities（§43.3 漏列、§4.2 L1106 要求，开发方补列并注释说明）= 68；0001 建 13 + 0002 建 55 = 68；allTables 列 68。Down 段 55 条 DROP 逆序完整
- 结果: PASS
- 关联 bug: 无

### T-013  [PASS]
- 时间戳: 2026-11-16T09:59:00Z
- 对应: Day 03（Owner 新增职责：代码质量审计首轮，覆盖 Day 01-03 全量代码、重点 Day 03） / 里程碑 M0
- 测试项: 代码质量审计：migrations×2 + store 全部 Go 文件 + domain 复审 + 配置卫生
- 命令: 人工全量阅读（1780 行）+ 交叉对照 spec
- 预期: 识别质量问题并按严重度分级
- 实际: 功能性 bug 0；质量问题 4 项 + 信息 2 项，见下方「代码质量审计」小节（Q-1 中 / Q-2~Q-4 低 / Q-5~Q-6 info）
- 结果: PASS（不阻断 Day 03 验收）
- 关联 bug: 无

### T-014  [PASS]
- 时间戳: 2026-11-16T10:19:00Z
- 对应: Day 03 质量审计修复复测（c2635d6 harden(store)，Q-1~Q-6） / 里程碑 M0
- 测试项: 质量审计 6 项修复独立复测：diff 逐项核对 + 双套件实跑 + 新增测试内容核验
- 命令: `go build ./... && go vet ./... && go test ./... -count=1` + `go test -tags sqlite_modernc ./internal/store/... -count=1 -v` + `git diff e17d402..c2635d6` 人工审读
- 预期: 全绿；Q-1 复合外键生效且有测试证明；Q-2 剥尾注+缺标记报错；Q-3 整型子单位；Q-4 索引齐；Q-6 测试增强落地
- 实际: 默认套件全绿（含 3 个迁移器白盒测试，默认模式即跑）；真库 12/12 PASS（新增 TestCrossRunFindingLinkCompositeFK 正反向用例、foreign_key_check、存在但不拥有的 run 用例全部落地）。Q-1 复合外键 `(linked_run_id, linked_finding_id)->findings(run_id,id)` 已建且实测拒绝脏链接；Q-3 `amount` INTEGER/micro_usd + `cost_micro` INTEGER；Q-4 补 10 索引；Q-5 README 不变量三条入「code review 盯住」
- 结果: PASS
- 关联 bug: 无
- 备注: 一条约定备忘（info）：本次修复以就地编辑 0002 方式落实，当前无持久库所以安全；Day 06 出现持久库后已推送迁移应视为不可变，修正走新增迁移文件

### T-015  [PASS]
- 时间戳: 2026-11-16T12:05:00Z
- 对应: Day 04（dev-plan/day-04.md §7，提交 504ae08） / 里程碑 M0
- 测试项: Day 04 默认套件（canon + schemas 新包合入）
- 命令: `go build ./... && go vet ./... && go test ./... -count=1`
- 预期: 全绿
- 实际: BUILD_OK / VET_OK；api/schemas ok、internal/canon ok、domain ok、store ok
- 结果: PASS
- 关联 bug: 无
- 备注: 环境记录：本机拉 proxy.golang.org 持续 EOF，改用 `GOPROXY=https://goproxy.cn,direct` 下载 jcs/jsonschema 两依赖（命令级环境变量，未写配置）。开发方 go 指令钉 1.23、驱动钉 v1.38.2 的决策与实测一致

### T-016  [PASS]
- 时间戳: 2026-11-16T12:08:00Z
- 对应: Day 04（§47.5「同内容对象在任意两处产生相同 digest」立身不变量） / 里程碑 M0
- 测试项: 官方用例独立复跑：键序打乱（含嵌套）同 digest、数字 1/1.0/1e0 归一、Go 值与等价 JSON 同 digest、非法 JSON/重复键报错、RawSHA256 空串锚点、6 组 Schema 正反样例
- 命令: `go test ./internal/canon/... ./api/schemas/... -count=1 -v`
- 预期: 全部 PASS（不采信开发方自报）
- 实际: 全部 PASS；canonical_test 5 个测试 + schema_test 编译自检 + 6 组正反样例全过
- 结果: PASS
- 关联 bug: 无

### T-017  [PASS]
- 时间戳: 2026-11-16T12:12:00Z
- 对应: Day 04（契约冻结日加测：JCS RFC 8785 边界对抗验证，测试方自研探针） / 里程碑 M0
- 测试项: 开发方用例未覆盖的 JCS 硬边界：UTF-16 码元键序（emoji vs U+FFFF）、\u 转义归一（A≡A、\u0009≡\t）、-0≡0、1e2/1E+2≡100、数组顺序敏感性、空白归一；大整数 2^53+1 行为记录
- 命令: 临时探针测试文件（internal/canon/zz_probe_test.go，跑完即删未入库）`go test -run TestProbe -v`
- 预期: RFC 8785 客观要求项全部成立
- 实际: P1-P5 全过——UTF-16 码元排序正确（emoji 键排在 U+FFFF 前）、转义归一、-0/指数归一、数组顺序敏感、空白归一；P6 行为记录：2^53+1 规范化为 2^53（按 double 舍入，一致性成立，见 Q-9）
- 结果: PASS
- 关联 bug: 无
- 备注: gowebpki/jcs 在最难搓错的边界上全部合规，选库决策验证有效

### T-018  [PASS]
- 时间戳: 2026-11-16T12:15:00Z
- 对应: Day 04（契约 Schema 对抗探针） / 里程碑 M0
- 测试项: Schema 疑点观察：S1 policy.actions 拼错开关名（write_mod）是否被拒；S2 垃圾日期串是否被 format 拒；S3 planpatch op 未知字段；S4 ports 越界拒绝（断言）
- 命令: 临时探针（同上，已删）`go test -run "TestProbePolicy|TestProbePlanPatch" -v`
- 预期: S4 必须拒；S1/S2/S3 观察行为记录审计
- 实际: S4 PASS（ports 0/70000 均被拒）；**S1 确认：拼错的 action 开关被放行**（additionalProperties:true）→ Q-7；**S2 确认：垃圾日期串过契约层**（format 未断言）→ Q-8；S3 放行（M0 宽松设计）→ Q-10
- 结果: PASS（S4 断言过；S1/S2 为质量发现不判 FAIL）
- 关联 bug: 无

---

## 代码质量审计（2026-11-16 首轮，随测试执行，Owner 指令新增）

> 覆盖：Day 01-03 全量代码（domain 14 文件 / store 8 文件 / migrations 2 / cmd 2 / 配置）。分级：中=建议随下一提交修；低=择机修；info=知悉即可。功能性 bug 记入上方 bug 表，此处只记质量项。

| 编号 | 严重度 | 位置 | 问题 | 建议 |
|---|---|---|---|---|
| Q-1 | 中 | migrations/0002_full.sql:661 `cross_run_finding_links` | 引用端 `linked_run_id`+`linked_finding_id` 是普通 TEXT，未加复合外键。§43.3 不变量明文「跨 Run 引用一律复合外键校验」，§47 此表的存在意义就是跨 Run 关联；且 `findings` 已有 `UNIQUE(run_id,id)`（0001 已建），加 `FOREIGN KEY (linked_run_id, linked_finding_id) REFERENCES findings(run_id, id)` 零额外成本。当前 schema 接受指向不存在 Finding 的脏链接，且尚无应用层校验兜底 | 补 0003 硬化迁移加复合外键；表头注释的「不加 FK」策略是给 identity_id 类引用的，不适用此表 |
| Q-2 | 低 | internal/store/migrate.go:110 `splitStatements` | 逐行去整行注释后按 `;` 盲切：inline 尾注（本项目大量使用）中的 `;` 或未来 DDL 字符串字面量含 `;` 会静默切出坏语句。注释已自知此限制但无防护 | 切分前剥离行内 `--` 尾注；补一个含 `;` 尾注的用例锁住行为。另 extractUp 对缺 `-- +goose Up` 标记的文件会连 Down 段一起执行，建议改为报错 |
| Q-3 | 低 | 0002_full.sql:276 `budget_ledger.amount REAL`、:337 `model_calls.cost REAL` | 预算账本（§24 RESERVED→SETTLED 的仲裁者）与模型成本用浮点存储，累计会有舍入误差，对账时 RESERVED≠SETTLED 可能恒差尾数 | 改整型微单位（amount_micro INTEGER + unit）或十进制文本；越早改迁移代价越小 |
| Q-4 | 低 | 0002_full.sql 多处 | 部分子表 FK/热路径列缺索引：user_directives.run_id、operations.run_id、tool_recovery_attempts.action_id、memory_outbox(published 扫描路径)、memory_links/gaps.memory_session_id、job_dependencies.depends_on_job_id（反向依赖查询）、evidence_citations.evidence_id、request_capabilities.action_id/attempt_id。当前量小无感，outbox 类表会持续增长 | 后续硬化日统一补一轮索引迁移 |
| Q-5 | info | 全局 | 时间列全 TEXT RFC3339Nano(UTC)，字典序=时间序的前提是**所有写入**都走 store 层 dbT()（强制 UTC）。后续模块（events/outbox 发布器等）若直写 SQL 且用了本地时区，排序即坏 | 保持「Store 是唯一 SQL 入口」（§0.2 已声明）即可，code review 时盯住 |
| Q-6 | info | store_full_test.go:121 | 跨 Run 测试用「不存在的 run」证明复合外键——schema 上成立（finding_evidence 无独立 run_id→runs FK，拒绝只能来自复合 FK），但更严格的版本是造一条「存在但不拥有该 finding 的 run」，顺带证明配对语义。另建议 TestAllTablesExist 末尾加 `PRAGMA foreign_key_check` 一行，迁移级 FK 一致性白捡 | 测试增强，非缺陷 |
| Q-7 | 中 | api/schemas/policy.schema.json:88 `actions` | `additionalProperties:true` 放行拼错的开关名：`"write_mod":"deny"`（实为 write_mode）实测通过契约校验，被静默忽略后回落默认 deny——本例失败方向安全，但红线契约里的配置拼错应当**响亮失败**而非静默吞掉（T-018 S1 探针实测确认） | 契约冻结日是最后的廉价修复窗口：`actions` 改 `additionalProperties:false`（patternProperties 已枚举 6 个合法键）；或保留 true 但在 Policy 装载层拒绝未知键 |
| Q-8 | 低 | api/schemas/policy.schema.json:37 + internal/canon/canonical.go:84 | `format:"date-time"` 未断言：santhosh-tekuri/jsonschema v6 默认不校验 format，垃圾串 `"not-a-date"` 实测通过（T-018 S2）。授权窗口 valid_until 是 §45 硬停依据，坏日期应在校验层就拒 | ValidateAgainst 编译 schema 时启用 format 断言（该库需显式开启），或应用层装载 Policy 时显式 time.Parse |
| Q-9 | info | internal/canon（JCS 数字语义） | 实测 2^53+1 规范化为 9007199254740992（ECMAScript double 舍入，RFC 8785 规定行为）：>2^53 的整数在 digest 里会塌缩，两个不同大数可能同 digest。纳秒时间戳(~1.7e18)最易踩 | §47.1 已规定时间字段不入 digest（正确）；建议 canon 包文档补一句「digest 载荷数值字段必须 <2^53，更大数用字符串」，防后续模块误用 |
| Q-10 | info | api/schemas 多处 | 宽松点：planpatch op items 无 additionalProperties:false 且 op 载荷未约束（实测任意字段放行）；policy methods.endpoint_rules items 无约束；runtime.max_cost 为空 schema `{}` | M0 冻结日的有意留白可接受；后续扩展 op 载荷时注意收口，防止载荷漂移进 digest 锚 |

**做得好的（保持）**：迁移器版本记录与 DDL 同事务（原子）；建表按依赖序 + Down 严格逆序；Day 02 复合外键设计纯净（无冗余单列 FK）；全部查询参数化；t.TempDir/t.Cleanup 测试卫生；PRAGMA 走 DSN（per-connection，不踩连接池坑）；注释全程带 spec 引用可回溯；BUG-001 教训已固化进 .gitignore 注释。

**开发方回应（Q-1~Q-6，已全部采纳并修复，commit `c2635d6`，已复测关闭）**：逐条核对后无站得住的反驳，全部认同。均在 Day 04「契约冻结」前完成，避免把已知问题冻进契约。
- Q-1 ✅ 补复合外键 `(linked_run_id, linked_finding_id)->findings(run_id,id)` + 新测试 `TestCrossRunFindingLinkCompositeFK`（脏链接被拒）。确为疏漏：原「不加 FK」注释只适用 identity_id 类注册引用。
- Q-2 ✅ `splitStatements` 先剥行内尾注再切 `;`；`extractUp` 缺 `-- +goose Up` 改报错 + 内部白盒测试 `migrate_internal_test.go`（默认模式即跑）。
- Q-3 ✅ 去浮点：`budget_ledger.amount`→INTEGER(子单位)、`model_calls.cost`→`cost_micro` INTEGER(微美元)。同意偏中。
- Q-4 ✅ 补齐所列 FK/热点列索引。
- Q-5 ✅ README 记 UTC-文本时间不变量。
- Q-6 ✅ 跨 Run 测试加「存在但不拥有」用例；`TestAllTablesExist` 末尾加 `PRAGMA foreign_key_check`。

**复测结论（T-014，2026-11-16T10:15Z）：Q-1~Q-6 全部修复确认，关闭。** 新增测试质量核验：`TestCrossRunFindingLinkCompositeFK` 含正向用例（合法跨 Run 关联应被接受）+ 反向用例（脏链接拒绝）——双向都测，写法正确；迁移器白盒测试默认模式即跑。一条约定备忘（info，非问题）：本次以**就地编辑 0002_full.sql** 方式落实修复，当前无持久库（测试库全是临时目录）所以安全；Day 06 出现持久库后，已推送迁移文件应视为不可变，后续 schema 修正一律走新增迁移文件。

**Day 04 审计补充（T-017/T-018，2026-11-16T12:15Z）：新增 Q-7(中)/Q-8(低)/Q-9/Q-10(info)。** JCS 实现本体在 RFC 8785 全部硬边界合规（UTF-16 键序/转义归一/-0/指数/数组敏感/空白，对抗探针实测）；问题集中在契约 Schema 的收口：Q-7 红线开关名拼错被静默放行（additionalProperties:true）、Q-8 日期格式未断言。**契约已冻结，Q-7/Q-8 建议开发方在进入 Day 05 前处置**（改 schema 文件本身即可，尚无代码依赖旧版宽松行为）。
验证：默认 build/vet/test 全绿（+3 迁移器内部测试）；`-tags sqlite_modernc` 真库 11/11 PASS。请复测确认。

---

## 已修复 bug（归档）

| Bug 编号 | 修复时间 | 修复 Commit | 复测结果 |
|---|---|---|---|
| BUG-001 | 2026-11-16 | `d9d9c62` | ✅ 复测通过（T-006/T-007，2026-11-16T09:30Z）：vet 干净、domain 8/8 全绿，Day 01 验收恢复；coverage.go 字段与 §4.1 coverage_cells 11/11 一致。根因：`.gitignore` 的 `coverage.*` 误伤源码，已改为具体产物名并补提交；另采纳 T-005 建议加 `.gitattributes`(行尾 LF) |
