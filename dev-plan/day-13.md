# Day 13 —— CLI 骨架 + Pause/Resume/Stop + 故障注入 Hook

> 里程碑：M1 | 依赖：Day 12 | 产出：`cmd/pentagent/`、`internal/faultinject/`
> 对照主文档：§17 CLI、§39 CLI 对等、§9.7 急停、§22.3 故障注入 Hook

## 0. 今天做什么
CLI 骨架（Cobra）：run create/start/pause/resume/stop/status/watch；Pause 两档语义；以及 **故障注入 Hook 接口**（M0/M1 就定义，后面各模块挂）。

## 1. 进度位置
第 13 天，M1 收尾。做完后：**命令行能开/停一个 Run**（还没有真实动作，但控制流通了）。

## 2. 模块架构
```
cmd/pentagent/        Cobra CLI，全部调本地 API（§39.1）
internal/faultinject/ Hook 点注册表（§22.3 清单）
internal/api/         最小 REST server（Run 生命周期路由）
```

## 3. 技术选型
- Cobra（§2.1）；CLI 只调 API，不碰库（§17）。
- Hook：全局可开关的注入点，测试时能"在这里崩溃"。

## 4. 设计细节
- Pause 两档（§9.7）：`after_current_action` / `now`；CLI `--mode`。
- `--json` 稳定输出；非 TTY 禁交互等待；`--yes` 不代表批准未来危险动作（§17）。
- Hook 点照 §22.3：before/after_permit_consume、before/after_network_send、after_model_response_commit、after_exit_gate_pass 等——**先定义接口**，各模块实现时挂上。

## 5. 分步开发
1. 最小 API server：POST /runs、/start、/pause、/resume、/stop、GET /status。
2. Cobra 命令包这些 API。
3. faultinject 包：`Hook(name)` 在关键路径调用；测试可注册"崩溃"。

## 6. 完成效果（DoD）
- `pentagent run create` → `start` → `status`（RUNNING）→ `pause --mode now` → `stop` 全走通。
- 故障注入 Hook 能在测试里触发模拟崩溃。

## 7. 怎么测试
- `tests/cli_lifecycle_test.go`：脚本跑 create→start→pause→resume→stop，断言状态流转。
- 命令：`go test ./cmd/pentagent/...`
- 对照验收：§23 M1 完成标准；§39.5。
