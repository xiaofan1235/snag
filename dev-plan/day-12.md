# Day 12 —— Reconciler + 每阻塞态超时

> 里程碑：M1 | 依赖：Day 09-11 | 产出：`internal/control/reconciler.go`
> 对照主文档：§19 启动顺序、§5.0.5 阻塞态超时表、§31 C5

## 0. 今天做什么
启动恢复器（Reconciler）：按**固定顺序**恢复（先立红线、作废过期 Permit、标过期 Attempt、增代、对账、恢复 Outbox、最后才允许新动作）；给每个会卡住的状态加超时+升级。

## 1. 进度位置
第 12 天，M1 的可靠性收口。做完后：**杀进程能自动恢复**。

## 2. 模块架构
```
internal/control/reconciler.go  启动时跑一遍恢复流程
```

## 3. 技术选型
纯 Go；用 Day 06-11 的各组件。

## 4. 设计细节
- 启动顺序严格照 §19：①查库与迁移 ②恢复全局 Pause/撤销 ③作废过期 Permit ④标过期 Session/Attempt ⑤增受影响 Job generation ⑥恢复 Action 对账 ⑦恢复 Outbox ⑧启动 Scheduler ⑨**最后才允许新目标动作**。
- 每阻塞态超时+升级表照 §5.0.5（RECONCILING/WAITING_APPROVAL/Identity BLOCKED/Proxy 过期/EXITING/Strategist）。
- DISPATCHED 后崩溃的动作 → OUTCOME_UNKNOWN（§5.3），不自动重试/不标成功。

## 5. 分步开发
1. `Reconcile()`：按 §19 九步实现。
2. 超时扫描器：周期扫各阻塞态，到点执行升级动作。
3. 在 cmd/pentagentd 启动时调用 Reconcile 后再开 Scheduler。

## 6. 完成效果（DoD）
- 模拟"DISPATCHED 后杀进程"→重启后该动作在 OUTCOME_UNKNOWN，不重复执行、不误判成功。
- 无阻塞态永久卡死（各命中超时）。

## 7. 怎么测试
- `tests/reconciler_test.go`：造各种"崩溃现场"（过期 Permit/孤儿 Attempt/未决动作）→Reconcile→断言状态正确、顺序正确。
- 命令：`go test ./internal/control/...`
- 对照验收：§19；§5.0.5；§23 M1"杀掉 Core 后 Job 可恢复"。
