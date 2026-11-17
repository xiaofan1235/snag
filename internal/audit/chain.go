// Package audit 实现篡改可证审计哈希链（§47.2）。
//
// 权威事件（动作、审批、策略变更、Reveal、导出、清理）入链；每 Run 内单调 sequence，
// 链头（seq=1）锚定 Run 创建。
//
//	entry_hash = SHA-256( JCS(关键字段) || prev_hash )
//
// 性质：任何事后篡改/删除/重排中间条目 → 后续 hash 不连续 → Verify 可检测。导出链带 JCS payload，
// 第三方可**离线重算**验链完整。注意：哈希链只证「内部记录自洽未被改」，不等于「内容客观为真」——
// 客观性仍靠网关留证 + 逐字引用 + 独立验证（§13），两者叠加才完整。
package audit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

	"pentagent/internal/canon"
	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// 权威事件类型（§47.2：入链的关键事件）。
const (
	EntryRunCreated   = "run_created"   // 链头锚点
	EntryAction       = "action"        // 已执行动作
	EntryApproval     = "approval"      // 审批决策
	EntryPolicyChange = "policy_change" // 策略版本变更
	EntryReveal       = "reveal"        // 原始证据 Reveal
	EntryExport       = "export"        // 报告/证据导出
	EntryCleanup      = "cleanup"       // 测试数据清理
)

var (
	// ErrChainBroken —— 验链失败（hash 不连续，疑似篡改/删除）。
	ErrChainBroken = &domain.DomainError{Code: "AUDIT_CHAIN_BROKEN", Msg: "audit hash chain is broken (§47.2)"}
	// ErrChainNotAnchored —— 链头不是 run_created 锚点。
	ErrChainNotAnchored = &domain.DomainError{Code: "AUDIT_CHAIN_NOT_ANCHORED", Msg: "audit chain head is not run_created (§47.2)"}
)

// Chain 持久化并校验审计哈希链。
type Chain struct{ w *store.Writer }

// NewChain 构造。
func NewChain(w *store.Writer) *Chain { return &Chain{w: w} }

// Entry 是链上一条（导出/验链用）。
type Entry struct {
	RunID     string
	Sequence  int
	PrevHash  string
	EntryHash string
	EntryType string
	EntryRef  string
	Payload   string // 已 JCS 规范化的关键字段
	CreatedAt string
}

// hashEntry 计算 entry_hash = SHA-256( JCS(关键字段) || prev_hash )。payloadJCS 必须已规范化。
func hashEntry(payloadJCS, prevHash string) string {
	h := sha256.New()
	h.Write([]byte(payloadJCS))
	h.Write([]byte(prevHash))
	return hex.EncodeToString(h.Sum(nil))
}

// Anchor 为 Run 建链头（seq=1，entry_type=run_created）。幂等：已有链头则直接返回其 hash。
// prev_hash 为空串（链头无前驱）。
func (c *Chain) Anchor(ctx context.Context, runID, authScopeDigest, createdAt string) (string, error) {
	var hash string
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		var existing string
		e := tx.QueryRowContext(ctx, `SELECT entry_hash FROM audit_chain WHERE run_id=? AND sequence=1`, runID).Scan(&existing)
		if e == nil {
			hash = existing
			return nil
		}
		if e != sql.ErrNoRows {
			return e
		}
		payload, e := canon.Digest(map[string]any{"type": EntryRunCreated, "run_id": runID, "scope_digest": authScopeDigest})
		if e != nil {
			return e
		}
		hash = hashEntry(payload, "")
		_, e = tx.ExecContext(ctx, `
			INSERT INTO audit_chain (run_id, sequence, prev_hash, entry_hash, entry_type, entry_ref, payload_json, created_at)
			VALUES (?,1,?,?,?,?,?,?)`,
			runID, "", hash, EntryRunCreated, runID, payload, createdAt)
		return e
	})
	return hash, err
}

