package agent

import (
	"context"
	"database/sql"
	"testing"

	"pentagent/internal/store"
)

type mockSummarizer struct {
	out    string
	tokens int
	err    error
}

func (m mockSummarizer) Summarize(_ context.Context, _ []CtxBlock) (string, int, error) {
	return m.out, m.tokens, m.err
}

// seedSession 造 run + agent_session + FULL 修订(rev 1) + 其块。
func seedSession(t *testing.T, w *store.Writer, ctx context.Context, sessionID string, blocks []CtxBlock) string {
	t.Helper()
	seedRun(t, w, ctx, "run-c")
	const tss = "2026-11-17T00:00:00Z"
	revID := "rev-full-1"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT INTO agent_sessions (id,run_id,role,state,created_at,last_active_at) VALUES (?,?,?,?,?,?)`,
			sessionID, "run-c", "executor", "RUNNING", tss, tss); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO agent_context_revisions (id,agent_session_id,revision,parent_revision,kind,token_estimate,created_at) VALUES (?,?,?,?,?,?,?)`,
			revID, sessionID, 1, 0, RevFull, sumTokens(blocks), tss); e != nil {
			return e
		}
		for i, b := range blocks {
			refs := ""
			if len(b.EvidenceRefs) > 0 {
				refs = `["` + b.EvidenceRefs[0] + `"]`
			}
			aged := 0
			if b.Aged {
				aged = 1
			}
			if _, e := tx.Exec(`INSERT INTO agent_context_blocks (id,context_revision_id,ordinal,kind,source,content,tokens,aged,evidence_refs,created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
				"blk-"+itoa(i), revID, i, b.Kind, b.Source, b.Content, blkTokens(b), aged, nz(refs), tss); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return revID
}

func itoa(i int) string { return string(rune('0' + i)) }

func revCount(t *testing.T, w *store.Writer, ctx context.Context, sessionID string) int {
	t.Helper()
	var n int
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_context_revisions WHERE agent_session_id=?`, sessionID).Scan(&n)
	})
	return n
}

