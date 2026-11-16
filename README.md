# Snag / 枯木

> **「地上枯寂，根系深扎。」**
> 一棵看似死去的树，根却在地下无声蔓延、摸清每一寸土壤——正如这个 agent 表面安静，却在持续测绘整个攻击面。
>
> *Snag*（林业术语：站着的枯木）亦是"钩住、逮到"——逮到一个漏洞。

**Snag** 是一个面向**授权** Web/API/SRC 测试的独立 AI 渗透 Agent 框架：支持黑盒/灰盒、连续运行数十小时、动态子 Agent、硬红线、证据驱动报告，CLI 与 Web UI 共用同一后端。

> 命名说明：开发文档里的占位名 `pentagent` / `pentagentd` 即本项目；代码中的二进制统一用 **`snag`（CLI）/ `snagd`（守护进程）**。文档内容暂未全量改名，以此为准。

---

## 仓库结构

```
snag/
├── README.md              本文件：项目简介 + 协作流程
├── PROGRESS.md            开发进度（Claude 维护，实时更新）
├── TESTING.md             测试记录（本地测试模型维护，含实例与格式）
├── docs/
│   ├── pentagent-dev-spec-v2.6.md   主开发设计方案 v2.6（53 章，权威参考）
│   └── pentagent-ui-spec-v1.6.md    UI 开发文档 v1.6
└── dev-plan/
    ├── 00-ROADMAP.md      先读这个：46 天施工路线图
    └── day-01.md … day-46.md   每天一个自包含小开发文档
```

（代码目录 `cmd/ internal/ api/ migrations/ apps/web/ tests/ …` 随开发逐步建立。）

---

## 协作流程（重要，双方都读）

**分工：**

| 谁 | 负责 | 维护的文档 |
|---|---|---|
| **Claude（开发）** | 写代码、推送、更新开发进度 | `PROGRESS.md` |
| **本地小模型（测试）** | 跑测试、找 bug、记录结果 | `TESTING.md` |
| **你（Owner）** | 审阅、批准每天推进 | — |

**节奏规则：**

```
1. 一个开发单元 = 一天（一个 day-NN.md）。
2. Claude 每次开发【前】，先读 TESTING.md 顶部「未解决 bug」：有 bug 先修，再做新功能。
3. Claude 做完一天：提交代码 + 更新 PROGRESS.md（标该天完成、附 commit）。
4. 本地模型按该天 day-NN.md 的「怎么测试」跑测试，把结果写进 TESTING.md（带时间戳、结果、对应天数）。
5. 【批准闸门】Claude 不自动进入下一天——必须等你明确说"可以继续 / 进入 Day N+1"才接着开发。
```

**一句话：Claude 开发并推代码、维护进度；本地模型测试、维护 bug 记录；你控节奏，一天一批准。**

---

## 怎么开始

1. 读 `dev-plan/00-ROADMAP.md`（全局 46 天、8 阶段、测试纪律）。
2. 开发从 `dev-plan/day-01.md` 起，一次一天。
3. 每天完成后看 `TESTING.md` 的测试结果；有 bug 下次开发先修。
4. 进度随时看 `PROGRESS.md`。

---

## 当前状态

**Day 0（仓库与文档就绪）已完成。等待批准开始 Day 01。** 详见 `PROGRESS.md`。
