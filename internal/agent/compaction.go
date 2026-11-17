package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// compaction.go —— 两阶段上下文压缩（§7.6，吸收 CAI）。
//
// 长 Run 的 Session 上下文不压缩必溢出，但压缩有损，要保关键元素：
//   阶段一（免费、确定性）：按块老化丢过期事件/已终结 Job 细节/超期 ToolResult；
//                           保留未决问题/活跃 Hypothesis/关键 Baseline 差异/IdentityRef/SecretRef/失败分类。
//   阶段二（付费、LLM 摘要）：阶段一仍超预算时，独立 Summarizer 结构化摘要，保 exploit 连续性
//                           （漏洞编号/关键参数名/差异结论/Evidence 引用/未验证假设）。
//
// 不变量（§7.6）：Journal/Evidence/原始 Revision 永不被压缩改——压缩只产生**新**派生 Revision；
// 明文凭据绝不进摘要（只保留 IdentityRef/SecretRef）；空/膨胀摘要（after≥before）=FAILED 回退重试；
// Secret Canary 绝不出现在任何摘要；崩溃可恢复（未完成的按 FAILED 处理，继续用 parent）。

// 块类型。
const (
	BlkEvent        = "event"
	BlkJobDetail    = "job_detail"
	BlkToolResult   = "tool_result"
	BlkQuestion     = "question"
	BlkHypothesis   = "hypothesis"
	BlkBaselineDiff = "baseline_diff"
	BlkIdentityRef  = "identity_ref"
	BlkSecretRef    = "secret_ref"
	BlkFailureClass = "failure_class"
	BlkEvidenceRef  = "evidence_ref"
	BlkSummary      = "summary"
)

// 压缩状态（§7.6）。
const (
	CompSucceeded = "SUCCEEDED"
	CompFailed    = "FAILED"
	CompEmpty     = "EMPTY"     // 空摘要（算 FAILED）
	CompTruncated = "TRUNCATED" // 膨胀摘要 after≥before（算 FAILED）
)

// Revision 种类。
const (
	RevFull      = "FULL"
	RevCompacted = "COMPACTED"
)

// protectedKind 报告某块类型是否为阶段一必须保留（不参与丢弃/摘要）。
func protectedKind(kind string) bool {
	switch kind {
	case BlkQuestion, BlkHypothesis, BlkBaselineDiff, BlkIdentityRef, BlkSecretRef, BlkFailureClass, BlkEvidenceRef:
		return true
	}
	return false
}

// CtxBlock 是上下文中的一块。
type CtxBlock struct {
	Ordinal      int
	Kind         string
	Source       string
	Content      string
	Tokens       int
	Aged         bool     // 阶段一老化标记：仅可丢弃类（event/job_detail/tool_result）被老化才丢
	EvidenceRefs []string // 关联 Evidence ID（压缩后仍可解析）
}

func blkTokens(b CtxBlock) int { return tokensOf(b.Tokens, b.Content) }

func sumTokens(bs []CtxBlock) int {
	n := 0
	for _, b := range bs {
		n += blkTokens(b)
	}
	return n
}

// Summarizer 生成结构化摘要（独立 Summarizer profile，§7.6 阶段二）。
type Summarizer interface {
	Summarize(ctx context.Context, blocks []CtxBlock) (summary string, tokens int, err error)
}

// CompactionRecord 压缩记录（§7.6）。
type CompactionRecord struct {
	ParentContextRevision int      `json:"parent_context_revision"`
	Trigger               string   `json:"trigger"`
	SourceRefs            []string `json:"source_refs,omitempty"`
	Status                string   `json:"status"`
	TokenBefore           int      `json:"token_before"`
	TokenAfter            int      `json:"token_after"`
	SummaryRef            string   `json:"summary_ref,omitempty"`
	Stage                 int      `json:"stage"` // 1=仅确定性丢弃，2=含 LLM 摘要
}

// CompactionResult 压缩结果。
type CompactionResult struct {
	Status        string
	NewRevision   int // 成功时新 COMPACTED 修订号；失败为 0（继续用 parent）
	NewRevisionID string
	Record        CompactionRecord
	Retries       int
}

