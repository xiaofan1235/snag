// Package oast 实现带外验证（OAST）的关联生命周期（§44.3）：盲注/异步/业务回调的带外确认。
//
// 自建 OAST 服务由用户预配置（interactsh 式），地址不由模型选（§8.2）。关联生命周期：
//
//	create_correlation → 得不可预测关联 ID → 嵌入 payload 经正常 Action 发往目标 → 目标异步回连 →
//	按关联 ID 配回原 action/experiment → 成为 oast_event 证据；超时未回 = UNABLE_TO_VERIFY（不等于不存在）。
//
// 安全：回调内容不可信；关联 ID 不可预测；不同 Run 关联隔离。
package oast

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// 关联状态（§44.3）。
const (
	StatePending  = "PENDING"
	StateResolved = "RESOLVED"
	StateTimeout  = "TIMEOUT"
)

// 验证结论（VerificationVerdict 子集，§13）。
const (
	VerdictConfirmed      = "CONFIRMED"
	VerdictUnableToVerify = "UNABLE_TO_VERIFY" // 超时未回：未证实（不等于不存在）
)

// Store 管理 OAST 关联。
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

// newToken 生成不可预测的关联 token（128 位随机）。
func newToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// CreateCorrelation 创建一个关联（state=PENDING），token 不可预测、绑定 action/experiment/run。
// 返回 (correlationID, token)。token 嵌入 payload 发往目标。
func (s *Store) CreateCorrelation(ctx context.Context, runID, actionID, experimentID, kind string, now time.Time) (string, string, error) {
	id := string(domain.NewID())
	token := newToken()
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO oast_correlations (id, run_id, action_id, experiment_id, token, interaction_kind, state, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			id, runID, nz(actionID), nz(experimentID), token, nz(kind), StatePending, ts(now))
		return e
	})
	if err != nil {
		return "", "", err
	}
	return id, token, nil
}

// ResolveCallback 收到目标异步回连：按 (run, token) 配回关联，置 RESOLVED，挂 oast_event 证据。
// 不同 Run 关联隔离（必须同 run_id + token）。回调内容（sourceIP/detail）是不可信数据，仅作证据存。
// 返回被配回的 correlationID；无匹配返回 ErrNotFound。
func (s *Store) ResolveCallback(ctx context.Context, runID, token, sourceIP, detailJSON, evidenceID string, now time.Time) (string, error) {
	var id string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		var state string
		e := tx.QueryRowContext(ctx,
			`SELECT id, state FROM oast_correlations WHERE run_id=? AND token=?`, runID, token).Scan(&id, &state)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "no correlation for token (run isolation)")
		}
		if e != nil {
			return e
		}
		if state != StatePending {
			return nil // 幂等：已 RESOLVED/TIMEOUT 不覆盖
		}
		_, e = tx.ExecContext(ctx, `
			UPDATE oast_correlations SET state=?, received_at=?, source_ip=?, detail_json=?, oast_event_evidence_id=?
			WHERE id=?`,
			StateResolved, ts(now), nz(sourceIP), nz(detailJSON), nz(evidenceID), id)
		return e
	})
	return id, err
}

// ExpireTimeouts 把超过 timeout 仍 PENDING 的关联置 TIMEOUT。cutoff = now - timeout。返回置 TIMEOUT 的数量。
func (s *Store) ExpireTimeouts(ctx context.Context, cutoff time.Time) (int, error) {
	n := 0
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		r, e := tx.ExecContext(ctx,
			`UPDATE oast_correlations SET state=? WHERE state=? AND created_at < ?`,
			StateTimeout, StatePending, ts(cutoff))
		if e != nil {
			return e
		}
		aff, _ := r.RowsAffected()
		n = int(aff)
		return nil
	})
	return n, err
}

// Verdict 返回某关联的带外验证结论：RESOLVED→CONFIRMED，TIMEOUT→UNABLE_TO_VERIFY，PENDING→""（待定）。
func (s *Store) Verdict(ctx context.Context, correlationID string) (string, error) {
	var state string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT state FROM oast_correlations WHERE id=?`, correlationID).Scan(&state)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "correlation "+correlationID)
		}
		return e
	})
	if err != nil {
		return "", err
	}
	switch state {
	case StateResolved:
		return VerdictConfirmed, nil
	case StateTimeout:
		return VerdictUnableToVerify, nil
	default:
		return "", nil // PENDING：pending_oast，Finding 停在 NEEDS_EVIDENCE
	}
}

// State 读关联状态。
func (s *Store) State(ctx context.Context, correlationID string) (string, error) {
	var state string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT state FROM oast_correlations WHERE id=?`, correlationID).Scan(&state)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "correlation "+correlationID)
		}
		return e
	})
	return state, err
}
