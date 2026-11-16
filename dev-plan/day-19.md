# Day 19 —— Sandbox + Egress + Fake Runner

> 里程碑：M2 | 依赖：Day 18 | 产出：`internal/sandbox/`、`runners/fake/`
> 对照主文档：§11 沙箱、§31 P5x 分级沙箱、§10.1 Egress、§25（Fake Runner）

## 0. 今天做什么
隔离执行面：分级沙箱（容器/降级）+ attestation（FULL/PARTIAL/NONE/UNKNOWN）+ Egress 管理；加一个 **Fake Runner**（不碰真目标，只回放，用于无 AI 测执行链）。

## 1. 进度位置
第 19 天。做完后：执行面闭环（提案→Permit→Proxy→Runner→回执）能用假组件跑通。

## 2. 模块架构
```
internal/sandbox/   manager/container/resources/network
internal/network/egress.go   代理租约、禁直连
runners/fake/       假 Runner：收动作→经 Proxy 打 Mock Target→回回执
```

## 3. 技术选型
- 分级沙箱（§31 P5x）：有容器用容器（seccomp+Landlock/AppContainer/Seatbelt），没有就降级，attestation 门禁。
- Runner↔Core：gRPC + Protobuf（§2.1）。

## 4. 设计细节
- 容器默认：non-root/cap_drop ALL/read_only_rootfs/no_docker_socket/no_host_network（§11.1）。
- **无默认路由**，唯一出口 Policy Proxy（§11.1）。
- attestation NONE/UNKNOWN → fail-closed 禁主动测试（§31 P5x）。
- Egress：代理租约、禁本机直连（§10.1，本机模式留到 §38/Day 后续）。
- Fake Runner：实现 runner.proto，收到动作后经 Proxy 请求 Mock Target，返回 Receipt。

## 5. 分步开发
1. runner.proto + gRPC server（Fake Runner）。
2. Sandbox manager（先支持"降级进程沙箱"即可跑起来）。
3. Egress lease。

## 6. 完成效果（DoD）
- Fake Runner 收到一个只读动作→经 Proxy→打 Mock Target→回 Receipt，全链路通。
- 沙箱 attestation 能报 FULL/PARTIAL/NONE。

## 7. 怎么测试
- `tests/fake_runner_test.go`：下发只读动作→Receipt 正确、流量经 Proxy 留证。
- 命令：`go test ./runners/... ./internal/sandbox/...`
- 对照验收：§23 M2"没有 AI 也能通过 CLI 发起一个受控只读动作"。
