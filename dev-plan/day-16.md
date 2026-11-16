# Day 16 —— Scope 规范化器 + 差分模糊（红线的地基）

> 里程碑：M2 | 依赖：Day 15 | 产出：`internal/policy/scope.go`、`tests/scope_fuzz_test.go`
> 对照主文档：§9.3 Scope 检查、§9.0.4、§31 P2x、§4.1 P2x

## 0. 今天做什么
实现 URL/Scope 规范化器，处理 §9.3 全部陷阱；并用**差分模糊 + 属性测试**验证它——这是红线安全**单位投入回报最高**的一天。

## 1. 进度位置
第 16 天。§9.0.4 说过：红线唯一真实攻击面就是这个规范化器。做不对，后面全套红线白搭。

## 2. 模块架构
```
internal/policy/scope.go   canonicalize(url) + inScope(url, policy)
tests/scope_fuzz_test.go   差分模糊 + 属性测试
```

## 3. 技术选型
- Go `net/url` + 自己补齐陷阱处理。
- 属性测试 rapid；差分对照多个独立解析实现。

## 4. 设计细节
- 处理 §9.3 全清单：大小写、末尾点、IDN/Punycode、默认端口、Userinfo 欺骗、多重 URL 编码、IPv4 异常表示、IPv6、DNS 重绑定、重定向、协议/端口变化、代理 Bypass、浏览器弹窗/子框架。
- **精确域名 vs 泛域名是两种匹配**（§9.3）：app.example.com 不自动允许 other.example.com。
- 硬属性：canonicalize 幂等；等价变形 inScope 判定一致；精确域名绝不匹配子域。
- **解析分歧 → fail-closed 到更严那侧**（§31 P2x）。

## 5. 分步开发
1. `canonicalize`：逐条处理陷阱。
2. `inScope`：精确/泛域名分别匹配。
3. fuzz：喂 §9.3 清单 + SSRF/bypass 字典；差分对照；属性断言。

## 6. 完成效果（DoD）
- §9.3 全部陷阱用例判定正确；fuzz 跑一段时间无"越权被判合规"。

## 7. 怎么测试
- `go test -fuzz=FuzzScope -fuzztime=5m ./internal/policy/`（或 rapid 大量随机）。
- `tests/scope_test.go`：§9.3 逐项用例。
- 对照验收：§9.0.5、§22.4 策略绕过清单。
