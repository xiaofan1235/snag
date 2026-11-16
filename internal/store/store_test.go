package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"pentagent/internal/domain"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// store_test.go —— Day 02 §7 验收测试。
// 命令：go test ./internal/store/...                （无驱动：自动 SKIP）
//      go test -tags sqlite_modernc ./internal/store/...  （接入 modernc 驱动后真跑，需先 go get）
//
// 需要真实 SQLite 的用例在驱动未接入时 t.Skip，以便无网环境 go test ./... 仍全绿。

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：go get modernc.org/sqlite 后用 -tags sqlite_modernc 运行")
	}
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st := store.New(db)
	if err := st.MigrateUp(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

// 建库 + 迁移后，13 张核心表 + schema_migrations 应存在。
func TestMigrateCreatesTables(t *testing.T) {
	st := openTestStore(t)
	want := []string{
		"workspaces", "runs", "policy_revisions", "agent_sessions", "jobs", "attempts",
		"actions", "evidence", "hypotheses", "findings", "finding_evidence",
		"coverage_cells", "events", "schema_migrations",
	}
	for _, tbl := range want {
		var name string
		err := st.DB().QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("表 %s 不存在: %v", tbl, err)
		}
	}
	// 迁移幂等：再跑一次不报错、不重复。
	if err := st.MigrateUp(); err != nil {
		t.Fatalf("重复迁移应幂等: %v", err)
	}
}

// 插入 Run、读取、乐观锁条件更新；旧 revision 再更新须冲突。
func TestRunOptimisticLock(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	ws := domain.NewWorkspace("acme", "mpk-"+string(domain.NewID()))
	if err := st.InsertWorkspace(ctx, ws); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	r := domain.NewRun(ws.ID, "run-a", "tester", "digest-abc")
	if err := st.InsertRun(ctx, r); err != nil {
		t.Fatalf("insert run: %v", err)
	}

	got, err := st.GetRun(ctx, r.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.State != domain.RunDraft || got.Revision != 1 || got.WorkspaceID != ws.ID {
		t.Fatalf("读回不符: state=%s rev=%d ws=%s", got.State, got.Revision, got.WorkspaceID)
	}
	if got.AuthorizationScopeDigest != "digest-abc" {
		t.Errorf("scope digest 丢失: %q", got.AuthorizationScopeDigest)
	}

	// 正确 revision 更新成功，revision 自增到 2。
	newRev, err := st.UpdateRunState(ctx, r.ID, 1, domain.RunPreflight)
	if err != nil {
		t.Fatalf("条件更新应成功: %v", err)
	}
	if newRev != 2 {
		t.Errorf("新 revision 应为 2, 得到 %d", newRev)
	}
	got2, _ := st.GetRun(ctx, r.ID)
	if got2.State != domain.RunPreflight || got2.Revision != 2 {
		t.Errorf("更新后应为 PREFLIGHT/2, 得到 %s/%d", got2.State, got2.Revision)
	}

	// 用已过期的 revision=1 再更新：必须 0 行受影响 → ErrConflict。
	_, err = st.UpdateRunState(ctx, r.ID, 1, domain.RunRunning)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("旧 revision 更新应返回 ErrConflict, 得到 %v", err)
	}
}

// 未找到应返回 ErrNotFound。
func TestGetRunNotFound(t *testing.T) {
	st := openTestStore(t)
	_, err := st.GetRun(context.Background(), domain.NewID())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("应 ErrNotFound, 得到 %v", err)
	}
}

// 插入 Job、读取，初值与 domain.NewJob 一致。
func TestJobInsertGet(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	ws := domain.NewWorkspace("acme", "mpk-"+string(domain.NewID()))
	_ = st.InsertWorkspace(ctx, ws)
	r := domain.NewRun(ws.ID, "run-a", "tester", "d")
	_ = st.InsertRun(ctx, r)

	j := domain.NewJob(r.ID, "recon", "枚举子域")
	if err := st.InsertJob(ctx, j); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	got, err := st.GetJob(ctx, j.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if got.State != domain.JobPending || got.Generation != 1 || got.Revision != 1 {
		t.Errorf("Job 初值应 PENDING/1/1, 得到 %s/%d/%d", got.State, got.Generation, got.Revision)
	}
	if got.RunID != r.ID || got.Kind != "recon" || got.Title != "枚举子域" {
		t.Errorf("Job 字段读回不符: run=%s kind=%s title=%s", got.RunID, got.Kind, got.Title)
	}
	if !got.ParentJobID.IsZero() {
		t.Errorf("无父 Job 应为空, 得到 %s", got.ParentJobID)
	}
}
