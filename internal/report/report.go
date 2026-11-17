// Package report 生成漏洞报告（§13.6）：**程序填事实，AI 只写描述**。
//
// 核心不变量（§13.6/§13.4）：
//   - 程序负责事实/请求/响应/Evidence 引用；AI 只提供「漏洞描述」与「修复建议」两段文本；
//   - 程序校验每个 Finding 都有逐字 EvidenceCitation（事实都有来源），否则不入报告；
//   - 报告只收 REPORTABLE Finding；非 REPORTABLE 一律不引用；
//   - 不自动向外部平台提交；
//   - 报告状态机 DRAFT → FINALIZED →（依赖失效）STALE：Finalize 重校验全部 verdict 版本与
//     Evidence 状态；已 FINALIZED 的报告在依赖失效后标 STALE，不得继续分发。
//
// 报告把 CVSS 由结构化字段确定性算出（§31 V7，见 findings.CVSSv31），AI 不自报分数。
package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"pentagent/internal/canon"
	"pentagent/internal/domain"
	"pentagent/internal/findings"
	"pentagent/internal/store"
)

// 报告状态机（§13.4）。
const (
	StateDraft     = "DRAFT"
	StateFinalized = "FINALIZED"
	StateStale     = "STALE"
)

var (
	// ErrNonReportable —— 试图把非 REPORTABLE Finding 放进报告（§13.6）。
	ErrNonReportable = &domain.DomainError{Code: "NON_REPORTABLE_IN_REPORT", Msg: "report may only cite REPORTABLE findings (§13.6)"}
	// ErrFactWithoutSource —— 某 Finding 缺逐字 EvidenceCitation（事实无来源）。
	ErrFactWithoutSource = &domain.DomainError{Code: "FACT_WITHOUT_SOURCE", Msg: "every reported fact must carry an evidence citation (§13.6)"}
	// ErrNoActiveVerdict —— 某 Finding 没有未被 supersede 的验证结论。
	ErrNoActiveVerdict = &domain.DomainError{Code: "NO_ACTIVE_VERDICT", Msg: "reportable finding lacks an active verification verdict (§13.4)"}
	// ErrReportStale —— 报告依赖已失效，不能 Finalize/分发。
	ErrReportStale = &domain.DomainError{Code: "REPORT_STALE", Msg: "report dependencies invalidated; marked STALE (§13.4)"}
)

// Store 持久化并渲染报告。
type Store struct{ w *store.Writer }

