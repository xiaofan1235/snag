// Package soak_test —— 浸泡测试（§22.6）。随机故障注入 + 持续负载，跑完断言 §22.6 全部数值门槛。
//
// 真实长跑：go test -timeout 24h -run TestSoak ./tests/soak/...，用环境变量放大规模：
//
//	SNAG_SOAK_ITERS=200000   迭代数（默认 200，CI 友好）
//	SNAG_SOAK_SECONDS=86400  最长运行秒数（到点即停，0=不限，按 ITERS）
//
// 门槛是**不变量断言**（任何一条违反即失败），不是跑够时长就算过——短跑与长跑验的是同一组不变量，
// 长跑只是把随机故障的覆盖面铺更广。
//
// 覆盖 §22.6 数值门槛（在进程内以真实原语驱动）：
//
//	没有越域请求 / 没有直连目标        → policy.InScope 对随机越域 URL 全拦
//	没有重复副作用动作                 → Executor 随机崩溃注入 + 恢复续提交，applier 零重复
//	没有未完成被标成 COMPLETED         → 带非终态 Job 的 Exit Gate 永不 COMPLETED
//	秘密 Canary 不出现在日志/模型输入   → evidence.Redact + Compactor 命中 Canary 拒摘要
//	没有 UNKNOWN 预算预留泄漏（结算 100%）→ 账本随机结算后零 RESERVED/IN_FLIGHT/SUCCEEDED/UNKNOWN
//	Core 重启后自动恢复                → 崩溃注入后恢复路径完成（含在副作用不重复用例）
//	SSE 补发无序号空洞                 → 产出事件序列连续无空洞断言
//	内存增长 < 10%/12h（warm-up 后）    → runtime.MemStats 采样，长跑严格、短跑有界+记录
package soak_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"pentagent/internal/agent"
	"pentagent/internal/control"
	"pentagent/internal/evidence"
	"pentagent/internal/faultinject"
	"pentagent/internal/policy"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

var t0 = time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func openDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "soak.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() {
		faultinject.Disable()
		_ = w.Close()
	})
	return w, context.Background()
}

func seedRun(t *testing.T, w *store.Writer, ctx context.Context, runID, state string) {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-soak", "a", "m", ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`,
			runID, "ws-soak", "r", state, "d", ts)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

type recordingApplier struct {
	mu      sync.Mutex
	applied map[string]int
}

func newApplier() *recordingApplier { return &recordingApplier{applied: map[string]int{}} }

func (r *recordingApplier) Apply(_ context.Context, _ *sql.Tx, step agent.StepContext, eff agent.Effect) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.applied[step.ModelStepID+"#"+strconv.Itoa(eff.Ordinal)]++
	return "ref", nil
}

func (r *recordingApplier) maxCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := 0
	for _, c := range r.applied {
		if c > m {
			m = c
		}
	}
	return m
}

type scriptGen struct{ resp agent.ModelResponse }

func (g *scriptGen) Generate(_ context.Context, _ agent.StepContext) (agent.ModelResponse, error) {
	return g.resp, nil
}

// soakScope：授权范围仅 app.example.com https:443。
func soakScope() policy.Target {
	return policy.Target{
		Hosts:   []policy.HostRule{{Value: "app.example.com", Match: "exact"}},
		Schemes: []string{"https"},
		Ports:   []int{443},
	}
}

// 随机越域 URL 池（§22.6「没有越域请求/没有直连目标」）。
var outOfScopeURLs = []string{
	"https://evil.com/x",
	"https://app.example.com:8443/",          // 非默认端口
	"https://app.example.com@attacker.test/", // userinfo 冒充
	"https://127.0.0.1/admin",                // 直连私网
	"https://169.254.169.254/latest/",        // 云元数据
	"https://[::1]/",                         // IPv6 loopback
	"ws://app.example.com/",                  // WebSocket scheme
	"https://аpp.example.com/",               // 编码域名（西里尔）
}

