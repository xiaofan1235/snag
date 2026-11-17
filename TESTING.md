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
| BUG-002 | 2026-11-17T12:00Z | Day 22 | `internal/execution/toolruntime.go:192` 用 `syscall.SysProcAttr{Setpgid: true}`（Unix-only 字段）——**Windows 构建直接失败**（vet: unknown field Setpgid）。Unix 依赖仅此一行（其余 Signal/WaitDelay 跨平台）。开发环境应为 Linux 故未暴露；测试机为 Windows，本地临时去掉该字段后 21 包全部构建+测试通过（证明无其他平台依赖）。进程组收割语义在 Unix 不受影响，仅测试机本地补丁期间行为略有差异 | 中（阻断 Windows 构建/测试；Linux 生产不受影响） | **已修复关闭（`0687407`，复测 T-055）**：拆 build tag——proc_unix.go（//go:build unix 设 Setpgid+SIGTERM）/proc_other.go（//go:build !unix 留空）；测试机裸 Windows 原生构建+vet+双模式全套件通过，不再需要本地补丁 |

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

### T-019  [PASS]
- 时间戳: 2026-11-16T12:46:00Z
- 对应: Day 04 审计修复复测（dcafa37 harden(contract)，Q-7/Q-8，定时监控触发） / 里程碑 M0
- 测试项: Q-7/Q-8 修复独立复测：探针断言「拼错开关名 write_mod 被拒 + 正确键 write_mode 通过」「垃圾日期 not-a-date 被拒 + 合法 RFC3339 通过」双向各验
- 命令: 临时探针（zz_retest_probe_test.go，跑完即删）`go test -run TestRetest -v` + 全量 `go test ./... -count=1`
- 预期: 双探针 PASS；AssertFormat 启用后不误伤既有合法样例
- 实际: TestRetestQ7TypoSwitchRejected / TestRetestQ8GarbageDateRejected 双 PASS；全套件绿（canon/schemas/domain/store）。Q-9 文档警示落地、Q-10 有意保留（理由成立：PlanPatch 非 digest 锚，经 PlanCompiler 应用）
- 结果: PASS
- 关联 bug: 无
- 备注: 开发方新增 TestPolicyHardening 与我的探针口径一致；Q-7~Q-10 全部闭环

### T-020  [PASS]
- 时间戳: 2026-11-16T12:57:00Z
- 对应: Day 05（dev-plan/day-05.md §7，提交 c7e93d6，定时监控触发） / 里程碑 M0 收尾
- 测试项: Day 05 默认套件 + §7 官方用例独立复跑（mocktarget 越权语义 + 契约完整性）
- 命令: `go build ./... && go vet ./... && go test ./... -count=1`（覆盖 §7 的 `go test ./api/... ./cmd/mocktarget/...`）
- 预期: 全绿
- 实际: 全绿——api（契约测试）、api/schemas、cmd/mocktarget（归属语义 6 断言）、canon、domain、store 全过
- 结果: PASS
- 关联 bug: 无

### T-021  [PASS]
- 时间戳: 2026-11-16T13:00:00Z
- 对应: Day 05（§15.6 事件注册表 + 三方生成物一致性独立复核） / 里程碑 M0
- 测试项: spec §15.6 事件清单 × events.yaml × expectedEvents × events_gen.go × events.gen.ts 五方对账
- 命令: awk/grep 精确提取 §15.6 行首事件名 vs contracts_test.go expectedEvents vs 两个生成物，逐一 diff
- 预期: 数量与名称完全一致
- 实际: 五方一致，各 49 个，diff 为空；Go/TS/YAML 同步有测试锁定。但发现 **spec 自身内部不一致**：§27.5(L4613) 声称 `external_message.updated`「已登记于 15.6」，§15.6 实无此事件 → Q-11
- 结果: PASS（开发方实现与权威登记节一致；不一致在 spec 侧）
- 关联 bug: 无（Q-11 为规格问题）

### T-022  [PASS]
- 时间戳: 2026-11-16T13:02:00Z
- 对应: Day 05（mocktarget 靶场对抗探针，测试方自研） / 里程碑 M0
- 测试项: 开发方用例外行为：Bearer 头路径（含大小写/双空格边界）、新建对象归属隔离（A 建 B 读 403）、list 跨账号泄漏、限流 429（121 次触发 + 跨账号不影响）、redirect/slow 行为与上限截断、reset 清理
- 命令: 临时探针（cmd/mocktarget/zz_probe_test.go，跑完即删）`go test -run TestProbe -v`
- 预期: 全部符合设计语义
- 实际: 5/5 PASS——Bearer token-a 200 / 小写 bearer 401 / 双空格 401；A 新建对象 B 读 403；A 列表不含 order-2001；第 121 次 429 且 B 不受影响；redirect 302+Location 正确；slow 50ms 与 5000ms 截断正确；reset 后新建对象 404
- 结果: PASS
- 关联 bug: 无
- 备注: 行为记录（供 Day 19+ Policy Proxy 测试参考）：Bearer 前缀大小写敏感；404/403 区分是故意的 IDOR 差异靶点；/admin/reset 无鉴权（仅 127.0.0.1 测试件）；/redirect 为开放重定向（by design 测重定向策略）

### T-023  [PASS]
- 时间戳: 2026-11-16T13:14:00Z
- 对应: M0 收尾审计修复复测（fff046f，Q-11/Q-12，定时监控触发） / 里程碑 M0
- 测试项: Q-11/Q-12 处置复测：全套件实跑 + 50 事件链核对 + 生成器错误传播代码核验
- 命令: `go test ./... -count=1` + grep 核对 events.yaml/生成物/spec §15.6/测试注释 + `git show fff046f -- cmd/gen-contracts/main.go`
- 预期: Q-12 未知类型响亮报错；Q-11 五方 50 事件同步
- 实际: 套件全绿（含新 gen_test.go 负向测试）；**Q-12 关闭**（goScalar/tsScalar/goType 均返回带上下文 error 并传播）；**Q-11 部分修复**——events.yaml/events_gen.go/events.gen.ts/expectedEvents 均含 external_message.updated（50 个）✅，但 spec §15.6 仍为 49（未补登记行）、contracts_test.go:16 注释仍写「49 个」
- 结果: PASS（Q-12 关闭；Q-11 留机械收尾，不阻断验收）
- 关联 bug: 无
- 备注: Q-11 残留两行级改动：spec §15.6 补一行登记 + 测试注释 49→50。不修的后果：登记链断裂，下次「events.yaml 对照 §15.6」对账会把第 50 个事件误判为多余而回退

### T-024  [PASS]
- 时间戳: 2026-11-16T13:34:00Z
- 对应: M0 收尾审计修复复测（bdc091e，Q-11 残留收尾，定时监控触发） / 里程碑 M0
- 测试项: Q-11 收尾验证：spec §15.6 登记行落地 + 测试注释更新 + 套件回归
- 命令: grep 登记行位置核对（L3110 落 §15.6 内、L4614 §27.5 引用有据）+ `go test ./... -count=1`
- 预期: spec 登记链恢复（§15.6=50 与 events.yaml/生成物/expectedEvents 一致）；注释 49→50
- 实际: contracts_test.go:16 注释已改「50 个，含 Q-11 补登的 external_message.updated」；spec §15.6 补登行在位；全套件绿。登记链 spec→yaml→Go→TS 全部 50 对齐
- 结果: PASS
- 关联 bug: 无
- 备注: Q-11 关闭。测试方工具修正备案：本测试方的的事件提取正则 `[a-z]+\.` 匹配不了首段带下划线的事件名（external_message），T-021 的对账方法已随之修正——后续对账用完整名单 diff 而非行首正则

### T-025  [PASS]
- 时间戳: 2026-11-16T13:50:00Z
- 对应: Day 06（dev-plan/day-06.md §7，提交 bacae41，定时监控触发） / 里程碑 M1 起步
- 测试项: Day 06 默认套件
- 命令: `go build ./... && go vet ./... && go test ./... -count=1`
- 预期: 全绿
- 实际: 全绿（api/schemas/gen-contracts/mocktarget/canon/domain/store）
- 结果: PASS
- 关联 bug: 无

### T-026  [PASS]
- 时间戳: 2026-11-16T13:52:00Z
- 对应: Day 06（§7 验收：3 Worker × 1000 事件无 SQLITE_BUSY + 数据完整） / 里程碑 M1
- 测试项: 真库全套（不带 -race）+ 环境限制记录
- 命令: `go test -tags sqlite_modernc ./internal/store/... -count=1 -v`
- 预期: 14/14 PASS，含 TestWriterConcurrencyNoBusy（无 BUSY、事件数 3×1000、revision 累加正确）
- 实际: 14/14 PASS（新增 2 项 writer 测试 + 既有 12 项回归）
- 结果: PASS
- 关联 bug: 无
- 备注: **-race 环境限制**：本测试机无 C 编译器（gcc/mingw 缺失），`go test -race` 需 cgo 无法执行——开发方自报「-race 全绿」在本机**不可独立复现**。已在 T-027 用行为级探针补位（故障隔离/幂等/并发读写），内存竞态检测仍缺独立验证。建议：开发方注明其 -race 运行平台，或后续 CI（Linux）跑 -race 锁定

### T-027  [PASS]
- 时间戳: 2026-11-16T13:56:00Z
- 对应: Day 06（测试方对抗探针，补偿 -race 不可用） / 里程碑 M1
- 测试项: durable 连接语义（synchronous=FULL/journal=wal）、group-commit 故障隔离（4 goroutine × 100 轮「1 合法 + 1 必失败」混发，合法 400 条须全落库且失败命令确实报错）、Close 双重幂等、写中并发读
- 命令: 临时探针（internal/store/zz_day06_probe_test.go，跑完即删）`go test -tags sqlite_modernc -run TestProbe -v`
- 预期: 4/4 PASS
- 实际: 4/4 PASS——durable=2(FULL)+wal 确认；SAVEPOINT 故障隔离在真实争用下成立（dup-key 恰 1 条、合法 400/400）；Close 幂等；并发读写无错误、终态 500 条精确
- 结果: PASS
- 关联 bug: 无
- 备注: 探针 D 首跑失败是探针自身 bug（漏插 runs 行触发 events 外键拒绝、且错误被丢弃），修正后通过——writer/reader 实现无恙

