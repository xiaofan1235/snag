package agent

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

func openBudgetDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "budget.db"))
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
	const ts = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-b", "a", "m", ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`, runID, "ws-b", "r", "RUNNING", "d", ts)
		return e
	}); err != nil {
		t.Fatalf("seed run: %v", err)
	}
}

// unitPricing：1 input/output token = 1 micro_usd，便于断言整数成本。
func unitPricing() Pricing {
	return Pricing{Version: "test-v1", InputPerMTok: 1_000_000, OutputPerMTok: 1_000_000}
}

var t0 = time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

// 多维计费：五个维度分别计价后整数累加 /1e6（§31 A4）。
func TestPricingCostMultiDim(t *testing.T) {
	p := Pricing{
		Version: "v", InputPerMTok: 3_000_000, OutputPerMTok: 15_000_000,
		CacheWritePerMTok: 3_750_000, CacheReadPerMTok: 300_000, ReasoningPerMTok: 15_000_000,
	}
	u := TokenUsage{Input: 1000, Output: 500, CacheWrite: 200, CacheRead: 10000, Reasoning: 100}
	// 3000 + 7500 + 750 + 3000 + 1500 = 15750
	if got := p.Cost(u); got != 15750 {
		t.Fatalf("多维成本=%d，应 15750", got)
	}
	if !(Pricing{}).Estimated() {
		t.Fatal("空版本应判估算/未知")
	}
}

// 结算释放最坏预留与真实成本的差额：预留 worst=100，实际仅 10 → committed 落到 10。
func TestReserveSettleReleasesWorstCase(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRun(t, w, ctx, "run-1")
	l := NewLedger(w)
	lim := Limits{TotalMicroUSD: 1000}

	r, err := l.Reserve(ctx, ReserveRequest{RunID: "run-1", MaxOutputTokens: 100, Pricing: unitPricing()}, lim, t0)
	if err != nil {
		t.Fatalf("预留失败：%v", err)
	}
	if c, _ := l.Committed(ctx, "run-1"); c != 100 {
		t.Fatalf("预留后 committed=%d，应 100（最坏）", c)
	}
	if err := l.Settle(ctx, r.ID, TokenUsage{Output: 10}, unitPricing(), t0); err != nil {
		t.Fatalf("结算失败：%v", err)
	}
	if c, _ := l.Committed(ctx, "run-1"); c != 10 {
		t.Fatalf("结算后 committed=%d，应 10（真实）", c)
	}
}

// DoD：并发预留到上限即拒绝，总占用不突破上限（-race）。
func TestConcurrentReserveRespectsLimit(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRun(t, w, ctx, "run-1")
	l := NewLedger(w)
	lim := Limits{TotalMicroUSD: 1000} // 每次 worst=100 → 恰 10 个可过

	const n = 20
	var ok, exceeded int64
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := l.Reserve(ctx, ReserveRequest{
				RunID: "run-1", ProviderRequestID: "", MaxOutputTokens: 100, Pricing: unitPricing(),
			}, lim, t0)
			if err == nil {
				atomic.AddInt64(&ok, 1)
			} else if errors.Is(err, ErrBudgetExceeded) {
				atomic.AddInt64(&exceeded, 1)
			} else {
				t.Errorf("意外错误：%v", err)
			}
		}(i)
	}
	wg.Wait()

	if ok != 10 || exceeded != 10 {
		t.Fatalf("应 10 过 10 拒，实得 ok=%d exceeded=%d", ok, exceeded)
	}
	if c, _ := l.Committed(ctx, "run-1"); c != 1000 {
		t.Fatalf("committed=%d，应恰好等于上限 1000，不突破", c)
	}
}

// DoD：UNKNOWN 不自动释放——崩溃态按最坏保留，占用不消失；对账 SettleUnknown 后才释放（§22.6）。
func TestUnknownNotAutoReleased(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRun(t, w, ctx, "run-1")
	l := NewLedger(w)
	lim := Limits{TotalMicroUSD: 150} // 仅容一个 worst=100

	a, err := l.Reserve(ctx, ReserveRequest{RunID: "run-1", MaxOutputTokens: 100, Pricing: unitPricing()}, lim, t0)
	if err != nil {
		t.Fatalf("预留 A 失败：%v", err)
	}
	if err := l.MarkInFlight(ctx, a.ID, t0); err != nil {
		t.Fatal(err)
	}
	if err := l.MarkUnknown(ctx, a.ID, t0); err != nil {
		t.Fatal(err)
	}
	// UNKNOWN 仍占 100：再预留 100 应被拒（100+100>150）。
	if _, err := l.Reserve(ctx, ReserveRequest{RunID: "run-1", MaxOutputTokens: 100, Pricing: unitPricing()}, lim, t0); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("UNKNOWN 未释放时应拒新预留，实得 %v", err)
	}
	// 对账确认未计费 → 释放。
	if err := l.SettleUnknown(ctx, a.ID, false, TokenUsage{}, unitPricing(), t0); err != nil {
		t.Fatalf("SettleUnknown 失败：%v", err)
	}
	if c, _ := l.Committed(ctx, "run-1"); c != 0 {
		t.Fatalf("释放后 committed=%d，应 0", c)
	}
	// 现在再预留应成功。
	if _, err := l.Reserve(ctx, ReserveRequest{RunID: "run-1", MaxOutputTokens: 100, Pricing: unitPricing()}, lim, t0); err != nil {
		t.Fatalf("释放后应可预留，实得 %v", err)
	}
}

