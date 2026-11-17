package oast

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

var t0 = time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

func openDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "oast.db"))
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

func seedRun(t *testing.T, w *store.Writer, ctx context.Context) {
	t.Helper()
	const tss = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-o", "a", "m", tss); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`, "run-1", "ws-o", "r", "RUNNING", "d", tss)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// DoD：OAST 回调按关联 ID 配回证据 → RESOLVED/CONFIRMED；不同 Run 关联隔离。
func TestOASTResolveAndIsolation(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	corrID, token, err := s.CreateCorrelation(ctx, "run-1", "", "exp-1", "dns", t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 32 { // 16 字节 hex = 不可预测关联 ID
		t.Fatalf("token 应为 32 hex 字符，实得 %d", len(token))
	}
	// 不同 Run 用同 token 回连 → 配不回（关联隔离）。
	if _, err := s.ResolveCallback(ctx, "run-other", token, "9.9.9.9", "{}", "ev-x", t0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("跨 Run 回调应隔离（ErrNotFound），实得 %v", err)
	}
	// 正确 Run 回连 → RESOLVED。
	if _, err := s.ResolveCallback(ctx, "run-1", token, "1.2.3.4", `{"proto":"dns"}`, "ev-1", t0); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.State(ctx, corrID); st != StateResolved {
		t.Fatalf("回连后应 RESOLVED，实得 %s", st)
	}
	if v, _ := s.Verdict(ctx, corrID); v != VerdictConfirmed {
		t.Fatalf("RESOLVED 的 Verdict 应 CONFIRMED，实得 %s", v)
	}
}

// DoD：OAST 超时未回 → TIMEOUT → UNABLE_TO_VERIFY（未证实，不等于不存在）。
func TestOASTTimeoutUnableToVerify(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	corrID, _, _ := s.CreateCorrelation(ctx, "run-1", "", "exp-1", "http", t0)
	// 待定时无结论（Finding 停在 NEEDS_EVIDENCE）。
	if v, _ := s.Verdict(ctx, corrID); v != "" {
		t.Fatalf("PENDING 应无结论，实得 %q", v)
	}
	// 超时扫描（cutoff 晚于创建时间）→ TIMEOUT。
	n, err := s.ExpireTimeouts(ctx, t0.Add(time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("应有 1 条超时，实得 n=%d err=%v", n, err)
	}
	if v, _ := s.Verdict(ctx, corrID); v != VerdictUnableToVerify {
		t.Fatalf("超时 Verdict 应 UNABLE_TO_VERIFY，实得 %s", v)
	}
}

// 两个关联 token 不同（不可预测）。
func TestOASTTokensUnpredictable(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	_, t1, _ := s.CreateCorrelation(ctx, "run-1", "", "e", "dns", t0)
	_, t2, _ := s.CreateCorrelation(ctx, "run-1", "", "e", "dns", t0)
	if t1 == t2 {
		t.Fatal("关联 token 应不可预测、各不相同")
	}
}