// Succeeded 报告是否产生了可用的新修订。
func (r CompactionResult) Succeeded() bool { return r.Status == CompSucceeded }

// Compactor 执行两阶段压缩。
type Compactor struct {
	w          *store.Writer
	summarizer Summarizer
	canaries   []string // Secret Canary 标记：命中即拒绝摘要（§7.6/§4213）
	maxRetries int
}

// NewCompactor 构造。canaries 为 Secret Canary 标记集；maxRetries 为阶段二有界重试次数。
func NewCompactor(w *store.Writer, s Summarizer, canaries []string, maxRetries int) *Compactor {
	if maxRetries <= 0 {
		maxRetries = 2
	}
	return &Compactor{w: w, summarizer: s, canaries: canaries, maxRetries: maxRetries}
}

// stage1 确定性丢弃：丢「可丢弃类 && 已老化」，保留其余（含全部受保护块）。返回保留块 + 被丢弃的块。
func stage1(blocks []CtxBlock) (kept, dropped []CtxBlock) {
	for _, b := range blocks {
		droppable := b.Kind == BlkEvent || b.Kind == BlkJobDetail || b.Kind == BlkToolResult
		if droppable && b.Aged {
			dropped = append(dropped, b)
			continue
		}
		kept = append(kept, b)
	}
	return kept, dropped
}

// containsCanary 报告文本是否命中任一 Secret Canary 标记。
func (c *Compactor) containsCanary(s string) bool {
	for _, ca := range c.canaries {
		if ca != "" && strings.Contains(s, ca) {
			return true
		}
	}
	return false
}

// Compact 触发一次两阶段压缩，预算内则仅阶段一，否则阶段二 LLM 摘要。成功生成**新** COMPACTED 修订
// （原修订一行不改）；失败（空/膨胀/Canary/Summarizer 错误）回退、继续用 parent（§7.6）。
func (c *Compactor) Compact(ctx context.Context, sessionID string, budget int, trigger string, now time.Time) (CompactionResult, error) {
	// 读 parent（该 Session 最新修订）的块。
	parentRev, parentRevID, parentBlocks, err := c.loadLatest(ctx, sessionID)
	if err != nil {
		return CompactionResult{}, err
	}
	before := sumTokens(parentBlocks)
	rec := CompactionRecord{ParentContextRevision: parentRev, Trigger: trigger, TokenBefore: before}

	// 阶段一：确定性丢弃。
	kept, _ := stage1(parentBlocks)
	after1 := sumTokens(kept)
	if after1 <= budget {
		rec.Stage, rec.Status, rec.TokenAfter = 1, CompSucceeded, after1
		return c.persist(ctx, sessionID, parentRev, parentRevID, kept, rec, now)
	}

	// 阶段二：对非受保护的存活块做 LLM 摘要；受保护块原样保留。有界重试。
	var protectedBlocks, summarizable []CtxBlock
	var srcRefs []string
	for _, b := range kept {
		if protectedKind(b.Kind) {
			protectedBlocks = append(protectedBlocks, b)
			srcRefs = append(srcRefs, b.EvidenceRefs...)
		} else {
			summarizable = append(summarizable, b)
		}
	}
	rec.Stage, rec.SourceRefs = 2, srcRefs

	var lastStatus = CompFailed
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		summary, stoks, serr := c.summarizer.Summarize(ctx, summarizable)
		if serr != nil {
			lastStatus = CompFailed
			continue
		}
		if strings.TrimSpace(summary) == "" {
			lastStatus = CompEmpty // 空摘要 = FAILED
			continue
		}
		if c.containsCanary(summary) {
			lastStatus = CompFailed // Secret Canary 命中：绝不入摘要，拒绝
			continue
		}
		summaryBlock := CtxBlock{Kind: BlkSummary, Source: "summarizer", Content: summary, Tokens: stoks}
		newKept := append(append([]CtxBlock{}, protectedBlocks...), summaryBlock)
		after2 := sumTokens(newKept)
		if after2 >= before {
			lastStatus = CompTruncated // 膨胀 = FAILED
			continue
		}
		rec.Status, rec.TokenAfter, rec.SummaryRef = CompSucceeded, after2, "inline"
		res, perr := c.persist(ctx, sessionID, parentRev, parentRevID, newKept, rec, now)
		res.Retries = attempt
		return res, perr
	}

	// 有界重试仍失败：回退，继续用 parent（崩溃/未完成同此处理）。升级为 Session 重建由上层决定。
	rec.Status, rec.TokenAfter = lastStatus, before
	return CompactionResult{Status: lastStatus, Record: rec, Retries: c.maxRetries}, nil
}

