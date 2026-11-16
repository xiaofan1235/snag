# Day 01 —— 项目骨架 + 领域类型（domain）

> 里程碑：M0 契约与骨架 | 依赖：无 | 主要产出：`go.mod`、`internal/domain/*.go`
> 对照主文档：§3 Monorepo、§4.1 核心实体、§25 开发切入顺序

## 0. 今天做什么
搭起 Go monorepo 骨架，定义**领域类型**（Run/Job/Attempt/Action/Evidence/Finding 等的 struct + 枚举 + 错误），零依赖、零 I/O。这是整栋楼的"砖"。

## 1. 进度位置
第 1 天。之后所有模块都 import 这些类型。做完后：能 `go build ./...` 通过，有一套干净的领域类型可用。

## 2. 模块架构
```
internal/domain/   纯类型，不碰数据库、不碰网络（§7 的 ports 思想：core 只依赖自己）
  run.go job.go attempt.go action.go evidence.go finding.go
  hypothesis.go coverage.go events.go errors.go ids.go states.go
```
它是最底层，被 control / scheduler / agent / policy / evidence 等所有包 import。

## 3. 技术选型
- Go 1.22+（§2.1）。ID 用 **UUIDv7**（`github.com/google/uuid` 或自实现时间有序 ID，§2.1）。
- 不引任何重库；枚举用 `type XState string` + 常量。

## 4. 设计细节
- 每个聚合一个文件；字段**照抄主文档 §4.1 的字段清单**（runs/jobs/attempts/actions/evidence/hypotheses/findings）。
- 枚举照抄 §43.5 术语登记表（Run/Job/Action 状态、cancel_code、恢复结果、FindingState）。
- `errors.go`：定义 `DomainError{Code, Msg}`，预置 `ErrConflict`（乐观锁冲突）、`ErrNotFound`、`ErrInvalid`。
- `ids.go`：`NewID()` 返回 UUIDv7 字符串；`key(inputs...)` 对相同输入返回相同 id（给幂等用）。
- 关键不变量（写成注释，后面测试会验证）：Run.revision 单调递增；状态是字符串枚举不是 int。

## 5. 分步开发
1. `go mod init pentagent`；建 §3 的目录骨架（cmd/、internal/、api/、migrations/、tests/）。
2. `internal/domain/states.go`：把 §43.5 的所有枚举写成常量组。
3. 每个聚合一个文件，struct 字段照 §4.1。先不加方法，只要类型。
4. `errors.go` + `ids.go`。
5. `go build ./...` 必须过。

## 6. 完成效果（DoD）
- `go build ./...` 通过，无未用 import。
- 能在 `go test` 里 `domain.NewRun(...)` 造一个 Run 值、设置状态常量。
- 枚举齐全：对照 §43.5 逐一核对，不缺。

## 7. 怎么测试
- 写 `tests/domain_test.go`：对每个枚举断言常量值；构造各聚合零值不 panic。
- 命令：`go test ./internal/domain/...` 全绿。
- 对照验收：主文档 §23 M0"Domain 类型"产出项。
