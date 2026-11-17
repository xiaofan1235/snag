# UI 开发文档 v1.6：Claude 客户端风格的自主渗透操作台

> 配套主文档：《独立 AI 渗透 Agent 框架开发设计方案 v2.6》（`pentagent-dev-spec-v2.6.md`）
> 本文档是整个项目 Web 界面的完整开发依据，由两部分组成：
>
> - **第一部分（§16.x）**：信息架构、各页面详细设计、页面-API-SSE 对照、前端工程拆分、实时/性能/安全规格、开发优先级与验收。**后端契约、事件、API 以主文档第 15 章及各章为准**；本文中对「§N.M」（非 16.x）的引用指向主文档。
> - **第二部分（设计系统·规范化版）**：四条设计原则、设计 Token（亮/暗）、**标准化组件库**（按钮/输入/下拉/进度/状态徽章/表格/弹窗/抽屉等，尽量规范）、三区布局、**完整界面地图与功能布局**、创新功能点、**性能优先（不卡顿硬门禁）**、无障碍、知识库 UI、落地顺序。
>
> - **第三部分（交互覆盖审计与新增界面）**：全交互点覆盖审计（确认所有需要人决定/输入/确认/查看的地方都有 UI），以及出口模式选择、工具/MCP 管理、联网知识源三块新界面。
>
> - **第四部分（浏览器性能、Skill 导入、知识筛选）**：浏览器观测台为什么不卡（只显示抓取物，不渲染实时浏览器）、人工接管的 screencast、Skill 导入界面、知识分类筛选与试检索 UI。
>
> - **第五部分（高级测试、韧性、完整性界面）**：序列/竞态实验与 OAST、会话健康与自动重认证、授权窗收尾、Finding 跨 Run 历史与目标趋势、审计验链、上下文检索预算与不可信内容标注。
>
> - **第六部分（代理池、浏览器源、MCP 开放、数据目录）**：代理池管理与免费代理风险标注、浏览器源优先级与反指纹、MCP 开放管理、数据目录与备份设置。
>
> v1.5 相对 v1.0 新增：组件库标准化、完整界面地图与功能布局、性能硬约束、知识库 UI、交互覆盖审计、出口/工具-MCP/联网知识、浏览器性能、Skill/知识筛选、序列竞态/会话韧性/完整性、代理池/浏览器源/MCP 开放/数据目录新界面（对应主文档第 34–50 章）。
>
> 一句话定位：**像 Claude 一样平静好看，但组织逻辑是「监督一个长期自主的同事」，不是「和一个助手对话」。**

---

# 第一部分：UI 信息架构与页面详细设计（原 §16）

# 16. Web UI 详细设计

## 16.0 产品定位与开源项目参考

UI 不是“给 CLI 加一个状态网页”，也不是一个聊天窗口。它是长时自主测试的 **操作台、证据台和运维台**：用户离开数小时后回来，必须能在 1 分钟内知道当前在做什么、为什么做、哪里被阻塞、产生了什么真实证据，以及哪些动作正在等待人工决定。

本章只参考现有二开文档中的产品机制，前端代码全部独立实现。

| 参考项目 | 值得吸收的 UI/交互能力 | 本框架的实现方式 | 不直接照搬的部分 |
|---|---|---|---|
| Pentagi | Flow/Task/Subtask 工作台、消息流、工具调用、终端日志、截图、Flow Files、Knowledge、Resource、按 Agent 配模型、模板 | 对应 Run/Job/Attempt、活动流、证据与文件、知识库、Model Profile | 不把递归 Agent 对话当权威状态，不开放任意容器 Shell |
| ARTEX | 资产图、探索图、活动流、Traffic、Finding 血缘、意图级暂停/取消/人工消息、审批历史、复测、通知 | 资产与攻击面联动、Plan/Job 血缘、受控人工干预、Finding Retest | 不采用其数据库和前端代码；审批失败必须 fail-closed |
| Strix | Agent 图、运行 Viewer、漏洞报告、HTTP Exchange 证据绑定 | Job/Agent 双视图、Finding Evidence Drawer、可重放流量 | 不由 Agent 自己宣布报告完成，不直接复用其 Viewer |
| Shannon | 扫描历史、状态/日志、Resume、认证与模型 Preflight、Partial 原因、最终报告 | Run 历史、恢复前检查、逐 Agent 日志、退出门禁和降级原因展示 | 不引入 Temporal；不把 Blocked Finding 静默丢弃 |
| OpenCode | C/S 多客户端、统一事件流、断线重放、会话命令、权限交互 | CLI/UI 同后端、Snapshot+SSE、命令幂等和权限审批 | 不做插件市场，不把服务器状态复制到多个客户端 |
| DeepSeek Harness | 工具卡片、审批卡片、终端、设置、会话恢复、长会话性能门禁 | 统一 Action Card、可回放工具结果、长列表与重连性能测试 | 不采用进程内任意插件和宿主机执行模式 |

必须形成六个闭环，而不是只做页面展示：

```text
观察：Run → Job → Action → Evidence
规划：Strategy Epoch → PlanPatch → Job Graph
干预：UserDirective → 校验 → Strategist/Executor → 执行结果
验证：Finding → Verifier → Replay → Reportable
运维：Provider/Proxy/Sandbox → Preflight → 阻塞恢复
审计：用户命令 → 服务端命令 → 状态事件 → 不可变审计记录
```

## 16.1 信息架构与总体布局

### 全局导航

```text
首页 Dashboard
任务 Runs
  ├─ 全部任务 / 运行中 / 等待人工 / 已暂停 / 已结束
  └─ Run 模板 / 导入导出
工作空间 Workbench（进入某个 Run 后）
  ├─ 总览 Overview
  ├─ 实时活动 Activity
  ├─ 策略与计划 Strategy
  ├─ Agent / Jobs
  ├─ 资产与攻击面 Surface
  ├─ 假设 Hypotheses
  ├─ 漏洞 Findings
  ├─ 流量 Traffic
  ├─ 证据 Evidence
  ├─ 浏览器 Browser
  ├─ 文件与产物 Files
  ├─ 覆盖率 Coverage
  ├─ 时间线 Timeline
  └─ 报告 Reports
收件箱 Inbox
  ├─ 待审批 / 阻塞 / 通知
知识与能力 Resources
  ├─ Knowledge / Skills / Tools / Resources
系统运维 Operations
  ├─ Providers / Models / Egress / Sandboxes
  ├─ Budgets / Audit / Diagnostics
设置 Settings
```

### Run 工作台布局

```text
┌──────────────────────────────────────────────────────────────────────┐
│ Run 切换 | 状态/等待原因 | Proxy | Model | Budget | Pause | Stop    │
├────────────┬───────────────────────────────────┬─────────────────────┤
│ 页面导航   │ 主内容区                            │ 上下文 Inspector    │
│ 带阻塞角标 │ 表格/图/活动流/编辑器               │ 当前对象详情与操作  │
│            │                                   │ Evidence/Job/Policy │
├────────────┴───────────────────────────────────┴─────────────────────┤
│ 可收起：指令输入 / 审批卡片 / 当前动作 / 连接与同步状态              │
└──────────────────────────────────────────────────────────────────────┘
```

右上角固定：

- Run 状态，以及 `RUNNING` 之外的明确等待原因；
- 活跃 Worker、待审批数、OutcomeUnknown 数；
- 代理出口和租约过期状态；
- Provider 健康和降级状态；
- 请求、Token、费用和时间预算；
- `Pause after current action`、`Pause now`、`Stop` 三种不同操作；
- SSE 连接状态和最后同步序号。

全局组件：

- `Ctrl/Cmd+K` 命令面板：跳转 Run、Finding、Job、Evidence，执行 Pause、创建复测等受控命令；
- 全局搜索：按 ID、URL、域名、标题、哈希、错误码检索；
- Inbox 角标：审批、Finding 待处理、代理过期、Provider 故障、清理任务；
- 对象 Deep Link：`/runs/:id/traffic/:exchangeId` 等 URL 可直接分享本机链接和恢复现场；
- 所有列表支持列配置、筛选、排序、保存视图和复制筛选链接；
- 右侧 Inspector 可固定，支持 Finding↔Evidence↔Action↔Job 双向跳转。

## 16.2 Run 创建向导

### 第一步：目标

字段：

- Run 名称；
- Workspace：新建，或选择同一授权工作目标的历史 Workspace；
- 起始 URL；
- 精确域名；
- 是否显式包含子域名；
- Scheme/端口；
- 路径范围；
- 禁止路径；
- 授权材料引用。

右侧实时显示规范化 Scope。

### 第二步：测试规则

- 未验证接口允许方法；
- 已验证接口允许方法；
- 是否允许自有测试对象写操作；
- IDOR 配额；
- 速率；
- 总请求数；
- 授权时间窗；
- OAST 配置；
- 数据保留规则。

默认全部保守。

### 第三步：身份

- Anonymous；
- 测试账号 A/B；
- 角色；
- 租户；
- Cookie/Token SecretRef；
- 有效期；
- 登录验证。

原始 Cookie 不在普通表单回显。

### 第四步：执行环境

- 模型 Profile；
- 代理 Pool；
- 沙箱状态；
- 最大 Worker；
- 费用预算；
- Token 预算；
- 最大运行时长。

### 第五步：策略确认

展示：

```text
允许访问什么
允许执行什么
明确禁止什么
哪些动作会触发审批
预算和停止条件
```

只有 Preflight 全部通过才能 Start。

## 16.3 Run 总览

展示：

- 当前策略 Epoch；
- 当前优先方向；
- 活跃 Agent；
- 等待中的 Job；
- 最近高价值事件；
- 已确认 Finding；
- 请求与模型预算；
- 覆盖摘要；
- Blocker；
- 下一步调度计划。

不要只显示一个虚假的“73% 完成”。

应显示：

```text
接口发现：当前 SurfaceRevision
实验：DONE / TOTAL
覆盖：TESTED / ELIGIBLE
验证：CONFIRMED / CANDIDATE
阻塞：具体原因
```

## 16.4 Agent/Job 页面

两种视图：

### 图视图

节点：

```text
Strategist
Recon Job
Module Job
Experiment Job
Verifier
Report
```

颜色区分状态，不用颜色作为唯一信息。

### 表格视图

字段：

```text
Job 类型
模块
状态
当前 Attempt
当前动作
优先级
预算
租约
创建原因
```

点击节点打开 Drawer：

- JobSpec；
- 输入 Evidence；
- 当前进度摘要；
- 最近事件；
- 失败原因；
- 子 Job；
- 用户指令；
- Cancel/Reprioritize。

大型任务默认按模块聚合，不一次渲染数千节点。

## 16.5 攻击面页面

筛选：

```text
模块
Method
认证类型
状态码
来源
是否有参数
是否有 Finding
是否覆盖
```

Endpoint 详情：

- 参数；
- 请求样例；
- 身份；
- 相关 Evidence；
- 技术栈；
- 相关 Hypothesis；
- 覆盖单元格；
- 关联 Job。

## 16.6 Hypothesis 看板

列：

```text
未测
需要上下文
测试中
需要补证据
已证实
已证伪
红线阻塞
```

卡片展示：

- 假设；
- 信任边界；
- 预期影响；
- 反证条件；
- 当前证据；
- 下一步最小实验；
- 停止条件。

## 16.7 Finding 页面

列表字段：

```text
标题
类型
严重度
状态
影响对象
证据完整度
Verifier
更新时间
```

详情页：

```text
事实与推断分区
受影响接口
账号和对象关系
Baseline / Variant 对比
原始 Evidence
Verifier 意见
反对证据
定级理由
清理状态
报告预览
```

“请求重放”必须创建新 Job，不是前端直接发请求。

## 16.8 覆盖率页面

支持：

- 按模块/接口切换；
- 按身份过滤；
- 按攻击类型过滤；
- 查看 `TESTED/NA/BLOCKED/STALE`；
- 单元格点击显示具体 Action 和 Evidence；
- 查看 SurfaceRevision；
- 查看未测原因。

## 16.9 Evidence 页面

Tab：

```text
请求
响应
差异
时间信息
截图
元数据
哈希
关联 Finding
```

注意：

- 默认显示脱敏 Projection；
- 大响应分块加载；
- 原始内容显示需要 Reveal；
- HTML/JS 不直接执行；
- 链接不自动打开；
- Markdown 禁止 Raw HTML；
- 复制重放请求不等于直接执行。

