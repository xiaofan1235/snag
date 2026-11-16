package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// writer_concurrency_test.go —— Day 06 §7 验收：单写者串行化。
// 命令：go test -race -tags sqlite_modernc ./internal/store/...
// 对照：§31 C1（持久性分级）、§31 C2（单写者，无 SQLITE_BUSY）。

func openTestWriter(t *testing.T) (*store.Writer, string) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "w.db")
	w, err := store.OpenWriter(path)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	return w, path
}

// 3 个并发 Worker 各写 1000 条事件（bulk）+ 偶发 durable 提交，必须无 SQLITE_BUSY、数据完整。
func TestWriterConcurrencyNoBusy(t *testing.T) {
	w, path := openTestWriter(t)
	ctx := context.Background()
	const workers, perWorker = 3, 1000

	// 种子：workspace + 3 个 Run（durable）。
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			"ws-1", "acme", "mpk-1", "2026-11-16T00:00:00Z"); err != nil {
			return err
		}
		for g := 0; g < workers; g++ {
			if _, err := tx.Exec(
				`INSERT INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
				 VALUES (?,?,?,?,?,?)`,
				fmt.Sprintf("run-%d", g), "ws-1", "r", "RUNNING", "digest", "2026-11-16T00:00:00Z"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for g := 0; g < workers; g++ {
		wg.Add(1)
		go func(run string) {
			defer wg.Done()
			for seq := 1; seq <= perWorker; seq++ {
				// bulk：写事件。
				if err := w.Bulk(ctx, func(tx *sql.Tx) error {
					_, e := tx.Exec(
						`INSERT INTO events (run_id, sequence, event_type, created_at) VALUES (?,?,?,?)`,
						run, seq, "job.updated", "2026-11-16T00:00:00Z")
					return e
				}); err != nil {
					errs <- fmt.Errorf("%s bulk seq=%d: %w", run, seq, err)
					return
				}
				// 每 100 条穿插一次 durable 提交（更新 Run revision）。
				if seq%100 == 0 {
					if err := w.Durable(ctx, func(tx *sql.Tx) error {
						_, e := tx.Exec(`UPDATE runs SET revision = revision + 1 WHERE id = ?`, run)
						return e
					}); err != nil {
						errs <- fmt.Errorf("%s durable seq=%d: %w", run, seq, err)
						return
					}
				}
			}
			errs <- nil
		}(fmt.Sprintf("run-%d", g))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("并发写出错（不应有 SQLITE_BUSY）: %v", err)
		}
	}

	// 用只读池核对数据完整性。
	rd, err := store.OpenReader(path, 4)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer rd.Close()

	var total int
	if err := rd.DB().QueryRow(`SELECT COUNT(*) FROM events`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != workers*perWorker {
		t.Errorf("事件总数应为 %d, 得到 %d", workers*perWorker, total)
	}
	for g := 0; g < workers; g++ {
		run := fmt.Sprintf("run-%d", g)
		var n, rev int
		if err := rd.DB().QueryRow(`SELECT COUNT(*) FROM events WHERE run_id=?`, run).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != perWorker {
			t.Errorf("%s 事件数应为 %d, 得到 %d", run, perWorker, n)
		}
		// Run 初始 revision=1（schema DEFAULT 1），每 100 条一次 durable +1，共 perWorker/100 次。
		if err := rd.DB().QueryRow(`SELECT revision FROM runs WHERE id=?`, run).Scan(&rev); err != nil {
			t.Fatal(err)
		}
		if want := 1 + perWorker/100; rev != want {
			t.Errorf("%s revision 应为 %d（1 初值 + durable 累加）, 得到 %d", run, want, rev)
		}
	}
}

// 关闭后再提交应返回错误而非 panic/死锁。
func TestWriterClosedRejects(t *testing.T) {
	w, _ := openTestWriter(t)
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	err := w.Bulk(context.Background(), func(tx *sql.Tx) error { return nil })
	if err == nil {
		t.Error("关闭后 Bulk 应返回错误")
	}
}
