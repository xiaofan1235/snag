# Day 20 —— Evidence Store（CEBS + 内容寻址 + 加密）

> 里程碑：M2 | 依赖：Day 18 | 产出：`internal/evidence/`
> 对照主文档：§13.1、§47.1 CEBS、§31 V1、§4.1 evidence、§1.5(v1.2)

## 0. 今天做什么
证据存储：规范证据字节流（CEBS）+ 内容寻址（明文哈希）+ zstd 压缩 + AEAD 加密 + 血缘三元组（identity/policy_revision/proxy_lease）。

## 1. 进度位置
第 20 天。证据是整套"防谎报"的事实源，字节流定义必须对（§47.1）。

## 2. 模块架构
```
internal/evidence/
  service.go blob_store.go redaction.go
```

## 3. 技术选型
- 内容寻址：路径 = f(plain_sha256)。
- zstd 压缩 → AES-256-GCM 加密（DEK）。FTS5 索引脱敏文本。

## 4. 设计细节
- **CEBS**（§47.1/§13）：de-chunk→解压(gunzip/brotli)→不转码；plain_sha256=SHA-256(CEBS)；偏移以 CEBS 为准；CRLF 归一化只用于比较。
- 血缘三元组（§4.1）：identity_id/policy_revision/proxy_lease_id。
- Evidence 只追加不可改（§4.3 #5）；明文哈希内容寻址，密文落盘（§1.5）。
- 脱敏投影 redacted_blob_path（给模型/普通 UI）。

## 5. 分步开发
1. 从 Proxy 捕获的交换 → 组装 CEBS → plain_sha256。
2. zstd+AEAD 落盘；写 evidence 元数据（血缘三元组）。
3. 脱敏投影生成。

## 6. 完成效果（DoD）
- 一个 HTTP 交换被存为 Evidence：plain_sha256 可独立重算、密文落盘、血缘齐全。

## 7. 怎么测试
- `tests/evidence_test.go`：存 gzip 响应→CEBS 解压正确→plain_sha256 可重算；密文落盘、明文不落盘；脱敏投影不含秘密。
- 命令：`go test ./internal/evidence/...`
- 对照验收：§47.5、§31 V1。
