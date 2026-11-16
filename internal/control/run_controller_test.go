package control_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"pentagent/internal/control"
	"pentagent/internal/domain"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// run_controller_test.go —— Day 07 §7：控制器把迁移落到 durable 库，revision 单调、非法被拒。
// 命令：go test -tags sqlite_modernc ./internal/control/...（无驱动自动 SKIP）。

func TestControllerApplyLifecycle(t *testing.T) {
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "c.db")
	w, err := store.OpenWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	ctx := context.Background()

	// 种子：workspace + run（DRAFT, revision 1）。
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			"ws-1", "acme", "mpk-1", "2026-11-16T00:00:00Z"); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
			VALUES (?,?,?,?,?,?)`, "run-1", "ws-1", "r", "DRAFT", "digest", "2026-11-16T00:00:00Z")
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	c := control.NewController(w)
	rd, err := store.OpenReader(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rd.Close() })

	readState := func() (string, int64) {
		var s string
		var rev int64
		if e := rd.DB().QueryRow(`SELECT state, revision FROM runs WHERE id=?`, "run-1").Scan(&s, &rev); e != nil {
			t.Fatal(e)
		}
		return s, rev
	}

	// 合法生命周期：DRAFT→…→COMPLETED，每步 revision+1。
	seq := []struct {
		ev   control.Event
		want domain.RunState
	}{
		{control.EventPreflight, domain.RunPreflight},
		{control.EventStart, domain.RunRunning},
		{control.EventPause, domain.RunPausing},
		{control.EventPauseDrained, domain.RunPaused},
		{control.EventResume, domain.RunResuming},
		{control.EventResumeReady, domain.RunRunning},
		{control.EventExit, domain.RunExiting},
		{control.EventComplete, domain.RunCompleted},
	}
	prevRev := int64(1)
	for _, step := range seq {
		got, err := c.Apply(ctx, "run-1", step.ev)
		if err != nil {
			t.Fatalf("Apply(%s) 失败: %v", step.ev, err)
		}
		if got != step.want {
			t.Errorf("Apply(%s) 返回 %s, 期望 %s", step.ev, got, step.want)
		}
		gotState, gotRev := readState()
		if gotState != string(step.want) {
			t.Errorf("库中状态 %s, 期望 %s", gotState, step.want)
		}
		if gotRev != prevRev+1 {
			t.Errorf("revision 应 %d（单调+1）, 得到 %d", prevRev+1, gotRev)
		}
		prevRev = gotRev
	}

	// 终态后再施加任何事件：非法，返回 ErrInvalid，库不变。
	_, err = c.Apply(ctx, "run-1", control.EventStart)
	if !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("终态后 Apply 应返回 ErrInvalid, 得到 %v", err)
	}
	if s, rev := readState(); s != "COMPLETED" || rev != prevRev {
		t.Errorf("非法事件不应改库, 得到 %s/%d", s, rev)
	}

	// 不存在的 Run → ErrNotFound。
	if _, err := c.Apply(ctx, "run-x", control.EventPreflight); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("不存在的 Run 应 ErrNotFound, 得到 %v", err)
	}
}

// Q-15 ③：EXITING 门禁发现新工作时经 reopen 回 RUNNING（而非误终结）。
func TestControllerExitingReopen(t *testing.T) {
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动")
	}
	path := filepath.Join(t.TempDir(), "c.db")
	w, _ := store.OpenWriter(path)
	_ = w.Migrate()
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	ctx := context.Background()
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		tx.Exec(`INSERT INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			"ws-1", "a", "m", "2026-11-16T00:00:00Z")
		_, e := tx.Exec(`INSERT INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
			VALUES (?,?,?,?,?,?)`, "run-1", "ws-1", "r", "RUNNING", "d", "2026-11-16T00:00:00Z")
		return e
	})
	c := control.NewController(w)
	if got, err := c.Apply(ctx, "run-1", control.EventExit); err != nil || got != domain.RunExiting {
		t.Fatalf("exit 应到 EXITING: %s %v", got, err)
	}
	// 门禁发现新工作 → reopen 回 RUNNING。
	if got, err := c.Apply(ctx, "run-1", control.EventReopen); err != nil || got != domain.RunRunning {
		t.Fatalf("reopen 应回 RUNNING: %s %v", got, err)
	}
}

// 非法迁移中途：RUNNING 收到 preflight 应被拒且不改库。
func TestControllerRejectsIllegal(t *testing.T) {
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动")
	}
	path := filepath.Join(t.TempDir(), "c.db")
	w, _ := store.OpenWriter(path)
	_ = w.Migrate()
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	ctx := context.Background()
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		tx.Exec(`INSERT INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			"ws-1", "a", "m", "2026-11-16T00:00:00Z")
		_, e := tx.Exec(`INSERT INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
			VALUES (?,?,?,?,?,?)`, "run-1", "ws-1", "r", "RUNNING", "d", "2026-11-16T00:00:00Z")
		return e
	})
	c := control.NewController(w)
	if _, err := c.Apply(ctx, "run-1", control.EventPreflight); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("RUNNING 收到 preflight 应 ErrInvalid, 得到 %v", err)
	}
}