func blockCount(t *testing.T, w *store.Writer, ctx context.Context, revID string) int {
	t.Helper()
	var n int
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_context_blocks WHERE context_revision_id=?`, revID).Scan(&n)
	})
	return n
}

// DoD：溢出 → 阶段一确定性丢弃即达预算，生成新 COMPACTED 修订；原 FULL 修订一行不改（§7.6）。
func TestCompactStage1DropsAgedKeepsOriginal(t *testing.T) {
	w, ctx := openBudgetDB(t)
	blocks := []CtxBlock{
		{Kind: BlkEvent, Content: "e1", Tokens: 100, Aged: true},
		{Kind: BlkEvent, Content: "e2", Tokens: 100, Aged: true},
		{Kind: BlkToolResult, Content: "t1", Tokens: 100, Aged: true},
		{Kind: BlkQuestion, Content: "未决：测哪个端点", Tokens: 50},
		{Kind: BlkEvidenceRef, Content: "ev 引用", Tokens: 50, EvidenceRefs: []string{"ev-1"}},
	}
	fullRevID := seedSession(t, w, ctx, "sess-1", blocks) // before=400
	c := NewCompactor(w, mockSummarizer{}, nil, 2)

	res, err := c.Compact(ctx, "sess-1", 150, "pre_step_budget", t0) // 丢 3 老化块→100≤150
	if err != nil {
		t.Fatal(err)
	}
	if !res.Succeeded() || res.Record.Stage != 1 || res.NewRevision != 2 {
		t.Fatalf("应阶段一成功、新修订 2，实得 %+v", res)
	}
	// 原 FULL 修订块数不变（5）、仍 FULL。
	if n := blockCount(t, w, ctx, fullRevID); n != 5 {
		t.Fatalf("原修订被改动：块数=%d 应 5", n)
	}
	if revCount(t, w, ctx, "sess-1") != 2 {
		t.Fatal("应有 2 个修订（FULL + COMPACTED）")
	}
	// 新修订保留受保护块（问题 + 证据引用）。
	if n := blockCount(t, w, ctx, res.NewRevisionID); n != 2 {
		t.Fatalf("COMPACTED 修订块数=%d，应 2（问题+证据）", n)
	}
	// 旧 Evidence 引用压缩后仍可解析。
	refs, _ := c.EvidenceRefsOf(ctx, res.NewRevisionID)
	if len(refs) != 1 || refs[0] != "ev-1" {
		t.Fatalf("Evidence 引用应保留 ev-1，实得 %v", refs)
	}
}

// 阶段一不够 → 阶段二 LLM 摘要成功，保 exploit 连续性，after<before。
func TestCompactStage2Summarizes(t *testing.T) {
	w, ctx := openBudgetDB(t)
	blocks := []CtxBlock{
		{Kind: BlkEvent, Content: "大段未老化事件", Tokens: 200}, // 非老化，阶段一不丢
		{Kind: BlkQuestion, Content: "未决问题", Tokens: 50},
	}
	seedSession(t, w, ctx, "sess-1", blocks) // before=250
	c := NewCompactor(w, mockSummarizer{out: "CVE-x 参数 id 可越权，见 ev-9", tokens: 10}, nil, 2)

	res, err := c.Compact(ctx, "sess-1", 100, "overflow", t0)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Succeeded() || res.Record.Stage != 2 {
		t.Fatalf("应阶段二成功，实得 %+v", res)
	}
	if res.Record.TokenAfter >= res.Record.TokenBefore {
		t.Fatalf("摘要后 token 应下降：before=%d after=%d", res.Record.TokenBefore, res.Record.TokenAfter)
	}
	// 新修订 = 受保护块(问题) + 摘要块。
	if n := blockCount(t, w, ctx, res.NewRevisionID); n != 2 {
		t.Fatalf("COMPACTED 块数=%d，应 2（问题+摘要）", n)
	}
}

// DoD：空摘要 / 膨胀摘要 / Summarizer 错误 → FAILED 回退，不产生新修订，继续用 parent（§7.6）。
func TestCompactStage2FailureFallback(t *testing.T) {
	cases := []struct {
		name string
		sum  mockSummarizer
		want string
	}{
		{"空摘要", mockSummarizer{out: "", tokens: 0}, CompEmpty},
		{"膨胀摘要", mockSummarizer{out: "x", tokens: 100000}, CompTruncated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, ctx := openBudgetDB(t)
			blocks := []CtxBlock{
				{Kind: BlkEvent, Content: "未老化事件", Tokens: 200},
				{Kind: BlkQuestion, Content: "未决", Tokens: 50},
			}
			seedSession(t, w, ctx, "sess-1", blocks)
			c := NewCompactor(w, tc.sum, nil, 2)

			res, err := c.Compact(ctx, "sess-1", 100, "overflow", t0)
			if err != nil {
				t.Fatal(err)
			}
			if res.Succeeded() || res.Status != tc.want {
				t.Fatalf("应 %s 回退，实得 %+v", tc.want, res)
			}
			if res.NewRevision != 0 || revCount(t, w, ctx, "sess-1") != 1 {
				t.Fatal("失败不应产生新修订，继续用 parent")
			}
		})
	}
}

// DoD：Secret Canary 命中摘要 → 拒绝（FAILED），绝不写入任何 COMPACTED 修订（§7.6）。
func TestCompactCanaryRejected(t *testing.T) {
	w, ctx := openBudgetDB(t)
	const canary = "CANARY-7f3a9b"
	blocks := []CtxBlock{
		{Kind: BlkEvent, Content: "含秘密 " + canary + " 的事件", Tokens: 200},
		{Kind: BlkQuestion, Content: "未决", Tokens: 50},
	}
	seedSession(t, w, ctx, "sess-1", blocks)
	// Summarizer 误把 Canary 写进摘要。
	c := NewCompactor(w, mockSummarizer{out: "摘要泄露了 " + canary, tokens: 10}, []string{canary}, 2)

	res, err := c.Compact(ctx, "sess-1", 100, "overflow", t0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Succeeded() {
		t.Fatal("Canary 命中摘要应拒绝，不得成功")
	}
	if revCount(t, w, ctx, "sess-1") != 1 {
		t.Fatal("Canary 命中不应产生新修订")
	}
}