### T-028  [PASS]
- 时间戳: 2026-11-16T14:17:00Z
- 对应: Day 06 审计修复复测（25affd7，Q-13/Q-14，定时监控触发） / 里程碑 M1
- 测试项: Q-13 守卫与 Q-14 清理独立复测
- 命令: `go build ./... && go vet ./... && go test ./... -count=1` + `go test -tags sqlite_modernc ./internal/store/... -count=1 -v` + diff 审读
- 预期: 守卫生效（Migrate 启动后报错 / DurableDBForTest 启动后 panic / Start 幂等）；reader 去掉冗余 journal_mode pragma；套件全绿
- 实际: 全部符合——真库 15/15 PASS（新增 TestMigrateAfterStartRejected）；diff 确认 started(atomic.Bool) 守卫、DurableDB→DurableDBForTest 改名+panic、Start Swap 幂等、reader DSN 去掉 journal_mode 并注释说明
- 结果: PASS
- 关联 bug: 无
- 备注: Q-13/Q-14 关闭。至此 Day 06 审计零未决项

### T-029  [PASS]
- 时间戳: 2026-11-16T14:20:00Z
- 对应: Day 07（dev-plan/day-07.md §7，提交 1bb88e5，定时监控触发） / 里程碑 M1
- 测试项: Day 07 默认套件 + §7 官方用例独立复跑（纯状态机 + 属性测试 + controller）
- 命令: `go build ./... && go vet ./... && go test ./... -count=1`
- 预期: 全绿
- 实际: 全绿，internal/control 含 5 组测试（状态集对照/Stop-Fail 全非终态/终态无出边/rapid 属性/关键迁移锚点 + controller CAS）
- 结果: PASS
- 关联 bug: 无
- 备注: `go generate ./...` 后 git status 干净——生成物与 states.yaml 同步 ✅

### T-030  [FAIL→发现]
- 时间戳: 2026-11-16T14:28:00Z
- 对应: Day 07（迁移表 × spec §5.1/§9.7 独立对账，测试方核心职责） / 里程碑 M1
- 测试项: states.yaml 迁移表对照主文档 §5.1（L1322-1355）与 §9.7（L2450-2458）逐条核对 + 行为级探针证实
- 命令: 静态对照 + 临时探针（internal/control/zz_probe_test.go，已删）`go test -run TestProbeSpecGaps -v`
- 预期: 迁移表覆盖 spec 全部规定转移
- 实际: **6 项偏差行为级证实**（Next() 实测全为 ok=false）：① QUEUED/BLOCKED/RESUMING 等 9 个非终态收 pause 被拒——§9.7 明文「所有非终态均可进入 PAUSING」；② BLOCKED→PARTIAL 不存在——§5.1 L1348「等待审批期间授权到期或预算耗尽时允许迁移到 PARTIAL/STOPPING/FAILED」；③ **EXITING→RUNNING 无任何事件可达**——§5.1 L1336 图示明确「门禁发现新工作」回 RUNNING，缺此转移则退出门禁发现新工作时 Run 只能错误终结；④ RUNNING→COMPLETED 直达口径——§5.1 L1351「RUNNING/EXITING → COMPLETED 在同一逻辑事务」措辞含直达路径（口径解释项）
- 结果: FAIL（实现符合自己的表，但表缺 spec 规定的转移）→ Q-15
- 关联 bug: Q-15
- 备注: 属性测试无法发现此类问题——它验证「实现符合 states.yaml」，不验证「states.yaml 符合 spec」。这正是人工对账的价值所在

### T-031  [PASS]
- 时间戳: 2026-11-16T14:36:00Z
- 对应: Day 07 审计修复复测（68e8f27，Q-15，定时监控触发） / 里程碑 M1
- 测试项: Q-15 四类缺口复测 + 报备偏差核验 + 生成物同步
- 命令: 临时探针（zz_retest_test.go，已删）`go test -run TestRetest -v` + `go generate ./...` 同步检查 + 全套件
- 预期: ①②③ 关闭，④ 按书面解读保持单路径
- 实际: ① pause 从 9 个非终态全部可入 PAUSING（STOPPING 按报备排除——stop 不可逆语义，理由成立）；② BLOCKED→PARTIAL ✅；③ reopen: EXITING→RUNNING ✅（controller 加 TestControllerExitingReopen）；RUNNING 直达 PARTIAL ✅；④ complete 保持 EXITING 单路径（L1351 解读为 controller 原子性，图示本为单路径，解读成立）；迁移 34→45；`go generate` 后工作区干净；全套件绿
- 结果: PASS
- 关联 bug: 无
- 备注: Q-15 关闭。STOPPING-pause 排除是开发方报备的对 §9.7 字面的有意偏差，理由（Stop 不可逆）成立——记录在案，如 Owner 有异议可要求 spec 侧措辞澄清

### T-032  [PASS]
- 时间戳: 2026-11-16T15:02:00Z
- 对应: Day 08（dev-plan/day-08.md §7，提交 0e721aa，定时监控触发） / 里程碑 M1
- 测试项: Day 08 默认套件 + 真库套件 + 生成物同步 + 新迁移核验
- 命令: `go build/vet/test ./... -count=1` + `go test -tags sqlite_modernc ./internal/...` + `go generate ./...` 后 git status 检查
- 预期: 全绿；0003 走新迁移文件（不改 0001/0002）
- 实际: 全部通过；`go generate` 后工作区干净；0003_reconcile.sql 只做 ALTER ADD COLUMN（reconcile_deadline），文件头写明迁移不可变约定 ✅；墙钟 vs monotonic 的选型理由（deadline 需跨进程存活）成立
- 结果: PASS
- 关联 bug: 无

### T-033  [PASS]
- 时间戳: 2026-11-16T15:05:00Z
- 对应: Day 08（job_states.yaml × spec §5.2/§4.1 独立对账 + RECONCILING 枢纽行为探针） / 里程碑 M1
- 测试项: 13 态对照 §5.2 清单；RECONCILING 语义（冻结 Claim/双出口/cancel 排除/fail 容灾/终态吸收）；§7 验收路径（超时→OUTCOME_UNRESOLVED→run_partial_reasons）实现与断言核验
- 命令: 静态对照 + 临时探针（zz_probe_test.go，已删）`go test -run TestProbeReconcilingHub -v` + controller/测试代码审读
- 预期: 全部符合 §5.2 文字语义
- 实际: 13/13 态一致；探针 6 断言全过（claim 冻结/cancel 拒绝/fail 放行/reconcile_retry→RETRYING/reconcile_unresolved→CANCELLED/终态吸收）；TestJobReconcileTimeoutTerminalizes 断言到 cancel_code 与 run_partial_reasons 的 code+entity_ref 字段级 ✅。开发方本次在 YAML 预埋「对测试模型」设计说明（Q-15 后流程改进，好现象）
- 结果: PASS
- 关联 bug: 无
- 备注: ① 审批拒绝/过期的 Job 出口未定义（spec 侧空白）→ Q-16；② 租约净过期（无未决动作）的重排路径 = RUNNING→reconcile→RECONCILING→reconcile_retry→RETRYING→retry→READY，绕行成立（表意自洽）；③ verify_retry 直达 RETRYING 的口径（§5.2「存在未决→先 RECONCILING」的触发锚在 RUNNING 侧）记录在案

### T-034  [PASS]
- 时间戳: 2026-11-17T03:49:00Z
- 对应: Day 08 审计修复复测（4941a04，Q-16，定时监控触发） / 里程碑 M1
- 测试项: Q-16 处置复测：deny 边行为 + 过期留白声明核验 + 全套件回归
- 命令: 临时探针（zz_rt_test.go，已删）`go test -run TestRetestQ16 -v` + `go test ./... -count=1` + `go generate` 同步检查
- 预期: deny: WAITING_APPROVAL→RUNNING 生效；approve 不变；无 expire 事件（与留白声明一致）；套件全绿
- 实际: 全部符合。处置理由成立：被动等待态不自行终结、动作侧转 POLICY_DENIED、Job 去留由规划器 RUNNING 侧重排——不把争议语义冻进 SM；过期留白到 Day 15 有无活性缺陷论证（cancel/fail 恒为安全出口）+ spec 空白依据（L3085 只有 deny API）
- 结果: PASS
- 关联 bug: 无
- 备注: Q-16 关闭（过期半边转 Day 15 跟踪，届时 Policy Engine 落地时验收）。Day 08 审计零未决项

### T-035  [PASS]
- 时间戳: 2026-11-17T03:54:00Z
- 对应: Day 09（dev-plan/day-09.md §7，提交 33e07f3，定时监控触发） / 里程碑 M1
- 测试项: Day 09 默认套件 + 真库专项（并发 Claim 单胜/过期增代/陈旧 fencing 拒绝）
- 命令: `go build/vet/test ./... -count=1` + `go test -tags sqlite_modernc ./internal/scheduler/... -v`
- 预期: 全绿（§7 验收三条全覆盖）
- 实际: 全绿——默认 9 包 ok；scheduler 真库 3/3（TestConcurrentClaimSingleWinner / TestLeaseExpiryRegenAndStaleFencingRejected / TestRenewFencing）
- 结果: PASS
- 关联 bug: 无
- 备注: -race 仍受本机无 gcc 限制（沿用 T-026 口径，行为级探针补位见 T-036）

### T-036  [PASS]
- 时间戳: 2026-11-17T03:59:00Z
- 对应: Day 09（对抗探针 + 代码审计） / 里程碑 M1
- 测试项: 跨 Job 全局 fencing 单调、双 Claim 拒绝、杀进程复活全链（过期→增代重领→旧令牌写/续/验三路全拒→新令牌可用）、终态 Job fencing 行为观察；claim.go/lease.go 审读
- 命令: 临时探针（internal/scheduler/zz_probe_test.go，已删）`go test -tags sqlite_modernc -run TestProbe -v`
- 预期: P1-P3 硬断言全过；P4 记录行为
- 实际: 4/4 PASS——全局令牌连续单调（跨 Job b=a+1）；重复 Claim ErrConflict；复活全链正确（generation 与 fencing_token 均递增、旧令牌 Write/Renew/Check 三路 ErrConflict、新令牌写成功）；P4 确认 guardFencingTx 不查 Job 终态（CANCELLED Job 当代令牌仍过 fencing）→ Q-17
- 结果: PASS
- 关联 bug: 无
- 备注: 实现质量高：MAX+1 发号依赖 attempts 追加式（注释明确单调性前提）；ExpireLeases 只收 CLAIMED 干净场景、RUNNING 有意留给 Day 12（范围切割正确）；UNIQUE(job_id,generation) 末端兜底在 0001 已建；AttemptState 三个取值按 §43.8 实现期细化原则落地

