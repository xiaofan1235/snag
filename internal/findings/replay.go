package findings

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
)

// replay.go —— 重放 + 反证自动降级（§13.4/§13.5）。REPORTABLE 非一次性授予：VerificationVerdict 带 revision，
// 反证/哈希失败/Evidence 失效/依赖 supersede 时自动降级。重放是独立 Verification Job→新 Attempt→重新 Policy
// →新 Evidence→对比→写 Verdict（§13.5），不是读旧文件说「确认」。

// 验证结论（§13.4）。
const (
	VerdictConfirmed      = "CONFIRMED"
	VerdictRejected       = "REJECTED"
	VerdictNeedsEvidence  = "NEEDS_EVIDENCE"
	VerdictUnableToVerify = "UNABLE_TO_VERIFY"
)

// 降级原因（§13.4）。
const (
	DowngradeCounterEvidence = "replay_counter_evidence"  // 新 Replay 反证
	DowngradeHashFail        = "evidence_hash_fail"       // 关联 Evidence 哈希失败（+Quarantine）
	DowngradeEvidenceGone    = "evidence_deleted_expired" // 关联 Evidence 删除/过期
	DowngradeSuperseded      = "dependent_verdict_superseded"
)

// DowngradeTarget 纯函数：按降级原因给出 Finding 的目标状态（§13.4）。反证可直接到 REJECTED（强反证）或
// NEEDS_EVIDENCE；哈希失败/证据失效/依赖 supersede → NEEDS_EVIDENCE（需重新取证/重估）。
func DowngradeTarget(reason string, strongRefutation bool) string {
	switch reason {
	case DowngradeCounterEvidence:
		if strongRefutation {
			return FindRejected
		}
		return FindNeedsEvidence
	default:
		return FindNeedsEvidence
	}
}

// WriteVerdict 写一条新的 VerificationVerdict（revision 递增），并把该 Finding 旧结论 supersede。
func (s *Store) WriteVerdict(ctx context.Context, findingID, runID, verdict, reason string, findingRevision int, evidenceManifest string, now time.Time) (int, error) {
	var rev int
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		var maxRev int
		_ = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(revision),0) FROM verification_verdicts WHERE finding_id=?`, findingID).Scan(&maxRev)
		rev = maxRev + 1
		// 旧结论 supersede。
		if _, e := tx.ExecContext(ctx,
			`UPDATE verification_verdicts SET superseded=1 WHERE finding_id=? AND superseded=0`, findingID); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, `
			INSERT INTO verification_verdicts (id, finding_id, run_id, revision, verdict, finding_revision, evidence_manifest_json, superseded, reason, created_at)
			VALUES (?,?,?,?,?,?,?,0,?,?)`,
			string(domain.NewID()), findingID, runID, rev, verdict, findingRevision, nz(evidenceManifest), nz(reason), ts(now))
		return e
	})
	return rev, err
}

// ApplyDowngrade 对一个已 REPORTABLE（或更高态）的 Finding 施加反证自动降级（§13.4）：写新 Verdict + 迁移
// Finding 到目标状态（Verifier 发起）+ 哈希失败时把关联 Evidence 进 Quarantine。
func (s *Store) ApplyDowngrade(ctx context.Context, findingID, runID, reason string, strongRefutation bool, quarantineEvidenceIDs []string, now time.Time) (string, error) {
	target := DowngradeTarget(reason, strongRefutation)
	verdict := VerdictNeedsEvidence
	if target == FindRejected {
		verdict = VerdictRejected
	}
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		var cur string
		e := tx.QueryRowContext(ctx, `SELECT state FROM findings WHERE id=?`, findingID).Scan(&cur)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "finding "+findingID)
		}
		if e != nil {
			return e
		}
		// 允许从 CONFIRMED/TRIAGED/REPORTABLE 降级（状态机已含这些边）。
		if !findTransitions[cur][target] {
			return domain.Errorf(ErrBadFindingTransition, cur+"→"+target)
		}
		if _, e = tx.ExecContext(ctx, `UPDATE findings SET state=?, updated_at=? WHERE id=?`, target, ts(now), findingID); e != nil {
			return e
		}
		// 哈希失败 → 关联 Evidence 进 Quarantine。
		if reason == DowngradeHashFail {
			for _, ev := range quarantineEvidenceIDs {
				if _, e = tx.ExecContext(ctx, `UPDATE evidence SET quarantined=1 WHERE id=?`, ev); e != nil {
					return e
				}
			}
		}
		// 写新 Verdict + supersede 旧的。
		var maxRev int
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision),0) FROM verification_verdicts WHERE finding_id=?`, findingID).Scan(&maxRev)
		if _, e = tx.ExecContext(ctx, `UPDATE verification_verdicts SET superseded=1 WHERE finding_id=? AND superseded=0`, findingID); e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `
			INSERT INTO verification_verdicts (id, finding_id, run_id, revision, verdict, finding_revision, superseded, reason, created_at)
			VALUES (?,?,?,?,?,1,0,?,?)`,
			string(domain.NewID()), findingID, runID, maxRev+1, verdict, reason, ts(now))
		return e
	})
	return target, err
}

// EvidenceQuarantined 读某 Evidence 是否已隔离（测试/报告用）。
func (s *Store) EvidenceQuarantined(ctx context.Context, evidenceID string) (bool, error) {
	var q int
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT quarantined FROM evidence WHERE id=?`, evidenceID).Scan(&q)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "evidence "+evidenceID)
		}
		return e
	})
	return q == 1, err
}

// ActiveVerdict 读某 Finding 当前未被 supersede 的结论（revision, verdict）。
func (s *Store) ActiveVerdict(ctx context.Context, findingID string) (int, string, error) {
	var rev int
	var v string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx,
			`SELECT revision, verdict FROM verification_verdicts WHERE finding_id=? AND superseded=0 ORDER BY revision DESC LIMIT 1`, findingID).
			Scan(&rev, &v)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "no active verdict")
		}
		return e
	})
	return rev, v, err
}