// NewStore 构造。
func NewStore(w *store.Writer) *Store { return &Store{w: w} }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func nz(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Descriptions 是 AI 为某个 Finding 提供的两段**仅描述性**文本。程序不信任其中任何「事实」，
// 只把它们放进「漏洞描述」「修复建议」小节；事实全部由程序从库里取。
type Descriptions struct {
	Description string // 漏洞描述
	Remediation string // 修复建议
}

// FactRow 是程序从库里取出的一条 Finding 事实（全部有来源）。
type FactRow struct {
	FindingID    string
	Title        string
	VulnType     string
	State        string
	CVSSVector   string
	CVSSScore    float64
	CVSSSeverity string
	VerdictRev   int
	Verdict      string
	Citations    []CitationFact
	EvidenceIDs  []string // 去重后的被引用 Evidence（进 Manifest，引用不搬运）
}

// CitationFact 是一条逐字引用（复现步骤/证据附录用）。
type CitationFact struct {
	EvidenceID    string
	ReceiptSeq    int
	ByteStart     int
	ByteEnd       int
	ContentSHA256 string
	Normalization string
	Truncated     bool
}

// snapshotEntry 绑定报告到具体 Finding/Verdict 修订（Finalize 重校验用）。
type snapshotEntry struct {
	FindingID  string `json:"finding_id"`
	VerdictRev int    `json:"verdict_revision"`
	Verdict    string `json:"verdict"`
}

// BuildResult 是一次报告生成的产物。
type BuildResult struct {
	ReportID string
	Markdown string
	JSON     string
	Manifest string
	Facts    []FactRow
}

// Build 生成一份 DRAFT 报告（§13.6）。severityByFinding 可选：调用方（程序侧）为每个 Finding 提供
// 结构化 CVSS 指标；descByFinding 可选：AI 的描述/修复建议文本。两者都缺省时仍能出报告（事实齐全）。
//
// 流程全部程序主导：
//  1. 取该 Run 所有 REPORTABLE Finding；无则零发现报告（与 PARTIAL 区分由 Exit Gate 负责）；
//  2. 每个 Finding 必须有 active verdict（否则 ErrNoActiveVerdict）且至少一条 EvidenceCitation
//     （否则 ErrFactWithoutSource）——事实都有来源；
//  3. 程序算 CVSS；AI 文本仅进描述小节；
//  4. 渲染 Markdown + JSON + Evidence Manifest（引用，不搬运）；
//  5. 落 reports 行（DRAFT），绑定当前 run/snapshot/surface 修订与 finding 快照。
func (s *Store) Build(ctx context.Context, runID string, severityByFinding map[string]findings.CVSSv31, descByFinding map[string]Descriptions, now time.Time) (*BuildResult, error) {
	var facts []FactRow
	var runRev, snapRev, surfRev int
	reportID := string(domain.NewID())

	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		// 绑定修订号（与 Exit Gate 的 CAS 同源）。
		if e := tx.QueryRowContext(ctx,
			`SELECT revision, current_snapshot_revision FROM runs WHERE id=?`, runID).Scan(&runRev, &snapRev); e != nil {
			if e == sql.ErrNoRows {
				return domain.Errorf(domain.ErrNotFound, "run "+runID)
			}
			return e
		}
		_ = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(surface_revision),0) FROM coverage_revisions WHERE run_id=?`, runID).Scan(&surfRev)

		// 1) 只取 REPORTABLE。
		rows, e := tx.QueryContext(ctx,
			`SELECT id, COALESCE(title,''), COALESCE(vulnerability_type,'') FROM findings
			 WHERE run_id=? AND state=? ORDER BY id`, runID, findings.FindReportable)
		if e != nil {
			return e
		}
		type fhead struct{ id, title, vt string }
		var heads []fhead
		for rows.Next() {
			var h fhead
			if e := rows.Scan(&h.id, &h.title, &h.vt); e != nil {
				rows.Close()
				return e
			}
			heads = append(heads, h)
		}
		rows.Close()
		if e := rows.Err(); e != nil {
			return e
		}

		for _, h := range heads {
			f := FactRow{FindingID: h.id, Title: h.title, VulnType: h.vt, State: findings.FindReportable}

			// 2a) active verdict（未被 supersede）。
			if e := tx.QueryRowContext(ctx,
				`SELECT revision, verdict FROM verification_verdicts
				 WHERE finding_id=? AND superseded=0 ORDER BY revision DESC LIMIT 1`, h.id).
				Scan(&f.VerdictRev, &f.Verdict); e != nil {
				if e == sql.ErrNoRows {
					return domain.Errorf(ErrNoActiveVerdict, "finding "+h.id)
				}
				return e
			}

			// 2b) 逐字 EvidenceCitation（事实都有来源）。
			crows, e := tx.QueryContext(ctx,
				`SELECT evidence_id, receipt_sequence, byte_start, byte_end, content_sha256,
				        COALESCE(normalization,''), truncated
				 FROM evidence_citations WHERE finding_id=? ORDER BY receipt_sequence, byte_start`, h.id)
			if e != nil {
				return e
			}
			seen := map[string]bool{}
			for crows.Next() {
				var c CitationFact
				var tr int
				if e := crows.Scan(&c.EvidenceID, &c.ReceiptSeq, &c.ByteStart, &c.ByteEnd, &c.ContentSHA256, &c.Normalization, &tr); e != nil {
					crows.Close()
					return e
				}
				c.Truncated = tr == 1
				f.Citations = append(f.Citations, c)
				if !seen[c.EvidenceID] {
					seen[c.EvidenceID] = true
					f.EvidenceIDs = append(f.EvidenceIDs, c.EvidenceID)
				}
			}
			crows.Close()
			if e := crows.Err(); e != nil {
				return e
			}
			if len(f.Citations) == 0 {
				return domain.Errorf(ErrFactWithoutSource, "finding "+h.id)
			}

			// 3) 程序算 CVSS（AI 不自报分数）。
			if m, ok := severityByFinding[h.id]; ok {
				f.CVSSVector = m.Vector()
				f.CVSSScore = m.BaseScore()
				f.CVSSSeverity = m.Severity()
			}
			facts = append(facts, f)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 4) 渲染（事务外，纯函数）。
	md := renderMarkdown(runID, facts, descByFinding, now)
	js, manifest, e := renderJSON(runID, facts, descByFinding, now)
	if e != nil {
		return nil, e
	}
	snap := buildSnapshot(facts)
	snapJSON, _ := json.Marshal(snap)
	manifestDigest, _ := canon.DigestJSON([]byte(manifest))

	// 5) 落 DRAFT 报告行。
	err = s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO reports (id, run_id, state, manifest_digest, format, markdown_body, json_body,
			                     evidence_manifest_json, finding_snapshot_json,
			                     run_revision, snapshot_revision, surface_revision, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			reportID, runID, StateDraft, nz(manifestDigest), "markdown", md, js,
			nz(manifest), string(snapJSON), runRev, snapRev, surfRev, ts(now))
		return e
	})
	if err != nil {
		return nil, err
	}
	return &BuildResult{ReportID: reportID, Markdown: md, JSON: js, Manifest: manifest, Facts: facts}, nil
}

func buildSnapshot(facts []FactRow) []snapshotEntry {
	out := make([]snapshotEntry, 0, len(facts))
	for _, f := range facts {
		out = append(out, snapshotEntry{FindingID: f.FindingID, VerdictRev: f.VerdictRev, Verdict: f.Verdict})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FindingID < out[j].FindingID })
	return out
}

// Finalize 把 DRAFT 报告转 FINALIZED：先**重校验全部 verdict 版本与 Evidence 状态**（§13.4）——
// 任一绑定 Finding 的 active verdict 版本已前进（被 supersede/降级）、或任一被引用 Evidence 进了
// Quarantine，则不 Finalize，直接把报告标 STALE 并返回 ErrReportStale。
func (s *Store) Finalize(ctx context.Context, reportID string, now time.Time) error {
	// staleReason 在事务内被赋值时表示依赖失效：事务提交 STALE 写入（不回滚），提交后再返回 ErrReportStale。
	var staleReason string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		staleReason = "" // 幂等：若 Durable 内部重试，重置
		var runID, state, snapJSON, manifestJSON string
		e := tx.QueryRowContext(ctx,
			`SELECT run_id, state, COALESCE(finding_snapshot_json,''), COALESCE(evidence_manifest_json,'')
			 FROM reports WHERE id=?`, reportID).Scan(&runID, &state, &snapJSON, &manifestJSON)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "report "+reportID)
		}
		if e != nil {
			return e
		}
		if state != StateDraft {
			return domain.Errorf(domain.ErrInvalid, "report not DRAFT: "+state)
		}

		var snap []snapshotEntry
		if snapJSON != "" {
			_ = json.Unmarshal([]byte(snapJSON), &snap)
		}
		// 重校验每个 Finding 的 active verdict 版本未变。
		for _, se := range snap {
			var rev int
			var vd string
			e := tx.QueryRowContext(ctx,
				`SELECT revision, verdict FROM verification_verdicts
				 WHERE finding_id=? AND superseded=0 ORDER BY revision DESC LIMIT 1`, se.FindingID).Scan(&rev, &vd)
			if e == sql.ErrNoRows || rev != se.VerdictRev || vd != se.Verdict {
				staleReason = "verdict_superseded:" + se.FindingID
				return markStaleTx(ctx, tx, reportID, staleReason)
			}
			if e != nil {
				return e
			}
		}
		// 重校验被引用 Evidence 未进 Quarantine。
		var man struct {
			Evidence []string `json:"evidence"`
		}
		if manifestJSON != "" {
			_ = json.Unmarshal([]byte(manifestJSON), &man)
		}
		for _, ev := range man.Evidence {
			var q int
			if e := tx.QueryRowContext(ctx, `SELECT COALESCE(quarantined,0) FROM evidence WHERE id=?`, ev).Scan(&q); e == nil && q == 1 {
				staleReason = "evidence_quarantined:" + ev
				return markStaleTx(ctx, tx, reportID, staleReason)
			}
		}
		_, e = tx.ExecContext(ctx,
			`UPDATE reports SET state=?, finalized_at=?, revision=revision+1 WHERE id=? AND state=?`,
			StateFinalized, ts(now), reportID, StateDraft)
		return e
	})
	if err != nil {
		return err
	}
	if staleReason != "" {
		return domain.Errorf(ErrReportStale, staleReason)
	}
	return nil
}

// MarkStale 显式把报告标 STALE（依赖失效后不得继续分发，§13.4）。成功返回 nil。
func (s *Store) MarkStale(ctx context.Context, reportID, reason string, now time.Time) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		return markStaleTx(ctx, tx, reportID, reason)
	})
}

// markStaleTx 把报告置 STALE（只做写，不返回哨兵错误——否则会回滚本写入）。
func markStaleTx(ctx context.Context, tx *sql.Tx, reportID, reason string) error {
	_, e := tx.ExecContext(ctx,
		`UPDATE reports SET state=?, stale_reason=?, revision=revision+1 WHERE id=?`,
		StateStale, reason, reportID)
	return e
}

// ReportState 读报告状态（测试/分发前检查用）。
func (s *Store) ReportState(ctx context.Context, reportID string) (string, error) {
	var st string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT state FROM reports WHERE id=?`, reportID).Scan(&st)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "report "+reportID)
		}
		return e
	})
	return st, err
}