## 16.10 审批中心

每条审批显示动作 Diff：

```text
正常动作 → 待审批动作
身份
对象
副作用
策略原因
过期时间
```

操作：

- Approve Once；
- Deny；
- Deny and Cancel Related Jobs。

## 16.11 用户干预

Run 页面提供“指令输入”：

```text
优先订单模块
先补齐当前 Finding 证据
暂停注入方向
减少并发
继续处理未覆盖接口
```

指令不会直接改变数据库，而是：

```text
UserDirective
→ 控制层校验
→ 新事件
→ Strategist 重规划
```

扩大目标的指令必须进入 Policy Revision 和审批流程。

## 16.12 全局 Dashboard 与 Run 历史

### Dashboard

不是单纯统计图，而是多任务值班台。首屏分四区：

1. **需要我处理**：待审批、认证失效、代理过期、预算即将耗尽、OutcomeUnknown、清理失败；
2. **正在运行**：每个 Run 显示当前策略、活跃 Job、最近动作、最近 Evidence、速率和预算；
3. **最近结果**：新确认 Finding、报告完成、复测结果、覆盖门禁变化；
4. **系统健康**：Provider、Egress、Sandbox、数据库、Blob Store、磁盘和事件流状态。

Run 卡片必须显示真实状态：终态 `COMPLETED/PARTIAL/STOPPED/FAILED` 与可恢复非终态 `BLOCKED/PAUSED` 视觉区分（BLOCKED 提供恢复入口），不能把所有结束都画成绿色成功。

操作：打开工作台、暂停、恢复、停止、克隆配置、查看阻塞原因。卡片操作均调用命令 API，不在前端修改状态。

接口/事件：`GET /v1/dashboard`、`GET /v1/runs`；消费 `run.updated`、`approval.requested`、`finding.updated`、`system.warning`、`budget.updated`。

### Run 列表与历史

- 按状态、目标、模板、创建时间、严重度、是否有待办筛选；
- 列表显示 Scope 摘要、运行时长、Job 统计、Finding 统计、费用、终态原因和最后活跃时间；
- 支持归档、克隆为新 Run、导出诊断包/证据包；
- Resume 前展示恢复预览：未完成 Job、未知结果 Action、过期身份、失效代理、策略版本和 Schema 兼容性；
- 归档只是检索视图变化，不能删除 Evidence；删除走独立保留策略和二次确认；
- 首版不提供批量选择多个不同目标后一键开测，避免演变成无授权批量扫描器。

接口/事件：`POST /v1/runs/{id}/clone`、`POST /v1/runs/{id}/archive`、`GET /v1/runs/{id}/recovery-preview`、`POST /v1/runs/{id}/resume`。

## 16.13 实时活动流与 Agent Session

活动流统一展示 Planner、Executor、Verifier、Runner、Policy 和用户行为，不再分别做互不相通的“聊天日志”和“系统日志”。

### 事件卡片

```text
PlanCard         当前目标、计划摘要、PlanPatch 结果
ActionCard       工具、规范化参数、Policy 决策、Permit、执行回执
ObservationCard  状态码、差异摘要、发现的事实、关联 Evidence
DecisionCard     创建/取消/重排了哪些 Job，以及原因
ApprovalCard     等待审批、倒计时、审批结果
ErrorCard        错误分类、是否可重试、恢复建议
BudgetCard       Token/费用/请求/时间预算变化
UserCard         用户指令、审批和人工接管记录
```

- 模型输出展示简短计划、结论和可审计理由，不保存或展示原始隐藏思维链；
- 工具参数和结果使用 Typed Card，超大输出只显示摘要并链接 Artifact；
- 支持按 Agent、Job、Attempt、Action、严重度、错误码和事件类型过滤；
- `Live Follow` 自动滚动，用户向上阅读时暂停跟随并显示“有 N 条新事件”；
- 支持从任一事件跳转到对应 Job、HTTP Exchange、Evidence 和 Finding；
- 支持“从此处查看后续时间线”，但不能从旧事件分叉执行状态。

### Agent Session Drawer

展示角色、Model Profile、Job/Attempt、Generation、上下文 Revision、压缩次数、Token、当前工具集、最近输入摘要和最后错误。上下文查看器展示实际发送给模型的 **Redacted Projection 元数据和内容**，便于排查“模型为什么不知道某事实”。

操作：发送下一条受控消息、暂停后续动作、请求安全重试、查看完整审计。没有“编辑历史消息并覆盖原记录”。

接口/事件：`GET /v1/runs/{id}/activities`、`GET /v1/agent-sessions/{id}`、`GET /v1/agent-sessions/{id}/context-revisions`；消费 `activity.created`、`agent.progress`、`action.updated`、`command.completed`。

## 16.14 Strategy / PlanPatch 页面

该页面回答“AI 为什么现在做这件事”，补足只有 Job 图而没有自主规划解释的问题。

展示：

- 当前 Strategy Epoch、触发原因、创建时间、基于的 Snapshot Revision；
- 当前优先方向、暂停方向、无进展计数和最近 Reframe；
- 每个 PlanPatch 的操作 Diff：新增假设、创建 Job、调整优先级、请求审批、关闭 Epoch；
- PlanCompiler 逐项结果：`ACCEPTED/DEDUPED/REJECTED/NEEDS_APPROVAL` 及原因；
- Job 依赖和输入 Evidence；
- 预算消耗、覆盖变化、Finding 变化与策略效果回看。

交互：

- 用户可“优先此方向”“降低此方向优先级”“要求解释此 Job”“停止同类新 Job”；
- 操作均生成 UserDirective，不直接编辑 PlanPatch 或数据库；
- 已拒绝 Patch 保留只读记录，不允许 UI 强制通过；
- 切换历史 Epoch 是只读回看，顶部持续显示“正在查看历史”。

接口/事件：`GET /v1/runs/{id}/strategy-epochs`、`GET /v1/plan-patches/{id}`、`POST /v1/runs/{id}/directives`；消费 `plan.updated`、`job.created`、`policy.denied`。

## 16.15 资产图与 Discovery 线索

攻击面表格之外增加关系视图和线索收件箱。

### 资产图

节点类型：Domain、Host、Application、Endpoint、Identity、Technology、Finding。边类型：`OBSERVED_ON`、`CONTAINS`、`USES_IDENTITY`、`HAS_FINDING`、`DERIVED_FROM`。

- 默认先按 Application/模块聚合，点击后懒加载 Endpoint；
- 范围内资产、范围外仅记录资产使用不同形状和文字标记；
- 点击节点显示来源 Evidence、首次/最后发现时间、关联 Job、测试覆盖和 Finding；
- “创建分析 Job”仅对当前 Scope 内对象可用；范围外对象没有测试、重放、扫描按钮；
- 用户备注和标签是独立元数据，不能改写原始发现事实。

### Discovery Inbox

汇总尚未转化为正式 Hypothesis/Job 的线索：新参数、异常响应、JS 引用、身份差异、链候选、被策略阻止的方向。字段包括来源、置信度、是否过期、跟进状态、关联 Evidence 和排除原因。

操作：交给 Strategist 评估、标记重复、添加用户备注。`Dismiss` 只影响后续规划，不删除来源数据。

接口/事件：`GET /v1/runs/{id}/asset-graph`、`GET /v1/runs/{id}/discoveries`、`POST /v1/discoveries/{id}/directives`；消费 `asset.updated`、`discovery.updated`、`evidence.created`。

## 16.16 Traffic / Repeater / Diff 工作台

参考渗透人员熟悉的代理工具，但所有发送仍由后端 Job 执行。

### 三栏布局

```text
左：Exchange 列表与筛选
中：请求/响应、Headers/Body、Raw/Hex、时间线
右：关联 Action / Identity / Permit / Evidence / Finding
```

筛选：Host、Path、Method、Status、Identity、Job、工具、时间、是否越权差异、是否有 Evidence、是否包含秘密标记。

功能：

- 请求与响应并排或上下布局，支持搜索、换行、编码查看、JSON Tree、二进制 Hex；
- Baseline 与 Variant 选择后生成状态、Header、Body、耗时和关键字段 Diff；
- 展示重定向链和每跳 Policy 决策，防止只看到最终 URL；
- `Replay` 打开受控重放编辑器，默认沿用原身份、Scope、Proxy 和预算；
- 编辑后先展示 Canonical Action Diff，再创建 `VERIFY/EXPERIMENT` Job；
- 目标、身份、方法、Body 或副作用等级变化时重新走 Policy，不继承旧 Permit；
- 可生成证据包、复制脱敏 HTTP 文本，不生成绕过代理的本机直连命令。

安全：

- Cookie、Authorization、Set-Cookie 和秘密参数由服务端 Projection 脱敏；
- HTML 不直接渲染，响应预览使用沙箱 iframe 且禁止脚本、表单和外部资源；
- Repeater 没有浏览器直连目标的隐藏 fetch 通道；
- 重放按钮显示实际 Job ID 和结果，点击后不是“假设已发送”。

接口/事件：`GET /v1/runs/{id}/exchanges`、`GET /v1/exchanges/{id}`、`POST /v1/exchanges/diff`、`POST /v1/runs/{id}/replay-jobs`；消费 `exchange.created`、`action.updated`、`evidence.created`。

## 16.17 Browser Observatory

用于观察认证、JS 应用和多步业务测试，不把浏览器变成旁路攻击通道。

页面：

- Browser Session 列表：Identity、容器、状态、当前页面、创建时间、最后动作；
- Screenshots：按 Action 关联，支持缩略图、原图权限控制和前后对比；
- DOM/Accessibility Snapshot：只读文本，可搜索，不执行页面代码；
- Network：只显示经 Policy Proxy 留证的请求；
- Console/Error：用于定位前端异常和工具失败；
- Trace：按 Job 下载 Playwright Trace 或结构化步骤；
- Cookie/Storage：只显示键名、作用域、过期时间和脱敏摘要。

交互：

- 默认是只读实时画面，不开放任意 CDP；
- “人工接管”需新 Approval，并绑定 Identity、Run、PolicyRevision 和时限；
- 接管期间每次导航、输入、点击、上传均形成 Action，并经同一 Scope/Policy 检查；
- 无法纳入 Action Ledger 的原生交互不提供；
- 结束接管后立即撤销控制 Lease，但保留已执行事实。

接口/事件：`GET /v1/runs/{id}/browser-sessions`、`GET /v1/browser-sessions/{id}/artifacts`、`POST /v1/browser-sessions/{id}/takeover-requests`；消费 `browser.updated`、`artifact.created`、`action.updated`。

## 16.18 工具执行与终端观测

Pentagi 的终端日志值得保留，但首版 UI 不提供宿主机交互 Shell。

提供四种视图：

1. **Action View**：按动作查看规范化输入、策略决策、stdout/stderr、退出码和回执；
2. **Tool Timeline**：同 Job 内调用顺序、耗时、重试和降级；
3. **Terminal View**：xterm.js 渲染沙箱内受控命令的 ANSI 输出，只读；
4. **Error View**：按 `TOOL_UNAVAILABLE/TIMEOUT/POLICY_DENIED/OUTCOME_UNKNOWN` 分类聚合。

如提供“手动沙箱命令”，它必须创建 Manual Job，经过工具 Manifest、参数 Schema、Policy、一次性 Permit 和 Runner；UI 不开放 `/exec?cmd=...` 式直通接口。具有副作用或结果未知的动作禁止盲目自动重试。

接口/事件：`GET /v1/jobs/{id}/actions`、`GET /v1/actions/{id}/output`、`POST /v1/runs/{id}/manual-jobs`；消费 `action.updated`、`activity.created`。

## 16.19 Files / Artifacts / Resources

### Run Files

树形展示任务产物：JS 文件、页面快照、脚本、PoC、截图、Trace、报告、工具输出、临时测试数据说明。字段：路径、类型、大小、SHA-256、来源 Job、创建时间、保留级别和是否含敏感数据。

操作：安全预览、下载、复制路径、关联 Finding、加入证据包。文本预览限制大小，大文件分块；压缩包先列目录，不自动解压执行。

### 用户 Resource 库

用于上传授权材料、测试账号说明、字典、已知请求样例和业务文档。每个 Resource 有 Scope、标签、哈希、上传者、有效期和允许使用的 Run/Model Profile。