### T-037  [PASS]
- 时间戳: 2026-11-17T04:08:00Z
- 对应: Day 09 审计修复复测（9c2b285，Q-17，定时监控触发） / 里程碑 M1
- 测试项: Q-17 fail-closed 存活性守卫复测：终态 Job 当代令牌三路（Check/Write/Renew）全拒
- 命令: 临时探针（zz_rt_test.go，已删）`go test -tags sqlite_modernc -run TestRetestQ17 -v` + 全套件
- 预期: 终态 Job fencing 全拒；套件全绿
- 实际: 探针 PASS（CANCELLED Job 当代令牌 CheckFencing/WriteAttemptResult/Renew 三路 ErrConflict）；9 包全绿；scheduler 真库 4/4（含开发方新增 TestFencingRejectsTerminalJob）
- 结果: PASS
- 关联 bug: 无
- 备注: Q-17 关闭——开发方未等 Day 17 直接以 fail-closed 方式处置（守卫叠加存活性），比留跟踪项更稳。Day 09 审计零未决项

### T-038  [PASS]
- 时间戳: 2026-11-17T04:12:00Z
- 对应: Day 10（dev-plan/day-10.md §7，提交 c8cebaf，定时监控触发） / 里程碑 M1
- 测试项: Day 10 默认套件 + 真库专项（回滚无事件/提交有序/publisher 重试不丢）
- 命令: `go build/vet/test ./... -count=1` + `go test -tags sqlite_modernc ./internal/events/... ./internal/outbox/... -v`
- 预期: 全绿（§7 三条全覆盖）
- 实际: 11 包全绿；events 2/2（TestAppendSequencesMonotonic / TestAppendRollbackNoEvent）+ outbox 3/3（TestStoreThenPublish / TestRollbackNoMessage / TestPublisherRetryNoLoss）
- 结果: PASS
- 关联 bug: 无
- 备注: 0004 迁移守不可变约定；global_sequence 的 spec 口径差（§4.2 只列 run 内 sequence vs §15.6/§3151 要求全局序）处理有据（向前补齐并注释）

### T-039  [PASS]
- 时间戳: 2026-11-17T04:18:00Z
- 对应: Day 10（组合探针 + 代码审计） / 里程碑 M1
- 测试项: 真实用法组合验证：领域变更+events.Append+outbox.Enqueue **同一事务**——提交侧全落、回滚侧（事务尾 FK 违反）全无且 run revision 不动；Drain 恰好一次 + 二次 Drain 为 0（幂等标记）；events.go/outbox.go/bus.go/publisher.go 审读
- 命令: 临时探针（internal/outbox/zz_probe_test.go，已删）`go test -tags sqlite_modernc -run TestProbeComposite -v`
- 预期: 组合原子性成立
- 实际: PASS——提交侧 ev=1/ob=1，回滚侧 ev=0/ob=0/revision=1（未动）；Drain n=1、bus 收 1、二次 Drain n=0
- 结果: PASS
- 关联 bug: 无
- 备注: 设计质量高：Append/Enqueue 接收调用方 tx——同事务原子性靠构造保证而非纪律；publisher 三段式（只读池取批/事务外 Publish/单写者标记）+ WHERE published=0 幂等守卫 + 持久化指数退避。info 一条见 Q-18（至少一次语义的消费者幂等要求，Day 40 SSE 接入时需去重）

### T-040  [PASS]
- 时间戳: 2026-11-17T07:45:00Z
- 对应: Day 11（dev-plan/day-11.md §7，提交 aa12f99，定时监控触发） / 里程碑 M1
- 测试项: Day 11 默认套件 + 真库专项（同 key 重试/并发/revision 冲突）
- 命令: `go build/vet/test ./... -count=1` + `go test -tags sqlite_modernc ./internal/control/... -v`
- 预期: 全绿（§7 三条全覆盖）
- 实际: 11 包全绿；control 真库全过（含新增 idempotency 测试组 + 全部既有回归）
- 结果: PASS
- 关联 bug: 无

### T-041  [PASS]
- 时间戳: 2026-11-17T07:52:00Z
- 对应: Day 11（对抗探针 + 代码审计） / 里程碑 M1
- 测试项: 幂等语义五分支行为级验证：①首次非回放 ②同 key 同 digest 重试回放且**不重执行 fn**（执行计数器佐证）refs 相同 ③同 key 不同 digest → ErrConflict ④**fn 失败整体回滚**（无命令行无对象）后同 key 重试**真正重执行** ⑤8 goroutine 并发同 key → fn 恰执行 1 次全员同 refs；commands.go 审读
- 命令: 临时探针（internal/control/zz_probe_test.go，已删）`go test -tags sqlite_modernc -run TestProbe -v`
- 预期: 全部分支符合 §15.7 语义
- 实际: 5/5 全过——含最容易做错的 ④（失败回滚后重试必须真正重执行，网络超时重试正确性的根基）
- 结果: PASS
- 关联 bug: 无
- 备注: 实现质量高：单事务「查重→fn→记账」+ 单写者串行消除 SELECT-then-act 竞争 + PK 末端兜底；GuardRevision 表名白名单防注入；Transition 终态不可变 + COALESCE 保 refs。零新审计发现（Q-18 之外无新增）

### T-042  [PASS]
- 时间戳: 2026-11-17T08:15:00Z
- 对应: Day 12（dev-plan/day-12.md §7，提交 c0c2b34，手动触发） / 里程碑 M1
- 测试项: Day 12 默认套件 + 真库专项（崩溃现场恢复/孤儿重上架/outbox 补发/幂等/超时扫描）
- 命令: `go build/vet/test ./... -count=1` + `go test -tags sqlite_modernc ./internal/control/... -v`
- 预期: 全绿（§7「造崩溃现场→Reconcile→断言状态与顺序」全覆盖）
- 实际: 11 包全绿；新增 5 个恢复测试全过（TestReconcileDispatchedCrash / TestReconcileClaimedOrphanReoffer / TestReconcileDrainsOutbox / TestReconcileCleanStartIdempotent / TestSweepTimeoutsReconcileExpiry）
- 结果: PASS
- 关联 bug: 无
- 备注: pentagentd main 已按 §19 接线（①开库迁移→Reconcile→之后才 Scheduler/API），次序由调用顺序保证

### T-043  [PASS]
- 时间戳: 2026-11-17T08:21:00Z
- 对应: Day 12（僵尸 fencing 探针 + 代码审计） / 里程碑 M1
- 测试项: §5.3 崩溃语义端到端：造 RUNNING 孤儿现场（含 DISPATCHED 动作+未发 outbox）→ Reconcile → 断言恢复计数、**僵尸旧令牌三路全拒**（增代饿死）、DISPATCHED→OUTCOME_UNKNOWN 三不原则、Job 入 RECONCILING 且 generation+1、二次 Reconcile 幂等全零；reconciler.go + pentagentd 接线审读
- 命令: 临时探针（internal/control/zz_probe_test.go，已删）`go test -tags sqlite_modernc -run TestProbeZombie -v`
- 预期: 全部符合 §5.3/§19
- 实际: PASS——恢复计数精确（1/1/1/1/1）；僵尸 Write/Check/Renew 三路 ErrConflict；动作 OUTCOME_UNKNOWN；generation 前移；幂等复跑全零
- 结果: PASS
- 关联 bug: 无
- 备注: 设计诚实：§19 ②③（全局 Pause、Permit）等未建组件留**显式占位注释**而非假装完成；SweepTimeouts 现只接 RECONCILING 超时，其余阻塞态超时按组件落地日映射（Day 13-17）。零新审计发现

### T-044  [PASS]
- 时间戳: 2026-11-17T08:30:00Z
- 对应: Day 13（dev-plan/day-13.md §7，提交 2edbb40，手动触发） / **里程碑 M1 收尾**
- 测试项: Day 13 默认套件 + 真库专项（CLI 生命周期/非法迁移/故障注入、API server、faultinject）
- 命令: `go build/vet/test ./... -count=1` + `go test -tags sqlite_modernc ./cmd/... ./internal/api/... ./internal/faultinject/... -v`
- 预期: 全绿（§7「create→start→pause→resume→stop 脚本断言流转」全覆盖）
- 实际: 14 包全绿；CLI 3 测试（生命周期/非法迁移/故障注入）、API 3 测试（创建/状态/不存在启动）、faultinject 4 测试（默认禁用/武装/一次性/清除）全过
- 结果: PASS
- 关联 bug: 无

### T-045  [PASS]
- 时间戳: 2026-11-17T08:36:00Z
- 对应: Day 13（HTTP 层对抗探针 + 代码审计） / 里程碑 M1
- 测试项: HTTP 边界：坏 mode 400、DRAFT resume 409 invalid_state、全周期 DRAFT→RUNNING→PAUSED→RUNNING→STOPPED、终态再 stop 409、故障注入 500 fault_injected **且状态不变**、注入消耗后重试成功；server.go/faultinject.go/controller 增量审读
- 命令: 临时探针（internal/api/zz_probe_test.go，已删）`go test -tags sqlite_modernc -run TestProbe -v`
- 预期: 全部边界行为正确
- 实际: 2/2 PASS
- 结果: PASS
- 关联 bug: 无
- 备注: 设计亮点：生命周期多步迁移（start=preflight→start 等）非原子但每个中间态都是可恢复的合法驻留态（PAUSING/STOPPING 有定义的恢复路径），语义自洽；faultinject 生产默认全局禁用（Hook 是一次布尔判断，近零开销）；错误映射稳定（not_found/invalid_state/conflict/fault_injected 机读 code）。零新审计发现

**🎉 里程碑 M1（Day 06-13）全部验收通过：单写者 Store、Run/Job 状态机、Claim/Lease/Fencing、Event Journal/Outbox、幂等账本、Reconciler、CLI/API 骨架。M0+M1 = Day 01-13 全绿，Q-1~Q-18 闭环（Q-18 info 跟踪 Day 40）。**