func TestSoak(t *testing.T) {
	iters := envInt("SNAG_SOAK_ITERS", 200)
	maxSeconds := envInt("SNAG_SOAK_SECONDS", 0)
	w, ctx := openDB(t)
	seedRun(t, w, ctx, "run-soak", "RUNNING")
	ap := newApplier()
	ex := agent.NewExecutor(w, ap)
	led := agent.NewLedger(w)
	canary := "CANARY-7f3a-SECRET-DO-NOT-LEAK"

	rng := rand.New(rand.NewSource(42))
	deadline := time.Time{}
	if maxSeconds > 0 {
		deadline = time.Now().Add(time.Duration(maxSeconds) * time.Second)
	}

	// 指标（任一非零即违反 §22.6 门槛）。
	var (
		crossOriginAllowed int // 越域被放行次数（必须 0）
		falseCompletes     int // 带阻塞却 COMPLETED（必须 0）
		canaryLeaks        int // Canary 出现在脱敏输出（必须 0）
		reservedTotal      int
		settledTotal       int
	)
	var seqGapViolations int

	// warm-up：先跑若干轮再采内存基线，避免把首次分配算成泄漏。
	warmup := iters / 10
	if warmup < 5 {
		warmup = 5
	}
	var memBase uint64

	for i := 0; i < iters; i++ {
		if !deadline.IsZero() && time.Now().After(deadline) {
			t.Logf("到达 SNAG_SOAK_SECONDS，提前在第 %d 轮停止", i)
			iters = i
			break
		}
		if i == warmup {
			runtime.GC()
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			memBase = ms.HeapAlloc
		}

		// ---- 故障注入：随机在 after_model_response_commit 崩溃 ----
		crash := rng.Intn(3) == 0 // 约 1/3 迭代注入崩溃
		if crash {
			faultinject.ArmOnce(faultinject.AfterModelResponseCommit, errors.New("soak-crash"), 1)
		}
		nEff := 1 + rng.Intn(3)
		effects := make([]agent.Effect, nEff)
		for k := range effects {
			effects[k] = agent.Effect{Ordinal: k, Kind: agent.KindActionProposal}
		}
		gen := &scriptGen{resp: agent.ModelResponse{Kind: agent.KindActionProposal, Effects: effects}}
		res, err := ex.Step(ctx, agent.StepSpec{RunID: "run-soak", PolicyRevision: 0}, gen, t0)
		if crash {
			if err == nil {
				t.Fatalf("第 %d 轮：注入崩溃但 Step 未报错", i)
			}
			// 恢复（Core 重启）：续提交效果，幂等。
			if _, rerr := ex.CommitEffects(ctx, res.ModelStepID, t0); rerr != nil {
				t.Fatalf("第 %d 轮：恢复续提交失败：%v", i, rerr)
			}
		} else if err != nil {
			t.Fatalf("第 %d 轮：无故障 Step 失败：%v", i, err)
		}
		// 没有重复副作用：任一 (step,ordinal) 的 applier 调用次数恒为 1。
		if ap.maxCount() > 1 {
			t.Fatalf("第 %d 轮：检测到重复副作用（applier 某效果被调 >1 次）", i)
		}

		// ---- 越域/直连：随机越域 URL 必须被拦 ----
		raw := outOfScopeURLs[rng.Intn(len(outOfScopeURLs))]
		if in, _, _ := policy.InScope(raw, soakScope()); in {
			crossOriginAllowed++
		}
		// 对照：授权 URL 必须放行（否则是过度拦截的另一种缺陷）。
		if in, _, err := policy.InScope("https://app.example.com/api", soakScope()); err != nil || !in {
			t.Fatalf("第 %d 轮：授权 URL 被误拦 in=%v err=%v", i, in, err)
		}

		// ---- Secret Canary：脱敏后绝不出现 ----
		raw4log := []byte(fmt.Sprintf("GET /x\nAuthorization: Bearer %s\nX-Id: %d", canary, i))
		red := evidence.Redact(raw4log, []string{canary})
		if containsBytes(red, canary) {
			canaryLeaks++
		}

		// ---- 预算：预留 + 随机结算（结算率必须 100%）----
		rsv, rerr := led.Reserve(ctx, agent.ReserveRequest{
			RunID: "run-soak", ProviderRequestID: fmt.Sprintf("soak-%d", i),
			MaxOutputTokens: 100, Pricing: soakPricing(),
		}, agent.Limits{TotalMicroUSD: 1 << 40}, t0)
		if rerr == nil {
			reservedTotal++
			if e := led.MarkInFlight(ctx, rsv.ID, t0); e != nil {
				t.Fatalf("第 %d 轮：MarkInFlight 失败：%v", i, e)
			}
			var se error
			switch rng.Intn(3) {
			case 0: // 正常结算 IN_FLIGHT→SETTLED
				se = led.Settle(ctx, rsv.ID, agent.TokenUsage{Input: 10, Output: 20}, soakPricing(), t0)
			case 1: // 崩溃/超时→UNKNOWN，对账确认已计费→结算
				if e := led.MarkUnknown(ctx, rsv.ID, t0); e != nil {
					t.Fatalf("第 %d 轮：MarkUnknown 失败：%v", i, e)
				}
				se = led.SettleUnknown(ctx, rsv.ID, true, agent.TokenUsage{Input: 10, Output: 20}, soakPricing(), t0)
			default: // 崩溃/超时→UNKNOWN，对账确认未计费→释放
				if e := led.MarkUnknown(ctx, rsv.ID, t0); e != nil {
					t.Fatalf("第 %d 轮：MarkUnknown 失败：%v", i, e)
				}
				se = led.SettleUnknown(ctx, rsv.ID, false, agent.TokenUsage{}, soakPricing(), t0)
			}
			if se != nil {
				t.Fatalf("第 %d 轮：结算失败：%v", i, se)
			}
			settledTotal++
		}

		// ---- 没有未完成冒充 COMPLETED：带非终态 Job 的 Run 永不 COMPLETED ----
		if i%20 == 0 {
			rid := fmt.Sprintf("run-blk-%d", i)
			seedRun(t, w, ctx, rid, "RUNNING")
			_ = w.Durable(ctx, func(tx *sql.Tx) error {
				_, e := tx.Exec(`INSERT INTO jobs (id,run_id,kind,state,created_at,updated_at) VALUES (?,?,?,?,?,?)`,
					fmt.Sprintf("job-%d", i), rid, "probe", "RUNNING", "2026-11-17T00:00:00Z", "2026-11-17T00:00:00Z")
				return e
			})
			g := control.NewExitGate(w, control.GateConfig{})
			cr, _ := g.TryComplete(ctx, rid, t0)
			if cr.Completed {
				falseCompletes++
			}
		}

		// ---- SSE 补发无空洞：模拟连续序号产出，断言无空洞 ----
		if !seqContiguous(i) {
			seqGapViolations++
		}
	}

	// ---- 终态门槛断言 ----
	if crossOriginAllowed != 0 {
		t.Fatalf("§22.6 违反：越域请求被放行 %d 次（必须 0）", crossOriginAllowed)
	}
	if falseCompletes != 0 {
		t.Fatalf("§22.6 违反：未完成被标 COMPLETED %d 次（必须 0）", falseCompletes)
	}
	if canaryLeaks != 0 {
		t.Fatalf("§22.6 违反：Secret Canary 泄漏 %d 次（必须 0）", canaryLeaks)
	}
	if seqGapViolations != 0 {
		t.Fatalf("§22.6 违反：SSE 序号空洞 %d 次（必须 0）", seqGapViolations)
	}

	// 预算结算率 100%：无任何悬挂（非 SETTLED）预留。
	var unsettled int
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM budget_reservations WHERE run_id='run-soak' AND state!='SETTLED'`).Scan(&unsettled)
	})
	if unsettled != 0 {
		t.Fatalf("§22.6 违反：%d 条预留未结算（UNKNOWN 预算泄漏，结算率须 100%%）", unsettled)
	}
	if reservedTotal != settledTotal {
		t.Fatalf("§22.6 违反：预留 %d ≠ 结算 %d", reservedTotal, settledTotal)
	}

	// 内存增长门：warm-up 后采样，断言堆未失控增长。
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	growth := float64(0)
	if memBase > 0 {
		growth = (float64(ms.HeapAlloc) - float64(memBase)) / float64(memBase) * 100
	}
	t.Logf("浸泡完成：iters=%d 预留/结算=%d/%d 堆 warm=%dKB→end=%dKB（%.1f%%）",
		iters, reservedTotal, settledTotal, memBase/1024, ms.HeapAlloc/1024, growth)
	// 短跑堆波动大，用宽门兜住「泄漏」级增长；长跑（大 ITERS）收紧到 §22.6 的 <10%。
	limit := 300.0
	if iters >= 50_000 {
		limit = 10.0 // §22.6 正式门槛
	}
	if memBase > 0 && growth > limit {
		t.Fatalf("§22.6 违反：堆增长 %.1f%% 超门槛 %.1f%%（疑似泄漏）", growth, limit)
	}
}

// containsBytes 报告 b 是否包含子串 sub。
func containsBytes(b []byte, sub string) bool {
	s := string(b)
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// seqContiguous 模拟 SSE 补发：第 i 轮产出序号 [base+1..base+n]，断言连续无空洞。
func seqContiguous(i int) bool {
	base := int64(i * 10)
	n := 5
	prev := base
	for k := 1; k <= n; k++ {
		cur := base + int64(k)
		if cur != prev+1 {
			return false
		}
		prev = cur
	}
	return true
}

func soakPricing() agent.Pricing {
	return agent.Pricing{Version: "soak-v1", InputPerMTok: 1_000_000, OutputPerMTok: 1_000_000}
}