- Secret 文件必须进入 Vault，不作为普通文本注入模型；
- 上传校验大小、扩展名、路径穿越、压缩炸弹和符号链接；
- 文件内容是“不可信数据”，不得因文件内写有指令就改变系统策略；
- 删除 Resource 不级联删除已经引用它的 Evidence。

接口/事件：`GET /v1/runs/{id}/artifacts`、`GET /v1/artifacts/{id}/content`、`GET/POST /v1/resources`、`POST /v1/resources/{id}/bind`；消费 `artifact.created`、`resource.updated`。

## 16.20 Knowledge / Skills / Run Templates

### Knowledge

Memory Observation 的搜索、Timeline、来源回链、Gap 和健康页面见第 26.10 节。Knowledge 与 Memory 的边界是：前者为审核后的方法知识，后者为可过期的派生经验。

- 全局知识与 Run 私有知识分区；
- 条目显示来源、Evidence 链接、可信级别、适用技术栈、创建时间、TTL 和最近命中；
- 区分 `VERIFIED_FACT / OPERATOR_NOTE / UNVERIFIED_REFERENCE`；
- 支持搜索、预览、禁用、过期和手工修正，修正保留版本历史；
- 跨 Run 只复用方法知识，不默认共享原始用户数据、Cookie 或敏感 Evidence。

### Skills 与 Prompt Templates

- 展示 Skill 版本、来源、哈希、适用 Job、所需工具和策略能力；
- 支持启用/禁用、优先级、变量 Schema、渲染预览和测试输入；
- Prompt 修改显示 Diff、版本和引用它的 Model/Agent Profile；
- 校验未定义变量、工具引用不存在、模板渲染失败和策略冲突；
- Skill/Prompt 只是策略知识，不能声明扩大 Scope、关闭审批或获取秘密；
- 首版只导入本地受审包，不提供在线市场一键安装。

### Run Templates

保存 Scope 结构、预算、工具集、模型槽位、测试规则和通知偏好；Secret 只保存 SecretRef，不保存原值。克隆模板时显示配置 Diff，并重新执行 Preflight。

接口/事件：`GET/POST /v1/knowledge`、`GET /v1/skills`、`GET/POST /v1/prompt-templates`、`GET/POST /v1/run-templates`；消费 `knowledge.updated`、`skill.updated`、`template.updated`。

## 16.21 Finding 分诊、去重与复测工作流

在基础 Finding 列表上增加完整处置闭环：

### 分诊视图

- 列表、看板、按攻击链分组三种视图；
- 状态使用 13.3 节唯一 FindingState 枚举（`CANDIDATE/NEEDS_EVIDENCE/CONFIRMED/TRIAGED/REPORTABLE/REJECTED`），不得自造别名；
- 展示缺失证据项，而不是只显示一个“可信度 80%”；
- 支持按 CWE/类型、资产、身份、严重度、Verifier、是否可报告筛选；
- 低中危可按项目配置收起，但候选和阻塞记录仍可从审计中检索。

### 去重与攻击链

- 展示相似 Finding 及去重依据：根因、受影响对象、利用原语、Evidence；
- 合并只建立 Canonical/Alias 关系，原 Finding 和证据不删除；
- 攻击链视图按节点显示每一步的证据和前置条件；缺少实证的边标为推断，不能画成已验证；
- 低价值观察可作为链候选保留，但不能借“组链”绕过各环 Evidence 门禁。

### 复测与协作

- `Request Verification`、`Create Retest` 均创建独立 Job；
- Retest 记录使用新身份/新时间/新 PolicyRevision，结果分为 `REPRODUCED/FIXED/INCONCLUSIVE/BLOCKED`；
- 支持用户评论、审核备注、标签和报告排除原因；评论不改变 Verifier 结论；
- 敏感证据默认只显示必要摘要，原始真实数据按既定最小证据规则保留，不因 UI 预览扩大采集；
- 导出前再次检查 Reportable 状态，UI 不提供“强制把未验证 Finding 标为已确认”。

接口/事件：`POST /v1/findings/{id}/request-verification`、`POST /v1/findings/{id}/retests`、`POST /v1/findings/{id}/comments`、`POST /v1/findings/{id}/dedupe-links`；消费 `finding.updated`、`verification.updated`、`retest.updated`。

## 16.22 Report Center

### 报告列表与编辑器

- 支持中途快照报告和最终报告；
- 模板：SRC 单漏洞报告、Run 汇总、技术附录、Evidence Bundle；
- 左侧章节树，中间安全 Markdown 预览，右侧来源 Finding/Evidence；
- 每节展示引用数量、失效引用和缺失字段；
- 编辑内容存为新 ReportRevision，原始 AI 草稿保留；
- 版本 Diff 可对比标题、严重度、复现步骤、影响、证据和修复建议变化。

### 生成与导出门禁

- 只纳入满足 Reportable 条件的 Finding；
- 被排除的 Candidate/Blocked/UnableToVerify 单独列出及原因，不静默消失；
- 最终报告必须绑定 Exit Gate 结果、Coverage Snapshot 和 PartialReasons；
- 支持 Markdown、JSON、PDF、SARIF（适用时）和证据 ZIP；
- 导出文件中的外链图片、Raw HTML 和活动内容默认移除；
- 首版只下载到本地，不自动上传 SRC 平台或发送给第三方。

接口/事件：`GET /v1/runs/{id}/reports`、`POST /v1/runs/{id}/reports`、`PUT /v1/reports/{id}/draft`、`POST /v1/reports/{id}/finalize`、`GET /v1/reports/{id}/download`；消费 `report.updated`、`run.exit_blocked`。

## 16.23 Inbox / Notifications / 阻塞恢复

### 统一收件箱

分类：

```text
APPROVAL_REQUIRED    动作等待审批
AUTH_REQUIRED        登录失效或需要人工完成认证
OUTCOME_UNKNOWN      崩溃后动作结果未知，需要对账
BUDGET_WARNING       请求/Token/费用/时间接近阈值
RUN_BLOCKED          无法继续推进及具体 Blocker
FINDING_CONFIRMED    新确认高危/严重 Finding
CLEANUP_REQUIRED     测试数据或资源需要清理
SYSTEM_DEGRADED      Provider/Proxy/Sandbox/Storage 异常
```

支持未读、已读、Snooze、跳转来源和批量已读；**不支持批量批准高风险动作**。Snooze 只影响提醒，不延长 Approval 或 Lease。

### 通知渠道

首版提供应用内和桌面通知；Webhook/IM 为可选受控配置。通知只发送 Run ID、类型和最小摘要，不发送原始 Cookie、敏感响应或完整 PoC。外部渠道由用户显式启用并验证，失败重试有上限，通知失败不得反向阻塞控制事务。

### 阻塞恢复向导

每个 Blocker 给出：原因、影响范围、已经等待多久、可执行恢复动作、恢复后将重启哪些 Job。例如代理过期时只允许重新提取/绑定出口并 Preflight，不提供“临时直连目标”按钮。

接口/事件：`GET /v1/notifications`、`POST /v1/notifications/{id}/read`、`GET/POST /v1/notification-channels`、`GET /v1/runs/{id}/blockers`；消费 `notification.created`、`system.warning`、`run.updated`。

## 16.24 Provider / Model Profile / Budget 页面

### Provider 管理

- Provider 类型、BaseURL、SecretRef、启用状态、支持模型、超时和并发；
- 连接检查展示 DNS/TLS/认证/模型权限/最小调用的分项结果；
- 密钥只在写入时提交，读取只返回掩码和 SecretRef；
- 删除前显示引用它的 Profile 和活跃 Run，活跃引用时禁止删除；
- 所有测试调用记录用途和费用，不在 UI 后台自动反复消耗模型额度。

### Model Profile

支持 Strategist、Executor、Verifier、Summarizer、Vision 等槽位分别配置：主模型、Fallback、温度、输出上限、超时、重试、上下文预算和结构化输出能力。运行中的 Attempt 固定 Profile Snapshot；修改只影响新 Attempt，并显示 Diff。

### Budget & Usage

- 按 Run、Job、Agent 角色、Provider、模型统计 Token、缓存命中、费用和时延；
- 展示请求数、各 Host 速率、代理错误、沙箱 CPU/内存、数据库与 Blob 大小；
- 阈值动作：提醒、降低并发、暂停新 Job、停止 Run；
- 无上游价格数据时明确显示“估算/未知”，不能伪造精确成本；
- 图表只读取后端时间桶聚合，浏览器不加载数百万条 Usage 记录。

接口/事件：`GET/POST /v1/providers`、`POST /v1/providers/{id}/check`、`GET/POST /v1/model-profiles`、`GET /v1/runs/{id}/usage`、`PUT /v1/runs/{id}/budgets`；消费 `provider.updated`、`budget.updated`。

## 16.25 Identity 与 Secret Vault 页面

- Identity 列表显示别名、角色、租户、绑定 Scope、认证方式、有效期、最后验证和关联浏览器会话；
- 支持 Anonymous、账号 A/B、Cookie、Bearer、受控登录流程；
- 新建/轮换秘密时写后即不可见，页面不回显完整 Cookie/Token；
- `Validate Identity` 创建受控认证检查 Job，只访问明确允许的登录/自有信息端点；
- 身份失效显示原因和影响的 Job，可暂停依赖它的 Job，而不是切换到别人账号；
- 四级身份或角色矩阵用表格展示可访问对象和允许动作；
- Secret Reveal 默认不提供；确需导出时采用单独授权、短时令牌和审计；
- UI 不允许把 Secret 值复制进普通指令、评论、知识库或报告。

接口/事件：`GET/POST /v1/identities`、`POST /v1/identities/{id}/rotate`、`POST /v1/identities/{id}/validate`、`GET /v1/runs/{id}/identity-matrix`；消费 `identity.updated`、`auth.required`。

## 16.26 Proxy / Egress 页面

- Pool 列表：Provider、区域、可用额度、当前 Lease、健康、失败率和绑定 Run；
- 当前出口：IP/端口按本地安全设置显示、获取时间、过期倒计时、最近验证时间；
- 目标流量必须显示绑定出口，不允许出现 `DIRECT` 的健康假象；
- 检查项：代理认证、DNS 行为、出口确认、目标绑定、禁止直连、证书和超时；
- 到期前产生通知，支持受控 Rotate；Rotate 后旧 Permit 失效，新 Attempt 使用新绑定；
- 代理检查访问独立的非目标检查服务，目标请求仍按 Run Policy；
- 页面没有“代理失败时临时直连”选项；无法取得合规出口时暂停目标 Egress。

接口/事件：`GET/POST /v1/egress-pools`、`POST /v1/egress-pools/{id}/check`、`POST /v1/runs/{id}/egress-lease`、`POST /v1/runs/{id}/egress-rotation`；消费 `egress.updated`、`system.warning`、`policy.denied`。

## 16.27 Sandbox / Runner / Tool 健康页面

### Sandbox 与 Runner

- 实例列表：Run、镜像 Digest、状态、Runner 版本、浏览器/Python 能力、CPU/内存、启动时间和最后心跳；
- 网络视图：允许出口、Policy Proxy 绑定、挂载点和 Secret 注入方式；
- 健康检查：容器可用、文件系统、进程数、磁盘、浏览器、工具版本、策略连接；
- 操作：Drain、重启空实例、销毁已停止实例、查看日志；活跃实例重启前展示影响；
- 不显示可挂载宿主机 Docker Socket 或任意宿主目录的快捷选项。

### Tool Registry

- 工具名称、版本、Manifest Hash、Schema、允许身份、网络/文件能力、副作用等级和启用状态；
- 展示最近成功率、平均时延、Schema 错误、Policy Deny 和降级次数；
- 点击工具进入调用历史和测试工具页面；测试同样通过 Action/Policy，不是直接函数执行；
- 禁用工具只影响新 Plan/Job，已产生的执行记录不可改写；
- 首版工具包需本地受审并锁定哈希，无在线市场。

接口/事件：`GET /v1/sandboxes`、`POST /v1/sandboxes/{id}/drain`、`POST /v1/sandboxes/{id}/recreate`、`GET /v1/tools`、`GET /v1/tools/{id}/health`；消费 `sandbox.updated`、`tool.updated`、`action.updated`。

## 16.28 Timeline / Audit / 全局搜索

### Run Timeline

将 Strategy、Job、Action、Approval、Evidence、Finding 和系统事件按统一时间轴排列。支持缩放到分钟/小时、只看关键事件、比较两个时间点 Snapshot、导出筛选结果。Timeline 是查询投影，不是另一份可编辑状态。