// DoD：finalization_reserve 受保护——普通预留耗尽 (Total-Reserve) 后再普通预留被拒，
// 但收尾预留仍可用（耗尽后仍能产 PARTIAL 报告，§7.5）。
func TestFinalizationReserveProtected(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRun(t, w, ctx, "run-1")
	l := NewLedger(w)
	lim := Limits{TotalMicroUSD: 1000, FinalizationReserve: 200} // 普通可用 800

	for i := 0; i < 8; i++ { // 8×100 = 800，占满普通额度
		if _, err := l.Reserve(ctx, ReserveRequest{RunID: "run-1", MaxOutputTokens: 100, Pricing: unitPricing()}, lim, t0); err != nil {
			t.Fatalf("第 %d 个普通预留应成功：%v", i+1, err)
		}
	}
	// 第 9 个普通预留吃进收尾额度 → 拒。
	if _, err := l.Reserve(ctx, ReserveRequest{RunID: "run-1", MaxOutputTokens: 100, Pricing: unitPricing()}, lim, t0); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("普通预留不得吃进 finalization_reserve，实得 %v", err)
	}
	// 收尾预留仍可（800+100 ≤ 1000）。
	if _, err := l.Reserve(ctx, ReserveRequest{RunID: "run-1", MaxOutputTokens: 100, Pricing: unitPricing(), Finalization: true}, lim, t0); err != nil {
		t.Fatalf("收尾额度应仍可用于 PARTIAL，实得 %v", err)
	}
}

// 幂等：同一 provider_request_id 不重复预留（防崩溃重试重复扣费）。
func TestReserveIdempotentByPRID(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRun(t, w, ctx, "run-1")
	l := NewLedger(w)
	lim := Limits{TotalMicroUSD: 1000}
	req := ReserveRequest{RunID: "run-1", ProviderRequestID: "prid-1", MaxOutputTokens: 100, Pricing: unitPricing()}

	r1, err := l.Reserve(ctx, req, lim, t0)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := l.Reserve(ctx, req, lim, t0)
	if err != nil {
		t.Fatal(err)
	}
	if r1.ID != r2.ID {
		t.Fatalf("同一幂等键应返回同一预留：%s vs %s", r1.ID, r2.ID)
	}
	if c, _ := l.Committed(ctx, "run-1"); c != 100 {
		t.Fatalf("幂等预留只应计一次，committed=%d 应 100", c)
	}
}

// DoD：结算率 100%——所有预留结算后无未结算残留（§22.6）。
func TestSettlementRateComplete(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRun(t, w, ctx, "run-1")
	l := NewLedger(w)
	lim := Limits{TotalMicroUSD: 1000}

	var ids []string
	for i := 0; i < 5; i++ {
		r, err := l.Reserve(ctx, ReserveRequest{RunID: "run-1", MaxOutputTokens: 50, Pricing: unitPricing()}, lim, t0)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, r.ID)
	}
	if un, _ := l.ListUnsettled(ctx, "run-1"); len(un) != 5 {
		t.Fatalf("结算前未结算数=%d，应 5", len(un))
	}
	for _, id := range ids {
		if err := l.Settle(ctx, id, TokenUsage{Output: 20}, unitPricing(), t0); err != nil {
			t.Fatal(err)
		}
	}
	if un, _ := l.ListUnsettled(ctx, "run-1"); len(un) != 0 {
		t.Fatalf("结算后应无残留，实得 %d（结算率未达 100%%）", len(un))
	}
}

// 非法迁移拒绝（fail-closed）：已 SETTLED 不能再结算/转 UNKNOWN。
func TestInvalidTransitionRejected(t *testing.T) {
	w, ctx := openBudgetDB(t)
	seedRun(t, w, ctx, "run-1")
	l := NewLedger(w)
	lim := Limits{TotalMicroUSD: 1000}

	r, err := l.Reserve(ctx, ReserveRequest{RunID: "run-1", MaxOutputTokens: 100, Pricing: unitPricing()}, lim, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(ctx, r.ID, TokenUsage{Output: 10}, unitPricing(), t0); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(ctx, r.ID, TokenUsage{Output: 10}, unitPricing(), t0); err == nil {
		t.Fatal("已结算不应允许二次结算")
	}
	if err := l.MarkUnknown(ctx, r.ID, t0); err == nil {
		t.Fatal("已结算不应允许转 UNKNOWN")
	}
}
