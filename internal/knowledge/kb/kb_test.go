package kb

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

var t0 = time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

func openDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "kb.db"))
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

func seedWorkspaces(t *testing.T, w *store.Writer, ctx context.Context, ids ...string) {
	t.Helper()
	const tss = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, id, "a", "m", tss); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// DoD：Canary 命中即拦截入库（§34.6）。
func TestIngestCanaryRejected(t *testing.T) {
	w, ctx := openDB(t)
	seedWorkspaces(t, w, ctx, "ws-1")
	s := NewStore(w, []string{"CANARY-7f3a9b"})
	if _, _, err := s.Ingest(ctx, "ws-1", Doc{Title: "x", Content: "泄露 CANARY-7f3a9b 的内容"}, t0); !errors.Is(err, ErrSecretCanary) {
		t.Fatalf("命中 Canary 应拒入库，实得 %v", err)
	}
}

func TestIngestDedup(t *testing.T) {
	w, ctx := openDB(t)
	seedWorkspaces(t, w, ctx, "ws-1")
	s := NewStore(w, nil)
	d := Doc{Title: "IDOR", Content: "越权测试方法", AppliesTo: []string{"idor"}}
	id1, dup1, err := s.Ingest(ctx, "ws-1", d, t0)
	if err != nil || dup1 {
		t.Fatalf("首次入库不应 dedup：%v", err)
	}
	id2, dup2, _ := s.Ingest(ctx, "ws-1", d, t0)
	if !dup2 || id2 != id1 {
		t.Fatalf("同内容应去重返回原 id，实得 dup=%v id=%s", dup2, id2)
	}
}

// DoD：检索按 applies_to 命中；两 Workspace 不互查（隔离）。
func TestRetrieveAppliesToAndIsolation(t *testing.T) {
	w, ctx := openDB(t)
	seedWorkspaces(t, w, ctx, "ws-1", "ws-2")
	s := NewStore(w, nil)
	_, _, _ = s.Ingest(ctx, "ws-1", Doc{Title: "IDOR 越权方法", Content: "遍历对象 id 测越权", AppliesTo: []string{"idor"}}, t0)
	_, _, _ = s.Ingest(ctx, "ws-2", Doc{Title: "IDOR 另一个 workspace", Content: "idor", AppliesTo: []string{"idor"}}, t0)

	// ws-1 查 idor → 只命中 ws-1 的文档。
	hits, _ := s.Retrieve(ctx, "ws-1", "IDOR 越权", "idor", 5)
	if len(hits) != 1 || hits[0].Title != "IDOR 越权方法" {
		t.Fatalf("应只命中 ws-1 的 idor 文档，实得 %+v", hits)
	}
	// applies_to 不符（sqli）→ 粗筛过滤掉。
	if hits, _ := s.Retrieve(ctx, "ws-1", "IDOR", "sqli", 5); len(hits) != 0 {
		t.Fatalf("applies_to=sqli 应无命中，实得 %d", len(hits))
	}
	// 跨 workspace：ws-3 查 → 0（隔离）。
	if hits, _ := s.Retrieve(ctx, "ws-3", "idor", "", 5); len(hits) != 0 {
		t.Fatalf("两 Workspace 不互查，实得 %d", len(hits))
	}
}

// DoD：Skill 声明越权能力拒导入；干净 Skill 按 applies_to 加载（§40）。
func TestImportSkillOverPrivilege(t *testing.T) {
	w, ctx := openDB(t)
	seedWorkspaces(t, w, ctx, "ws-1")
	s := NewStore(w, nil)

	if _, err := s.ImportSkill(ctx, "ws-1", Skill{Name: "evil", DeclaredPerms: []string{"modify_scope"}}, t0); !errors.Is(err, ErrOverPrivilegedSkill) {
		t.Fatalf("声明越权能力应拒，实得 %v", err)
	}
	if _, err := s.ImportSkill(ctx, "ws-1", Skill{Name: "idor-skill", AppliesTo: []string{"idor"}, DeclaredPerms: []string{"http.request"}}, t0); err != nil {
		t.Fatalf("干净 Skill 应可导入：%v", err)
	}
	sks, _ := s.LoadSkills(ctx, "ws-1", "idor")
	if len(sks) != 1 || sks[0].Name != "idor-skill" {
		t.Fatalf("应按 applies_to 加载到 idor-skill，实得 %+v", sks)
	}
	// 不匹配 applies_to → 不加载。
	if sks, _ := s.LoadSkills(ctx, "ws-1", "xss"); len(sks) != 0 {
		t.Fatalf("applies_to=xss 不应加载，实得 %d", len(sks))
	}
}

type fakeSearcher struct{ rs []WebResult }

func (f fakeSearcher) Search(context.Context, string) ([]WebResult, error) { return f.rs, nil }

// 联网结果统一标不可信参考（§36/§46.3）；卡住即检索触发（§36.2）。
func TestWebSearchAndStuckTrigger(t *testing.T) {
	rs, err := WebSearch(context.Background(), fakeSearcher{rs: []WebResult{{Title: "ref", URL: "http://x"}}}, "q")
	if err != nil || len(rs) != 1 || !rs[0].Untrusted || !rs[0].Reference {
		t.Fatalf("联网结果应标不可信参考，实得 %+v", rs)
	}
	tr := StuckTrigger{Threshold: 3}
	if tr.ShouldSearch(2, false) {
		t.Fatal("未到阈值不应触发")
	}
	if !tr.ShouldSearch(3, false) {
		t.Fatal("到阈值且未搜过应触发")
	}
	if tr.ShouldSearch(5, true) {
		t.Fatal("已搜过不应重复触发")
	}
}