// persist 原子写入**新** COMPACTED 修订 + 其块（parent 一行不改）。
func (c *Compactor) persist(ctx context.Context, sessionID string, parentRev int, _ string, blocks []CtxBlock, rec CompactionRecord, now time.Time) (CompactionResult, error) {
	newRev := parentRev + 1
	revID := string(domain.NewID())
	recJSON, _ := json.Marshal(rec)
	tstr := ts(now)
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.ExecContext(ctx, `
			INSERT INTO agent_context_revisions
			  (id, agent_session_id, revision, parent_revision, kind, compaction_record_json, token_estimate, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			revID, sessionID, newRev, parentRev, RevCompacted, string(recJSON), rec.TokenAfter, tstr); e != nil {
			return e
		}
		for i, b := range blocks {
			refsJSON := ""
			if len(b.EvidenceRefs) > 0 {
				j, _ := json.Marshal(b.EvidenceRefs)
				refsJSON = string(j)
			}
			aged := 0
			if b.Aged {
				aged = 1
			}
			if _, e := tx.ExecContext(ctx, `
				INSERT INTO agent_context_blocks
				  (id, context_revision_id, ordinal, kind, source, content, tokens, aged, evidence_refs, created_at)
				VALUES (?,?,?,?,?,?,?,?,?,?)`,
				string(domain.NewID()), revID, i, b.Kind, nz(b.Source), b.Content, blkTokens(b), aged, nz(refsJSON), tstr); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return CompactionResult{}, err
	}
	return CompactionResult{Status: CompSucceeded, NewRevision: newRev, NewRevisionID: revID, Record: rec}, nil
}

// loadLatest 读该 Session 的最新修订号/ID 及其块。
func (c *Compactor) loadLatest(ctx context.Context, sessionID string) (rev int, revID string, blocks []CtxBlock, err error) {
	err = c.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `
			SELECT id, revision FROM agent_context_revisions
			WHERE agent_session_id=? ORDER BY revision DESC LIMIT 1`, sessionID).Scan(&revID, &rev)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "no context revision for session "+sessionID)
		}
		if e != nil {
			return e
		}
		rows, e := tx.QueryContext(ctx, `
			SELECT ordinal, kind, COALESCE(source,''), COALESCE(content,''), tokens, aged, COALESCE(evidence_refs,'')
			FROM agent_context_blocks WHERE context_revision_id=? ORDER BY ordinal`, revID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var b CtxBlock
			var refs string
			var aged int
			if e = rows.Scan(&b.Ordinal, &b.Kind, &b.Source, &b.Content, &b.Tokens, &aged, &refs); e != nil {
				return e
			}
			b.Aged = aged == 1
			if refs != "" {
				_ = json.Unmarshal([]byte(refs), &b.EvidenceRefs)
			}
			blocks = append(blocks, b)
		}
		return rows.Err()
	})
	return rev, revID, blocks, err
}

// EvidenceRefsOf 读某修订所有块的 Evidence 引用（验证压缩后旧引用仍可解析）。
func (c *Compactor) EvidenceRefsOf(ctx context.Context, revisionID string) ([]string, error) {
	var refs []string
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx,
			`SELECT COALESCE(evidence_refs,'') FROM agent_context_blocks WHERE context_revision_id=?`, revisionID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var r string
			if e = rows.Scan(&r); e != nil {
				return e
			}
			if r != "" {
				var ids []string
				_ = json.Unmarshal([]byte(r), &ids)
				refs = append(refs, ids...)
			}
		}
		return rows.Err()
	})
	return refs, err
}
