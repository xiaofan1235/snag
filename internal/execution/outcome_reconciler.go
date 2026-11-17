package execution

import (
	"context"
	"database/sql"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// outcome_reconciler.go —— 三种孤儿对账（§5.3 / §0.5）。
//
// 执行→证据→记录的闭环收口。一个动作要进 RECORDED 必须 Receipt 与 Evidence **成对齐备**；
// 单边孤儿不得自动标成功：
//   - Receipt-only（有回执、缺证据）：EXECUTED → OUTCOME_UNKNOWN 待配对；
//   - Evidence-only（有证据、缺回执）：DISPATCHED → OUTCOME_UNKNOWN 待配对；
//   - 孤儿 Blob（有 blob、无 evidence 元数据）：定期回收（见 ReclaimNote）。
// 非幂等写未对账前禁止同 Job 增代重发（§4.1，由 CanRedispatchUnreconciled + Job RECONCILING 把关）。

// OutcomeReconciler 扫描单边孤儿、转 OUTCOME_UNKNOWN。
type OutcomeReconciler struct {
	w *store.Writer
}

// NewOutcomeReconciler 构造。
func NewOutcomeReconciler(w *store.Writer) *OutcomeReconciler { return &OutcomeReconciler{w: w} }

// ReconcileReport 汇总一次对账。
type ReconcileReport struct {
	ReceiptOnly  int // 有回执缺证据 → OUTCOME_UNKNOWN
	EvidenceOnly int // 有证据缺回执 → OUTCOME_UNKNOWN
	Paired       int // 两者齐备（留给 RECORDED）
}

// ReconcileOrphans 扫描 DISPATCHED/EXECUTED 动作，单边孤儿转 OUTCOME_UNKNOWN（不自动成功）。
// 两者齐备的留给 Receipts.RecordAction 走 RECORDED；两者皆无的视为在飞，不动。
func (o *OutcomeReconciler) ReconcileOrphans(ctx context.Context) (ReconcileReport, error) {
	var rep ReconcileReport
	err := o.w.Durable(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx,
			`SELECT id, state FROM actions WHERE state IN (?,?)`,
			string(domain.ActionDispatched), string(domain.ActionExecuted))
		if e != nil {
			return e
		}
		type act struct{ id, state string }
		var acts []act
		for rows.Next() {
			var a act
			if e := rows.Scan(&a.id, &a.state); e != nil {
				rows.Close()
				return e
			}
			acts = append(acts, a)
		}
		if e := rows.Err(); e != nil {
			rows.Close()
			return e
		}
		rows.Close()

		for _, a := range acts {
			hasReceipt, e := existsReceipt(ctx, tx, a.id)
			if e != nil {
				return e
			}
			hasEvidence, e := existsEvidenceBlob(ctx, tx, a.id)
			if e != nil {
				return e
			}
			switch {
			case hasReceipt && hasEvidence:
				rep.Paired++
			case hasReceipt != hasEvidence: // 单边孤儿
				to, ok := ActionNext(domain.ActionState(a.state), ActionEventUnknown)
				if !ok {
					continue // 该态不接受 unknown（不应发生于 DISPATCHED/EXECUTED）
				}
				res, e := tx.ExecContext(ctx, `UPDATE actions SET state=? WHERE id=? AND state=?`,
					string(to), a.id, a.state)
				if e != nil {
					return e
				}
				if n, _ := res.RowsAffected(); n == 1 {
					if hasReceipt {
						rep.ReceiptOnly++
					} else {
						rep.EvidenceOnly++
					}
				}
			default:
				// 两者皆无：在飞，不动（真卡住的由崩溃恢复/超时处理，Day 12）。
			}
		}
		return nil
	})
	return rep, err
}

func existsReceipt(ctx context.Context, tx *sql.Tx, actionID string) (bool, error) {
	var n int
	e := tx.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM execution_receipts WHERE action_id=? AND completed_at IS NOT NULL AND outcome=?`,
		actionID, string(domain.ActionExecuted)).Scan(&n)
	return n > 0, e
}

func existsEvidenceBlob(ctx context.Context, tx *sql.Tx, actionID string) (bool, error) {
	var n int
	e := tx.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM evidence WHERE action_id=? AND plain_sha256 IS NOT NULL AND plain_sha256<>'' AND blob_path IS NOT NULL AND blob_path<>''`,
		actionID).Scan(&n)
	return n > 0, e
}