### T-046  [PASS]
- 时间戳: 2026-11-17T08:45:00Z
- 对应: Day 14（dev-plan/day-14.md §7，提交 3af1c1d，手动触发） / 里程碑 M2 开端
- 测试项: Day 14 默认套件 + `go generate` 同步
- 命令: `go build/vet/test ./... -count=1` + `go generate ./...` 后 git status 检查
- 预期: 全绿；生成物同步
- 实际: 15 包全绿（新增 internal/execution）；生成物同步 ✅
- 结果: PASS
- 关联 bug: 无

### T-047  [PASS]
- 时间戳: 2026-11-17T08:50:00Z
- 对应: Day 14（§5.3 对账 + 不变量探针 + 代码审计） / 里程碑 M2
- 测试项: action_states.yaml 对照 §5.3 逐条（9 态/正常链/审批分支/崩溃双出口/取消范围/终态）；行为探针六组：正常链到 RECORDED 终态、dispatch→unknown→reconcile_record/cancel 双出口、**DISPATCHED 与 OUTCOME_UNKNOWN 均拒 cancel（不得绕过对账）**、3 终态×12 事件吸收、审批双分支、digest 嵌套键序不变性；action.go 审读
- 命令: 临时探针（internal/execution/zz_probe_test.go，已删）`go test -run TestProbeAction -v`
- 预期: 与 §5.3 完全一致
- 实际: 六组全过；yaml 对照零缺口（设计锚点齐全：取消排除 DISPATCHED 有书面理由、失败走 EXECUTED+failure_class 非独立终态、IdempotencyClass 三值按 §43.8 细化）
- 结果: PASS
- 关联 bug: 无
- 备注: CanonicalDigest 直接用 Day 04 的 canon 包（JCS）——契约冻结的复利兑现。零新审计发现

### T-048  [PASS]
- 时间戳: 2026-11-17T08:56:00Z
- 对应: Day 15（dev-plan/day-15.md §7，提交 04d8774，手动触发） / 里程碑 M2
- 测试项: Day 15 默认套件 + 官方用例独立复跑
- 命令: `go build/vet/test ./... -count=1` + `go test ./internal/policy/... -v`
- 预期: 全绿（§7「各违规输入→deny；合规→allow」全覆盖）
- 实际: 16 包全绿；9 个策略测试全过（含 TestBadAuthTimeFailClosed / TestDeterministic / 通配符/端口/IDOR 限额）
- 结果: PASS
- 关联 bug: 无

### T-049  [PASS]
- 时间戳: 2026-11-17T09:03:00Z
- 对应: Day 15（红线绕过对抗探针 + 匹配层代码审计——本项目安全心脏，最高强度） / 里程碑 M2
- 测试项: 九组绕过向量：①userinfo 欺骗（@前伪装 host）②通配符点边界（evilexample.test 拒 / 深层子域放行）③端口 8443 拒+缺省 443 映射 ④host 大小写 ⑤空/坏 URL fail-closed ⑥混合类默认拒优先 ⑦require_approval 不被 allow 淹没 ⑧授权窗边界 1ns 精度 ⑨IDOR 配额边界；⑩空租户行为观察；engine/identities/methods/quota 全审读
- 命令: 临时探针（internal/policy/zz_probe_test.go，已删）`go test -run TestProbe -v`
- 预期: 全部绕过被拦截
- 实际: ①-⑨ 全部正确拦截/放行——PDP 本体零绕过；⑩ 确认空 Tenant 在 AllowedTenants 设置下放行 → Q-19
- 结果: PASS
- 关联 bug: 无
- 备注: 实现质量高：通配符点边界无子串匹配缺陷、userinfo 由 Hostname() 剥离、全链 fail-closed（坏日期/坏 URL/未登记类/缺方法均拒）、元操作一律拒、判定确定性可回放。Q-19/Q-20 为前瞻性集成项非当前缺陷

### T-050  [PASS]（本地补丁后）
- 时间戳: 2026-11-17T12:10:00Z
- 对应: Day 16-23 连续八天批次（提交 46435d2..ee51ea5，手动触发） / M2 收口 + M3 开端
- 测试项: 批次全量套件（21 包）——因 BUG-002（Setpgid Windows 构建阻断）本地临时移除该字段后执行，测毕已还原（git checkout），补丁未入库
- 命令: `go build/vet/test ./... -count=1`
- 预期: 全绿
- 实际: BUILD_OK/VET_OK；21 包全绿——新增 evidence/execution(扩展)/network/providers/sandbox/runners.fake 全过。各包测试名与安全语义 1:1 映射（断路器触闸撤出口、Proxy 六向拦截含能力逐请求消费与撤销后拒发、Blob 篡改检测、证明门控、Provider 安全拒答不重试不换路）
- 结果: PASS
- 关联 bug: BUG-002（构建阻断，本地补丁绕过）
- 备注: Permit 层测试矩阵完整（并发一次性消费/重放/旧代/过期/Run 不可执行/策略翻版/撤销/能力限额——§4.3 #4 全语义），本机套件实跑通过即采信

### T-051  [PASS]
- 时间戳: 2026-11-17T12:05:00Z
- 对应: Q-19 修复复测（46435d2，租户 fail-closed） / M2
- 测试项: AllowedTenants 设置时空租户应拒（专用码 tenant_required）；白名单租户放行
- 命令: 临时探针（zz_t_test.go，已删）`go test -run TestRetestQ19 -v`
- 实际: 双向断言通过
- 结果: PASS——Q-19 关闭
- 关联 bug: 无

### T-052  [PASS]
- 时间戳: 2026-11-17T12:18:00Z
- 对应: Q-20 验收（Day 16 Scope 规范化器 + Day 18 Policy Proxy 同栈一致性） / M2
- 测试项: ①幂等性电池：18 种恶意变形（userinfo 欺骗/末尾点/默认端口/编码 path/IPv6/IPv4 四种异常形式/fragment/首尾空白）全部满足 Canonicalize(String(c))==c；②等价变形 PDP 判定一致（授权内三种写法全 ALLOW）；③IPv4 异常形式→127.0.0.1→scope 拒绝；④Proxy 源码核验：复用 policy.Canonicalize/CheckNetwork 同栈（policy_proxy.go L153-157）；⑤scope.go 全文审读
- 命令: 临时探针（zz_q20_test.go，已删）`go test -run TestQ20 -v` + 源码审读
- 实际: 全部通过。规范化器处理 §9.3 全清单（大小写/末尾点/默认端口/userinfo/多重编码/inet_aton 四式/IPv6 压缩/控制符/退化点全 fail-closed），非 ASCII host 无 IDN 库时显式拒绝并注明取舍——设计成熟
- 结果: PASS——Q-20 关闭
- 关联 bug: 无

**八天批次验收结论：Day 16-23 全部通过。M2（Day 14-22）里程碑完成：Action 状态机、PDP、Scope 规范化器、两层 Permit、Policy Proxy、沙箱/Egress、Evidence CEBS、Receipt 对账闭环、安全熔断器。遗留：BUG-002（Windows 构建，build tag 拆分待修）。**

### T-053  [PASS]（Owner 质询后的深测补全轮）
- 时间戳: 2026-11-17T12:27:00Z
- 对应: Day 16-23 深度补测（首轮 T-050~T-052 只做了套件+Q-19/Q-20 深钻，Owner 质询"细致一点"后补全） / M2
- 测试项: ①补读八天全部 §7 验收口径（首轮遗漏）；②Permit 一次性消费独立探针（8 goroutine 并发抢消费）；③Evidence 链六向探针（往返/哈希重算/**全目录扫描无明文落盘**/篡改检出/内容寻址去重/错误 DEK 隔离+脱敏无误伤）；④断路器双探针（3 次阈值精确触闸/回调恰一次/锁外调用/threshold≤0 最严）；⑤Proxy 执法链全文审读（六步顺序、配额最后消耗不烧被拦请求、CheckNetwork fail-closed 封死守卫条件分支疑点）；⑥**全量真库套件**（首轮只跑了默认模式——真库用例会被 SKIP，此为实际缺口）；⑦重定向守卫（禁自动跟随+重过 Scope）；⑧Day 19/21/23 核心实现扫读（配对门控/证明分级 fail-closed/修复恰一次）
- 命令: `go test -tags sqlite_modernc ./... -count=1`（169 测试全过 **0 SKIP**）+ 三个临时探针（已删）+ 源码审读
- 实际: 全部通过。新发现 Q-21（egressGuard 默认关）
- 结果: PASS
- 关联 bug: BUG-002（本地补丁绕过后执行，测毕还原）
- 备注: 本轮方法论修正记录：**"测试名覆盖良好"不能替代行为级验证**——首轮对六天只看了测试名，本轮以独立探针+真库全量补齐。默认模式套件会静默 SKIP 真库用例，批次验收必须双模式都跑

### T-055  [PASS]（Day 24-30 批次 + 修复复测）
- 时间戳: 2026-11-17T13:35:00Z
- 对应: Day 24-30 连续七天批次（e5f8483..0ea4138）+ BUG-002/Q-21 修复（0687407），手动触发 / **M3 收口**
- 测试项: ①BUG-002 复测：测试机**裸 Windows 原生** build+vet（不打本地补丁）；②Q-21 复测：守卫默认方向核验；③双模式全量套件；④七天 §7 全读 + 测试枚举
- 命令: `go build ./... && go vet ./...`（裸跑）+ `go test [-tags sqlite_modernc] ./... -count=1 -v`
- 预期: 全绿；修复生效
- 实际: 裸 Windows 构建+vet 通过（BUG-002 关闭）；egressGuard 默认 true + `WithEgressGuardDisabled()` 显式放宽带书面理由（Q-21 关闭，方向反转正确）；默认模式 23 包 ok；真库模式 **207 测试全过 0 SKIP**（新增 internal/agent 35 测试 + tests/lab 自主推进端到端 + scheduler 评分 8 测试）
- 结果: PASS
- 关联 bug: 无

