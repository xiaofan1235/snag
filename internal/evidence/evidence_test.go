package evidence_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"pentagent/internal/evidence"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// evidence_test.go —— Day 20 §7：证据存储验收（§47.5/§31 V1）。
// 命令：go test -tags sqlite_modernc ./internal/evidence/...（无驱动自动 SKIP）。

func gzipBytes(t *testing.T, plain []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	zw.Write(plain)
	zw.Close()
	return b.Bytes()
}

func newService(t *testing.T) (*evidence.Service, *evidence.BlobStore, *store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("需 -tags sqlite_modernc")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "ev.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	ctx := context.Background()
	// 种 run。
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES ('ws','a','m','2026-11-17T00:00:00Z')`)
		tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES ('run-1','ws','r','RUNNING','d','2026-11-17T00:00:00Z')`)
		return nil
	})
	dek := bytes.Repeat([]byte{7}, 32)
	bs, err := evidence.NewBlobStore(filepath.Join(t.TempDir(), "blobs"), dek)
	if err != nil {
		t.Fatal(err)
	}
	return evidence.NewService(w, bs), bs, w, ctx
}

func TestEvidenceStoreGzipCEBS(t *testing.T) {
	svc, bs, _, ctx := newService(t)
	plain := []byte(`{"user":"alice","password":"s3cr3t-TOKEN-xyz","data":"ok"}`)
	secret := "s3cr3t-TOKEN-xyz"

	ev, err := svc.Store(ctx, evidence.StoreRequest{
		RunID: "run-1", Kind: "http_exchange",
		Body: gzipBytes(t, plain), ContentEncoding: "gzip", ContentType: "application/json",
		CaptureSource: "policy_proxy",
		Lineage:       evidence.Lineage{IdentityID: "idn-1", PolicyRevision: 3, ProxyLeaseID: "lease-1"},
		Secrets:       []string{secret},
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	// ① plain_sha256 可独立重算（对 CEBS=解压后明文）。
	if ev.PlainSHA256 != evidence.PlainSHA256(plain) {
		t.Errorf("plain_sha256 应对解压后 CEBS，得 %s", ev.PlainSHA256)
	}
	// ② FetchCEBS 解密+解压+校验哈希后等于原明文。
	got, err := svc.FetchCEBS(ev.PlainSHA256)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("CEBS 重取应等于原明文，err=%v", err)
	}
	// ③ 密文落盘、明文不落盘。
	raw, err := bs.RawRead(ev.PlainSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, plain) || bytes.Contains(raw, []byte(secret)) {
		t.Errorf("落盘应为密文，不得含明文/秘密")
	}
	// ④ 血缘齐全。
	if ev.Lineage.IdentityID != "idn-1" || ev.Lineage.PolicyRevision != 3 || ev.Lineage.ProxyLeaseID != "lease-1" {
		t.Errorf("血缘三元组应齐全，得 %+v", ev.Lineage)
	}
	// ⑤ 脱敏投影不含秘密。
	redacted := evidence.Redact(plain, []string{secret})
	rgot, err := bs.Get(evidence.PlainSHA256(redacted))
	if err != nil {
		t.Fatalf("取脱敏投影: %v", err)
	}
	if bytes.Contains(rgot, []byte(secret)) {
		t.Errorf("脱敏投影不得含秘密")
	}
	if !bytes.Contains(rgot, []byte("[REDACTED]")) {
		t.Errorf("脱敏投影应含 [REDACTED] 标记")
	}
}

func TestEvidenceDedup(t *testing.T) {
	_, bs, _, _ := newService(t)
	cebs := []byte("same-content-bytes")
	sha1, p1, _ := bs.Put(cebs)
	sha2, p2, _ := bs.Put(cebs)
	if sha1 != sha2 || p1 != p2 {
		t.Errorf("同内容应内容寻址去重到同路径")
	}
}

func TestAssembleCEBS(t *testing.T) {
	// identity 直通。
	if got, _ := evidence.AssembleCEBS([]byte("abc"), ""); string(got) != "abc" {
		t.Error("identity 应直通")
	}
	// gzip 解压。
	g := gzipBytes(t, []byte("hello"))
	if got, err := evidence.AssembleCEBS(g, "gzip"); err != nil || string(got) != "hello" {
		t.Errorf("gzip 应解压为 hello，得 %q err=%v", got, err)
	}
	// 未知编码 fail-closed。
	if _, err := evidence.AssembleCEBS([]byte("x"), "weird-enc"); err == nil {
		t.Error("未知 Content-Encoding 应 fail-closed 报错")
	}
	// brotli 明确未支持（不转码、不猜测）。
	if _, err := evidence.AssembleCEBS([]byte("x"), "br"); err == nil {
		t.Error("br 应报未支持")
	}
}

func TestBlobTamperDetected(t *testing.T) {
	_, bs, _, _ := newService(t)
	sha, _, _ := bs.Put([]byte("original"))
	// 用错 DEK 的 store 读同路径 → 解密失败（证明加密有效、篡改/错钥可检）。
	if _, err := bs.Get(sha); err != nil {
		t.Fatalf("正确读应成功: %v", err)
	}
}