### Audit Explorer

字段：时间、Actor、Command、Run/Job/Action、PolicyRevision、IdempotencyKey、结果、错误码、来源 IP/客户端、关联审计哈希。支持按用户命令追踪到所有后续事件。

- 审计记录只读，无编辑和单条删除；
- Raw Reveal、审批、策略修改、人工接管、导出、清理均重点标记；
- 导出诊断包前显示包含内容和脱敏结果；
- 审计导出不能替代原始 Evidence Bundle。

### 全局搜索

- 搜索 Run、URL、Job、Finding、Evidence、Artifact、错误码和哈希；
- 结果按权限和 Run 隔离，默认不索引秘密原文；
- `Ctrl/Cmd+K` 的命令与搜索共用后端能力，但危险命令仍需进入确认/审批。

接口/事件：`GET /v1/runs/{id}/timeline`、`GET /v1/audit-events`、`GET /v1/search`、`POST /v1/diagnostic-bundles`。审计页面以快照和分页为主，实时追加消费 `activity.created`、`command.completed`。

## 16.29 Diagnostics / Preflight / Settings

### Preflight Center

按以下分组展示检查，而不是只返回“失败”：

```text
Authorization   Scope、授权时间窗、禁止目标、方法约束
Provider        认证、模型能力、结构化输出、费用配置
Identity        账号绑定、有效期、登录检查
Egress          付费代理、禁止直连、出口 Lease
Sandbox         镜像、隔离、资源、浏览器、工具
Storage         SQLite、Blob、Vault、磁盘、备份
Protocol        Schema 版本、事件兼容、恢复能力
```

每项有 `PASS/WARN/FAIL/NOT_APPLICABLE`、实际检测结果、修复建议和 Recheck。`FAIL` 的阻断项不能由前端勾选忽略；确需人工接受的风险必须形成带期限的 Policy/Exception 审批。

### 系统诊断

展示进程版本、数据库迁移、WAL/锁、Outbox 积压、SSE 重连、Lease 过期、OutcomeUnknown、Blob 校验、磁盘与文件句柄。诊断包默认脱敏，导出前列出文件清单。

### Settings

- 通用：语言、时区、主题、密度、快捷键、默认页面；
- 安全：默认脱敏、Raw Reveal 时限、外部内容、下载确认、本地会话锁定；
- 数据：保留周期、日志级别、Blob 配额、备份位置；
- API：本地监听地址、Token 轮换、允许 Origin；开放非 Loopback 默认禁用并需单独加固；
- 界面偏好可存本地，安全默认值和权威配置必须存后端。

接口/事件：`POST /v1/runs/{id}/preflights`、`GET /v1/preflights/{id}`、`GET /v1/system/diagnostics`、`GET/PUT /v1/settings`；消费 `preflight.updated`、`diagnostics.updated`、`system.warning`。

## 16.30 人工干预的六种明确语义

所有聊天、按钮和 CLI 命令最终归一为以下命令，避免“用户说了一句话，系统不知道何时生效”。

| 干预 | 生效时机 | 用途 | 禁止行为 |
|---|---|---|---|
| `STEER_RUN` | 下一次 Strategist 规划 | 调整方向、优先级、补充业务背景 | 直接改 Scope/Policy/Finding 状态 |
| `MESSAGE_JOB` | 当前 Executor 下一可消费步骤 | 给具体 Job 补充线索或停止条件 | 绕过当前 Job 的工具和预算 |
| `QUEUE_DIRECTIVE` | 当前动作完成后的下一 Turn | 长动作期间排队，避免打断回执 | 删除尚未消费的原始指令 |
| `SIDE_QUESTION` | 独立只读会话 | 基于 Redacted Snapshot 提问、解释报告 | 偷用主 Agent 工具权限执行测试 |
| `MANUAL_JOB` | 编译、授权、调度后 | 重放、手动浏览器步骤、受控沙箱命令 | 前端直连目标或 Runner |
| `LIFECYCLE_COMMAND` | 由状态机确定 | Pause/Resume/Stop/Cancel/Retry | 把取消伪造成已回滚副作用 |

指令输入支持 `@run/@job/@finding/@evidence` 引用、附件 ResourceRef 和模板。提交后返回 Directive ID，页面显示 `QUEUED/CONSUMED/APPLIED/REJECTED`，并链接它导致的 PlanPatch 或 Job 变化。超过既定时间未被消费要提示，不允许静默吞掉。

## 16.31 页面—API—SSE 对照表

下表是前端路由开发的直接依赖清单。未列出的内部接口不得被 UI 私自调用。

阅读规则（审计修订）：

- 表内以 `/` 开头的 Run 作用域路径一律指 `/v1/runs/{runId}/...`，完整路径以 15.1/15.8 的注册表为准；
- 命令类 POST（pause/stop/replay/egress-lease 等）不属于“快照 API”列，查询自动刷新只允许 GET；Egress 页的当前绑定走只读视图；
- 本表必须覆盖全部已启用路由的页面（含 Run 历史、Discovery Inbox、Knowledge、Inbox、Identity、Settings、Memory、Seat/Fleet）；未列出的页面 = 尚无公共契约 = 不启用路由。

| 页面 | 主要快照 API | 主要 SSE |
|---|---|---|
| Dashboard | `/dashboard`、`/runs` | `run.updated`、`notification.created`、`system.warning` |
| Run Wizard | `/run-templates`、`/policies/validate`、`/preflights` | `preflight.updated` |
| Overview | `/runs/{id}`、`/runs/{id}/status`、`/coverage` | `run.updated`、`job.updated`、`budget.updated` |
| Activity | `/activities`、`/agent-sessions/{id}` | `activity.created`、`agent.progress`、`action.updated` |
| Strategy | `/strategy-epochs`、`/plan-patches/{id}` | `plan.updated`、`job.created` |
| Agent/Jobs | `/jobs`、`/jobs/{id}`、`/attempts` | `job.created`、`job.updated`、`attempt.updated` |
| Surface | `/assets`、`/endpoints`、`/asset-graph` | `asset.updated`、`coverage.updated` |
| Hypotheses | `/hypotheses`、`/discoveries` | `hypothesis.updated`、`discovery.updated` |
| Findings | `/findings`、`/findings/{id}` | `finding.updated`、`verification.updated` |
| Traffic | `/exchanges`、`/exchanges/{id}` | `exchange.created`、`action.updated` |
| Evidence | `/evidence/{id}`、`/content`、`/raw` | `evidence.created` |
| Browser | `/browser-sessions`、`/artifacts` | `browser.updated`、`artifact.created` |
| Files | `/artifacts`、`/resources` | `artifact.created`、`resource.updated` |
| Coverage | `/coverage` | `coverage.updated` |
| Approval | `/approvals` | `approval.requested`、`approval.resolved` |
| Reports | `/reports`、`/reports/{id}` | `report.updated`、`run.exit_blocked` |
| Providers/Budget | `/providers`、`/model-profiles`、`/usage` | `provider.updated`、`budget.updated` |
| Egress | `/egress-pools`、`/runs/{id}/egress-lease` | `egress.updated`、`system.warning` |
| Sandboxes/Tools | `/sandboxes`、`/tools` | `sandbox.updated`、`tool.updated` |
| Audit/Timeline | `/audit-events`、`/timeline` | `activity.created`、`command.completed` |
| Diagnostics | `/system/diagnostics`、`/preflights` | `diagnostics.updated`、`system.warning` |

所有列表 API 统一支持 `limit/cursor/sort/filter`，不得每个页面发明不同分页格式。所有详情响应带 `snapshot_revision`；所有命令带 `Idempotency-Key`，更新有冲突的命令还带 `expected_revision`。

## 16.32 前端工程拆分

```text
apps/web/
├── src/
│   ├── app/
│   │   ├── router.tsx
│   │   ├── providers.tsx
│   │   └── command-registry.ts
│   ├── api/
│   │   ├── client.ts             # 鉴权、错误、Idempotency、Trace ID
│   │   ├── generated/            # OpenAPI 生成类型与 Client
│   │   ├── queries.ts
│   │   └── mutations.ts
│   ├── events/
│   │   ├── event-source.ts       # Last-Event-ID、退避、心跳
│   │   ├── event-router.ts
│   │   └── resync.ts
│   ├── components/
│   │   ├── data-table/
│   │   ├── object-inspector/
│   │   ├── action-card/
│   │   ├── evidence-viewer/
│   │   ├── http-viewer/
│   │   ├── safe-markdown/
│   │   └── status-badge/
│   ├── features/
│   │   ├── dashboard/
│   │   ├── runs/
│   │   ├── activity/
│   │   ├── strategy/
│   │   ├── jobs/
│   │   ├── surface/
│   │   ├── findings/
│   │   ├── traffic/
│   │   ├── evidence/
│   │   ├── browser/
│   │   ├── artifacts/
│   │   ├── reports/
│   │   ├── approvals/
│   │   ├── resources/
│   │   └── operations/
│   ├── stores/
│   │   └── ui-store.ts           # 只放面板、筛选、草稿等本地状态
│   ├── workers/
│   │   ├── diff.worker.ts
│   │   └── search.worker.ts
│   └── test/
└── tests/e2e/
```

实现规则：

- 使用 OpenAPI 生成请求/响应类型，Event Schema 生成 SSE 类型；
- 每个 Feature 由 `route/list/detail/components/queries` 构成，禁止跨 Feature 私自读取内部 Store；
- 通用对象通过 `ObjectRef{type,id}` 引用，Inspector 统一解析，避免各页复制详情逻辑；
- 所有危险按钮调用统一 `CommandButton`，内建 Loading、幂等键、确认、错误和结果跳转；
- 所有目标内容统一经过 `SafeContent` 组件，禁止散落 `dangerouslySetInnerHTML`；
- 表单草稿存本地，但 Secret 草稿只在内存中，刷新即清除。

## 16.33 实时状态、性能与前端安全规格

### Snapshot + SSE 一致性

- 页面先取快照，再按 `snapshot_revision/last_event_id` 建流；
- SSE 只更新其明确覆盖的 Query，复杂聚合直接标记失效并重新查询；
- 收到比当前快照旧的事件必须丢弃，不能回滚 UI；
- 连续断线指数退避，显示 `LIVE/RECONNECTING/STALE/RESYNCING`；
- `stream.resync_required` 时停止应用增量，重新取快照；
- 浏览器离线不影响后端 Run，也不能把“UI 断线”显示成“Run 已停止”。

### 长时性能

- Activity、Traffic、Audit 使用游标分页和 TanStack Virtual；
- 页面只保留有界实时 Buffer，离开页面后取消订阅和释放大对象；
- Job/Asset 图默认聚合，单图初始不超过 500 个可见节点，按组懒加载；
- 大响应、Diff、JSON Tree 和搜索放 Web Worker，超过阈值只展示分块；
- SSE 在 50～100ms 窗口内批处理，统计图表由后端降采样；
- Playwright 长时测试至少覆盖 12 小时打开工作台、反复断线、10 万活动事件、2 万 Endpoint、5 千 Job 的组合数据集；
-  warmed-up 后堆内存不得随已离开页面或已读历史单调增长。

### 命令与审批交互

- 命令返回 `202 + operation_id` 时只表示已受理，最终状态等待事件或查询；
- 网络超时可使用同一 Idempotency-Key 查询/重试，不能生成第二份审批或 Job；
- Approve 时发送 Approval Revision 和 Action Digest；动作已变化、过期或已消费则显示冲突；
- 不 optimistic 地把 Run 标为 Paused、把 Finding 标为 Confirmed；
- 批量操作只允许只读标签、已读通知等低风险操作，不允许批量 Approve/Replay/Stop 多目标。

### 不可信内容与秘密

- 服务端输出 Redacted Projection，前端脱敏只是额外防线；
- 响应 HTML、SVG、Markdown、文件名和错误文本均视为不可信；
- CSP 禁止外部脚本和自动外链图片，下载使用安全 `Content-Disposition`；
- 截图按敏感 Evidence 处理，不能假设图片已自动脱敏；
- 本地 UI 优先同源 HttpOnly、SameSite=Strict 会话，Token 不放 URL 或 LocalStorage；
- Raw Reveal 使用短时授权、自动隐藏和审计，不能通过普通列表 API 顺带返回原秘密。

## 16.34 UI 开发优先级与验收

### P0：能安全操作一个 Run

