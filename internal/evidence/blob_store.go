// Package evidence 实现证据存储（§13.1 / §47.1 CEBS / §31 V1 / §4.1 evidence / §1.5）。
//
// 证据是整套「防谎报」的事实源。关键不变量：
//   - CEBS（§47.1）：de-chunk → 解压(gunzip) → 不转码；plain_sha256=SHA-256(CEBS)，内容寻址用明文哈希。
//   - 密文落盘、明文不落盘（§1.5）：zstd/gzip 压缩 → AES-256-GCM 加密后写盘；路径=f(plain_sha256)。
//   - 血缘三元组（§4.1）：identity_id / policy_revision / proxy_lease_id。
//   - 只追加不可改（§4.3 #5）：同一 plain_sha256 内容寻址天然去重，不覆盖已存在的证据语义。
//   - 脱敏投影 redacted_blob_path：给模型/普通 UI，绝不含秘密。
//
// 【有据偏离】dev-plan 用 zstd 压缩，本环境网络受限无法引入 zstd 依赖，改用 stdlib compress/gzip；
// AES-256-GCM 用 stdlib crypto。压缩算法可日后平滑换 zstd（不影响 plain_sha256，它对 CEBS 取值）。
package evidence

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// BlobStore 是内容寻址的加密 blob 存储：路径=f(plain_sha256)，内容=AEAD(gzip(CEBS))。
type BlobStore struct {
	root string
	gcm  cipher.AEAD
}

// NewBlobStore 用 32 字节 DEK 构造（AES-256-GCM）。root 是落盘根目录。
func NewBlobStore(root string, dek []byte) (*BlobStore, error) {
	if len(dek) != 32 {
		return nil, fmt.Errorf("DEK 必须 32 字节（AES-256），得 %d", len(dek))
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	return &BlobStore{root: root, gcm: gcm}, nil
}

// PlainSHA256 计算 CEBS 的明文 SHA-256（十六进制）。内容寻址的锚。
func PlainSHA256(cebs []byte) string {
	sum := sha256.Sum256(cebs)
	return hex.EncodeToString(sum[:])
}

// pathFor 由明文哈希派生分片路径（root/ab/cd/<sha>）。
func (b *BlobStore) pathFor(plainSHA string) string {
	if len(plainSHA) < 4 {
		return filepath.Join(b.root, plainSHA)
	}
	return filepath.Join(b.root, plainSHA[0:2], plainSHA[2:4], plainSHA)
}

// Put 把 CEBS 压缩+加密写盘，返回相对路径。内容寻址去重：已存在则跳过写（只追加不可改）。
func (b *BlobStore) Put(cebs []byte) (plainSHA, path string, err error) {
	plainSHA = PlainSHA256(cebs)
	full := b.pathFor(plainSHA)
	rel, _ := filepath.Rel(b.root, full)
	if _, e := os.Stat(full); e == nil {
		return plainSHA, rel, nil // 已存在，去重
	}
	// gzip 压缩。
	var zbuf bytes.Buffer
	zw := gzip.NewWriter(&zbuf)
	if _, e := zw.Write(cebs); e != nil {
		return "", "", e
	}
	if e := zw.Close(); e != nil {
		return "", "", e
	}
	// AES-256-GCM 加密：nonce || ciphertext。
	nonce := make([]byte, b.gcm.NonceSize())
	if _, e := rand.Read(nonce); e != nil {
		return "", "", e
	}
	ct := b.gcm.Seal(nonce, nonce, zbuf.Bytes(), nil)

	if e := os.MkdirAll(filepath.Dir(full), 0o700); e != nil {
		return "", "", e
	}
	if e := os.WriteFile(full, ct, 0o600); e != nil {
		return "", "", e
	}
	return plainSHA, rel, nil
}

// Get 读回 CEBS（解密+解压），并校验 plain_sha256 一致（§47.5 可独立重算）。
func (b *BlobStore) Get(plainSHA string) ([]byte, error) {
	ct, err := os.ReadFile(b.pathFor(plainSHA))
	if err != nil {
		return nil, err
	}
	ns := b.gcm.NonceSize()
	if len(ct) < ns {
		return nil, fmt.Errorf("密文过短")
	}
	nonce, enc := ct[:ns], ct[ns:]
	zbytes, err := b.gcm.Open(nil, nonce, enc, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zbytes))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	cebs, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	if PlainSHA256(cebs) != plainSHA {
		return nil, fmt.Errorf("plain_sha256 不一致：blob 损坏或被篡改")
	}
	return cebs, nil
}

// RawRead 读密文原始字节（测试用：证明明文不落盘）。
func (b *BlobStore) RawRead(plainSHA string) ([]byte, error) {
	return os.ReadFile(b.pathFor(plainSHA))
}
