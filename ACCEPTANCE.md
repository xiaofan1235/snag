# Snag 验收报告 —— 可连续自主运行版本（§23 M7）

> 本报告对照主文档各章"验收"小节与 UI §16.34，逐条记录达标情况、执法位置与证明测试。
> 结论见文末。日期：2026-11-17。对应提交见 `PROGRESS.md` 进度总表。

**范围说明（实事求是）**：代码层的不变量、执法与崩溃恢复已逐项以真实原语在进程内验证并通过；
单一 daemon 的 HTTP 监听 + 调度循环 + CLI 真连、以及 12–24h 正式长跑属部署/长跑范畴，见"接线与长跑"。
哈希链只证"内部记录自洽未被改"，不等于"内容客观为真"——客观性靠网关留证 + 逐字引用 + 独立验证叠加（§47.2/§13）。

---

## A. 控制与安全（§22.4 策略绕过 / §22.6 浸泡）

| 项 | 执法位置 | 证明测试 | 状态 |
|---|---|---|---|
| 越域 Redirect 被拦 | `network.RedirectInScope`（重定向重过 Scope） | `tests/policybypass` TestCrossOriginRedirectBlocked | ✅ |
| DNS 重绑定/字面私网/云元数据 | `policy.InScope` + `dns_guard`（连接时） | TestDNSRebindLiteralPrivateBlocked | ✅ |
| 非默认端口 / IPv6 / 编码域名 / userinfo 冒充 / WebSocket | `policy.Canonicalize`+`HostInTarget`（fail-closed） | TestNonDefaultPort/IPv6/EncodedDomain/UserinfoSpoof/WebSocketScheme | ✅ |
| Runner 直连 / 浏览器新窗口越域 | Scope 逐请求回查 + EgressLease 架构（无直连出口） | TestOutOfScopeHostBlocked / TestBrowserNewWindowRechecksScope | ✅ |
| Proxy 失效后直连 | `network.Breaker` 熔断 → `RevokeAllTargetEgress` Kill Switch | TestProxyFailureTripsKillSwitch | ✅ |
| 伪造 Evidence ID / 释义冒充 / 跨 Run 引用 | `findings.AddCitation`（逐字核对 + 复合 FK） | `tests/policybypass/evidence_test` 三例 | ✅ |
| Agent 自报升 REPORTABLE | `findings.TransitionFinding`（verifier-only 守卫，§4.3 #7） | TestAgentSelfReportFindingRejected | ✅ |
| Memory 声称"已测"改 Coverage | `coverage.MarkTested`（绑定已验证 Experiment） | TestSelfReportCoverageRejected | ✅ |
| Permit 重放 / 过期 Attempt / 多 Worker 抢配额 | `network`/`scheduler`（Permit 一次性、fencing CAS、capability CAS） | `internal/network/proxy_test`、`internal/scheduler` | ✅（单测；端到端随 daemon） |
| 摘要相同复用已消费 Approval | `control`（approvals consumed_action_id 一次性） | `internal/control` | ✅（单测） |
| Policy 收窄瞬间在途子请求走旧 Revision | Proxy 逐请求 policy_revision 回查 | `internal/network` | ✅（单测） |
| 沙箱 PARTIAL/UNKNOWN 主动测试 fail-closed | `internal/sandbox` 完备度门 | `internal/sandbox` | ✅（单测） |
| 浸泡 §22.6 全部数值门槛 | `tests/soak` 不变量断言 | TestSoak（越域全拦/副作用零重复/无假完成/Canary 零泄漏/预算结算 100%/崩溃恢复/序号无空洞/内存门） | ✅（200/2000 轮；12–24h 长跑在 CI 外） |

## B. 证据与报告（§13 / §47.2）

| 项 | 执法位置 | 证明测试 | 状态 |
|---|---|---|---|
| 伪造无法进报告（只收 REPORTABLE，缺引用拒） | `report.Build`（程序填事实，校验 active verdict + 逐字引用） | `internal/report` TestBuildOnlyReportable / TestBuildRejectsFactWithoutSource | ✅ |
| 手工改 Markdown 不进权威 | `report`（程序渲染并持久化，CVSS 程序算） | TestBuildProgramAuthoritative | ✅ |
| 零发现与未完成严格区分 | `report`（零发现报告）+ `control.ExitGate.Partial`（类型化原因） | TestBuildZeroFinding + `internal/control` | ✅ |
| 反证自动降级 | `findings.ApplyDowngrade`（Verdict revision + Quarantine） | `internal/findings` replay 测试 | ✅ |
| 依赖失效报告标 STALE 不得分发 | `report.Finalize`（重校验 verdict 版本 + 证据 Quarantine） | TestFinalizeStaleOnSupersededVerdict | ✅ |
| 逐字引用可独立重算 + 血缘三元组齐全 | `findings.AddCitation`（content_sha256）+ evidence 血缘列 | `tests/lab` auditEvidenceLineage | ✅ |
| 篡改可证哈希链（删除/重排/篡改内容可检出） | `internal/audit.Chain`（entry_hash = SHA256(JCS‖prev)，离线可验） | `internal/audit` 篡改检测 5 例 + `tests/lab` 验链 | ✅ |

