# Day 30 —— Scheduler（评分 + 滞回 + ε 探索）+ 无进展/转向

> 里程碑：M3 | 依赖：Day 29 | 产出：`internal/scheduler/scheduler.go priority.go dedupe.go`
> 对照主文档：§6.8 调度优先级、§6.9 无进展、§32.5 卡住转向、§1⑤

## 0. 今天做什么
Scheduler：先过滤再评分派发 Job；动作指纹去重（软降权）；无进展检测 + 转向阶梯（Job→方向→Epoch→Reframe）。做完 M3 = **AI 能自主推进**。

## 1. 进度位置
第 30 天，M3 收口、项目脊椎完成。做完后：**AI 能根据新证据动态创建并推进后续 Job**。

## 2. 模块架构
```
internal/scheduler/
  scheduler.go priority.go dedupe.go budget.go
```

## 3. 技术选型
- 内存优先队列（container/heap）+ DB claim（Day 09）。
- 评分纯函数（可测）。

## 4. 设计细节
- 先过滤（Scope/Policy/Dependency/Identity/Budget/Approval/Rate）再评分（§6.8）。
- 评分因子照 §6.8（potential_impact/expected_information/chain_value/.../age 防饿死）。
- **滞回**（§1⑤）：切主攻方向需分差超阈值；**ε 探索配额**防隧道视野。
- 动作指纹（§6.9）去重：**默认软降权**，硬拒只留字节级全重复（§2.3-2，别杀 AI 思考）。
- 无进展/转向（§32.5）：动作/新信息比趋 0=停滞→方向降权/泊车→Worker 流向他处→Epoch 切换→3 Epoch 无进展→Reframe；进展含 insight（新假设也算）。
- 同 Scope 多 Run 默认不并行（§6.8）。

## 5. 分步开发
1. 过滤 + 评分 + 派发。
2. 指纹去重（软降权）。
3. 无进展检测 + 转向阶梯 + 方向泊车。

## 6. 完成效果（DoD）
- 用 Mock Provider + Fake Runner + Mock Target 跑一个小 Run：AI 创建 Job→执行→观察→Strategist 据新证据再创建 Job（不是固定脚本）；一个方向停滞→Worker 转向。

## 7. 怎么测试
- `tests/scheduler_test.go`：评分纯函数；重复指纹降权；停滞→转向；饿死防护。
- 端到端：`tests/lab/autonomous_test.go`（Mock 全家桶）跑通自主推进。
- 命令：`go test ./internal/scheduler/... ./tests/lab/...`
- 对照验收：§23 M3"AI 能根据新 Evidence 动态创建后续 Job"。
