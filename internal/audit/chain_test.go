package audit

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

const tss = "2026-11-17T00:00:00Z"

func openDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "audit.db"))
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

func seedRun(t *testing.T, w *store.Writer, ctx context.Context, runID string) {
	t.Helper()
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-a", "a", "m", tss); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`,
			runID, "ws-a", "r", "RUNNING", "scope-x", tss)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// 正常链：Anchor + Append 若干 → Verify OK。
func TestChainAppendAndVerify(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	c := NewChain(w)
	if _, err := c.Anchor(ctx, "run-1", "scope-x", tss); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, _, err := c.Append(ctx, "run-1", EntryAction, "act", map[string]any{"i": i, "tool": "http"}, tss); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	res, err := c.Verify(ctx, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Length != 6 { // 锚点 + 5
		t.Fatalf("验链应 OK 且长度 6，实得 %+v", res)
	}
}

// Append 前未 Anchor → 拒。
func TestAppendRequiresAnchor(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	c := NewChain(w)
	_, _, err := c.Append(ctx, "run-1", EntryAction, "act", map[string]any{"x": 1}, tss)
	if err == nil {
		t.Fatal("未 Anchor 的 Append 应拒")
	}
}

// 篡改中间条目内容（改 payload 不改 hash）→ Verify 检出 entry_hash 不符。
func TestTamperContentDetected(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	c := NewChain(w)
	_, _ = c.Anchor(ctx, "run-1", "scope-x", tss)
	for i := 0; i < 3; i++ {
		_, _, _ = c.Append(ctx, "run-1", EntryAction, "act", map[string]any{"i": i}, tss)
	}
	// 篡改 sequence=3 的 payload（但不重算 entry_hash）。
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`UPDATE audit_chain SET payload_json=? WHERE run_id=? AND sequence=3`, `{"i":999}`, "run-1")
		return e
	}); err != nil {
		t.Fatal(err)
	}
	res, _ := c.Verify(ctx, "run-1")
	if res.OK || res.BreakAt != 3 {
		t.Fatalf("内容篡改应在 seq=3 检出，实得 %+v", res)
	}
}

// 删除中间条目 → prev_hash 不连续被检出。
func TestTamperDeleteMiddleDetected(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	c := NewChain(w)
	_, _ = c.Anchor(ctx, "run-1", "scope-x", tss)
	for i := 0; i < 4; i++ {
		_, _, _ = c.Append(ctx, "run-1", EntryAction, "act", map[string]any{"i": i}, tss)
	}
	// 删除 sequence=3（中间）。导出后剩 1,2,4,5 → VerifyEntries 应检出不连续/断链。
	entries, _ := c.Export(ctx, "run-1")
	var tampered []Entry
	for _, e := range entries {
		if e.Sequence == 3 {
			continue
		}
		tampered = append(tampered, e)
	}
	res := VerifyEntries(tampered)
	if res.OK {
		t.Fatalf("删除中间条目应被检出，实得 %+v", res)
	}
}

// 篡改 entry_ref 并重算 entry_hash（攻击者补算本条）→ 下一条 prev_hash 不符仍被检出（链传播）。
func TestTamperRecomputedStillBreaksNext(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	c := NewChain(w)
	_, _ = c.Anchor(ctx, "run-1", "scope-x", tss)
	for i := 0; i < 4; i++ {
		_, _, _ = c.Append(ctx, "run-1", EntryAction, "act", map[string]any{"i": i}, tss)
	}
	entries, _ := c.Export(ctx, "run-1")
	// 攻击者改 seq=2 的 payload 并重算其 entry_hash（使本条自洽），但不改后续。
	for i := range entries {
		if entries[i].Sequence == 2 {
			entries[i].Payload = `{"i":777}`
			entries[i].EntryHash = hashEntry(entries[i].Payload, entries[i].PrevHash)
		}
	}
	res := VerifyEntries(entries)
	// seq=3 的 prev_hash 仍指向旧 seq=2 hash → 在 seq=3 断裂。
	if res.OK || res.BreakAt != 3 {
		t.Fatalf("重算自洽的篡改应在下一条（seq=3）断裂，实得 %+v", res)
	}
}

// 空链 Verify OK。
func TestVerifyEmptyChain(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	c := NewChain(w)
	res, _ := c.Verify(ctx, "run-1")
	if !res.OK || res.Length != 0 {
		t.Fatalf("空链应 OK，实得 %+v", res)
	}
}

// 两个 Run 的链互不干扰（§4.0 隔离）。
func TestChainPerRunIsolated(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-1")
	seedRun(t, w, ctx, "run-2")
	c := NewChain(w)
	_, _ = c.Anchor(ctx, "run-1", "scope-x", tss)
	_, _ = c.Anchor(ctx, "run-2", "scope-y", tss)
	_, _, _ = c.Append(ctx, "run-1", EntryAction, "a", map[string]any{"x": 1}, tss)
	r1, _ := c.Verify(ctx, "run-1")
	r2, _ := c.Verify(ctx, "run-2")
	if !r1.OK || r1.Length != 2 || !r2.OK || r2.Length != 1 {
		t.Fatalf("两 Run 链应各自独立且 OK，实得 r1=%+v r2=%+v", r1, r2)
	}
}