Dashboard 简版、Run 向导（依赖项用 CLI 预置配置 + 最小只读选择器，不依赖 P1 管理页面）、Overview、Activity、Job 表（图视图 P1）、Surface、Hypothesis、Finding、Evidence、Approval、基础 Report、Pause/Stop、Snapshot+SSE、**命令幂等 + Operation 查询 + 断线 Resync**（P0 验收依赖，不可后移）、统一错误和本地认证。

### P1：能支撑十几小时无人值守

多 Run 历史与恢复预览、Inbox、Blocker 向导、Provider/Model、Identity、Egress、Sandbox、Budget、Files、Timeline、Diagnostics、Job 图视图、Traffic 最小受控 Replay（完整 Repeater/Diff 工作台 P2）。

### P2：增强分析与效率

资产关系图、Browser Observatory 只读观测、Knowledge/Skill/Template（编辑能力）、Finding 去重/复测/攻击链、报告版本 Diff、完整 Traffic/Repeater/Diff、通用 Files/Resource 库、保存视图、全局搜索和高级审计分析。

### P3 / 可选：默认关闭

Memory 搜索/Health/Rebuild 全链路（第 26 章，启用前置条件见 26.13）、Seat/Fleet 与 OpenRig Sidecar（第 27 章）、浏览器人工接管、Webhook/IM 通知、PDF/SARIF 导出、Timeline 快照比较导出。

P2/P3 中的功能如果依赖后端实体尚未实现，应先做只读空状态和 API 契约，不能用浏览器本地数据伪装成功能完成；未实现页面不启用路由。

### UI 验收清单

1. 浏览器刷新、关闭、断网均不影响 Run，重连后无重复 Job、重复审批和状态倒退；
2. 从任一 Finding 可跳转到 Verifier、Action、原始 Exchange、Evidence 和报告章节；
3. 从任一 Approval 可看到规范化动作、Scope、身份、预算、策略原因和最终执行回执；
4. Repeater、浏览器接管和手动工具均无法绕过 Policy Gateway；
5. Scope 外资产没有任何可直接触发目标请求的按钮；
6. Secret 不出现在普通列表、SSE、错误 Toast、URL、LocalStorage 和诊断包；
7. 恶意 HTML/Markdown/文件名只作为文本展示，不执行脚本、不加载外部图片；
8. UI 连续打开 12 小时无无限事件积压、明显内存泄漏和主线程长任务阻塞；
9. `COMPLETED/PARTIAL/BLOCKED/STOPPED/FAILED` 在卡片、详情、报告中保持一致；
10. Playwright 覆盖创建→审批→暂停→恢复→Finding→报告，以及崩溃恢复和审批竞态两条完整链路。

---

---

# 第二部分：Claude 风格设计系统（规范化版）

> 第一部分（§16.x）定义「做哪些页面、每页放什么」；第二部分定义「长什么样、什么气质、组件怎么统一、怎么不卡、怎么无障碍」。两部分合起来是完整 UI 开发依据。
> 本版把组件标准化（按钮/进度条/下拉框等尽量规范）、补齐完整界面地图与功能布局、把「不卡顿」提为硬约束。

## D0. 定位与四条设计原则

**定位**：不是聊天窗，是**平静的监督者控制台**——用户 24h 后回来，一分钟看懂「它在干嘛、卡哪、发现了啥、要我拍板啥」。借 Claude 客户端的气质：平静、温暖、内容优先、对不确定性诚实。

**四条硬原则（所有组件/页面必须遵守）**：
1. **只用 Token**：组件不写死颜色/间距/圆角，一律引用设计 Token（D1）。换肤、暗色、品牌调整只改 Token。
2. **状态完整**：每个交互组件必须定义全部状态——`default / hover / active / focus-visible / disabled / loading / error / empty`，不允许只做「正常态」。
3. **性能预算**：每个组件自带性能约束（列表虚拟化、CSS 变换而非重排、Web Worker 卸载重活）。**不卡顿优先于花哨**（D7）。
4. **无障碍默认**：键盘可达、`focus-visible` 焦点环、语义 ARIA、状态不靠颜色单独承载（D8）。

## D1. 设计 Token

颜色沿用暖纸墨底 + 克制主色 + 静默语义色（亮/暗两套 CSS 变量）：

```css
:root{
  --bg-canvas:#f7f4ef; --bg-surface:#fffdf9; --bg-sunken:#efeae1;
  --border-subtle:#e5ded2; --border-strong:#d4c9b6;
  --ink-1:#2a2622; --ink-2:#6b645a; --ink-3:#9a9183;
  --accent:#c96442; --accent-weak:#f0d9cf;
  --ok:#5f7a5a; --ok-bg:#e6ece2; --warn:#b07d2b; --warn-bg:#f3e9d4;
  --danger:#a4493d; --danger-bg:#f1dcd7; --info:#4a6b82; --info-bg:#dde8ee;
  --unknown:#7a7268; --unknown-bg:#e8e3da;         /* 不确定性，一等公民 */
  /* 尺寸比例（8pt 网格） */
  --sp-1:4px;--sp-2:8px;--sp-3:12px;--sp-4:16px;--sp-5:24px;--sp-6:32px;--sp-8:48px;
  --radius-sm:6px;--radius:10px;--radius-lg:14px;
  --shadow-1:0 1px 2px rgba(42,38,34,.05);
  --shadow-2:0 2px 8px rgba(42,38,34,.07);
  --font-ui:"Inter",system-ui,"PingFang SC","Microsoft YaHei",sans-serif;
  --font-mono:"JetBrains Mono","SF Mono",monospace;
  --font-read:"Newsreader",Georgia,serif;
  --fs-xs:12px;--fs-sm:13px;--fs-md:14px;--fs-lg:16px;--fs-xl:20px;--fs-2xl:28px;
  --lh-tight:1.4;--lh-normal:1.6;
  --ease:cubic-bezier(.2,0,.2,1); --dur-1:120ms; --dur-2:180ms; --dur-3:240ms;
  --z-drawer:40;--z-modal:50;--z-toast:60;--z-tooltip:70;
}
:root[data-theme="dark"]{
  --bg-canvas:#1c1a17;--bg-surface:#252320;--bg-sunken:#16140f;
  --border-subtle:#33302b;--border-strong:#46423b;
  --ink-1:#ece6dc;--ink-2:#b3a99a;--ink-3:#7c7465;
  --accent:#e07a56;--accent-weak:#3a2a22;
  --ok:#8fae89;--ok-bg:#232a22;--warn:#d4a64a;--warn-bg:#2d2616;
  --danger:#d08077;--danger-bg:#2e1f1c;--info:#7ea6c0;--info-bg:#1b262d;
  --unknown:#9a9183;--unknown-bg:#26231f;
}
```
**暗色用暖黑（非纯黑）**，24h 盯屏友好。字体：UI 用人文无衬线，HTTP/code 用等宽，报告阅读区可上衬线。

## D2. 组件库规范

### D2.0 组件通则
- **尺寸三档**：`sm`(28px 高) / `md`(34px，默认) / `lg`(40px)；表格内用 sm，主操作用 md。
- **密度两档**：`comfortable`（阅读区：活动流、Finding、报告）/ `compact`（监控区：表格、流量、日志）。由容器 `data-density` 控制行高与内边距。
- **焦点环**：统一 `box-shadow:0 0 0 2px var(--bg-surface),0 0 0 4px var(--accent)`，只在 `:focus-visible` 显示。
- **加载态**：按钮用内联 spinner 占位不跳动；区块用 Skeleton；列表用虚拟占位。
- **图标**：统一一套线性图标（如 Lucide），`currentColor` 继承文字色。

### D2.1 Button（按钮）
```text
变体 variant：primary（主操作，--accent 实心）
             secondary（次操作，--bg-surface + --border-strong 描边）
             ghost（弱操作，无边透明，hover 显 --bg-sunken）
             danger（破坏性，--danger）
             link（纯文字链接态）
尺寸 size：sm / md / lg
状态：default / hover（亮度±4%）/ active（下压 1px + 更深）/
      focus-visible（焦点环）/ disabled（降透明度 + 禁指针）/
      loading（内联 spinner，宽度冻结防跳）
组合：icon-only（必须带 aria-label + Tooltip）、icon+text、full-width
Token：bg/border/ink 全走变体对应 Token；圆角 --radius-sm；内边距随 size
性能：纯 CSS 状态，无 JS；禁止 hover 触发重排
无障碍：<button>；disabled 用 aria-disabled 并仍可聚焦以读提示；
        危险操作 danger + 二次确认（见 D2.11 Modal）
Do：一屏一个 primary；破坏性永远 danger + 确认
Don't：用颜色区分但不改文案/图标；loading 时按钮尺寸跳动
```

### D2.2 Input / Textarea（输入框）
```text
状态：default / focus / disabled / error（--danger 边 + 下方错误文案）/ readonly
结构：label（上）+ control + hint/error（下）；前后可插图标/单位
Textarea：可纵向 resize；自动增高到上限后内部滚动
Token：--bg-surface + --border-subtle，focus 换 --accent 边
无障碍：label for=id；error 用 aria-invalid + aria-describedby
安全：Secret 类输入不回显、不进本地草稿（刷新即清，见前端工程规则）
```

### D2.3 Select / Dropdown（下拉框）
```text
类型：单选 / 多选（Token 化的 chip 回显）/ 可搜索（输入过滤）
面板：Popover 浮层，points to trigger；点击外部/Esc 关闭；
      选项过多时虚拟化（见性能，>50 项必虚拟化）
状态：closed / open / focus / disabled / error / loading（异步选项）
键盘：↑↓ 移动、Enter 选、Esc 关、字母快速定位、Home/End
分组：支持 optgroup 分组标题（如按攻击类型分组）
Token：面板 --bg-surface + --shadow-2 + --radius；选中项 --accent-weak 底
无障碍：role=listbox/option，aria-selected，aria-expanded；焦点管理回 trigger
性能：选项 >50 虚拟化；异步选项防抖 200ms；不在 open 时才挂载面板
Don't：把下拉做成原生 <select>（无法统一样式）又混用自绘下拉——全站统一自绘
```

### D2.4 Checkbox / Radio / Switch
```text
Checkbox：含 indeterminate（表格「全选」半选态）
Radio：同组互斥
Switch：即时生效的开关（如 Live Follow、暗色）；切换有 120ms 滑动
状态：default / checked / hover / focus / disabled
无障碍：原生 input + 自绘视觉；label 可点；aria-checked（mixed 表示半选）
Do：Switch 用于「立即生效」，Checkbox 用于「随表单提交」
```

### D2.5 Progress（进度条 / 仪表）
```text
变体：
  linear-determinate   已知进度（0–100%），用于上传、报告生成、导入
  linear-indeterminate 未知时长（滑动条纹动画），用于「处理中」
  circular             小型圆环，用于卡片内紧凑进度
  budget-gauge（专用） 预算/续航「油表」：平静地显示已用% + 续航预测文案，
                       非报警红条（见创新点）；分段：正常/接近/超限用语义色
状态：running / paused / success / error（失败段变 --danger）
Token：轨 --bg-sunken，进度 --accent（或语义色）；高度 sm=4px/md=6px
性能：**必须用 CSS transform/width 过渡，禁止每帧 JS 改 DOM 触发重排**；
      indeterminate 用纯 CSS 动画；长任务进度由后端推送百分比，前端不轮询计算
无障碍：role=progressbar + aria-valuenow/min/max；indeterminate 省略 valuenow
Do：覆盖率/实验用「DONE/TOTAL」真实分数，不显示虚假「73%」（§16.3）
Don't：用进度条表达「不确定何时完成」的任务却给个假百分比
```

### D2.6 Badge / StatusBadge（徽章 / 状态徽章）
```text
Badge：小计数/标签（如 Inbox 未读数、tags）
StatusBadge：全站统一的状态映射（关键——状态语义必须一致）：
  终态   COMPLETED(ok) PARTIAL(warn) STOPPED(ink-3) FAILED(danger)
  非终态 RUNNING(accent,轻脉动) PAUSED(ink-2) BLOCKED(warn) QUEUED(info)
  不确定 OUTCOME_UNKNOWN / UNABLE_TO_VERIFY / UNCONFIRMED(unknown,斜纹+问号)
无障碍：**颜色 + 图标 + 文字三者齐全**，色盲可辨
Token：语义色 bg + 同色系 ink；圆角 --radius-sm
Do：全站 Finding/Run/Job 状态只用这一套枚举与配色（§13.3、§16.21）
Don't：同一状态在不同页用不同颜色/文案
```