## C. UI（§16.34 验收清单）

| 项 | 实现 | 证明测试 | 状态 |
|---|---|---|---|
| 设计系统 + 核心组件全状态 + 无障碍 | `apps/web` D1 Token + D2 组件 | Vitest 组件测试 | ✅ |
| 快照 + SSE 一致性（补发无空洞/丢旧/resync/断线退避） | `events/consistency`+`stream` | Vitest SSE 端到端 | ✅ |
| 创建→Preflight 门禁→Start | `features/runs/RunWizard` | Vitest 门禁测试 | ✅ |
| Dashboard 真实状态 / Overview 真实分数（无虚假%） | `features/dashboard`+`overview` | Vitest | ✅ |
| Finding 事实/推断分区 + CEBS 高亮 | `features/findings/FindingDetail` | Vitest | ✅ |
| 审批动作 Diff + 不批量批高风险 | `features/approvals` | Vitest | ✅ |
| Evidence 默认脱敏 + Raw 需 Reveal | `components/domain/EvidenceViewer` | Vitest | ✅ |
| 报告只纳 Reportable + 被排除单列 + STALE 禁导出 | `features/report/ReportCenter` | Vitest | ✅ |
| 命令幂等（同 Idempotency-Key 不生成第二份）+ 不 optimistic | `features/commands/idempotency` | Vitest | ✅ |
| UI 断线≠Run 停止 | `components/ConnectionStatus` | Vitest | ✅ |
| Playwright 完整链路 e2e | — | — | ⏸ 本环境不可跑（需浏览器二进制 + 真实 daemon），以 Vitest 组件/集成覆盖同等流程 |

## D. 隔离（§4.0）

| 项 | 执法位置 | 证明测试 | 状态 |
|---|---|---|---|
| 多 Run 并发零串扰（报告不含他 Run Finding） | 复合外键禁跨 Run + report 按 run 过滤 | `tests/lab` TestFullRunMultiRunIsolation | ✅ |
| 跨 Run 证据引用编译期拒 | `findings.AddCitation`（ErrCrossRun） | 同上 + policybypass | ✅ |
| 跨 Run 关联只连同 Workspace 同 Scope | `findings.LinkCrossRun` | `internal/findings` cross_run 测试 | ✅ |
| 审计链每 Run 独立 | `internal/audit`（PK run_id,sequence） | TestChainPerRunIsolated | ✅ |

## E. 卡住 / 韧性（§45 / §51 / §22.3）

| 项 | 执法位置 | 证明测试 | 状态 |
|---|---|---|---|
| Core 崩溃后自动恢复 | 故障注入 Hook + 幂等效果账本 | `tests/faultinjection` + `tests/soak` | ✅ |
| after_model_response_commit 崩溃→续提交不重复 | `agent.Executor`（committed=0 幂等） | TestAfterModelResponseCommitCrashRecovery | ✅ |
| after_exit_gate_pass 崩溃→不半完成留 EXITING | `control.ExitGate`（终态 CAS 同事务 + 幂等重入） | TestAfterExitGatePassCrashNoHalfComplete | ✅ |
| 停滞检测 + 转向（不饿死/不隧道视野） | `scheduler`（无进展/转向/滞回/ε） | `internal/scheduler` + `tests/lab` autonomous | ✅ |
| 自主推进（据新证据建并推进后续 Job） | Strategist→PlanCompiler→Scheduler | `tests/lab` autonomous + fullrun | ✅ |

---

## 接线与长跑（部署/长跑范畴，非代码缺陷）

逐天开发先造组件、再接线，以下属已知待接线项（`PROGRESS.md`「集成待接线清单」逐条登记）：
- 单一 `pentagentd` 的 HTTP 监听 + 调度循环 + CLI 真连（Q-23）；前端连真实后端随之接线。
- 部分故障注入 Hook（permit/network_send/browser_subrequests 等）随对应子系统落地逐个挂。
- 12–24h 正式浸泡长跑：`SNAG_SOAK_ITERS` 放大 + `go test -timeout 24h`，在 CI 外执行；门槛逻辑与短跑一致。
- BlobStore DEK 轮换、providers APIKey、Playwright e2e：各有归属天/环境要求。

## 结论

**代码层 §22.4 全拦、§22.6 不变量全达标、§13 证据/报告不变量全持有、§47.2 哈希链可自证、
§4.0 多 Run 零串扰、§22.3 关键崩溃点恢复符合预期**，并以真实原语的自动化测试逐项证明
（后端全仓双模 + `-race` + `GOOS=windows build` 全 exit 0；前端 115 Vitest 用例 + typecheck + build 全绿）。

达到 **"可连续自主运行版本"** 的代码基线；余下为 daemon 装配与正式长跑的部署工作，不改变上述不变量。