// Body 读报告的某种渲染体（"markdown"|"json"），用于分发/测试。STALE 报告仍可读但调用方应拒绝分发。
func (s *Store) Body(ctx context.Context, reportID, format string) (string, error) {
	col := "markdown_body"
	if format == "json" {
		col = "json_body"
	}
	var body string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT COALESCE(`+col+`,'') FROM reports WHERE id=?`, reportID).Scan(&body)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "report "+reportID)
		}
		return e
	})
	return body, err
}

// --- 渲染（纯函数，事实已在程序侧取齐） ---

func renderMarkdown(runID string, facts []FactRow, desc map[string]Descriptions, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 渗透测试报告\n\n")
	fmt.Fprintf(&b, "- Run: `%s`\n- 生成时间: %s\n- REPORTABLE 发现数: %d\n\n", runID, ts(now), len(facts))
	if len(facts) == 0 {
		b.WriteString("## 结论\n\n本次 Run 未产出 REPORTABLE 发现（零发现）。\n")
		b.WriteString("\n> 注：零发现 ≠ 未完成。若 Run 以 PARTIAL/FAILED 收尾，参见 run_partial_reasons。\n")
		return b.String()
	}
	for i, f := range facts {
		d := desc[f.FindingID]
		fmt.Fprintf(&b, "## %d. %s\n\n", i+1, orPlaceholder(f.Title, "（无标题）"))
		// 漏洞信息（程序事实）。
		b.WriteString("### 漏洞信息\n\n")
		fmt.Fprintf(&b, "- Finding: `%s`\n- 类型: %s\n- 状态: %s\n- 验证结论: %s (rev %d)\n",
			f.FindingID, orPlaceholder(f.VulnType, "-"), f.State, f.Verdict, f.VerdictRev)
		if f.CVSSVector != "" {
			fmt.Fprintf(&b, "- CVSS: %.1f (%s) `%s`\n", f.CVSSScore, f.CVSSSeverity, f.CVSSVector)
		}
		b.WriteString("\n")
		// 漏洞描述（AI 文本，仅描述）。
		b.WriteString("### 漏洞描述\n\n")
		b.WriteString(orPlaceholder(d.Description, "（待补充描述）") + "\n\n")
		// 复现步骤（程序：逐字引用）。
		b.WriteString("### 复现步骤 / 证据引用\n\n")
		for _, c := range f.Citations {
			tr := ""
			if c.Truncated {
				tr = " [TRUNCATED]"
			}
			fmt.Fprintf(&b, "- Evidence `%s` seq=%d bytes[%d:%d] sha256=%s%s\n",
				c.EvidenceID, c.ReceiptSeq, c.ByteStart, c.ByteEnd, short(c.ContentSHA256), tr)
		}
		b.WriteString("\n")
		// 修复建议（AI 文本）。
		b.WriteString("### 修复建议\n\n")
		b.WriteString(orPlaceholder(d.Remediation, "（待补充修复建议）") + "\n\n")
		// 证据附录（引用，不搬运）。
		b.WriteString("### 证据附录\n\n")
		fmt.Fprintf(&b, "- 引用 Evidence: %s\n\n", strings.Join(f.EvidenceIDs, ", "))
	}
	b.WriteString("---\n\n## 红线与停止点\n\n本报告仅含 REPORTABLE 发现；证据以引用方式附录，不复制原始 Blob。\n")
	b.WriteString("本框架仅用于授权测试，不自动向外部平台提交。\n")
	return b.String()
}

type jsonFinding struct {
	FindingID    string         `json:"finding_id"`
	Title        string         `json:"title"`
	VulnType     string         `json:"vulnerability_type"`
	State        string         `json:"state"`
	Verdict      string         `json:"verdict"`
	VerdictRev   int            `json:"verdict_revision"`
	CVSSVector   string         `json:"cvss_vector,omitempty"`
	CVSSScore    float64        `json:"cvss_score,omitempty"`
	CVSSSeverity string         `json:"cvss_severity,omitempty"`
	Description  string         `json:"description"`
	Remediation  string         `json:"remediation"`
	Citations    []CitationFact `json:"citations"`
	Evidence     []string       `json:"evidence"`
}

type jsonReport struct {
	Run       string        `json:"run_id"`
	Generated string        `json:"generated_at"`
	Count     int           `json:"reportable_count"`
	Findings  []jsonFinding `json:"findings"`
}

type manifestDoc struct {
	Run      string   `json:"run_id"`
	Evidence []string `json:"evidence"` // 去重、排序；引用不搬运
}

func renderJSON(runID string, facts []FactRow, desc map[string]Descriptions, now time.Time) (string, string, error) {
	rep := jsonReport{Run: runID, Generated: ts(now), Count: len(facts)}
	evSet := map[string]bool{}
	for _, f := range facts {
		d := desc[f.FindingID]
		rep.Findings = append(rep.Findings, jsonFinding{
			FindingID: f.FindingID, Title: f.Title, VulnType: f.VulnType, State: f.State,
			Verdict: f.Verdict, VerdictRev: f.VerdictRev,
			CVSSVector: f.CVSSVector, CVSSScore: f.CVSSScore, CVSSSeverity: f.CVSSSeverity,
			Description: d.Description, Remediation: d.Remediation,
			Citations: f.Citations, Evidence: f.EvidenceIDs,
		})
		for _, ev := range f.EvidenceIDs {
			evSet[ev] = true
		}
	}
	repBytes, e := json.MarshalIndent(rep, "", "  ")
	if e != nil {
		return "", "", e
	}
	evs := make([]string, 0, len(evSet))
	for ev := range evSet {
		evs = append(evs, ev)
	}
	sort.Strings(evs)
	manBytes, e := json.MarshalIndent(manifestDoc{Run: runID, Evidence: evs}, "", "  ")
	if e != nil {
		return "", "", e
	}
	return string(repBytes), string(manBytes), nil
}

func orPlaceholder(s, ph string) string {
	if strings.TrimSpace(s) == "" {
		return ph
	}
	return s
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