### T-056  [PASS]（Day 26 注入隔离 + Day 24 预算独立探针）
- 时间戳: 2026-11-17T13:44:00Z
- 对应: Day 24/26 安全关键不变量独立验证 / M3
- 测试项: ①注入隔离：带 nonce 的恶意载荷（指令覆盖/伪造 JobSpec 标题/伪造 source 标记/prompt smuggling 标签）注入 Untrusted → 指令区（StablePrefix+InstructionBody）与干净构建**字节级相同**、nonce 零泄漏、载荷完整保留数据区；②预算：20 goroutine 并发预留对 80u 普通额度（每预留 worst 10u）→ 恰 8 成功 12 拒；finalization 通道 20u → 2 成功第 3 拒（总额度保护）；③伪标记未中和观察
- 命令: 临时探针（internal/agent/zz_probe_test.go，已删）`go test -tags sqlite_modernc -run TestProbe -v`
- 实际: 全部通过。①的指令区完整性是 §46 的立身之本——实测字节级不可撼动；②的并发边界与守恒精确
- 结果: PASS
- 关联 bug: 无（Q-24 观察项）
- 备注: 探针初版关键词"伪造"误报（固定指令区安全措辞本身含该词），改用唯一 nonce 后通过——教训：注入探针的判定标记必须用不可能自然出现的 nonce

### T-057  [PASS]（Day 25/29 安全闸门代码审读）
- 时间戳: 2026-11-17T13:48:00Z
- 对应: Day 25 Executor / Day 29 PlanCompiler / M3
- 测试项: PlanCompiler 同事务 CAS（L207 `UPDATE runs SET current_snapshot_revision=+1 WHERE ...=?`——过期 Snapshot 0 行+dirty）、patch 幂等（INSERT OR IGNORE patch_id）；Executor StepContext 冻结语义（Generate 前持久化快照/STALE 整体失效/(model_step_id,ordinal) 效果幂等键）
- 命令: 源码审读 + 对应枚举测试核对（TestCompileStaleSnapshotRejectedAndDirty / TestEffectIdempotentCrashRecovery / TestStaleResponseInvalidated 全过）
- 实际: 实现与声明一致
- 结果: PASS
- 关联 bug: 无

**Day 24-30 批次验收结论：全部通过。M3（Day 23-30）里程碑完成：预算账本、Executor Loop、Context Builder（不可信隔离）、两阶段压缩、Strategist、PlanCompiler CAS、Scheduler 评分调度、自主推进端到端。新增 Q-24（低）。**

### T-058  [PASS]（Day 31-37 批次 + Q-24 复测）
- 时间戳: 2026-11-17T14:20:00Z
- 对应: Day 31-37 连续七天批次（65b8c9a..cda86d5）+ Q-24 修复（6f9b5df），手动触发 / **M4 收口**
- 测试项: ①Q-24 复测（定界符中和落地）；②双模式全量套件；③七天 §7 全读 + 新包测试枚举（runners/js 2 + coverage 10 + experiments 5 + oast 3 + findings 13 + knowledge 9 + tools 3 = 45 项）
- 命令: `go build/vet/test ./... [-tags sqlite_modernc] -count=1 -v`
- 实际: Q-24 关闭（neutralizeDelimiters 按建议转义、保留前导空白）；默认模式 32 包 ok；真库 **256 测试全过 0 SKIP**；§7 全项有测试映射且过
- 结果: PASS
- 关联 bug: 无

### T-059  [PASS]（防谎报三闸独立探针——本批核心安全价值）
- 时间戳: 2026-11-17T14:36:00Z
- 对应: Day 36/37 防谎报不变量独立验证（渗透工具可信度的命根） / M4
- 测试项: ①逐字引用：原始字节空间偏移 + CRLF 归一双侧比对 → 正确引用通过；②**释义冒充拒**（改写文本冒充原文）；③**伪造 Evidence ID 拒**；④坏区间拒（超界/倒序）；⑤降级纯函数：强反证→REJECTED、弱反证→NEEDS_EVIDENCE、哈希失败→NEEDS_EVIDENCE
- 命令: 临时探针（internal/findings/zz_probe_test.go，已删）`go test -tags sqlite_modernc -run TestProbe -v`
- 实际: 全部通过——释义/伪造 ID/坏区间三路攻击全部被拒，降级语义正确
- 结果: PASS
- 关联 bug: 无
- 备注: 语义澄清记档（供 Day 38 报告实现参考）：**Citation 字节偏移定义在原始（未归一）字节空间**，归一化只作用于两侧比对容差——符合 §13.3「映射回原文」。另：探针两轮失败均为测试方自身错误（关键词误报/归一化空间偏移/转义 no-op），实现无恙——探针修正过程本身验证了比对逻辑的严格性

**Day 31-37 批次验收结论：全部通过。M4（Day 31-37）里程碑完成：攻击面发现（JS AST/规格驱动/运行时捕获）、模块聚类+技术栈指纹（无证据不生成）、Hypothesis/Experiment/Coverage（baseline-only 不可 TESTED）、序列/竞态实验+OAST、工具恢复/知识库/Skill、Candidate Finding+逐字引用+独立 Verifier、Replay+CVSS 确定性+反证自动降级。Q-1~Q-24 全闭环，零未决审计项。**

### T-060  [PASS]（Day 38-44 批次 Go 侧）
- 时间戳: 2026-11-17T15:30:00Z
- 对应: Day 38-44 连续七天批次（df41806..090a808），手动触发 / **M5 收口 + M6 UI + M7 开端**
- 测试项: 双模式全量 + 七天 §7 映射 + 新套件枚举（report 6 + faultinjection 3 + policybypass 19）
- 命令: `go build/vet/test ./... [-tags sqlite_modernc] -count=1 -v`
- 实际: 默认 35 包 ok；真库 **291 测试全过 0 SKIP**；报告侧（非 REPORTABLE 不入/无源事实拒/零发现区分/手改不进权威/STALE）全过；故障注入（model_response_commit 崩溃幂等/exit_gate 崩溃不半完成/默认禁用）全过
- 结果: PASS
- 关联 bug: 无

### T-061  [PASS]（UI 三件套独立验证——开发方环境跑不了的补位）
- 时间戳: 2026-11-17T16:00:00Z
- 对应: Day 39-43 M6 Web UI / M6
- 测试项: 本机（Windows + Node 24 + pnpm 11.28）装依赖→typecheck→vitest→生产构建；SSE 一致性消费端实现核验
- 命令: `pnpm install --config.minimum-release-age=0` + `pnpm typecheck` + `pnpm test` + `pnpm build`
- 实际: typecheck exit 0；**Vitest 19 文件 115 用例全过**（SSE 重放不丢/旧事件丢/重复乱序丢/resync/命令幂等同 key 重试不生成第二份/STALE 禁导出/恢复预览 Schema 阻断/Preflight 门禁）；生产构建 286KB 721ms——三项与开发方声称完全一致。SSE 消费端（consistency.ts）8 项语义齐全 → Q-18 闭环
- 结果: PASS
- 关联 bug: 无（Q-25 记录）
- 备注: 安装过程踩两个 pnpm 11 供应链默认坑（见 Q-25）；Playwright e2e 双方环境都不可跑（无真实 daemon 可连——Q-23 仍开放），UI 行为验证以 Vitest 为准

### T-062  [PASS]（策略绕过套件审读——本批安全核心价值）
- 时间戳: 2026-11-17T16:01:00Z
- 对应: Day 44 Policy Bypass Suite / M7
- 测试项: 19 项绕过向量实跑核验 + 实现走真实 guard（policy.InScope/RedirectInScope/Breaker/findings/coverage）确认非自演自测
- 命令: `go test -tags sqlite_modernc ./tests/policybypass/... -v` + 源码抽查
- 实际: 全过——跨源重定向/DNS 重绑定字面私网（含云元数据 169.254.169.254）/非默认端口/userinfo 冒充/IPv6 越域+loopback/编码域名 fail-closed/浏览器新窗口重查/WebSocket scheme fail-closed/Proxy 失效熔断/伪造 Evidence/释义/跨 Run 引用/Agent 自报/Memory 假称已测——与 T-049/T-059 探针口径互补印证
- 结果: PASS
- 关联 bug: 无
- 备注: 开发方把我的 Q-22/Q-23 接线清单吸收为 PROGRESS 正式表格（L43-45）；Q-23（daemon 不 serve）仍开放，开发方已文档化「端到端接线随 daemon」

**Day 38-44 批次验收结论：全部通过。M5（报告+Exit Gate）、M6（Web UI P0 全链路页面+SSE 一致性+命令幂等）、M7 开端（故障注入+策略绕过套件）完成。Q-18 闭环；新增 Q-25（UI 依赖可复现性）；Q-23 维持开放（开发方已跟踪）。**

### T-063  [记录]（Owner UI 验收反馈录入 + 根因诊断）
- 时间戳: 2026-11-17T16:16:00Z
- 对应: M6 P0 预览验收（Owner 本机查看 http://127.0.0.1:4173 PagesDemo） / M6
- 测试项: Owner 反馈两条——①字体发虚模糊 ②Toast 不好看；测试方根因诊断后按 Owner 指示录入需求文档
- 命令: CSS/产物诊断（tokens.css L110 / font 栈 / dist 字体文件检查）
- 实际: 已追加 `docs/pentagent-ui-spec-v1.6.md` §16.35「Owner 验收反馈」：①**元凶定位**——`-webkit-font-smoothing: antialiased` 在 Windows Chrome 禁用子像素渲染（macOS 惯例误用于跨平台）+ Inter 未打包实际走系统回退（雅黑+antialiased 观感劣化），附修复方向与验收标准；②Toast 深色反转/图标结构/动效收敛三方向，审美项 Owner 一票定稿
- 结果: 记录在案（非缺陷，属需求打磨）
- 关联 bug: 无
- 备注: 预览服务器由测试方后台维持（vite preview :4173），Owner 看完即关。UI 打磨窗口建议放在 daemon 接线前一并处理（改动成本最低）

| 新编号 | 级别 | 内容 |
|---|---|---|
| Q-25 | 低 | **UI 依赖在新环境不可直接安装**（T-061 实测）：①pnpm 11 默认 `minimumReleaseAge` 供应链策略拒 pnpm-lock.yaml 内条目（@rollup 等 14 包「发布过新」），需 `--config.minimum-release-age=0` 绕过；②`pnpm.onlyBuiltDependencies` 写在 package.json（pnpm 11 不再读该位置）→ esbuild 构建脚本被拦。开发方环境（旧 pnpm/宽松策略）无感，新克隆按 README 步骤装不上 | ①按官方迁移把 onlyBuiltDependencies 移入 pnpm-workspace.yaml；②README 记最低 pnpm 版本或提供 .npmrc 固定策略；③刷新 lockfile 使与现行 pnpm 兼容 |

