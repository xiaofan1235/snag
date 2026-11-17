package experiments

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
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "exp.db"))
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
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-e", "a", "m", tss); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`, "run-1", "ws-e", "r", "RUNNING", "d", tss)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// JSONPath-lite 结构化提取（§44.1，确定性）。
func TestExtractJSONPath(t *testing.T) {
	body := []byte(`{"data":{"id":"order-99","items":[{"sku":"A"},{"sku":"B"}]}}`)
	if v, err := Extract(body, "body.$.data.id"); err != nil || v != "order-99" {
		t.Fatalf("提取 data.id 应 order-99，实得 %q err=%v", v, err)
	}
	if v, _ := Extract(body, "$.data.items.1.sku"); v != "B" {
		t.Fatalf("数组下标提取应 B，实得 %q", v)
	}
	if _, err := Extract(body, "$.data.missing"); err == nil {
		t.Fatal("缺字段应报错")
	}
}

// DoD：序列实验步骤间传值（结构化提取，不靠模型记忆）。
func TestRunSequencePassesValues(t *testing.T) {
	spec := SequenceSpec{Steps: []Step{
		{ID: "s1", Identity: "user_a", RequestTemplate: "GET /api/orders", Extract: map[string]string{"order_id": "body.$.data.id"}},
		{ID: "s2", Identity: "user_a", RequestTemplate: "POST /api/orders/{order_id}/pay"},
	}}
	var s2rendered string
	exec := func(_ context.Context, stepID, _, rendered string) ([]byte, error) {
		switch stepID {
		case "s1":
			return []byte(`{"data":{"id":"order-99"}}`), nil
		case "s2":
			s2rendered = rendered
			return []byte(`{"ok":true}`), nil
		}
		return []byte(`{}`), nil
	}
	results, err := (&Store{}).RunSequence(context.Background(), spec, exec)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Extracted["order_id"] != "order-99" {
		t.Fatalf("s1 应提取 order_id=order-99，实得 %+v", results[0].Extracted)
	}
	if s2rendered != "POST /api/orders/order-99/pay" {
		t.Fatalf("s2 应代入上一步 order_id，实得 %q", s2rendered)
	}
}

func TestCreateSequencePersists(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	id, err := s.CreateSequence(ctx, "run-1", SequenceSpec{
		Assertion: "状态机绕过", Falsifier: "s2 被拒",
		Steps: []Step{{ID: "s1", RequestTemplate: "GET /x"}},
	}, t0)
	if err != nil || id == "" {
		t.Fatalf("持久化序列实验失败：%v", err)
	}
}

// 竞态并发度超上限 → 拒。
func TestRaceConcurrencyCap(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)
	if _, err := s.CreateRace(ctx, "run-1", RaceSpec{Concurrency: DefaultConcurrencyCap + 1}, t0); !errors.Is(err, ErrConcurrencyExceeded) {
		t.Fatalf("超并发上限应拒，实得 %v", err)
	}
}

// DoD：未审批竞态不跑；审批后并发，但每请求仍过执法——越权并发仍被拒（只豁免串行，不豁免执法，§44.2）。
func TestRaceApprovalAndPerRequestEnforcement(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	id, err := s.CreateRace(ctx, "run-1", RaceSpec{EndpointID: "ep-1", IdentityID: "user_a", Concurrency: 10, MaxTotal: 10}, t0)
	if err != nil {
		t.Fatal(err)
	}
	// 未审批 → 不跑。
	if _, err := s.RunRace(ctx, id, func(int) (bool, string) { return true, "" }, func(int) bool { return true }, t0); !errors.Is(err, ErrNeedsApproval) {
		t.Fatalf("未审批应 ErrNeedsApproval，实得 %v", err)
	}
	// 审批后跑：逐请求执法——偶数放行、奇数被拒（模拟越权/越域仍被 Proxy 拒）。
	if err := s.Approve(ctx, id, t0); err != nil {
		t.Fatal(err)
	}
	check := func(i int) (bool, string) { return i%2 == 0, "scope/fencing" }
	fire := func(i int) bool { return true } // 放行的都「成功」
	out, err := s.RunRace(ctx, id, check, fire, t0)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Approved || out.Dispatched != 5 || out.Denied != 5 {
		t.Fatalf("10 并发应 5 过执法 5 被拒（执法不豁免），实得 %+v", out)
	}
	if out.SuccessCount != 5 {
		t.Fatalf("成功计数应 5（仅通过执法的），实得 %d", out.SuccessCount)
	}
}
