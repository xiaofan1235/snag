# Day 07 —— Run 状态机（表驱动 + 代码生成 + 属性测试）

> 里程碑：M1 | 依赖：Day 06 | 产出：`internal/control/run_sm.go`、`run_sm.gen.go`
> 对照主文档：§5.1 Run 状态、§1④（迁移表代码生成）、§43.5 枚举

## 0. 今天做什么
把 Run 状态机实现为**完整迁移表**（事件 × 当前状态 × Guard × 下一状态），用代码生成 + 穷举校验，杜绝"隐式漏态"。

## 1. 进度位置
第 7 天。Job/Action 状态机（Day 08/14）照同一套路。做对这个，后面两个很快。

## 2. 模块架构
```
internal/control/
  states.yaml     声明式迁移表（state×event→guard→next）
  gen.go          go:generate：读 yaml 生成 run_sm.gen.go + 状态图.md
  run_controller.go  调用状态机
```

## 3. 技术选型
- 声明式 YAML → `go:generate` 生成 Go（§1④）。
- 属性测试：`pgregory.net/rapid`（§1.1）。

## 4. 设计细节
- 状态照 §5.1：DRAFT/PREFLIGHT/RUNNING/PAUSING/PAUSED/RESUMING/BLOCKED/EXITING/COMPLETED/PARTIAL/STOPPING/STOPPED/FAILED（+QUEUED §33）。
- `Stop/Kill` 必须能从所有非终态进入；`BLOCKED` 恢复统一经 `RESUMING`。
- 生成"未定义组合"的**显式拒绝分支**（收到非法 event 返回 ErrInvalid，不 panic、不卡原态）。
- EXITING 是冻结态（§5.1）：进它要冻结新 Job/Permit/PlanPatch/Surface。

## 5. 分步开发
1. `states.yaml`：照 §5.1 把每个 (状态,事件)→(guard,下一状态) 写全。
2. `gen.go`：生成迁移函数 + 穷举所有 (状态×事件) 是否有定义，没定义的生成拒绝分支。
3. 状态迁移一律**条件更新**（`WHERE revision=?`，走 durable）。

## 6. 完成效果（DoD）
- 任意 (状态, 事件) 都有确定结果（合法迁移或明确拒绝），无 panic、无卡死。
- 生成的状态图 .md 和 §5.1 一致。

## 7. 怎么测试
- 属性测试（rapid）：随机事件序列，断言永不进入未定义转移、revision 单调。
- `tests/run_sm_test.go`：逐行验迁移表；Stop 能从每个非终态进入。
- 命令：`go generate ./... && go test ./internal/control/...`
- 对照验收：§5.1"完整迁移表"；§1④。
