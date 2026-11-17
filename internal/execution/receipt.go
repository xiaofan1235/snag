package execution

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// receipt.go —— ExecutionReceipt 终结 + RECORDED 确定性前置（§5.3）。M2 收口：执行→证据→记录闭环。
//
// RECORDED 前置（§5.3，全满足才进）：
//   ① Receipt 已终结（execution_receipts 存在、completed_at 已写、outcome=EXECUTED，非仅已发出）；
//   ② Proxy 流已关闭、HTTP Exchange 齐备（该 action 有 evidence 行）；
//   ③ 每个 Evidence 元数据与 Blob 哈希已持久化（evidence.plain_sha256 + blob_path 已落）。
// 任一不满足不得进 RECORDED（fail-closed，不自动标成功，§0.5）。

// Receipts 写执行回执、做 RECORDED 前置检查与 action 终结。
type Receipts struct {
	w *store.Writer
}

// NewReceipts 构造。
func NewReceipts(w *store.Writer) *Receipts { return &Receipts{w: w} }

func rcptNow(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// ReceiptInput 是一条执行回执的入参。
type ReceiptInput struct {
	ActionID       domain.ID
	RunID          domain.ID
	AttemptID      domain.ID
	FencingToken   int64
	IdentityID     string
	PolicyRevision int64
	ProxyLeaseID   string
	StatusCode     int
	Outcome        domain.ActionState // EXECUTED / OUTCOME_UNKNOWN
	RequestDigest  string
	ResponseDigest string
	Bytes          int
	StartedAt      time.Time
	CompletedAt    time.Time
}

// Write 终结写入一条 execution_receipt（durable 持久），并回填 actions.execution_receipt_id。
func (r *Receipts) Write(ctx context.Context, in ReceiptInput) (domain.ID, error) {
	id := domain.NewID()
	err := r.w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.ExecContext(ctx,
			`INSERT INTO execution_receipts
			 (id, action_id, run_id, attempt_id, fencing_token, identity_id, policy_revision, proxy_lease_id,
			  status_code, outcome, request_digest, response_digest, bytes, started_at, completed_at)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			string(id), string(in.ActionID), string(in.RunID), nzID(in.AttemptID), in.FencingToken,
			nz2(in.IdentityID), in.PolicyRevision, nz2(in.ProxyLeaseID), in.StatusCode, string(in.Outcome),
			nz2(in.RequestDigest), nz2(in.ResponseDigest), in.Bytes,
			rcptNow(in.StartedAt), rcptNow(in.CompletedAt)); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, `UPDATE actions SET execution_receipt_id=? WHERE id=?`, string(id), string(in.ActionID))
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// RecordedPreconditions 是 RECORDED 的三项前置状态。
type RecordedPreconditions struct {
	ReceiptFinalized bool
	ExchangeComplete bool
	BlobPersisted    bool
}

// AllMet 报告三项是否全满足。
func (p RecordedPreconditions) AllMet() bool {
	return p.ReceiptFinalized && p.ExchangeComplete && p.BlobPersisted
}

// Missing 返回首个未满足项的原因码（供诊断）。
func (p RecordedPreconditions) Missing() string {
	switch {
	case !p.ReceiptFinalized:
		return "receipt_not_finalized"
	case !p.ExchangeComplete:
		return "exchange_incomplete"
	case !p.BlobPersisted:
		return "blob_not_persisted"
	default:
		return ""
	}
}

// CheckRecordedPreconditions 查询 action 的三项 RECORDED 前置。
func (r *Receipts) CheckRecordedPreconditions(ctx context.Context, actionID domain.ID) (RecordedPreconditions, error) {
	var p RecordedPreconditions
	err := r.w.Durable(ctx, func(tx *sql.Tx) error {
		// ① Receipt 已终结：存在、completed_at 非空、outcome=EXECUTED。
		var n int
		if e := tx.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM execution_receipts WHERE action_id=? AND completed_at IS NOT NULL AND outcome=?`,
			string(actionID), string(domain.ActionExecuted)).Scan(&n); e != nil {
			return e
		}
		p.ReceiptFinalized = n > 0

		// ② HTTP Exchange 齐备：至少一条 evidence。
		if e := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM evidence WHERE action_id=?`, string(actionID)).Scan(&n); e != nil {
			return e
		}
		p.ExchangeComplete = n > 0

		// ③ Blob 已持久化：所有 evidence 都有 plain_sha256 + blob_path（无一缺失）。
		var missing int
		if e := tx.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM evidence WHERE action_id=? AND (plain_sha256 IS NULL OR plain_sha256='' OR blob_path IS NULL OR blob_path='')`,
			string(actionID)).Scan(&missing); e != nil {
			return e
		}
		p.BlobPersisted = p.ExchangeComplete && missing == 0
		return nil
	})
	return p, err
}

// RecordAction 在三前置全满足时把 action EXECUTED→RECORDED；否则返回 (false, 原因码)，不终结。
func (r *Receipts) RecordAction(ctx context.Context, actionID domain.ID) (bool, string, error) {
	pre, err := r.CheckRecordedPreconditions(ctx, actionID)
	if err != nil {
		return false, "", err
	}
	if !pre.AllMet() {
		return false, pre.Missing(), nil
	}
	var done bool
	err = r.w.Durable(ctx, func(tx *sql.Tx) error {
		var st string
		if e := tx.QueryRowContext(ctx, `SELECT state FROM actions WHERE id=?`, string(actionID)).Scan(&st); e != nil {
			if e == sql.ErrNoRows {
				return domain.ErrNotFound
			}
			return e
		}
		to, ok := ActionNext(domain.ActionState(st), ActionEventRecord)
		if !ok {
			return domain.Errorf(domain.ErrInvalid, fmt.Sprintf("action 状态 %s 不接受 record", st))
		}
		res, e := tx.ExecContext(ctx, `UPDATE actions SET state=?, completed_at=? WHERE id=? AND state=?`,
			string(to), rcptNow(time.Now()), string(actionID), st)
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n == 1 {
			done = true
		}
		return nil
	})
	return done, "", err
}

func nzID(id domain.ID) any {
	if id == "" {
		return nil
	}
	return string(id)
}
func nz2(s string) any {
	if s == "" {
		return nil
	}
	return s
}
