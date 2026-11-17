// evidence_test.go —— 策略绕过的证据完整性与自报组（§22.4，需 DB）。
// 命令：go test -tags sqlite_modernc ./tests/policybypass/...（无驱动自动 SKIP）。
//
// 覆盖：
//
//	伪造 Evidence ID               → TestForgedEvidenceIDRejected
//	释义冒充逐字引用               → TestParaphraseCitationRejected
//	跨 Run/跨 Attempt 证据引用     → TestCrossRunCitationRejected
//	模型自报把 Finding 升 REPORTABLE → TestAgentSelfReportFindingRejected
//	Memory 声称「已测」改 Coverage   → TestSelfReportCoverageRejected
package policybypass_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/coverage"
	"pentagent/internal/findings"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

var t0 = time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

func openDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "pb.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	return w, context.Background()
}

const ts = "2026-11-17T00:00:00Z"

func seedRun(t *testing.T, w *store.Writer, ctx context.Context, runID string) {
	t.Helper()
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-pb", "a", "m", ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`,
			runID, "ws-pb", "r", "RUNNING", "d", ts)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// memReader 实现 findings.EvidenceReader：只暴露登记过的真实 Evidence 的字节与归属 Run。
type memReader struct {
	data map[string]struct {
		content []byte
		runID   string
	}
}

func (m *memReader) ReadBytes(_ context.Context, id string) ([]byte, string, error) {
	e, ok := m.data[id]
	if !ok {
		return nil, "", nil // 伪造/不存在 ID：返回 nil → AddCitation 判 ErrForgedEvidence
	}
	return e.content, e.runID, nil
}

// §22.4 伪造 Evidence ID：引用不存在的 Evidence → 编译期拒。
func TestForgedEvidenceIDRejected(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	s := findings.NewStore(w)
	fid, _ := s.CreateCandidate(ctx, "run-1", "", "t", "IDOR", t0)
	reader := &memReader{data: map[string]struct {
		content []byte
		runID   string
	}{}}
	_, err := s.AddCitation(ctx, fid, "run-1", reader, findings.CitationInput{
		EvidenceID: "ev-forged", ByteStart: 0, ByteEnd: 4, QuotedText: "data",
	}, t0)
	if !errors.Is(err, findings.ErrForgedEvidence) {
		t.Fatalf("伪造 Evidence ID 应拒（ErrForgedEvidence），实得 %v", err)
	}
}

// §22.4 释义冒充逐字引用：引用内容与声称不符 → 拒。
func TestParaphraseCitationRejected(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	s := findings.NewStore(w)
	fid, _ := s.CreateCandidate(ctx, "run-1", "", "t", "IDOR", t0)
	reader := &memReader{data: map[string]struct {
		content []byte
		runID   string
	}{"ev-1": {content: []byte("account B secret order"), runID: "run-1"}}}
	// 声称原文 "account A"，实际字节是 "account B" → 释义冒充。
	_, err := s.AddCitation(ctx, fid, "run-1", reader, findings.CitationInput{
		EvidenceID: "ev-1", ByteStart: 0, ByteEnd: 9, QuotedText: "account A",
	}, t0)
	if !errors.Is(err, findings.ErrParaphrase) {
		t.Fatalf("释义冒充应拒（ErrParaphrase），实得 %v", err)
	}
}

// §22.4 跨 Run/跨 Attempt 证据引用：证据属于另一个 Run → 拒。
func TestCrossRunCitationRejected(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	s := findings.NewStore(w)
	fid, _ := s.CreateCandidate(ctx, "run-1", "", "t", "IDOR", t0)
	reader := &memReader{data: map[string]struct {
		content []byte
		runID   string
	}{"ev-other": {content: []byte("data"), runID: "run-OTHER"}}}
	_, err := s.AddCitation(ctx, fid, "run-1", reader, findings.CitationInput{
		EvidenceID: "ev-other", ByteStart: 0, ByteEnd: 4, QuotedText: "data",
	}, t0)
	if !errors.Is(err, findings.ErrCrossRun) {
		t.Fatalf("跨 Run 引用应拒（ErrCrossRun），实得 %v", err)
	}
}

// §22.4 模型自报：Agent 不能把 Finding 直接升到 Verifier-only 状态（CONFIRMED/REPORTABLE）。
func TestAgentSelfReportFindingRejected(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	s := findings.NewStore(w)
	fid, _ := s.CreateCandidate(ctx, "run-1", "", "t", "IDOR", t0)
	// Agent 自报试图 CANDIDATE→CONFIRMED（verifier-only）→ 拒。
	err := s.TransitionFinding(ctx, fid, findings.FindConfirmed, findings.ActorAgent, t0)
	if !errors.Is(err, findings.ErrAgentSelfReport) {
		t.Fatalf("Agent 自报升 CONFIRMED 应拒（ErrAgentSelfReport），实得 %v", err)
	}
	// 独立 Verifier 可以。
	if err := s.TransitionFinding(ctx, fid, findings.FindConfirmed, findings.ActorVerifier, t0); err != nil {
		t.Fatalf("Verifier 升 CONFIRMED 应成功：%v", err)
	}
}

// §22.4 Memory Observation 声称「已测」改 Coverage：MarkTested 必须绑定已验证 Experiment Job，
// 不接受自报——未验证实验 → 拒。
func TestSelfReportCoverageRejected(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	cs := coverage.NewStore(w)
	cell := coverage.Cell{ModuleID: "mod-1", EndpointID: "ep-1", ParameterName: "id", IdentityClass: "A", AttackType: "IDOR"}
	if err := cs.EnsureCell(ctx, "run-1", cell, 0, t0); err != nil {
		t.Fatal(err)
	}
	// 种一个未验证（PROPOSED）实验——相当于「自报」未经验证。
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO experiments (id,run_id,spec_hash,attack_type,identity_class,parameter_name,falsifier,state,falsifier_result,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			"exp-1", "run-1", "sh", "IDOR", "A", "id", "fals", coverage.ExpProposed, "not_evaluated", ts, ts)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	err := cs.MarkTested(ctx, "run-1", cell, "exp-1", t0)
	if !errors.Is(err, coverage.ErrNotVerifiedExperiment) {
		t.Fatalf("未验证实验标 TESTED 应拒（ErrNotVerifiedExperiment），实得 %v", err)
	}
}