| 新编号 | 级别 | 内容 |
|---|---|---|
| Q-24 | 低 | **数据区定界符未中和**（T-056 ③ 实测）：Untrusted/Retrieved 内容原样写入数据区，内容中伪造的 `--- [source=skill untrusted] ... ---` 标记与 `## 指令标题` 原样保留——指令区完整性不受影响（结构隔离靠拼接顺序，字节级验证过），但模型读到数据区时可能被内容内伪造标记诱导重分区（§46.5 二阶注入面） | 对不可信内容做定界符中和：行首 `--- [source=` 与 `## ` 模式转义（如替换为 `~~[source=` / `＃＃ `）；Day 26 补丁式修复即可，不影响 digest 契约（request_digest 对渲染后整体取值） **复测（T-058）：已修复关闭**——6f9b5df neutralizeDelimiters 按建议落地（`--- [source=`→`~~ [source=`、`## `→`＃＃ `，保留前导空白），随批全绿 |

### T-054  [PASS]（全量代码审计，Owner 指令）
- 时间戳: 2026-11-17T12:31:00Z
- 对应: Day 01-23 全仓审计（112 个 Go 文件 / 9148 行非测试代码） / 全里程碑
- 测试项: 七个横切面 + 补读未细读文件：①SQL 参数化纪律 ②单写者旁路 ③秘密/密钥泄漏面 ④时间不变量（UTC/RFC3339）⑤TLS/出网构造 ⑥忽略错误的方向性 ⑦配置卫生（gitignore/go.mod/监听面）；补读 cmd/pentagent、anthropic 适配器、sandbox、runners、pentagentd、egress 全文
- 命令: 系统化 grep 溯源 + 逐文件审读
- 实际: **零高危发现**。清洁面：SQL 全参数化（唯一表名拼接是 GuardRevision 白名单，Day 11 已审）；sql.Open 仅存在于 store 构造器（零 Writer 旁路）；时间全 UTC（reconciler 的疑点为 grep 误报，上行即 .UTC()）；无 InsecureSkipVerify；egress.Client 无直连兜底（无代理即无路由）；APIKey 仅入请求头不入日志/错误串；三处忽略错误均为 fail-closed 方向的 best-effort（过期标记失败仍拒/wal_checkpoint/ROLLBACK-TO 清理）；gitignore 已吸收 BUG-001 教训且不误伤 fuzz 语料；go.mod 仅 4 个外部依赖且全部钉版有据。新发现两条集成真空记录（Q-22/Q-23，均为 info 非缺陷）
- 结果: PASS（代码质量整体评级：高）
- 关联 bug: 无新增

| 新编号 | 级别 | 内容 |
|---|---|---|
| Q-22 | info | **组件接线真空清单**（T-054 盘点，均属路线图后续天的正常状态，非缺陷）：evidence.BlobStore 无生产调用方（DEK 来源/轮换策略无归属——spec §45 C2「密钥丢失=证据丢失」待落到开发天）；providers 适配器（anthropic）无生产调用方（APIKey 来源待 Day 25 接线）；audit_chain 表零写入（Day 46）；internal/api Server 未被 pentagentd serve（Day 39+）。建议在 PROGRESS 或 ROADMAP 汇总此清单防漏接 |
| Q-23 | info | pentagentd 仍为 reconcile-only 骨架（70 行，无 HTTP 监听/无调度循环），占位注释诚实；但 CLI 默认连 127.0.0.1:8787——**当前版本 CLI 对真实 daemon 不可用**（生命周期测试走进程内）。Day 39+ daemon 接线时验收 |

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
| Q-11 | 低 | docs spec §27.5(L4613) × §15.6 | **spec 内部不一致**：§27.5 声称 `external_message.updated`「已登记于 15.6」，§15.6 实无此事件（49 个里没有）。开发方实现忠实跟随 §15.6（五方对账 49/49 一致，实现无误）。§27.5 的多窗口消息/Handoff 功能要用这个事件 | 契约新鲜期决策便宜：要么 §15.6+events.yaml 补第 50 个事件（重跑 gen-contracts），要么改 §27.5 措辞并给 Day 12+ 留跟踪项。需 Owner 拍板。**复测（T-023/T-024）：已修复关闭**——fff046f 向前补齐第 50 事件（实现侧五方同步），bdc091e 补 spec §15.6 登记行 + 测试注释 49→50，登记链 spec→yaml→生成物全部对齐 |
| Q-12 | 低 | cmd/gen-contracts/main.go goScalar/tsScalar/goType | 类型映射 default 分支**静默回退 string**：events.yaml/openapi.yaml 里类型拼错（如 `integre`）会静默生成 Go/TS string 字段，不报错——与 Q-7 同款「契约工具链静默吞错」模式 | default 分支改为返回错误（生成器响亮失败）；补一个拼错类型的负向用例。**复测（T-023）：已修复关闭**——三个映射函数均改 (string, error) 带上下文传播，负向测试 gen_test.go（拼错标量/Schema/数组元素类型均报错）随套件通过 |
| Q-13 | 低 | internal/store/writer.go `Migrate()`/`DurableDB()` | 两者都要求「只能在 Start() 前调用/只给测试用」（注释声明），但**无运行期防护**：Start() 后调 Migrate() 会在 writer goroutine 正用 durable 连接时并发使用同一条连接（数据竞态）；DurableDB() 同理。当前唯一调用方（测试）用法正确，属于误用面问题 | Migrate() 加 started 守卫（atomic.Bool，已启动则报错）；DurableDB() 同样加守卫或改命名（DurableDBForTest）表明约束。**复测（T-028）：已修复关闭**——25affd7 加 started 守卫（Migrate 启动后报错、DurableDBForTest 启动后 panic、Start Swap 幂等）+ TestMigrateAfterStartRejected |
| Q-14 | info | internal/store/writer.go `checkpoint()`、reader.go DSN | checkpoint 忽略 TRUNCATE 结果（读者忙时返回 busy 属正常，best-effort 可接受）；reader 的 query_only 连接上设 journal_mode(WAL) 是冗余（库已 WAL，只读连接改不了也不需要）——无害，知悉即可 | 保持现状可接受；后续若做 checkpoint 可观测再加日志。**复测（T-028）：已处置**——checkpoint best-effort 注释化；reader DSN 去掉冗余 journal_mode |
| Q-15 | 中 | internal/control/states.yaml × spec §5.1/§9.7 | **迁移表缺 spec 规定的转移（T-030 行为级证实 6 项）**：① pause 仅 RUNNING 可入 PAUSING，§9.7 要求所有非终态（QUEUED/BLOCKED/RESUMING 等 9 态实测被拒）——用户暂停排队中的 Run 会被 ErrInvalid 拒绝；② BLOCKED→PARTIAL 缺失，§5.1 L1348 明文审批期到期允许 PARTIAL/STOPPING/FAILED；③ **EXITING→RUNNING 缺失（无对应事件）**，§5.1 L1336 门禁发现新工作须回 RUNNING——缺它则退出门禁循环断裂，EXITING 误成准终态；④ RUNNING→COMPLETED 直达口径待与 spec 确认（L1351）。属性测试检不出：它只验证实现符合 states.yaml，不验证表符合 spec | states.yaml 补转移行（pause 扩 from 列表、加 partial 的 BLOCKED 来源、加 exit_reopen: EXITING→RUNNING 事件）→ `go generate` → 补 TestKeyTransitions 锚点。趁 controller 调用方尚未依赖「拒绝」行为时改最便宜；③是功能缺口，①②是用户可感行为偏差。**复测（T-031）：已修复关闭**——68e8f27 迁移 34→45：pause 扩至 9 非终态（STOPPING 报备排除：stop 不可逆）、BLOCKED/RUNNING→PARTIAL、reopen: EXITING→RUNNING；④ 采纳「EXITING 单路径 + controller 原子性」解读（图示本为单路径，成立） |
| Q-16 | 低 | internal/control/job_states.yaml WAITING_APPROVAL | 审批**被拒或过期**后 Job 的出口未定义：SM 只有 approve→RUNNING 一条出边，DENIED/EXPIRED 的审批对应 Job 只能靠用户 cancel 或 fail 终结。spec 侧也空白（只有 L3085 deny API 端点，无 Job 去向语义）——审批流 Day 15/17 要用 | Day 15 前补：SM 加 denied: WAITING_APPROVAL→CANCELLED（cancel_code=PLANNER_SUPERSEDED 或专用码）与 expired 边，或 spec 侧先定义语义；属 spec 空白而非实现违规。**复测（T-034）：已处置关闭（被拒半边）**——4941a04 加 deny: WAITING_APPROVAL→RUNNING，理由成立（被动等待态不自行终结、去留由规划器重排，不把争议语义冻进 SM）；**过期半边有意留白到 Day 15**（耦合 Policy Engine，且 cancel/fail 恒为安全出口无活性缺陷），届时随 Day 15 验收 |
| Q-17 | info | internal/scheduler/claim.go guardFencingTx | fencing 守卫只校验「令牌=当代」，不查 Job 状态：CANCELLED/FAILED 等终态 Job 的当代令牌仍能通过 CheckFencing/WriteAttemptResult/Renew（T-036 P4 实测确认）。当前无实际危害（终态 Job 无后续写路径）；Day 17 Permit 发放、Day 18 Proxy 校验若复用 guardFencingTx 判「可写」，会放行已终态 Job 的请求 | Day 17 设计 Permit 校验时叠加 Job 存活性（state 非终态）检查；或在 guardFencingTx 加可选参数。届时随 Day 17 验收。**复测（T-037）：已修复关闭（提前处置）**——9c2b285 guardFencingTx fail-closed 叠加存活性：终态 Job 令牌匹配也拒写，三路（Check/Write/Renew）实测全拒 |
| Q-18 | info | internal/outbox（设计语义，非缺陷） | outbox 为**至少一次**投递：bus.Publish 成功但 markPublished 前崩溃 → 重启后重投（WHERE published=0 守卫决定）；多实例 Publisher 也会重复投递。代码注释已声明该语义 | Day 40 SSE 接入时消费者按 event id / sequence 去重（SSE 客户端 event id 已有全局序可用）；单实例部署假设写入运维约定。届时随 Day 40 验收 **复测（T-062）：已闭环**——apps/web/src/events/consistency.ts 实现消费端序号去重（快照 cursor+1 按序投递不丢不重/旧于快照丢/等于 cursor 边界丢/重复乱序丢/resync_required 决策/游标只进不退），8 项 Vitest 全过；fetch-based SSE 线解析（sse.ts）支持首帧行——至少一次生产者+幂等消费者契约成立 |
| Q-19 | 低 | internal/policy/identities.go checkIdentity | **空租户 fail-open**（T-049 ⑩ 实测）：策略设置 `identities.allowed_tenants=[tenant-a]` 时，`Tenant=""` 的请求放行——租户限制只对携带租户的请求生效。若租户是硬边界（跨租户对象=IDOR 高危场景），无租户请求应拒或显式区分「不适用」 | Day 16/18 身份→租户接入时收口：PolicyInput.Tenant 由 Identity 注册表填充后此口子自然闭合；建议 PDP 在 AllowedTenants 非空且 Tenant 为空时改判 deny 或标注语义。spec §9.2 租户语义待澄清 |
| Q-20 | info | internal/policy/methods.go checkScope × 未来 Day 16/18 | **parser differential 前瞻**：PDP 用 Go net/url 解析 CanonicalURL；Day 16 规范化器与 Day 18 Policy Proxy 若用不同 URL 解析栈（或不同规范化规则），可能出现「PDP 判 A 放行、Proxy 按解析 B 转发到 C」的绕过。代码已声明「只接收已规范化 URL」，但一致性未验证 | Day 16 验收时核对：规范化器输出的 CanonicalURL 经 PDP 的 url.Parse 再解析必须幂等（host/scheme/port 三元组不变）；Day 18 Proxy 与 PDP 同源解析。记入两天验收清单。**复测（T-052）：已验证关闭**——scope.go 兑现承诺：Proxy 复用 policy.Canonicalize/CheckNetwork 同栈；幂等性探针 18 种恶意变形（userinfo/末尾点/默认端口/编码 path/IPv6/IPv4 异常/fragment/空白）全部 Canonicalize∘String 幂等；等价变形 PDP 判定一致；**IPv4 异常形式（0x7f.1/0177.0.0.1/2130706433/127.1）全部规范化为 127.0.0.1 且被 scope 拒绝**（SSRF 绕过堵死） |
| Q-21 | 低 | internal/network/policy_proxy.go egressGuard（默认 false） | **私网 egress 守卫默认关**（T-053 发现）：`WithEgressGuard()` 是可选项，默认不启用——防 SSRF/云元数据(169.254.169.254)/CGNAT 的纵深防御成为 opt-in。默认关对本地 mocktarget(127.0.0.1:8099) 测试合理，但生产 snagd 若忘开，配合 Scope 误配即可打内网。另：守卫只查字面 IP（Canonicalize 后），域名解析到私网的 DNS 重绑定不防（解析时校验属 Day 31 真实网络栈） | 生产启动路径（snagd 构造 Proxy 处）显式 `WithEgressGuard()`，或按「非本地调试模式」默认开；DNS 重绑定解析时校验记入 Day 31 验收清单。**复测（T-055）：已修复关闭**——0687407 改为默认开（fail-safe）+ `WithEgressGuardDisabled()` 显式放宽（书面理由：授权内网渗透/本地调试），方向反转正确 |

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