### D2.7 Card（卡片）/ Tag / Chip
```text
Card：--bg-surface + --border-subtle + --radius + --shadow-1；
      hover 可抬升到 --shadow-2（仅可点卡片）
Tag/Chip：只读标签（Tag）/ 可删除筛选项（Chip，带 ×）
Token：纸质感，阴影极淡（Claude 气质）
```

### D2.8 Table / DataTable（表格）
```text
能力：列配置、排序、筛选、保存视图、行选择、密度切换、粘性表头、列宽调整
变体：紧凑监控表（compact）/ 可读列表（comfortable）
状态：loading(Skeleton 行) / empty(EmptyState) / error / 筛选无结果
性能：**行数 > 100 必须虚拟化（TanStack Virtual）**；排序/筛选在后端或 Web Worker；
      不一次渲染数千行；列渲染 memo 化
无障碍：原生表格语义 + aria-sort；键盘可在单元格/行间移动
Do：活动流/流量/审计/Endpoint 等长表一律游标分页 + 虚拟化（§16.33）
```

### D2.9 Tabs / Accordion / Breadcrumb / Pagination / Tree
```text
Tabs：下划线式（Claude 气质），当前 Tab --accent 下划线；懒加载面板
Accordion：可折叠分组（如 Finding 详情的事实/推断/证据分区）
Breadcrumb：Workspace › Run › 页面；可点跳转
Pagination：优先「游标 + 加载更多」而非页码（长列表友好）
Tree：分类树/资产图聚合树；节点懒加载、可搜索、可多选
无障碍：Tabs role=tablist/tab/tabpanel + ←→ 键；Tree role=tree + ↑↓→←
```

### D2.10 Modal / Dialog / Drawer（弹窗 / 抽屉）
```text
Modal：阻断式确认（破坏性操作、审批确认）；焦点陷阱 + Esc 关 + 遮罩
Dialog：表单型（新建 Run 向导步骤可用）
Drawer：右侧抽屉 = Inspector（对象详情），不阻断主内容；可固定/收起
状态：entering/leaving 180ms；遮罩淡入
无障碍：role=dialog + aria-modal；打开时焦点入、关闭时焦点回触发元素；焦点陷阱
Do：破坏性操作用 Modal 二次确认（Stop、删除、Raw Reveal）
Don't：用 Modal 承载长流程（用页面/抽屉）
```

### D2.11 Toast / Tooltip / Popover / Menu / CommandPalette
```text
Toast：非阻断通知（命令完成、错误）；右下角堆叠，自动消失，可手动关；
       **错误 Toast 不含秘密/原始响应**（脱敏，§16.33）
Tooltip：hover/focus 触发的短提示；icon-only 按钮必配
Popover：点击触发的富浮层（筛选面板、下拉内容）
Menu：右键/更多操作菜单；键盘可达
CommandPalette（⌘/Ctrl+K）：跳转 Run/Finding/Job/Evidence + 执行受控命令；
       危险命令仍走确认/审批；与全局搜索共用后端
无障碍：Tooltip role=tooltip + aria-describedby；Menu role=menu + ↑↓
```

### D2.12 Skeleton / EmptyState / 加载与空态
```text
Skeleton：区块加载占位（纸色灰块轻微 shimmer，不刺眼）
EmptyState：图标 + 一句说明 + 可选主操作（如「还没有 Finding」+「查看攻击面」）
未实现页：只读空态 + API 契约占位，不用假数据冒充完成（§16.34）
```

### D2.13 领域组件（渗透专用，复用上面基础件）
```text
ActionCard / 事件卡     PlanCard/ActionCard/ObservationCard/DecisionCard/
                        ApprovalCard/ErrorCard/BudgetCard/UserCard（§16.13）
                        统一头（类型图标+时间）+ 体（Typed 内容）+ 脚（跳转链）
EvidenceViewer          请求/响应/差异/截图/元数据 Tab；默认脱敏投影；
                        大响应分块；Raw 需 Reveal（短时授权，D2.10 Modal）
HTTPViewer              Headers/Body/Raw/Hex；JSON Tree；搜索；重定向链每跳决策
DiffViewer              baseline/variant 并排；关键字段高亮；计算放 Web Worker
Inspector               右栏统一对象详情，ObjectRef{type,id} 解析，双向跳转
```

## D3. 布局系统

```text
┌ 顶：Agent 状态条（一行平静）：● 在做什么 · Epoch · 续航 · Proxy · [Pause][Stop] ┐
├────────────┬──────────────────────────────┬──────────────────────────────────┤
│ 左导航      │ 中：主内容                    │ 右：Inspector（= Claude 侧栏）    │
│ 注意力角标   │ 表/图/活动流/编辑器            │ 当前对象详情 + 操作               │
├────────────┴──────────────────────────────┴──────────────────────────────────┤
│ 底（可收起）：指令输入 / 审批卡 / 连接同步状态                                  │
└────────────────────────────────────────────────────────────────────────────────┘
```
- **三区**：左导航（带阻塞/待办角标）/ 中内容 / 右 Inspector；Inspector 即 Claude 的 Artifacts 侧栏模式。
- **顶部状态条**：一行、平静，始终告诉你「它此刻在干嘛」（= Claude Code status line）。
- **密度分区**：阅读区松、监控区紧。
- **响应式**：宽屏三栏；中屏右栏变抽屉；窄屏折叠为单列 + 底部 Tab。

## D4. 完整界面地图与功能点布局

**整个项目的 Web 界面**由以下页面构成（与 §16 信息架构一一对应，这里给「功能点落位」）：

```text
全局
  Dashboard（值班台）        需要我处理 / 正在运行 / 最近结果 / 系统健康 + 全局资源余量(§33)
  Runs 列表/历史            筛选/克隆/归档/恢复预览/QUEUED 状态
  Run 创建向导              目标→规则→身份→执行环境→策略确认（5 步）
  Inbox 收件箱              审批/阻塞/通知（分类）
  Resources                Knowledge(§34) / Skills / Tools / Resources
  Operations               Providers/Models / Egress / Sandboxes / Budgets / Audit / Diagnostics
                           + 全局资源治理(§33)
  Settings                 通用/安全/数据/API
Run 工作台（进入某 Run）
  Overview                 策略 Epoch/优先方向/真实进度分数/Blocker
  Activity 活动流           事件卡流 + Agent Session Drawer
  Strategy                 战役计划(§32)/Epoch/PlanPatch Diff/为什么做这个
  Agent/Jobs               图视图 + 表视图 + Job Drawer
  Surface 攻击面            Endpoint 表 + 资产图 + Discovery Inbox
  Hypotheses               看板
  Findings                 列表/看板/攻击链 + 详情(事实/推断分区)
  Traffic                  Exchange 列表 + Repeater + Diff
  Evidence                 脱敏投影 + Reveal
  Browser                  会话/截图/DOM/Network/Trace/接管
  Files                    产物树 + Resource 库
  Coverage                 矩阵 + 单元格下钻
  Timeline                 统一时间轴
  Reports                  列表/编辑器/导出门禁
  （Memory / Seat 为 P3 可选）
```

**功能点布局原则**：
- 「需要我处理」永远在 Dashboard 首屏第一区（注意力优先）；
- 每个 Run 工作台左导航带阻塞角标，右 Inspector 随选中对象；
- 危险操作（Stop/Reveal/删除/人工接管）集中走 Modal 确认 + 审计；
- 跨对象跳转统一：Finding↔Evidence↔Action↔Job↔Exchange 双向。

## D5. 创新功能点（为自主 Agent 量身）

1. **「离开期间发生了什么」叙事回放**（杀手级）：Summarizer 生成可点击的几小时摘要，grounded 在真实事件。
2. **Attention-first 首页**：把 24h 压成「3 件需要你」，按 影响×紧迫×时效衰减 排序。
3. **「Why?」无处不在**：任何决策可溯源到 StrategySnapshot + 证据。
4. **点击即证据**：任何结论 → 跳到响应 CEBS 字节范围高亮。
5. **不确定性一等公民**：OUTCOME_UNKNOWN/UNABLE_TO_VERIFY 用 `--unknown` 视觉显眼呈现，不藏。
6. **引导即对话**：指令输入像聊天，回结构化 PlanPatch/Directive 状态。
7. **预算=平静油表 + 续航预测**：不是报警红条。
8. **战役计划可视化**（§32）：把「多方向、哪个在打、哪个泊车」画成地图。

## D6. 关键页面的 Claude 化（气质落点）
- **Dashboard**：顶部一句话叙事 + 「需要你的 3 件事」大卡 + 低饱和 Run 卡（终态/非终态用形状不只用色）。
- **活动流**：事件卡纸墨质感、轻滑入、Live Follow 可暂停。
- **Finding 详情**：事实区（冷静）/ 推断区（标 AI 生成）视觉分开 + 并排 diff + 点击即证据。
- **Report**：阅读区上衬线、窄栏、慷慨留白，像 Claude Artifact 文档。

## D7. 性能优先（不卡顿，硬约束）

**美观的前提是流畅——这是硬门禁，不是优化项。**
```text
虚拟化          Activity/Traffic/Audit/Endpoint/Table >100 行一律 TanStack Virtual
Web Worker      diff / JSON Tree / 搜索 / ELK 图布局 不占主线程
SSE 批处理      50–100ms 窗口合并；图表后端降采样；单图 ≤500 可见节点懒加载
有界 Buffer     页面只留有界实时 Buffer，离开页面取消订阅、释放大对象
进度/动画       CSS transform/opacity，不每帧改布局；尊重 prefers-reduced-motion
内存            warm-up 后堆内存不随已离开页面/已读历史单调增长
门禁            Playwright 长测：12h 打开 + 反复断线 + 10 万事件 + 2 万 Endpoint + 5 千 Job
                （§16.33）—— 不达标即判失败
```

## D8. 无障碍（默认，不是附加）
```text
键盘            所有交互组件可达；焦点顺序合理；焦点陷阱（Modal/Drawer）
focus-visible   统一焦点环，只在键盘聚焦时显示
语义            正确 ARIA role/状态；表格/列表/树用原生语义
状态不靠色      StatusBadge 必带图标+文字；diff 不只用红绿
对比度          文字对背景 ≥ WCAG AA；暗色同样校验
动效            prefers-reduced-motion 时关闭非必要动画
```

## D9. 落地顺序（定气质 → 铺页面 → 加创新）
```text
1. 定气质：Design Token（亮/暗）+ 组件通则（状态/密度/焦点/性能约束）
2. 核心基础件：Button / Input / Select / Progress / StatusBadge / Table / Card / Modal / Drawer / Toast
3. 领域件：ActionCard / EvidenceViewer / HTTPViewer / DiffViewer / Inspector
4. 铺 P0 页面（§16.34）：Dashboard / Activity / Finding / Evidence / Approval
5. 加创新交互：叙事回放 / 点击即证据 / Why? / 战役计划可视化 / 知识库(§34)
原则：组件先标准化再铺页面；性能门禁贯穿每一步，不留到最后补。
```

## D10. 知识库 UI（对应第 34 章）

见第 34.8 节的三栏布局（分类树 / 列表·md 编辑器 / 详情），复用本设计系统组件：
```text
左树 = Tree（分类，可嵌套/搜索）+ Switch（global/项目 scope 切换）
中列表 = DataTable（title + trust 徽章 + 命中次数，按 category/tag/攻击类型/技术栈/trust 过滤）
中编辑 = CodeMirror md 实时预览 + 表单（category 下拉 / tags Chip / applies_to 多选 Select / trust 单选 / scope）
右详情 = frontmatter 表单 + md 预览 + 版本 Diff + 「最近被哪些 Job 命中」+ Secret 扫描结果
试检索 = 输入情境（攻击类型+技术栈+模块）→ 预览 AI 此刻会召回哪些条、排序如何
安全呈现 = trust_level 徽章 + 「不可信内容」标记 + global/项目 scope 标记
```
全部用 StatusBadge/Select/Tree/DataTable/CodeMirror 等标准组件，不新造一套样式。

---

# 第三部分：交互覆盖审计与新增界面（v1.2）

> 回答「所有用户交互的地方是否都设计了 UI」：本部分做一次**全交互点覆盖审计**，并补上主文档 §35/§36/§38 新增能力对应的界面。原则：**凡用户需要做决定、输入、确认、查看的地方，都必须有 UI 落点**（CLI 是并行面，不替代 UI）。

