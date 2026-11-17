package findings

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"pentagent/internal/domain"
)

// citation.go —— 逐字 EvidenceCitation（§13.3，吸收 PentestGPT，防「释义冒充证据」）。
//
// Finding 的每条事实性陈述必须挂 EvidenceCitation{evidence_id, receipt_sequence, byte_start, byte_end,
// content_sha256, normalization}，映射到不可变 Evidence 的**连续片段**：
//   - 引用内容与结论不符（释义冒充）→ 拒晋级；
//   - 伪造 Evidence ID、跨 Run/跨 Attempt 引用 → 编译期拒绝（FK + run 校验）；
//   - CRLF/LF 归一化后仍须映射回原文；截断显式标 TRUNCATED，不静默截断；
//   - 诊断/模型摘要/Memory 不作 grounding（由 reader 只暴露真实 Evidence 保证）。

var (
	// ErrForgedEvidence —— 引用了不存在的 Evidence ID（编译期拒）。
	ErrForgedEvidence = &domain.DomainError{Code: "FORGED_EVIDENCE", Msg: "citation references nonexistent evidence (§13.3)"}
	// ErrCrossRun —— 跨 Run/跨 Attempt 引用（编译期拒）。
	ErrCrossRun = &domain.DomainError{Code: "CROSS_RUN_CITATION", Msg: "evidence belongs to a different run (§13.3)"}
	// ErrParaphrase —— 引用内容与声称不符（释义冒充）。
	ErrParaphrase = &domain.DomainError{Code: "PARAPHRASE_CITATION", Msg: "quoted text does not match cited evidence bytes (§13.3)"}
	// ErrBadRange —— 字节区间非法。
	ErrBadRange = &domain.DomainError{Code: "BAD_CITATION_RANGE", Msg: "invalid byte range"}
)

// EvidenceReader 暴露不可变 Evidence 的原始字节与归属 Run（只给真实 Evidence，不给诊断/摘要）。
type EvidenceReader interface {
	ReadBytes(ctx context.Context, evidenceID string) (content []byte, runID string, err error)
}

// CitationInput 是一条待校验的逐字引用。
type CitationInput struct {
	EvidenceID      string
	ReceiptSequence int
	ByteStart       int
	ByteEnd         int
	QuotedText      string // 结论声称的原文（将与实际字节逐字核对）
	Normalization   string // "" | "crlf_lf"
	Truncated       bool
}

// normalize 按归一化策略处理（CRLF→LF 后仍映射回原文偏移，§13.3）。
func normalize(s, mode string) string {
	if mode == "crlf_lf" {
		return strings.ReplaceAll(s, "\r\n", "\n")
	}
	return s
}

// AddCitation 校验并持久化一条逐字引用。校验顺序：存在性（伪造 ID 拒）→ 同 Run（跨 Run 拒）→ 字节区间 →
// 逐字核对（释义拒）→ 计算 content_sha256 入库。全部通过才写 evidence_citations。
func (s *Store) AddCitation(ctx context.Context, findingID, runID string, reader EvidenceReader, in CitationInput, now time.Time) (string, error) {
	content, evRun, err := reader.ReadBytes(ctx, in.EvidenceID)
	if err != nil || content == nil {
		return "", ErrForgedEvidence // 伪造/不存在 Evidence ID
	}
	if evRun != runID {
		return "", ErrCrossRun // 跨 Run/跨 Attempt 引用
	}
	if in.ByteStart < 0 || in.ByteEnd > len(content) || in.ByteStart >= in.ByteEnd {
		return "", ErrBadRange
	}
	slice := content[in.ByteStart:in.ByteEnd]
	normSlice := normalize(string(slice), in.Normalization)
	// 逐字核对：结论声称的原文（归一化后）必须与实际字节一致，否则释义冒充。
	if in.QuotedText != "" && normalize(in.QuotedText, in.Normalization) != normSlice {
		return "", ErrParaphrase
	}
	sum := sha256.Sum256([]byte(normSlice))
	contentSHA := hex.EncodeToString(sum[:])

	id := string(domain.NewID())
	trunc := 0
	if in.Truncated {
		trunc = 1
	}
	err = s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO evidence_citations (id, finding_id, evidence_id, receipt_sequence, byte_start, byte_end, content_sha256, normalization, truncated)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			id, findingID, in.EvidenceID, in.ReceiptSequence, in.ByteStart, in.ByteEnd, contentSHA, nz(in.Normalization), trunc)
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// CitationCount 返回某 Finding 的引用数（晋级需非空）。
func (s *Store) CitationCount(ctx context.Context, findingID string) (int, error) {
	var n int
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM evidence_citations WHERE finding_id=?`, findingID).Scan(&n)
	})
	return n, err
}