**Day 05 审计（T-020~T-022，2026-11-16T13:02Z，M0 收尾）：新增 Q-11(低)/Q-12(低)。** 五方事件对账 49/49 全一致、mocktarget 对抗探针 5/5 过；新发现集中在工具链与规格侧：Q-11 spec §27.5 引用未登记事件（实现无误，需 Owner 决策补登记或改措辞）、Q-12 生成器类型拼错静默回退 string（与 Q-7 同款模式）。**M0（Day 01-05）至此全部验收通过。**
验证：默认 build/vet/test 全绿（+3 迁移器内部测试）；`-tags sqlite_modernc` 真库 11/11 PASS。请复测确认。

**开发方回应（Q-7~Q-10，commit `dcafa37`，已复测关闭 T-019）**：契约冻结日对抗审计质量高，JCS 本体全硬边界合规的独立复验很有价值。逐条处置如下，均在进入 Day 05 前完成：
- Q-7 ✅ `policy.actions` 改 `additionalProperties:false`——红线开关名拼错应响亮失败而非静默吞。认同，当初图"宽容"是对红线契约的错误判断。
- Q-8 ✅ `ValidateAgainst` 启用 `c.AssertFormat()`——垃圾授权日期现被 format 断言拒绝（valid_until 是 §45 硬停依据）。
- Q-9 ✅ canon 包文档补「digest 载荷数值字段必须 <2^53，大量用字符串」——RFC 8785 固有的 ES double 舍入，系统 ID 用 UUIDv7 字符串已满足。
- Q-10 ⏸ **有意保留**（审查亦认可 M0 留白）：planpatch op 载荷暂不收口，schema 加说明，实现各 op 时用 if/then 按 op 收口。PlanPatch 经 PlanCompiler 应用、非 digest 锚，宽松风险低。
- 新增 `TestPolicyHardening`（拼错开关名被拒 / 正确键通过 / 垃圾 date-time 被拒）。canon+schemas+默认全量全绿。请复测确认。

**开发方回应（Q-11~Q-12，commit `fff046f`，复测见 T-023）**：M0 收尾审计。五方事件对账与靶场探针的独立复核很有价值。
- Q-11 ✅ **向前补齐**：确认是 spec §27.5 与 §15.6 两节打架，我的实现忠实跟随权威登记节 §15.6（无误）。决策上选择补事件而非改规格——events.yaml 增 `external_message.updated`（§27.5 多窗口消息/Handoff 要用，external_messages 表 Day 03 已建），成为第 50 个事件，重跑 gen-contracts，expectedEvents 同步到 50。若 Owner 更想改 §27.5 措辞，可回退此条。
- Q-12 ✅ 生成器**改为未知类型响亮报错**（goScalar/tsScalar/goType 返回 error 并传播），与 Q-7 同款「静默吞错」一并消除。+ 负向测试 `gen_test.go`（拼错标量/Schema/数组元素类型均报错）。
验证：生成器重跑 ok，五方对账 50/50 仍一致（测试锁定同步）；默认 + `-tags sqlite_modernc` 全量全绿。请复测确认。

**开发方回应（Q-13~Q-14，commit `25affd7`，待复测）**：单写者审计 + 行为探针补 -race 的做法很到位。
- Q-13 ✅ 加运行期守卫：`started`(atomic.Bool)，`Start()` 幂等置位；`Migrate()` 启动后返回错误；`DurableDB()` 改名 `DurableDBForTest()` 且启动后 panic（命名即约束）。+ 测试 `TestMigrateAfterStartRejected`。认同，注释声明而无防护确是误用面。
- Q-14 ✅ reader 移除冗余 `journal_mode(WAL)`（只读连接改不了也不需要）；checkpoint best-effort 加注释。
- **关于 -race 环境**：测试机无 C 编译器、`go test -race` 需 cgo 跑不了，我的 -race 是在 **Linux + gcc** 上运行并通过；感谢用行为探针（T-027 故障隔离/幂等/并发读写 4/4）补位。认同后续在 Linux CI 跑 -race 独立锁定内存竞态。
验证：默认全绿；`go test -race -tags sqlite_modernc ./internal/store/...`（Linux+gcc）全绿，含 Q-13 守卫测试。请复测确认。

**开发方回应（Q-15，commit `68e8f27`，待复测）**：这次迁移表对账是本轮最有价值的发现——属性测试只能验「实现符合 states.yaml」，验不出「states.yaml 符合 spec」，正是独立人工对账的不可替代处。逐项核对 spec 原文后**全部确认并修复**（迁移 34→45）：
- ① ✅ pause 扩至所有非终态（§9.7 L2458 明文）。**一个有意例外 STOPPING**：它已提交 stop 朝 STOPPED 收敛，若允许 pause 会反悔 stop，违背 §9.7 Stop 不可逆语义——故排除并在 states.yaml 标注。这点请测试方/Owner 留意：是我对两条 spec 语句轻微张力的工程判断（Stop 优先于 Pause）。
- ② ✅ partial 补来源：RUNNING 直达（§5.1 图 L1340）+ BLOCKED（L1348）。
- ③ ✅ **功能缺口**已补：新增 `reopen` 事件 EXITING→RUNNING（§5.1 L1336/L1351 门禁发现新工作/CAS 变化放弃完成）。此前缺失确使 EXITING 成准终态陷阱。+ 控制器测试 TestControllerExitingReopen。
- ④ RUNNING→COMPLETED：按 §5.1 图示保持 EXITING→COMPLETED 单路径；L1351「同一逻辑事务」解读为 controller 的门禁读+迁移原子性（run_controller 已在一个 durable 事务内完成），非 SM 直达边。若测试方认为应补直达边，请指出。
验证：go generate 可复现；默认 + `-tags sqlite_modernc` 全量全绿（TestKeyTransitions 补 5 锚点 + STOPPING-pause 排除 + reopen 控制器测试）。请复测确认。

---

**开发方回应（Q-16，commit `4941a04`，待复测）**：确认是真实 spec 空白（独立核对 §5.2 原文属实，WAITING_APPROVAL 此前仅 approve 一条前向出边）。分两半处置，实事求是：
- **被拒**：✅ 加 `deny: WAITING_APPROVAL→RUNNING`。这条是**架构上被迫、非猜测**——被动等待态不能自行决定 Job 终结；审批被拒只意味该红线动作不可行（动作侧转 POLICY_DENIED），Job 去留只能由规划器在 RUNNING 侧重排决定（换等价无红线动作 / 放弃→届时 cancel/fail/partial）。故不把「被拒=Job 死」这类争议语义冻进 SM。与 approve 同落 RUNNING、事件不同、下游含义不同。
- **过期**：**有意留白、延后 Day 15**。「重发审批 vs 超时放弃」耦合 Day 15 Policy Engine / Day 17 两层 Permit，spec 空白，此刻定边必是猜测；且**无活性缺陷**（cancel/fail 恒为安全出口，非 Q-15 那种无出边陷阱）。待 Day 15 审批流落地时与 Permit 机制一并定义。
- 这半边与测试方建议（denied→CANCELLED/PLANNER_SUPERSEDED）口径不同：我判断审批被拒不应在 SM 层直接判 Job 死，而应交回规划器裁量——若测试方/Owner 认为被拒就该终态，请指出，可改。
理由全文已写入 `job_states.yaml` 审批段头注。验证：迁移 34→35，TestJobKeyTransitions 补 deny 锚点，双模全绿 + go generate 幂等。请复测确认。

