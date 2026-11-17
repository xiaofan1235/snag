package events_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"pentagent/internal/domain"
	"pentagent/internal/events"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// events_test.go —— Day 10 §7：事件日志序号验收（run 内 + 全局单调、无空洞；回滚则无事件）。
// 命令：go test -tags sqlite_modernc ./internal/events/...（无驱动自动 SKIP）。

func openEventsDB(t *testing.T) (*store.Writer, *store.Reader, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "ev.db")
	w, err := store.OpenWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	rd, err := store.OpenReader(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rd.Close() })
	return w, rd, context.Background()
}

func seedRun(t *testing.T, w *store.Writer, ctx context.Context, runID string) {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			"ws-1", "acme", "mpk-1", ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT OR IGNORE INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
			VALUES (?,?,?,?,?,?)`, runID, "ws-1", "r", "RUNNING", "digest", ts)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// run 内 sequence 从 1 单调递增且无空洞；全局 global_sequence 跨 run 全局单调。
func TestAppendSequencesMonotonic(t *testing.T) {
	w, rd, ctx := openEventsDB(t)
	seedRun(t, w, ctx, "run-1")
	seedRun(t, w, ctx, "run-2")

	type want struct {
		run       string
		seq, gseq int64
	}
	var got []want
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		for _, r := range []string{"run-1", "run-1", "run-2", "run-1", "run-2"} {
			a, e := events.Append(ctx, tx, events.Event{RunID: domain.ID(r), Type: "run.updated", EntityType: "run", EntityID: domain.ID(r), EntityVersion: 1})
			if e != nil {
				return e
			}
			got = append(got, want{r, a.Sequence, a.GlobalSequence})
		}
		return nil
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	// run-1 的 sequence 应为 1,2,3；run-2 的为 1,2；全局依次 1..5。
	expSeq := map[string][]int64{"run-1": {1, 2, 3}, "run-2": {1, 2}}
	seen := map[string][]int64{}
	var gseqs []int64
	for _, g := range got {
		seen[g.run] = append(seen[g.run], g.seq)
		gseqs = append(gseqs, g.gseq)
	}
	for r, exp := range expSeq {
		if len(seen[r]) != len(exp) {
			t.Fatalf("%s 事件数 %d != %d", r, len(seen[r]), len(exp))
		}
		for i := range exp {
			if seen[r][i] != exp[i] {
				t.Errorf("%s 第 %d 个 sequence=%d，期望 %d", r, i, seen[r][i], exp[i])
			}
		}
	}
	for i, g := range gseqs {
		if g != int64(i+1) {
			t.Errorf("第 %d 个 global_sequence=%d，期望 %d（全局单调无空洞）", i, g, i+1)
		}
	}

	// 库内复核：run-1 的 sequence 连续无空洞。
	rows, err := rd.DB().Query(`SELECT sequence FROM events WHERE run_id=? ORDER BY sequence`, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var prev int64
	for rows.Next() {
		var s int64
		if e := rows.Scan(&s); e != nil {
			t.Fatal(e)
		}
		if s != prev+1 {
			t.Errorf("run-1 sequence 空洞：%d 之后是 %d", prev, s)
		}
		prev = s
	}
}

// 事务回滚 → 无事件（§0.5：提交失败则无事件）。
func TestAppendRollbackNoEvent(t *testing.T) {
	w, rd, ctx := openEventsDB(t)
	seedRun(t, w, ctx, "run-1")

	boom := errors.New("boom")
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := events.Append(ctx, tx, events.Event{RunID: "run-1", Type: "run.updated", EntityType: "run", EntityID: "run-1", EntityVersion: 1}); e != nil {
			return e
		}
		return boom // 触发回滚
	}); !errors.Is(err, boom) {
		t.Fatalf("应回滚并返回 boom，得 %v", err)
	}
	var n int
	if e := rd.DB().QueryRow(`SELECT COUNT(1) FROM events WHERE run_id=?`, "run-1").Scan(&n); e != nil {
		t.Fatal(e)
	}
	if n != 0 {
		t.Fatalf("回滚后应无事件，得 %d", n)
	}
}
