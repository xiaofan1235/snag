# Day 44 —— 故障注入全套 + Policy Bypass Suite

> 里程碑：M7 加固 | 依赖：全部 | 产出：`tests/faultinjection/`、`tests/policybypass/`
> 对照主文档：§22.3 故障注入、§22.4 策略绕过

## 0. 今天做什么
在所有 Hook 点逐个模拟崩溃验证恢复；跑完整策略绕过测试套件。

## 1. 进度位置
第 44 天，M7 开端。这是"敢说它安全"的前提。

## 3. 技术选型
Day 13 的 faultinject Hook + Testcontainers（§2.1）。

## 4. 设计细节
- 故障注入关键断言照 §22.3（after_network_send 崩溃→RECONCILING 不直接重排写动作；after_model_response_commit→续提交不重复；after_exit_gate_pass 前并发→放弃完成；浏览器第 N 子请求崩溃→Capability 与流量一致；旧 Fencing 恢复→Permit 消费与 Submit 全拒）。
- 策略绕过至少覆盖 §22.4 全清单（越域 Redirect/DNS 重绑定/非默认端口/IPv6/编码域名/Runner 直连/Browser 新窗口/WebSocket/Permit 重放/过期 Attempt/多 Worker 抢配额/Proxy 失效直连/伪造 Evidence ID/释义冒充/摘要相同复用 Approval/收窄瞬间在途子请求/沙箱 PARTIAL 主动测试/Memory 改 Coverage）。

## 5. 分步开发
1. 逐 Hook 崩溃注入测试。
2. 策略绕过套件（照 §22.4 逐条）。

## 6. 完成效果（DoD）
- 每个 Hook 崩溃后恢复符合预期；策略绕过套件全部被拦（fail-closed）。

## 7. 怎么测试
- 命令：`go test ./tests/faultinjection/... ./tests/policybypass/...`
- 对照验收：§22.3、§22.4 全清单。