---

**开发方回应（Q-17，commit `9c2b285`，待复测）**：确认属实（guardFencingTx 只比令牌不看 Job 状态，T-036 P4 的独立实测很准）。虽判 info 且「随 Day 17 验收」，但该修**清晰无歧义、普适不变量**（终态 Job 不接受任何回写/续租），不预设 Day 17 Permit 的任何设计，且仅几行——对安全框架 fail-closed 更稳，故**立即硬化**不拖到 Day 17：
- currentFencingTx 顺带读 Job `state`；guardFencingTx 在令牌校验前先拒终态 Job（DONE/CANCELLED/FAILED）→ ErrConflict。Day 17/18 可在此之上再叠加 Permit 专属校验（不冲突）。
- 新增 TestFencingRejectsTerminalJob：认领后把 Job 打到 CANCELLED，令牌仍当代但 CheckFencing/WriteAttemptResult/Renew 全 ErrConflict。
验证：-race + 双模全绿。请复测确认。

---

**开发方回应（Q-18，无需改码，确认即可）**：同意——这是**刻意且已注释**的「至少一次」投递语义，非缺陷。精确一次需跨实例分布式共识（leader 选举/分布式锁），对单实例控制面是过度工程；选至少一次 + 消费端幂等是正确取舍。确认两点：① markPublished 前崩溃→重启重投，由 `WHERE published=0` 守卫兜住（已published 的不会二次 Drain）；② 多实例 Publisher 会重复投递——当前**单实例部署约定**（运维层），Day 40 若要多实例再加 leader 租约。消费端去重契约（Day 40 SSE）：客户端按 global_sequence/event id 幂等，Day 10 的全局单调 global_sequence 正是为此预留。无代码改动，随 Day 40 验收。

---

**开发方回应（Q-19，commit `46435d2`，待复测）**：确认是我引入的 fail-open（T-049 ⑩ 很准）。已修：去掉 checkIdentity 里 `in.Tenant!=""` 的逃生门——`allowed_tenants` 非空即表示租户是硬边界，空租户无法核验 → 一律拒（新 code `tenant_required`）。PDP 不依赖上游填充、自身 fail-closed 更稳。+ 测试断言「空租户+配了 allowed_tenants→DENY」。

---

**开发方回应（Q-20，commit `3fe8686`，待复测）**：这条前瞻非常关键，已在 Day 16 闭合。① checkScope 改走 Day 16 规范化器 Canonicalize（不再自己 url.Parse），PDP 与 Scope 规范化**同源解析**；② 新增一致性不变量测试 TestCanonicalParserConsistency：Canonicalize 输出经 PDP 的 net/url 再解析，host/scheme/port 三元组必不变；③ 幂等性 TestCanonicalizeIdempotent + FuzzScope 的幂等断言兜底。**差分模糊当场抓到两个真绕过**并修复：path 多重编码出 NUL 控制符（String 原始拼接产出不可再解析串→非幂等/走私）、`http://..` 退化点 host（再解析缺 host）。Day 18 Policy Proxy 的验收清单里会落实「Proxy 复用 policy.Canonicalize 做同源解析」，不自建第二套 URL 解析栈——届时请一并复核。

---

**开发方回应（BUG-002，commit `0687407`，待复测）**：确认属实，是真实的平台可移植性缺陷——`syscall.SysProcAttr{Setpgid}` 为 Unix-only 字段，Linux 开发环境天然编不出这个错，测试机为 Windows 才暴露，独立跨平台构建很有价值。已按你的建议修：build tag 拆 `proc_unix.go`（`//go:build unix`，设 Setpgid + SIGTERM 优雅取消）/`proc_other.go`（`//go:build !unix`，留空，保留 CommandContext 默认 Kill 语义 + WaitDelay 宽限），`toolruntime.go` 改调 `configureProcGroup(cmd)` 平台分流。复验：`GOOS=windows GOARCH=amd64 go build/vet ./...` 现 **exit 0（全仓交叉编译通过）**，Linux 行为不变（进程组收割仅 Unix，缺失不影响超时/看门狗正确性）。

**开发方回应（Q-21，commit `0687407`，待复测）**：确认 opt-in 隐患真实存在（T-053 发现很准）。实事求是地补一点：原「默认关」并非纯疏忽——**授权内网渗透（护网/HVV）的目标本就在私网**，是本框架的合法核心用例，默认关能让内网靶场开箱即用。但你的判断对：生产面向云 SaaS 时应 fail-safe，忘开守卫 + Scope 误配即可打内网/元数据，这个静默失败方向不可接受。已反转：**egress 守卫默认开**，新增 `WithEgressGuardDisabled()` 供授权内网/本地调试显式放行——把放宽变成操作者**知情刻意的选择**，而非忘开的静默隐患。另一关键事实：当前**无生产接线点**可供「记得开」（Proxy 暂无生产调用方，见 Q-22/Q-23），所以「生产路径显式 WithEgressGuard()」此刻无处可加，反转默认是此刻闭合隐患的唯一途径，与 Q-17/Q-19 一致的立即 fail-closed 硬化。回归锁定：`TestProxyEgressGuard` 去掉选项、仅靠默认仍拦私网（`egress_private_ip`）；环回测试显式 `WithEgressGuardDisabled()`。DNS 重绑定解析时校验确属真实网络栈范畴，随 Day 31 验收（已记 Day 31 清单）。操作者侧「授权内网模式」配置开关随 Day 39+ daemon 接线暴露。

**开发方回应（Q-22，无需改码，已记清单）**：同意，均为逐天开发的正常接线真空、**非缺陷**。已按建议把清单落到 `PROGRESS.md` 新增「集成待接线清单」节，逐条登记组件现状 + 接线归属天（BlobStore DEK→Day 25+、providers APIKey→Day 25、audit_chain→Day 46、api.Server/pentagentd→Day 39+），防里程碑推进漏接。T-054「零高危 + 清洁面逐项核对」这轮全仓审计质量很高，无站得住的反驳。

**开发方回应（Q-23，无需改码，已记清单）**：确认属实——pentagentd 当前是 reconcile-only 骨架（无 HTTP 监听/无调度循环），CLI 默认连 127.0.0.1:8787 对真实 daemon 暂不可用，生命周期测试走进程内（API handler 直调）。占位注释诚实，属路线图已知状态。已记入「集成待接线清单」，Day 39+ daemon 接线时一并落地 HTTP 监听 + 调度循环 + CLI 真连，届时请复核。

**开发方回应（Q-24，commit `6f9b5df`，待复测）**：确认属实，是真实的二阶注入面（T-056 ③ 的实测很到位——Day 26 的结构隔离只保证了「指令区字节级不被污染」，但没防「数据区内部被内容伪造的标记诱导重分区」，这是 §46.5 点出的下一层）。已按你的建议补丁式修（不动 digest 契约）：对写入数据区的不可信内容与来源标签做**定界符中和**——行首 `--- [source=` → `~~ [source=`、`## ` → 全角 `＃＃ `，来源标签去换行防越界。只中和这两种会被误判为结构标记的模式，避免过度转义破坏正常目标响应内容；我方真实 wrapper 不经中和、provenance 标签完好；request_digest 对渲染后整体取值故契约不变。新增回归 TestUntrustedDelimiterNeutralized：伪造的 `--- [source=skill untrusted] ...` 被中和为 `~~ [source=`、伪造 `## 指令标题` 被中和、真实来源标签完好、注入内容仍不渗入指令区。双模全绿。注：这是纵深隔离的加固；即便中和被绕过，执法纵深（层三 Policy/Permit/Proxy）仍兜底——模型中招也只能提案、动作仍被拒。

**开发方回应（Q-25，commit `3a08393`，待复测）**：确认属实，是真实的可复现性缺陷（T-061 在 pnpm 11 新环境的实测很有价值——开发方本会话是 pnpm 10.28，`minimumReleaseAge` 默认宽松、`pnpm.onlyBuiltDependencies` 在 package.json 仍被读，故两坑都无感）。逐条采纳你的三点建议并修（新增 `apps/web/pnpm-workspace.yaml`，删除只含注释的 `.npmrc`）：① **构建白名单迁出 package.json**——pnpm 11 起 `.npmrc` 只读 auth/registry、旧的 `onlyBuiltDependencies` 键已移除改 `allowBuilds`，故在 pnpm-workspace.yaml 同时写 `onlyBuiltDependencies:[esbuild]`（pnpm 10）与 `allowBuilds:{esbuild:true}`（pnpm 11）跨版本兼容；package.json 的 `pnpm.onlyBuiltDependencies` 保留作 pnpm<10.20 兜底（无害）。② **关闭供应链冷却**——pnpm-workspace.yaml 显式 `minimumReleaseAge: 0`（本仓锁文件已钉精确版本，安全边界由钉版+审阅保证，故全局关冷却以保证任何时间点可复现；并在 README 注记可用 `minimumReleaseAgeExclude` 精确排除代替全局放宽）。③ **README + 环境要求**——新增 `apps/web/README.md` 记 Node≥22 / pnpm≥10.20 / 安装命令 / 两坑背景；lockfile 经干净重装验证确定性不变（同锁文件），无需另刷。复验（pnpm 10.28，模拟新克隆：删 node_modules 干净 `pnpm install`）：install 无「Ignored build scripts」警告（白名单生效）、esbuild 平台二进制就位、`pnpm typecheck`+`pnpm test`（19 文件 115 用例）+`pnpm build`（286KB）全绿。pnpm 11 侧配置键依官方 11.0 迁移文档（`.npmrc`→pnpm-workspace.yaml、`onlyBuiltDependencies`→`allowBuilds`、`minimumReleaseAge:0` 于 pnpm-workspace.yaml），请在 pnpm 11 环境复测 T-061。

## 已修复 bug（归档）

| Bug 编号 | 修复时间 | 修复 Commit | 复测结果 |
|---|---|---|---|
| BUG-001 | 2026-11-16 | `d9d9c62` | ✅ 复测通过（T-006/T-007，2026-11-16T09:30Z）：vet 干净、domain 8/8 全绿，Day 01 验收恢复；coverage.go 字段与 §4.1 coverage_cells 11/11 一致。根因：`.gitignore` 的 `coverage.*` 误伤源码，已改为具体产物名并补提交；另采纳 T-005 建议加 `.gitattributes`(行尾 LF) |
