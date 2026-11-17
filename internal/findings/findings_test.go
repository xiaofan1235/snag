package findings

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/knowledge/surface"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

var t0 = time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

// DoD：端点按业务语义聚类正确（§12.2，auth 先于 user、payment 先于 order）。
func TestClassifyModule(t *testing.T) {
	cases := map[string]string{
		"/api/login":          ModAuth,
		"/oauth/token":        ModAuth,
		"/api/users/{id}":     ModUser,
		"/api/payment/charge": ModPayment,
		"/api/orders":         ModOrder,
		"/api/files/upload":   ModFile,
		"/admin/dashboard":    ModAdmin,
		"/api/messages":       ModMessage,
		"/api/reports/export": ModReport,
		"/api/widgets":        ModOther,
	}
	for in, want := range cases {
		if got := ClassifyModule(in); got != want {
			t.Errorf("ClassifyModule(%q)=%q，应 %q", in, got, want)
		}
	}
}

func openDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "findings.db"))
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
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-f", "a", "m", tss); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`, "run-1", "ws-f", "r", "RUNNING", "d", tss)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// 端点聚类并持久化模块。
func TestClusterEndpointsPersists(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	eps := []surface.Endpoint{
		{URLPattern: "/api/login"}, {URLPattern: "/api/users/{id}"}, {URLPattern: "/api/orders"},
	}
	res, err := s.ClusterEndpoints(ctx, "run-1", eps, t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Modules[ModAuth]) != 1 || len(res.Modules[ModUser]) != 1 || len(res.Modules[ModOrder]) != 1 {
		t.Fatalf("聚类结果不符：%+v", res.Modules)
	}
	if _, err := s.ModuleID(ctx, "run-1", ModAuth); err != nil {
		t.Fatalf("auth 模块应已持久化：%v", err)
	}
}

// DoD（§12.3 核心验收）：无 EvidenceRef 的技术栈指纹不生成；有证据才入库。
func TestFingerprintRequiresEvidence(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	// 无证据 → 拒（不把猜测升级成事实）。
	if _, err := s.AddFingerprint(ctx, "run-1", Fingerprint{Category: FpFramework, Value: "Spring Boot"}, t0); !errors.Is(err, ErrNoEvidence) {
		t.Fatalf("无 EvidenceRef 应拒（ErrNoEvidence），实得 %v", err)
	}
	// 有证据 → 入库。
	id, err := s.AddFingerprint(ctx, "run-1", Fingerprint{
		Category: FpFramework, Value: "Spring Boot", EvidenceRef: "ev-42", Confidence: "high",
	}, t0)
	if err != nil || id == "" {
		t.Fatalf("有证据应入库：%v", err)
	}
	fps, _ := s.Fingerprints(ctx, "run-1")
	if len(fps) != 1 || fps[0].EvidenceRef != "ev-42" {
		t.Fatalf("指纹应带 EvidenceRef：%+v", fps)
	}
}

// DoD：Discovery 状态机流转正确；非法迁移拒绝。
func TestDiscoveryStateMachine(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	id, err := s.AddDiscovery(ctx, "run-1", "new_endpoint", true, `{"url":"/api/secret"}`, t0)
	if err != nil {
		t.Fatal(err)
	}
	// 非法：OPEN→PROMOTED 直接跳级应拒。
	if err := s.TransitionDiscovery(ctx, id, DiscPromoted, t0); !errors.Is(err, ErrBadDiscoveryTransition) {
		t.Fatalf("OPEN→PROMOTED 应拒，实得 %v", err)
	}
	// 合法链：OPEN→FOLLOWING→FOLLOWED→PROMOTED。
	for _, to := range []string{DiscFollowing, DiscFollowed, DiscPromoted} {
		if err := s.TransitionDiscovery(ctx, id, to, t0); err != nil {
			t.Fatalf("迁移到 %s 失败：%v", to, err)
		}
	}
	if st, _ := s.DiscoveryState(ctx, id); st != DiscPromoted {
		t.Fatalf("终态应 PROMOTED，实得 %s", st)
	}
	// PROMOTED 为终态：再迁移应拒。
	if err := s.TransitionDiscovery(ctx, id, DiscDismissed, t0); !errors.Is(err, ErrBadDiscoveryTransition) {
		t.Fatalf("终态 PROMOTED 不应再迁移，实得 %v", err)
	}
}

// DoD：退出前所有高价值 Discovery 必须有处置（§12.4）。
func TestHighValueDispositionGate(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	hv, _ := s.AddDiscovery(ctx, "run-1", "identity_diff", true, "", t0) // 高价值
	_, _ = s.AddDiscovery(ctx, "run-1", "waf_pattern", false, "", t0)    // 非高价值（不卡退出）

	// 高价值线索未处置 → 不可退出。
	if ready, _ := s.ReadyToExit(ctx, "run-1"); ready {
		t.Fatal("高价值线索未处置时不应可退出")
	}
	if ids, _ := s.HighValueUndisposed(ctx, "run-1"); len(ids) != 1 {
		t.Fatalf("应有 1 条未处置高价值线索，实得 %d", len(ids))
	}
	// 处置（DISMISSED）后 → 可退出。
	if err := s.TransitionDiscovery(ctx, hv, DiscDismissed, t0); err != nil {
		t.Fatal(err)
	}
	if ready, _ := s.ReadyToExit(ctx, "run-1"); !ready {
		t.Fatal("高价值线索已处置后应可退出")
	}
}
