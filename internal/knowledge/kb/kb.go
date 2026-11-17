// Package kb 是知识库与 Skill 的入库、检索（§34/§40/§41）。
//
// 入库校验：Secret Canary 命中即拒（§34.6，知识库是不可信数据不是指令）、按内容哈希去重、按 Workspace 隔离。
// 检索三段式（§41）：分类粗筛（applies_to）→ 混合索引精排 → 取详情，预算内。
//
// 有据偏离（网络关闭）：§41 推荐 FTS5 + sqlite-vec 混合检索，sqlite-vec 是需下载的扩展、本环境不可得；
// 故退化为纯 Go 关键词打分（分类粗筛 + term 命中加权），接口不变，具备环境后可换 FTS5+向量精排。
package kb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// 信任层级（§34）。
const (
	TrustVerifiedFact  = "VERIFIED_FACT"
	TrustOperatorNote  = "OPERATOR_NOTE"
	TrustUnverifiedRef = "UNVERIFIED_REFERENCE"
)

// ErrSecretCanary —— 入库内容命中 Secret Canary，拒绝（§34.6/§26.8）。
var ErrSecretCanary = &domain.DomainError{Code: "SECRET_CANARY", Msg: "content hit secret canary, ingest rejected"}

// Store 知识库存储。
type Store struct {
	w        *store.Writer
	canaries []string
}

// NewStore 构造。canaries 为 Secret Canary 标记集。
func NewStore(w *store.Writer, canaries []string) *Store { return &Store{w: w, canaries: canaries} }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func nz(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Doc 是一篇知识文档（md+frontmatter 解析后）。
type Doc struct {
	Title     string
	Content   string
	AppliesTo []string // 攻击类型/模块
	TrustTier string
	Source    string
	URI       string
}

func (s *Store) hitCanary(text string) bool {
	for _, c := range s.canaries {
		if c != "" && strings.Contains(text, c) {
			return true
		}
	}
	return false
}

// Ingest 入库一篇知识：Canary 拦截 + 去重（content_sha256）+ Workspace 隔离。返回 (docID, deduped)。
func (s *Store) Ingest(ctx context.Context, workspaceID string, d Doc, now time.Time) (string, bool, error) {
	if s.hitCanary(d.Content) || s.hitCanary(d.Title) {
		return "", false, ErrSecretCanary // 知识库是不可信数据，命中 Canary 的秘密绝不入库
	}
	sum := sha256.Sum256([]byte(d.Content))
	hash := hex.EncodeToString(sum[:])
	var id string
	var deduped bool
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		// 去重：同 workspace 同内容哈希。
		var existing string
		e := tx.QueryRowContext(ctx,
			`SELECT id FROM knowledge_docs WHERE workspace_id=? AND content_sha256=?`, workspaceID, hash).Scan(&existing)
		if e == nil {
			id, deduped = existing, true
			return nil
		}
		if e != sql.ErrNoRows {
			return e
		}
		id = string(domain.NewID())
		trust := d.TrustTier
		if trust == "" {
			trust = TrustUnverifiedRef
		}
		_, e = tx.ExecContext(ctx, `
			INSERT INTO knowledge_docs (id, workspace_id, source, uri, title, content, applies_to, content_sha256, trust_tier, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, workspaceID, nz(d.Source), nz(d.URI), nz(d.Title), d.Content, strings.Join(d.AppliesTo, ","), hash, trust, ts(now))
		return e
	})
	return id, deduped, err
}

// Hit 是一条检索命中。
type Hit struct {
	DocID     string
	Title     string
	Score     int
	TrustTier string
}

// Retrieve 三段式检索（§41）：① 分类粗筛（applies_to 命中）② 关键词混合打分 ③ 取预算内 topN。
// **Workspace 隔离**：只查同 workspace 的文档（两 Workspace 不互查）。
func (s *Store) Retrieve(ctx context.Context, workspaceID, query, appliesTo string, budget int) ([]Hit, error) {
	if budget <= 0 {
		budget = 5
	}
	terms := tokenize(query)
	var hits []Hit
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		// ① 分类粗筛：同 workspace + applies_to 命中（为空则不限分类）。
		rows, e := tx.QueryContext(ctx, `
			SELECT id, COALESCE(title,''), COALESCE(content,''), COALESCE(applies_to,''), COALESCE(trust_tier,'')
			FROM knowledge_docs WHERE workspace_id=?`, workspaceID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var id, title, content, at, trust string
			if e = rows.Scan(&id, &title, &content, &at, &trust); e != nil {
				return e
			}
			if appliesTo != "" && !appliesToMatch(at, appliesTo) {
				continue // 分类粗筛未过
			}
			// ② 关键词混合打分（标题命中权重更高）。
			score := scoreDoc(terms, title, content)
			if score <= 0 {
				continue
			}
			hits = append(hits, Hit{DocID: id, Title: title, Score: score, TrustTier: trust})
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	// ③ 精排 + 预算内 topN（确定性：分数降序，同分按 DocID）。
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].DocID < hits[j].DocID
	})
	if len(hits) > budget {
		hits = hits[:budget]
	}
	return hits, nil
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 0x4e00 && r <= 0x9fff)
	})
	return fields
}

func scoreDoc(terms []string, title, content string) int {
	lt, lc := strings.ToLower(title), strings.ToLower(content)
	score := 0
	for _, t := range terms {
		if t == "" {
			continue
		}
		score += 3 * strings.Count(lt, t) // 标题命中权重高
		score += strings.Count(lc, t)
	}
	return score
}

func appliesToMatch(docApplies, query string) bool {
	set := map[string]bool{}
	for _, a := range strings.Split(docApplies, ",") {
		a = strings.TrimSpace(strings.ToLower(a))
		if a != "" {
			set[a] = true
		}
	}
	for _, q := range strings.Split(query, ",") {
		if set[strings.TrimSpace(strings.ToLower(q))] {
			return true
		}
	}
	return false
}