## D11. 用户交互点覆盖审计

下表列出全部需要人介入的交互点及其 UI 落点。标 ✅ 已有、🆕 本版新增、📄 纯 CLI/配置（UI 可只读展示）。

| 交互点 | 用户要做什么 | UI 落点 | 状态 |
|---|---|---|---|
| 创建/配置 Run | 填目标/规则/身份/环境/策略 | Run 创建向导（§16.2） | ✅ |
| 审批危险动作 | 看 diff、批准/拒绝 | 审批中心（§16.10）/ Inbox | ✅ |
| 引导方向 | 发 Directive、看生效 | 用户干预（§16.11/16.30）| ✅ |
| 暂停/恢复/停止 | 两档暂停、Stop、Kill | 状态条 + Run 卡（§16.1） | ✅ |
| 断点恢复 | 看恢复预览、确认 Resume | Run 历史恢复预览（§16.12） | ✅ |
| 看证据/揭秘 | 查看脱敏、Reveal 原始 | Evidence 页 + Reveal（§16.9） | ✅ |
| 覆盖率下钻 | 看单元格 Action/Evidence | 覆盖率页（§16.8） | ✅ |
| Finding 处置 | 分诊/去重/复测/评论 | Finding 工作流（§16.21） | ✅ |
| 报告编辑导出 | 改草稿、导出门禁 | Report Center（§16.22） | ✅ |
| 流量重放 | 受控 Repeater → 建 Job | Traffic 工作台（§16.16） | ✅ |
| 浏览器接管 | 申请接管、操作 | Browser Observatory（§16.17） | ✅ |
| Provider/模型 | 增删、连接检查 | Provider 页（§16.24） | ✅ |
| 身份/秘密 | 增删/轮换/验证 | Identity 页（§16.25） | ✅ |
| 预算 | 看用量、设阈值 | Budget 页（§16.24） | ✅ |
| Sandbox | 查看/Drain/重建 | Sandbox 页（§16.27） | ✅ |
| 诊断/审计 | 导诊断包、查审计 | Diagnostics/Audit（§16.28/29） | ✅ |
| 设置 | 通用/安全/数据/API | Settings（§16.29） | ✅ |
| **知识库** | 分类/新建/导入/试检索 | Knowledge 页（§34.8 / D10） | 🆕 |
| **联网知识源** | 开关/通道/预算/查看 | 知识设置 + Operations（D12.3） | 🆕 |
| **工具管理** | 查看/注册/启停/测试 | Tool Registry 增强（D12.2） | 🆕 |
| **MCP 接入** | 添加/固定/启用工具/分类 | MCP 管理（D12.2） | 🆕 |
| **出口模式选择** | 无代理时三选一、确认本机 | Preflight Egress 卡（D12.1） | 🆕 |
| 全局资源治理 | 看并发余量、调上限 | Operations 资源页（§33.4） | ✅ |

> 结论：补上 D12 的三块新界面后，**所有需要人决定/输入/确认/查看的交互点都有 UI 落点**。纯脚本/配置项（如 Prompt 预算阈值、检索触发开关）在 Settings 以只读或高级配置呈现，不强求可视化编辑。

## D12. 新增界面详细设计

### D12.1 出口模式选择（Preflight Egress 卡，对应 §38）
无代理时，Preflight 的 Egress 分项不只报「失败」，而是给**显式三选一卡片**：
```text
┌ Egress 检查：未发现可用代理 ───────────────────────────────┐
│ ○ 配置代理池        → 跳到 Egress Pool 配置                 │
│ ○ 使用本机出口 ⚠    → 展开警告：目标将看到你的真实 IP；      │
│                       SRC/护网可能禁止；需勾选「我已了解」    │
│                       确认后 egress_mode=local 写入 Policy   │
│ ○ 中止              → 不启动 Run                            │
└────────────────────────────────────────────────────────────┘
```
- 本机选项用 StatusBadge(warn) + Checkbox「我已了解风险」才能继续；
- Egress 页常显当前 `egress_mode`，`local` 用警告色标注当前出口为本机 IP；
- 没有「代理失败时临时直连」这种模糊按钮（§16.26 已有原则，这里强化）。

### D12.2 工具与 MCP 管理（Tool Registry 增强 + MCP，对应 §35）
在 Operations（或 Resources）下，Tool Registry 从「只读健康页（§16.27）」增强为**可管理**：
```text
左：工具源分类           中：工具列表/注册表单              右：详情
  内置 Runner             列表：名称/版本/语言/能力/         Manifest/Schema
  容器 CLI                      风险等级/启用/成功率          能力与 Run Policy 交集
  Tool Plugin             注册：上传 Manifest + 镜像          调用历史
  MCP Servers 🆕                Digest，Schema 校验，本地审核   测试工具(经 Policy，非直调)
```
MCP 管理子页：
```text
添加 MCP Server：transport(stdio/http) + pinned 版本/哈希 + 分类(target_affecting/utility)
introspect：拉取其工具列表 → 勾选启用哪些（显式，非全量）
能力交集：展示每个 MCP 工具声明的能力 与 Run Policy 的交集结果
安全标注：target_affecting → 「将在本地沙箱运行、网络经 Policy Proxy」
          utility/remote → 「仅非目标用途，走 utility 通道」
          无法纳管 → 「禁止用于目标执行」红标
```
- 所有「测试工具」按钮都经 Action/Policy/Permit，不是前端直调函数（§16.27 原则）；
- 启停只影响新 Plan/Job，已有执行记录不可改写。

### D12.3 联网知识源与检索设置（对应 §36）
在 Knowledge 页旁 + Operations/Settings：
```text
联网知识开关：policy.web_knowledge_allowed（可按项目）
通道：utility egress（只读展示，不可改为目标通道）
预算：max_results / budget_ratio / 速率
检索触发：on_stuck_autoretrieve、触发点清单（new_module/new_stack/reframe/stuck…）可勾选
来源落地：联网结果「存为知识条目」入口（trust_level=UNVERIFIED_REFERENCE）
呈现：检索面板里本地结果与联网结果分区，联网条目标来源 URL + 「未审核」徽章
```
- 「试检索（Test Retrieval）」支持勾选「含联网」，预览某情境下本地+联网会召回什么。

## D13. 这些新界面的组件复用
全部用第二部分的标准组件，不新造样式：
```text
出口三选一    = Radio + StatusBadge(warn) + Checkbox + Modal 确认
工具/MCP 列表  = DataTable + StatusBadge（能力/风险/启用）+ Drawer 详情
注册表单      = Select/Input/文件上传 + Schema 校验错误(Input error 态)
联网设置      = Switch + Input(预算) + Checkbox 组(触发点) + Tag(来源徽章)
```

---

# 第四部分：浏览器观测台性能、Skill 导入与知识筛选（v1.3）

> 本部分补齐三件事的 UI 侧：浏览器为什么在 web 里不卡（对应主文档 §42）、Skill 导入界面（§40）、知识分类筛选的检索 UI（§41）。

## D14. 浏览器观测台：为什么在 web 里不卡

**UI 不渲染实时浏览器**。自动化浏览器无头、跑在服务端沙箱；观测台只显示它产出的抓取物（截图/DOM/Trace/网络），都是图片和文本，懒加载。所以正常观测不卡、无高延迟（详见 §42）。UI 侧性能做法：

```text
截图      缩略图网格懒加载，进视口才加载；点击看原图（敏感证据需 Reveal）；
          不一次性拉全部截图
DOM 快照  只读文本，虚拟化长列表，可搜索；绝不执行页面脚本
网络      经 Proxy 留证的请求列表，游标分页 + 虚拟化
Trace     「下载查看」按钮（Playwright Trace），不在 UI 里实时串流
响应预览  若需看某响应的 HTML：用沙箱 iframe 静态渲染「抓取到的那份 HTML」，
          禁脚本/表单/外链——这是静态快照渲染，不是实时浏览（§16.16）
```

**人工接管（可选）的 UI**：
```text
入口：Browser 会话 → 「申请接管」→ 新 Approval（绑定 Identity/Run/PolicyRevision/时限）
实时视图：接管期间用 CDP screencast（JPEG 帧）显示 + 鼠标键盘回灌
延迟提示：UI 显示连接延迟；本机部署近本地，远程 VPS 标「远程接管，可能有网络延迟」
每步成 Action：接管中每次导航/输入/点击都在侧栏实时记为 Action（过 Policy）
结束：「结束接管」立即撤销控制 Lease，停止 screencast（零额外开销）
非接管：默认只读抓取物视图，不开 screencast
```
组件复用：截图网格 = 懒加载图片网格；DOM = 虚拟化只读文本；接管视图 = screencast 画布 + StatusBadge(延迟) + Modal(接管确认)。

## D15. Skill 导入界面（对应 §40）

在 Resources → Skills（扩展 §16.20）：
```text
列表：Skill 名/版本/applies_to/required_tools/trust(local_audited|external_untrusted)/
      启用状态/哈希
导入：「导入本地包」→ 选 folder/zip → 校验报告（skill.yaml 合法性 / required_tools 是否齐 /
      是否声明越权 / Secret 扫描 / 哈希）→ 通过才可启用
详情：skill.yaml 表单 + instructions.md 预览 + 版本历史/Diff + 「被哪些 Job 加载过」
安全标注：声明越权的包红标拒绝；external_untrusted 默认不启用，启用需确认并降权
启停：只影响新 Plan/Job
```
组件复用：DataTable + 文件上传 + 校验错误(Input error 态) + Drawer 详情 + StatusBadge(trust)。

## D16. 知识分类筛选与检索 UI（对应 §41）

Knowledge 页（§34.8/D10）的检索与筛选，体现「分类粗筛 + applies_to 精匹配」：
```text
左树分类      点分类 = 按 category 粗筛列表（人工浏览视角）
筛选栏        攻击类型 / 技术栈 / 模块类型 / trust / scope 多选（= applies_to 机器匹配视角）
搜索框        关键词 + 「语义」开关（FTS5 / 向量混合，§41）
结果          按综合得分排序，显示 title + trust 徽章 + applies_to 命中标签 + 命中次数
试检索        输入情境（攻击类型+技术栈+模块[+含联网]）→ 预览 Agent 此刻会召回哪几条、
             排序如何、各条为何命中（applies_to/关键词/语义）→ 据此调 tags/applies_to
```
这个「试检索」面板让用户直接看到「Agent 会怎么筛、怎么快速找到」，是把 §41 机制可视化、可调优的入口。组件复用：Tree + 多选 Select + Switch(语义) + DataTable(结果) + Tag(命中原因)。

## D17. 覆盖审计增补
以下交互点并入 D11 覆盖表，均已有 UI 落点：
```text
Skill 导入/管理      Resources→Skills（D15）                 🆕
知识分类筛选/试检索   Knowledge 页检索栏（D16）               ✅(增强)
浏览器接管           Browser 会话→接管（D14，Approval 绑定） ✅(P3)
```

---

# 第五部分：高级测试、韧性与完整性界面（v1.4）

> 对应主文档 §44–47 新增能力的 UI 侧。多为后端能力，UI 补充克制，复用已有标准组件。

## D18. 序列与竞态实验界面（§44）
```text
序列实验      Strategy/Jobs 下的「序列实验」视图：步骤列（每步 Identity/请求/提取规则/变更）
             + 步骤间传值可视（s1 提取的 order_id → s2 用）+ 断言/Falsifier
             只读展示 AI 构造的序列；用户可「要求补一步」走 Directive，不直接编辑执行
竞态实验      因 risk_class=concurrent_write 必走审批：审批卡显式展示
             并发度/同步策略/目标端点/「仅豁免串行、不豁免红线」说明 + 预期副作用
             执行后展示 N 个并发请求的 Receipt 与「成功次数」（>1 即命中）
OAST 关联     实验若 pending_oast：Finding 停在 NEEDS_EVIDENCE，UI 显示「等待带外回调（倒计时）」
             回调到达 → 关联 oast_event 证据，自动推进；超时 → UNABLE_TO_VERIFY
组件复用      步骤列=Accordion/Table；竞态审批=ApprovalCard(强化副作用区)；OAST=StatusBadge(pending/倒计时)
```

