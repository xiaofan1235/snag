package evidence

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// AssembleCEBS 从 Proxy 捕获的响应体组装规范证据字节流（§47.1）：
// de-chunk（http 已解）→ 按 Content-Encoding 解压（gunzip）→ **不转码**。CRLF 归一化只用于比较、不入 CEBS。
func AssembleCEBS(body []byte, contentEncoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(contentEncoding)) {
	case "", "identity":
		return body, nil
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("gunzip: %w", err)
		}
		defer zr.Close()
		return io.ReadAll(zr)
	case "br":
		return nil, fmt.Errorf("brotli 解压需外部依赖，本构建未支持（CEBS 不转码，待接入）")
	default:
		return nil, fmt.Errorf("未知 Content-Encoding: %q（fail-closed，不猜测解码）", contentEncoding)
	}
}

// Lineage 是证据血缘三元组（§4.1）。
type Lineage struct {
	IdentityID     string
	PolicyRevision int64
	ProxyLeaseID   string
}

// StoreRequest 是一次证据写入请求。
type StoreRequest struct {
	RunID           domain.ID
	ActionID        domain.ID
	Kind            string // 如 "http_exchange"
	Body            []byte // Proxy 捕获的原始响应体（可能 gzip/chunked）
	ContentEncoding string
	ContentType     string
	CaptureSource   string // 如 "policy_proxy"
	Lineage         Lineage
	Secrets         []string // 本动作用到的字面秘密（脱敏用）
}

// Evidence 是一条证据的元数据视图。
type Evidence struct {
	ID               domain.ID
	PlainSHA256      string
	BlobPath         string
	RedactedBlobPath string
	SizeBytes        int
	Lineage          Lineage
	CapturedAt       string
}

// Service 组装 CEBS、落密文、写元数据（血缘齐全）、产脱敏投影。
type Service struct {
	w     *store.Writer
	blobs *BlobStore
}

// NewService 构造。
func NewService(w *store.Writer, blobs *BlobStore) *Service { return &Service{w: w, blobs: blobs} }

// Store 把一次 HTTP 交换存为 Evidence：CEBS→plain_sha256→密文落盘→脱敏投影→元数据（血缘）。
func (s *Service) Store(ctx context.Context, req StoreRequest) (Evidence, error) {
	cebs, err := AssembleCEBS(req.Body, req.ContentEncoding)
	if err != nil {
		return Evidence{}, err
	}
	plainSHA, blobPath, err := s.blobs.Put(cebs)
	if err != nil {
		return Evidence{}, err
	}
	redacted := Redact(cebs, req.Secrets)
	_, redactedPath, err := s.blobs.Put(redacted)
	if err != nil {
		return Evidence{}, err
	}

	id := domain.NewID()
	now := time.Now().UTC().Format(time.RFC3339)
	kind := req.Kind
	if kind == "" {
		kind = "http_exchange"
	}
	err = s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`INSERT INTO evidence
			 (id, run_id, action_id, kind, identity_id, policy_revision, proxy_lease_id,
			  plain_sha256, blob_path, redacted_blob_path, content_type, size_bytes, captured_at, capture_source)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			string(id), string(req.RunID), nz(string(req.ActionID)), kind,
			nz(req.Lineage.IdentityID), req.Lineage.PolicyRevision, nz(req.Lineage.ProxyLeaseID),
			plainSHA, blobPath, redactedPath, nz(req.ContentType), len(cebs), now, nz(req.CaptureSource))
		return e
	})
	if err != nil {
		return Evidence{}, err
	}
	return Evidence{
		ID: id, PlainSHA256: plainSHA, BlobPath: blobPath, RedactedBlobPath: redactedPath,
		SizeBytes: len(cebs), Lineage: req.Lineage, CapturedAt: now,
	}, nil
}

// FetchCEBS 读回某证据的明文 CEBS（解密+解压+校验哈希）。
func (s *Service) FetchCEBS(plainSHA string) ([]byte, error) { return s.blobs.Get(plainSHA) }

func nz(s string) any {
	if s == "" {
		return nil
	}
	return s
}