// Append 追加一条权威事件到链尾。keyFields 为该事件的关键字段（将被 JCS 规范化）。
// entry_hash = SHA256(JCS(keyFields) || prev_hash)，sequence = 上一条 +1；链必须已 Anchor。
func (c *Chain) Append(ctx context.Context, runID, entryType, entryRef string, keyFields any, createdAt string) (int, string, error) {
	payload, err := canon.Digest(keyFields)
	if err != nil {
		return 0, "", err
	}
	var seq int
	var entryHash string
	err = c.w.Durable(ctx, func(tx *sql.Tx) error {
		var maxSeq int
		var prevHash string
		e := tx.QueryRowContext(ctx,
			`SELECT sequence, entry_hash FROM audit_chain WHERE run_id=? ORDER BY sequence DESC LIMIT 1`, runID).
			Scan(&maxSeq, &prevHash)
		if e == sql.ErrNoRows {
			return domain.Errorf(ErrChainNotAnchored, "run "+runID+": call Anchor first")
		}
		if e != nil {
			return e
		}
		seq = maxSeq + 1
		entryHash = hashEntry(payload, prevHash)
		_, e = tx.ExecContext(ctx, `
			INSERT INTO audit_chain (run_id, sequence, prev_hash, entry_hash, entry_type, entry_ref, payload_json, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			runID, seq, prevHash, entryHash, entryType, nz(entryRef), payload, createdAt)
		return e
	})
	return seq, entryHash, err
}

// Export 导出某 Run 的整条链（按 sequence 升序），供离线第三方验链。
func (c *Chain) Export(ctx context.Context, runID string) ([]Entry, error) {
	var out []Entry
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `
			SELECT run_id, sequence, COALESCE(prev_hash,''), entry_hash, entry_type, COALESCE(entry_ref,''),
			       COALESCE(payload_json,''), created_at
			FROM audit_chain WHERE run_id=? ORDER BY sequence`, runID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var en Entry
			if e := rows.Scan(&en.RunID, &en.Sequence, &en.PrevHash, &en.EntryHash, &en.EntryType, &en.EntryRef, &en.Payload, &en.CreatedAt); e != nil {
				return e
			}
			out = append(out, en)
		}
		return rows.Err()
	})
	return out, err
}

// VerifyResult 是验链结果。
type VerifyResult struct {
	OK       bool
	Length   int
	BreakAt  int    // 首个断裂的 sequence（OK 时为 0）
	BreakWhy string // 断裂原因
}

// Verify 读出整条链并离线式重算校验（§47.2）：
//   - 链头必须是 run_created、prev_hash 空、sequence=1；
//   - 每条 entry_hash 必须等于 SHA256(payload_json || prev_hash)（内容未被改）；
//   - 每条 prev_hash 必须等于上一条 entry_hash（无删除/重排/插入）；
//   - sequence 必须连续 1..N。
//
// 任一不符即 OK=false 并给出首个断点。空链视为 OK（length 0）。
func (c *Chain) Verify(ctx context.Context, runID string) (VerifyResult, error) {
	entries, err := c.Export(ctx, runID)
	if err != nil {
		return VerifyResult{}, err
	}
	return VerifyEntries(entries), nil
}

// VerifyEntries 是纯函数验链（第三方可据导出的 []Entry 离线调用，无需 DB）。
func VerifyEntries(entries []Entry) VerifyResult {
	res := VerifyResult{OK: true, Length: len(entries)}
	if len(entries) == 0 {
		return res
	}
	prev := ""
	for i, en := range entries {
		wantSeq := i + 1
		if en.Sequence != wantSeq {
			return VerifyResult{OK: false, Length: len(entries), BreakAt: en.Sequence, BreakWhy: "sequence 不连续"}
		}
		if i == 0 {
			if en.EntryType != EntryRunCreated || en.PrevHash != "" {
				return VerifyResult{OK: false, Length: len(entries), BreakAt: en.Sequence, BreakWhy: "链头非 run_created 锚点"}
			}
		} else if en.PrevHash != prev {
			return VerifyResult{OK: false, Length: len(entries), BreakAt: en.Sequence, BreakWhy: "prev_hash 与上一条 entry_hash 不符（疑似删除/重排/插入）"}
		}
		if hashEntry(en.Payload, en.PrevHash) != en.EntryHash {
			return VerifyResult{OK: false, Length: len(entries), BreakAt: en.Sequence, BreakWhy: "entry_hash 重算不符（疑似内容篡改）"}
		}
		prev = en.EntryHash
	}
	return res
}

func nz(s string) any {
	if s == "" {
		return nil
	}
	return s
}
