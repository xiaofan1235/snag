package findings

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"pentagent/internal/store"
)

// fakeReader 模拟 EvidenceReader：映射 evidenceID → (内容, 归属 run)。
type fakeReader struct {
	m map[string]struct {
		content []byte
		run     string
	}
}

func (f fakeReader) ReadBytes(_ context.Context, id string) ([]byte, string, error) {
	e, ok := f.m[id]
	if !ok {
		return nil, "", errors.New("not found")
	}
	return e.content, e.run, nil
}

func seedEvidence(t *testing.T, w *store.Writer, ctx context.Context, evID, runID string) {
	t.Helper()
	const tss = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO evidence (id, run_id, kind, plain_sha256, captured_at) VALUES (?,?,?,?,?)`,
			evID, runID, "http_exchange", "deadbeef", tss)
		return e
	}); err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
}

// DoD：Agent 自报不能直接 REPORTABLE；独立 Verifier 才能逐级晋级（§4.3 #7）。
func TestFindingSelfReportCannotPromote(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	fid, err := s.CreateCandidate(ctx, "run-1", "", "IDOR on /api/users", "idor", t0)
	if err != nil {
		t.Fatal(err)
	}
	// Agent 自报 CANDIDATE→CONFIRMED（verifier-only）被拒。
	if err := s.TransitionFinding(ctx, fid, FindConfirmed, ActorAgent, t0); !errors.Is(err, ErrAgentSelfReport) {
		t.Fatalf("Agent 自报到 CONFIRMED 应拒，实得 %v", err)
	}
	// Verifier 逐级晋级 CANDIDATE→CONFIRMED→TRIAGED→REPORTABLE。
	for _, to := range []string{FindConfirmed, FindTriaged, FindReportable} {
		if err := s.TransitionFinding(ctx, fid, to, ActorVerifier, t0); err != nil {
			t.Fatalf("Verifier 迁移到 %s 失败：%v", to, err)
		}
	}
	if st, _ := s.FindingState(ctx, fid); st != FindReportable {
		t.Fatalf("终态应 REPORTABLE，实得 %s", st)
	}
}

// 非法跳级（CANDIDATE→REPORTABLE）即便 Verifier 也拒（状态机不允许该边）。
func TestFindingNoSkipToReportable(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	fid, _ := s.CreateCandidate(ctx, "run-1", "", "x", "idor", t0)
	if err := s.TransitionFinding(ctx, fid, FindReportable, ActorVerifier, t0); !errors.Is(err, ErrBadFindingTransition) {
		t.Fatalf("CANDIDATE→REPORTABLE 跳级应拒，实得 %v", err)
	}
}

// DoD：逐字引用匹配 → 入库；释义冒充 → 拒；伪造 ID/跨 Run → 编译期拒（§13.3）。
func TestEvidenceCitationVerbatim(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	fid, _ := s.CreateCandidate(ctx, "run-1", "", "IDOR", "idor", t0)

	seedEvidence(t, w, ctx, "ev-1", "run-1")
	content := []byte(`{"user_id":2,"email":"victim@corp.com","balance":9999}`)
	reader := fakeReader{m: map[string]struct {
		content []byte
		run     string
	}{
		"ev-1": {content, "run-1"},
		"ev-2": {[]byte("other run data"), "run-2"}, // 属 run-2
	}}

	// 逐字匹配：引用 email 字段的精确字节。
	start := len(`{"user_id":2,"email":"`)
	end := start + len("victim@corp.com")
	if _, err := s.AddCitation(ctx, fid, "run-1", reader, CitationInput{
		EvidenceID: "ev-1", ByteStart: start, ByteEnd: end, QuotedText: "victim@corp.com",
	}, t0); err != nil {
		t.Fatalf("逐字匹配应入库：%v", err)
	}
	if n, _ := s.CitationCount(ctx, fid); n != 1 {
		t.Fatalf("应有 1 条引用，实得 %d", n)
	}

	// 释义冒充：QuotedText 与实际字节不符 → 拒。
	if _, err := s.AddCitation(ctx, fid, "run-1", reader, CitationInput{
		EvidenceID: "ev-1", ByteStart: start, ByteEnd: end, QuotedText: "attacker@evil.com",
	}, t0); !errors.Is(err, ErrParaphrase) {
		t.Fatalf("释义冒充应 ErrParaphrase，实得 %v", err)
	}

	// 伪造 Evidence ID → 编译期拒。
	if _, err := s.AddCitation(ctx, fid, "run-1", reader, CitationInput{
		EvidenceID: "ev-forged", ByteStart: 0, ByteEnd: 1, QuotedText: "x",
	}, t0); !errors.Is(err, ErrForgedEvidence) {
		t.Fatalf("伪造 ID 应 ErrForgedEvidence，实得 %v", err)
	}

	// 跨 Run 引用（ev-2 属 run-2）→ 拒。
	if _, err := s.AddCitation(ctx, fid, "run-1", reader, CitationInput{
		EvidenceID: "ev-2", ByteStart: 0, ByteEnd: 5, QuotedText: "other",
	}, t0); !errors.Is(err, ErrCrossRun) {
		t.Fatalf("跨 Run 引用应 ErrCrossRun，实得 %v", err)
	}
}

// CRLF 归一化后仍能逐字映射（§13.3）。
func TestCitationCRLFNormalization(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	fid, _ := s.CreateCandidate(ctx, "run-1", "", "x", "idor", t0)
	seedEvidence(t, w, ctx, "ev-1", "run-1")
	content := []byte("HTTP/1.1 200 OK\r\nSet-Cookie: admin=1\r\n")
	reader := fakeReader{m: map[string]struct {
		content []byte
		run     string
	}{"ev-1": {content, "run-1"}}}
	// 引用含 CRLF 的片段；QuotedText 用 LF，归一化后匹配。
	end := len("HTTP/1.1 200 OK\r\n")
	if _, err := s.AddCitation(ctx, fid, "run-1", reader, CitationInput{
		EvidenceID: "ev-1", ByteStart: 0, ByteEnd: end, QuotedText: "HTTP/1.1 200 OK\n", Normalization: "crlf_lf",
	}, t0); err != nil {
		t.Fatalf("CRLF 归一化后应匹配：%v", err)
	}
}
