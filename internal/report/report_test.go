package report

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pentagent/internal/domain"
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
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "report.db"))
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

const tss = "2026-11-17T00:00:00Z"

func seedRun(t *testing.T, w *store.Writer, ctx context.Context, runID string) {
	t.Helper()
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-r", "a", "m", tss); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`,
			runID, "ws-r", "r", "RUNNING", "scope-x", tss)
		return e
	}); err != nil {
		t.Fatalf("seed run: %v", err)
	}
}

// seedReportableFinding 造一个 REPORTABLE Finding + active verdict + 一份 evidence + 一条逐字引用。
func seedReportableFinding(t *testing.T, w *store.Writer, ctx context.Context, runID, fid string, withCitation bool) {
	t.Helper()
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT INTO findings (id,run_id,title,vulnerability_type,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
			fid, runID, "IDOR on /orders", "IDOR", findings.FindReportable, tss, tss); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO verification_verdicts (id,finding_id,run_id,revision,verdict,finding_revision,superseded,created_at) VALUES (?,?,?,?,?,?,0,?)`,
			"vd-"+fid, fid, runID, 1, "CONFIRMED", 1, tss); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO evidence (id,run_id,kind,plain_sha256,captured_at) VALUES (?,?,?,?,?)`,
			"ev-"+fid, runID, "http_response", "deadbeef", tss); e != nil {
			return e
		}
		if withCitation {
			if _, e := tx.Exec(`INSERT INTO evidence_citations (id,finding_id,evidence_id,receipt_sequence,byte_start,byte_end,content_sha256,truncated) VALUES (?,?,?,?,?,?,?,0)`,
				"ct-"+fid, fid, "ev-"+fid, 1, 0, 10, "abc123"); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed finding: %v", err)
	}
}

// DoD：只含 REPORTABLE；非 REPORTABLE Finding 不入报告（§13.6）。
func TestBuildOnlyReportable(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	seedReportableFinding(t, w, ctx, "run-1", "f-rep", true)
	// 一个 CANDIDATE（非 REPORTABLE）——不应入报告。
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO findings (id,run_id,title,state,created_at,updated_at) VALUES (?,?,?,?,?,?)`,
			"f-cand", "run-1", "maybe", findings.FindCandidate, tss, tss)
		return e
	}); err != nil {
		t.Fatal(err)
	}

	s := NewStore(w)
	res, err := s.Build(ctx, "run-1", nil, nil, t0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(res.Facts) != 1 || res.Facts[0].FindingID != "f-rep" {
		t.Fatalf("报告应只含 REPORTABLE f-rep，实得 %+v", res.Facts)
	}
	if strings.Contains(res.Markdown, "f-cand") || strings.Contains(res.JSON, "f-cand") {
		t.Fatalf("非 REPORTABLE 不应出现在报告里")
	}
}

// DoD：REPORTABLE 但缺逐字引用（事实无来源）→ 拒出报告（§13.6）。
func TestBuildRejectsFactWithoutSource(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-2")
	seedReportableFinding(t, w, ctx, "run-2", "f-nocite", false) // 无 citation
	s := NewStore(w)
	if _, err := s.Build(ctx, "run-2", nil, nil, t0); !errors.Is(err, ErrFactWithoutSource) {
		t.Fatalf("缺引用应拒（ErrFactWithoutSource），实得 %v", err)
	}
}

// DoD：零发现正常出报告（与 PARTIAL 区分由 Exit Gate 负责）。
func TestBuildZeroFinding(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-z")
	s := NewStore(w)
	res, err := s.Build(ctx, "run-z", nil, nil, t0)
	if err != nil {
		t.Fatalf("零发现应能出报告：%v", err)
	}
	if len(res.Facts) != 0 || !strings.Contains(res.Markdown, "零发现") {
		t.Fatalf("零发现报告应显式标注，实得 md=%q", res.Markdown)
	}
}

// DoD：手改 Markdown 不进权威——报告体由程序渲染并持久化；重取以库为准，CVSS 由程序算。
func TestBuildProgramAuthoritative(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-3")
	seedReportableFinding(t, w, ctx, "run-3", "f-cvss", true)
	s := NewStore(w)
	sev := map[string]findings.CVSSv31{
		"f-cvss": {AV: "N", AC: "L", PR: "N", UI: "N", S: "U", C: "H", I: "N", A: "N"},
	}
	// AI「描述」文本——只进描述小节，不改事实。
	desc := map[string]Descriptions{"f-cvss": {Description: "AI 写的描述", Remediation: "AI 修复建议"}}
	res, err := s.Build(ctx, "run-3", sev, desc, t0)
	if err != nil {
		t.Fatal(err)
	}
	body, err := s.Body(ctx, res.ReportID, "markdown")
	if err != nil {
		t.Fatal(err)
	}
	if body != res.Markdown {
		t.Fatalf("库中权威 Markdown 应与生成一致（手改副本不影响权威）")
	}
	// CVSS 必须由程序从结构化字段算出（7.5 High），AI 不自报。
	if res.Facts[0].CVSSSeverity != "High" || !strings.Contains(body, "CVSS:3.1/AV:N") {
		t.Fatalf("CVSS 应由程序确定性算出，实得 %.1f %s", res.Facts[0].CVSSScore, res.Facts[0].CVSSSeverity)
	}
}

// DoD：Finalize 前依赖失效（verdict 被 supersede）→ 报告标 STALE，不得 FINALIZED（§13.4）。
func TestFinalizeStaleOnSupersededVerdict(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-4")
	seedReportableFinding(t, w, ctx, "run-4", "f-s", true)
	s := NewStore(w)
	res, err := s.Build(ctx, "run-4", nil, nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	// 依赖失效：把 active verdict supersede（模拟反证降级后写了新 verdict）。
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`UPDATE verification_verdicts SET superseded=1 WHERE finding_id=?`, "f-s"); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO verification_verdicts (id,finding_id,run_id,revision,verdict,finding_revision,superseded,created_at) VALUES (?,?,?,?,?,?,0,?)`,
			"vd2-f-s", "f-s", "run-4", 2, "NEEDS_EVIDENCE", 1, tss)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(ctx, res.ReportID, t0); !errors.Is(err, ErrReportStale) {
		t.Fatalf("依赖失效应标 STALE 并返回 ErrReportStale，实得 %v", err)
	}
	st, _ := s.ReportState(ctx, res.ReportID)
	if st != StateStale {
		t.Fatalf("报告应 STALE，实得 %s", st)
	}
}

// Finalize 正常路径：依赖不变 → FINALIZED。
func TestFinalizeHappy(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-5")
	seedReportableFinding(t, w, ctx, "run-5", "f-ok", true)
	s := NewStore(w)
	res, err := s.Build(ctx, "run-5", nil, nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(ctx, res.ReportID, t0); err != nil {
		t.Fatalf("Finalize 正常路径应成功：%v", err)
	}
	if st, _ := s.ReportState(ctx, res.ReportID); st != StateFinalized {
		t.Fatalf("应 FINALIZED，实得 %s", st)
	}
}

var _ = domain.ErrNotFound
