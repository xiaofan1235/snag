# Snag 开发进度 PROGRESS

> 本文件由 **Claude（开发方）** 维护，实时记录开发进度。
> 每完成一天：把该行状态改为 `✅已完成`，填 commit 短哈希与日期，必要时写备注。
> 规则见 `README.md` 协作流程；详细任务见 `dev-plan/day-NN.md`。

## 状态图例
```
⬜未开始   🟨开发中   ✅已完成(已推送)   🧪测试中   🐞有bug待修   ✔️测试通过
```

## 当前位置

**Day 01 —— 项目骨架 + 领域类型 ✅（已提交 `860272b`，待推送）**
下一步：**Day 02（数据模型 Part1 + Store 骨架）**，等待 Owner 批准后开始。

Claude 开发前自检：`TESTING.md` 顶部「未解决 bug」= 无。
待推送说明：`snag` 仓库已建，但本会话 GitHub 凭据暂无该仓库访问权，需 Owner 在 Claude GitHub App 可访问仓库列表中加入 `snag`，之后由 Claude 推送 `860272b`。

---

## 进度总表

| Day | 里程碑 | 模块 | 状态 | Commit | 日期 | 备注 |
|---|---|---|---|---|---|---|
| 01 | M0 | 项目骨架 + 领域类型（domain） | ✅已完成 | `860272b` | 2026-11-16 | go build/vet/test 全绿；8/8 测试；枚举对照 §43.5 齐全。待推送 |
| 02 | M0 | 数据模型 Part1 + Store 骨架 | ⬜未开始 |  |  |  |
| 03 | M0 | 数据模型 Part2（补齐全部表） | ⬜未开始 |  |  |  |
| 04 | M0 | 四大契约冻结 + JCS 规范化（最关键的一天） | ⬜未开始 |  |  |  |
| 05 | M0 | 事件注册表 + OpenAPI 骨架 + Mock Target | ⬜未开始 |  |  |  |
| 06 | M1 | 单写者 Store（durable/bulk 双连接） | ⬜未开始 |  |  |  |
| 07 | M1 | Run 状态机（表驱动 + 代码生成 + 属性测试） | ⬜未开始 |  |  |  |
| 08 | M1 | Job 状态机 + RECONCILING + 取消语义 | ⬜未开始 |  |  |  |
| 09 | M1 | Attempt + Claim/Lease/Fencing | ⬜未开始 |  |  |  |
| 10 | M1 | Event Journal + Outbox（提交后发布） | ⬜未开始 |  |  |  |
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
- **2026-11-16** — Day 01 完成（提交 `860272b`）：`go mod init pentagent` + §3 目录骨架；`internal/domain` 全套零依赖领域类型（states/ids/errors + 12 个聚合照 §4.1）；UUIDv7 自实现（时间有序+同毫秒单调），避免引入外部库以适配无网络构建；8 组单元测试全绿，枚举对照 §43.5 逐一核对无缺。阻塞：本会话 GitHub 凭据暂无 `snag` 访问权，`860272b` 待 Owner 授权后推送。下一步：Day 02。
