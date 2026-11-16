# Day 32 —— 模块聚类 + 技术栈指纹 + Discovery

> 里程碑：M4 | 依赖：Day 31 | 产出：`internal/findings`、modules、discoveries
> 对照主文档：§12.2 模块聚类、§12.3 指纹、§12.4 Discovery

## 0. 今天做什么
把端点聚成业务模块、推断技术栈（带证据）、把非漏洞线索收进 Discovery。

## 1. 进度位置
第 32 天。给 AI 一张"战场地图"（模块/技术栈/线索）。

## 2. 模块架构
```
internal/findings/   模块聚类、指纹、Discovery 管理
```

## 3. 技术选型
- 聚类：URL 前缀 + 参数共现 + 认证方式（§12.2）；可选嵌入增强（§31 E6）。

## 4. 设计细节
- 模块聚类输入/输出照 §12.2（用户/认证/订单/支付/文件/后台/消息/报表/其他）。
- 技术栈指纹（§12.3）：来源（正常响应/错误格式/Cookie/鉴权流程/JS 框架/...）；**输出必须带 EvidenceRef**，不把猜测升级成事实。
- Discovery（§12.4）：线索状态 OPEN/FOLLOWING/FOLLOWED/DISMISSED/PROMOTED/BLOCKED；退出前所有高价值 Discovery 必须有处置。

## 5. 分步开发
1. 模块聚类。
2. 指纹（带 EvidenceRef）。
3. Discovery CRUD + 状态机。

## 6. 完成效果（DoD）
- 端点被聚成模块；技术栈推断带证据引用；线索进 Discovery。

## 7. 怎么测试
- `tests/cluster_test.go`：聚类正确；无证据的指纹不生成；Discovery 状态流转。
- 命令：`go test ./internal/findings/...`
- 对照验收：§12.3"技术栈推断必须有 EvidenceRef"。