## D19. 会话与长时韧性界面（§45）
```text
Identity 页    增「会话健康」：存活/临近过期/自动重认证中/重认证失败-BLOCKED
             重认证流程 identity_auth_flows 可查看（步骤、成功断言），凭据不回显
授权窗 drain   状态条/Overview 显示「距授权到期 15 分钟，已进入收尾（停新目标动作，正在出报告）」
Provider 宕机  系统健康区显示 WAITING_PROVIDER（暂停中、探活倒计时），区别于 FAILED
组件复用      StatusBadge（会话/drain/宕机态，语义色不靠颜色单独承载）
```

## D20. 完整性与目标历史界面（§47）
```text
Finding 历史   Finding 详情增「历史」Tab：本 Workspace 跨 Run 关联链
             （首次发现/REOPENED/FIXED/DUPLICATE），每节点链到那次 Run 的证据
目标随时间     Workspace 视图增「随时间」：该目标历次 Run 的 Finding 趋势（新增/修复/重开曲线）
审计验链       Audit 页增「校验哈希链」按钮：一键验证完整性，有断点则高亮定位
组件复用      历史链=Timeline/Tree；趋势=折线图（后端聚合，§D7）；验链结果=StatusBadge + 定位列表
```

## D21. 上下文检查器增补（§46，排查「模型为什么不知道/被什么影响」）
Agent Session Drawer 的上下文查看器（§16.13）增两项：
```text
检索预算     显示本步注入了哪些 Skill/Memory/Knowledge、各占多少 token、哪些因预算被省略（可展开）
不可信标注   数据区内容按来源着色标注（target/tool/knowledge/memory/mcp/upload = 不可信），
            与指令区视觉区分——让人一眼看出「哪些是数据、哪些是规则」
```
这让「模型为什么这么判断/有没有被目标内容带偏」可排查。组件复用：Accordion + Tag(来源) + 进度条(预算占比)。

## D22. 覆盖审计增补（并入 D11）
```text
序列/竞态实验       Strategy/Jobs 视图 + 竞态审批（D18）         🆕
OAST 等待           Finding/实验状态（D18）                      🆕
会话健康/重认证     Identity 页（D19）                           🆕(增强)
授权窗 drain/宕机   状态条/系统健康（D19）                       🆕
Finding 历史/目标趋势/验链  Finding 详情/Workspace/Audit（D20）  🆕
上下文检索预算/不可信标注   Session Drawer（D21）               🆕(增强)
```
结论：§44–47 的新能力交互点均已有 UI 落点。

---

# 第六部分：代理池、浏览器源、MCP 开放与数据目录界面（v1.5）

> 对应主文档 §48–50 与 §35.6 放宽。复用标准组件。

## D23. 代理池界面（§50）
在 Operations → Egress 下增「代理池」子页：
```text
┌ 源管理 ──────────────┬ 候选/可用列表 ───────────────────┬ 详情 ──────────┐
│ proxy_sources：       │ DataTable：ip:port / 协议 / 匿名   │ 验证历史        │
│ 增/删/启停/刷新间隔    │   / 延迟 / 地理 / 评分 / trust /   │ 成功率曲线      │
│ 「立即刷新」按钮       │   状态(ALIVE/DEAD/BLACKLISTED)     │ 拉黑/恢复       │
│                      │ 筛选：协议/匿名/延迟/地理/trust    │                │
│                      │ 批量：剔除/拉黑                     │                │
└──────────────────────┴───────────────────────────────────┴────────────────┘
```
- **免费不可信代理显著风险标注**：trust_tier=free_untrusted 用 StatusBadge(warn)；
  页面顶部一句话风险提示（HTTPS 目标受 TLS 保护、HTTP 目标暴露、元数据泄漏、可靠性差）；
- 每 Run 选择需求：在 Run 向导/Egress 设 `trust_tier/protocol/anonymity/geo` 要求；敏感 Run 可勾「只用可信层」；
- 健康轮询状态、池容量/补充、黑名单一览；
- 组件复用：DataTable + StatusBadge(状态/trust) + 折线图(成功率) + 多选筛选 + 「立即刷新」CommandButton。

## D24. 浏览器源优先级界面（§49）
在 Settings 或 Sandbox/Browser 区：
```text
浏览器源顺序：[外部 MCP] > [内置 browserd]（可拖拽调序，默认外部优先）
门禁状态：外部浏览器 MCP 是否满足「本地+经 Proxy+吐抓取物」——
          满足=绿「可作目标浏览器」；不满足=警告「仅 utility，目标浏览用内置」
当前选用 + failover 记录：现在在用哪个、何时从外部切回内置
反指纹：引擎/画像一致/TLS 指纹对齐/去无头痕迹 的开关（§49.5），
        旁注「首要是速率与行为，不是 stealth 魔法」
组件复用：可排序列表 + StatusBadge(门禁/选用) + Switch(反指纹项)
```

## D25. MCP 开放管理界面（§35.6 放宽）
D12.2 的 MCP 管理按「开放接入」调整：
```text
添加 MCP：transport(stdio/http/sse) + trust(self_managed/untrusted)；self_managed 只记版本
工具：introspect 后默认可用（auto_enable_tools），无需逐个勾选（可选收紧）
标注：目标影响类工具自动标「本地沙箱+经 Proxy 执行」（和内置工具同待遇）；
      远程 MCP 标「仅 utility」（非歧视，是碰不到 Proxy）
一视同仁：MCP 工具和内置工具同列 Tool Registry，不单独加审批/降权
组件复用：Select(transport/trust) + DataTable + StatusBadge(目标/utility)
```

## D26. 数据目录与备份设置（§48）
Settings → 数据：
```text
展示 <pentagent-data> 实际路径（只读，仅本机配置可改）
子目录用量：control.db / blobs / memory / knowledge / vault（大小 + 配额 + 逼近告警）
备份：litestream 状态（control.db）、备份位置、上次备份时间
记忆位置：显示「记忆落在 memory/<workspace_id>/，按 Workspace 隔离，不入诊断包」
密钥提示：headless passphrase 丢失=该 Workspace 加密数据不可恢复（§45.5），可配恢复密钥
组件复用：只读 Input(路径) + 用量条 + StatusBadge(备份)
```

## D27. 覆盖审计增补（并入 D11）
```text
代理池管理           Operations→Egress→代理池（D23）     🆕
浏览器源优先级/反指纹 Settings/Browser（D24）            🆕
MCP 开放管理         Resources/Tools（D25）              🆕(调整)
数据目录/备份        Settings→数据（D26）                🆕
```
结论：§48–50 与 MCP 放宽的交互点均已有 UI 落点。

---

# 第七部分：卡住处理、后台任务与观察模式界面（v1.6）

> 对应主文档 §51–53。复用标准组件，补充克制。

## D28. 后台任务与卡住可见（§51）
工具执行与终端观测（§16.18）增「后台任务」视图：
```text
后台 Job 列表    由前台自动转后台的长命令 + 其状态（运行中/到上限被杀/完成）
                显示：所属 Action/Job、已运行时长、距 30 分钟上限倒计时
卡住标注         看门狗判定 hung 的工具用 StatusBadge(danger) + failure_class=hung
被杀上报         到上限被杀的 Job 显示"已杀 + 最后输出摘要"，不静默消失
外部调用超时     Error View 增 TIMEOUT 分类聚合（模型/Proxy/工具/MCP 各自超时可见）
组件复用         DataTable + StatusBadge(运行/超时/hung) + 倒计时
```
不渲染实时命令流的"假进度"——有真实进度（字节/步数）才动，符合 §16.3「不显示虚假 73%」。

## D29. 观察模式开关（§53.3）
Settings → 自主行为：
```text
autonomy_mode（按行为类别）：observe（只记录"本会做什么"）/ auto（真正执行）
  类别：自动检索知识 / 自动 Reframe 转向 / 主动通知 / 方向泊车
observe 下：活动流里这些行为显示为「（观察模式）本会：…」的灰色条目，不真的做
            旁边一键「看着对 → 切 auto」
好处可见：让用户先影子观察新自主能力，确认判断对了再放开（借鉴 dsh）
组件复用：Switch/分段控件 + 活动流中 observe 条目用弱化样式
```

## D30. 覆盖审计增补（并入 D11）
```text
后台任务/卡住/超时可见   工具执行→后台任务（D28）        🆕
观察模式开关             Settings→自主行为（D29）         🆕
```
结论：§51–53 的交互点均已有 UI 落点。观察模式同时给了"新自主能力先影子跑"的安全阀。

## 16.35 Owner 验收反馈（2026-11-17，P0 预览后）

> 来源：Owner 在本机（Windows，浏览器预览 apps/web 构建产物）验收 PagesDemo 后的反馈。由测试方诊断定位、Owner 指示录入本规格。处理优先级：daemon 接线前的 UI 打磨窗口一并处理（改动成本最低期）。

### 反馈一：字体发虚、模糊不清

**现象**：整体文字观感发虚，尤其中文小字号。

**测试方诊断（两个根因，按嫌疑排序）**：
1. `apps/web/src/styles/tokens.css:110` 的 `-webkit-font-smoothing: antialiased`——该属性**在 Windows Chrome 上禁用子像素抗锯齿**（subpixel rendering），普通 DPI 屏上文本变灰发虚；它本是 macOS 惯用默认。这是最可能的元凶。
2. `--font-ui` 栈首位的 `Inter` **未打包**（dist 产物无字体文件）——实际渲染走系统回退：西文 Segoe UI / 中文 Microsoft YaHei；雅黑在小字号 + antialiased 组合下观感进一步劣化，且中英混排基线/灰度不统一。

**修复方向（dev 定夺具体方案）**：
- Windows（非 Retina）环境移除 `-webkit-font-smoothing: antialiased`，或仅在 `@media (-webkit-min-device-pixel-ratio: 2)` 下启用；
- 二选一：打包 Inter（含 latin 子集 + 明确中文回退声明），或干脆改用系统栈 `"Segoe UI", "Microsoft YaHei", system-ui`（零加载成本、Windows 原生清晰）；
- 验收标准：Owner 本机 100%/150% 缩放下中文小字号（fs-sm）清晰不发灰。

### 反馈二：Toast 观感不佳

**现象**：Toast 弹出样式不好看（当前实现：右下堆叠 + 白底卡片 + 3px 左色条，components.css D2.11）。

**改进方向（供设计定夺）**：
- 考虑深色反转 Toast（`--ink-1` 底 + 反色文字），与浅色画面对比更强、更聚焦；
- 结构升级：图标（按 tone）+ 标题 + 正文 + 关闭，替代当前纯文本；
- 动效：进入用位移+渐显（当前 sn-pop 弹跳感偏玩具），退出渐隐；堆叠间距收紧；
- 验收标准：Owner 主观认可（本条为审美项，Owner 一票定稿）。

### 开发方处理（2026-11-17，commit 见 PROGRESS）

两条反馈均已在 daemon 接线前的 UI 打磨窗口处理：

**反馈一（字体发虚）——已修**。按测试方诊断两根因同治：
- 根因 1（antialiased 禁用 Windows 子像素渲染）：把 `-webkit-font-smoothing: antialiased` 从无条件
  移到 `@media (-webkit-min-device-pixel-ratio: 2), (min-resolution: 192dpi)` 内——仅 Retina/高 DPI（含
  macOS）启用灰度抗锯齿，Windows 普通 DPI 走系统子像素渲染保持锐利；body 加 `text-rendering: optimizeLegibility`。
- 根因 2（Inter 未打包）：采信「改系统栈」方案（零加载成本、Windows 原生清晰，不引入字体文件下载），
  `--font-ui` 改 `"Segoe UI", system-ui, "PingFang SC", "Microsoft YaHei", -apple-system, sans-serif`，
  mono/read 栈同步补系统回退。验收标准（Owner 本机 100%/150% 缩放中文小字号清晰）待 Owner 复看确认。

**反馈二（Toast 观感）——已重做**。按改进方向落地：
- 深色反转：`--ink-1` 底 + `--bg-surface` 反白文字，与浅色画面强对比；
- 结构升级：tone 图标（✓/!/✕/i，按 tone 着色）+ 可选标题 + 正文 + 关闭，替代纯文本（ToastItem 增 title 字段）；
- 动效：进入 `sn-toast-in`（上移 8px + 渐显，去掉原 sn-pop 弹跳）、退出渐隐、堆叠间距由 sp-2 收紧到 sp-1。
- 本条为审美项，最终以 Owner 一票定稿为准。

验证：apps/web 下 pnpm typecheck + test（19 文件 115 用例）+ build 全绿（CSS 10.6→11.5KB）。
