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

---

## 已修复 bug（归档）

| Bug 编号 | 修复时间 | 修复 Commit | 复测结果 |
|---|---|---|---|
| BUG-001 | 2026-11-16 | `d9d9c62` | ✅ 复测通过（T-006/T-007，2026-11-16T09:30Z）：vet 干净、domain 8/8 全绿，Day 01 验收恢复；coverage.go 字段与 §4.1 coverage_cells 11/11 一致。根因：`.gitignore` 的 `coverage.*` 误伤源码，已改为具体产物名并补提交；另采纳 T-005 建议加 `.gitattributes`(行尾 LF) |
