package findings

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
)

// discovery.go —— Discovery 线索管理（§12.4）。所有非漏洞线索（新接口/异常响应/身份差异/敏感键名/WAF
// 规律/签名机制/需其他身份入口…）进入 Discovery，走状态机；**退出前所有高价值 Discovery 必须有处置**。

// Discovery 状态（§12.4）。
const (
	DiscOpen      = "OPEN"
	DiscFollowing = "FOLLOWING"
	DiscFollowed  = "FOLLOWED"
	DiscDismissed = "DISMISSED"
	DiscPromoted  = "PROMOTED"
	DiscBlocked   = "BLOCKED"
)

// 状态机合法迁移（DISMISSED/PROMOTED 为终态）。
var discTransitions = map[string]map[string]bool{
	DiscOpen:      {DiscFollowing: true, DiscDismissed: true, DiscBlocked: true},
	DiscFollowing: {DiscFollowed: true, DiscDismissed: true, DiscBlocked: true},
	DiscFollowed:  {DiscPromoted: true, DiscDismissed: true},
	DiscBlocked:   {DiscFollowing: true, DiscDismissed: true},
}

// 处置语义：OPEN/FOLLOWING = 未处置（pending）；其余（FOLLOWED/DISMISSED/PROMOTED/BLOCKED）= 有处置结果。

// ErrBadDiscoveryTransition —— 非法 Discovery 状态迁移。
var ErrBadDiscoveryTransition = &domain.DomainError{Code: "BAD_DISCOVERY_TRANSITION", Msg: "invalid discovery state transition (§12.4)"}

// AddDiscovery 新建一条线索（state=OPEN）。highValue 标高价值（退出前必须有处置）。
func (s *Store) AddDiscovery(ctx context.Context, runID, kind string, highValue bool, detailJSON string, now time.Time) (string, error) {
	id := string(domain.NewID())
	hv := 0
	if highValue {
		hv = 1
	}
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO discoveries (id, run_id, kind, state, high_value, detail_json, created_at)
			VALUES (?,?,?,?,?,?,?)`,
			id, runID, kind, DiscOpen, hv, nz(detailJSON), ts(now))
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// TransitionDiscovery 迁移线索状态；非法迁移返回 ErrBadDiscoveryTransition（fail-closed）。
func (s *Store) TransitionDiscovery(ctx context.Context, id, to string, now time.Time) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		var cur string
		e := tx.QueryRowContext(ctx, `SELECT state FROM discoveries WHERE id=?`, id).Scan(&cur)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "discovery "+id)
		}
		if e != nil {
			return e
		}
		if !discTransitions[cur][to] {
			return domain.Errorf(ErrBadDiscoveryTransition, cur+"→"+to)
		}
		_, e = tx.ExecContext(ctx, `UPDATE discoveries SET state=? WHERE id=?`, to, id)
		return e
	})
}

// DiscoveryState 读某线索当前状态。
func (s *Store) DiscoveryState(ctx context.Context, id string) (string, error) {
	var st string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT state FROM discoveries WHERE id=?`, id).Scan(&st)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "discovery "+id)
		}
		return e
	})
	return st, err
}

// HighValueUndisposed 返回仍未处置（OPEN/FOLLOWING）的高价值线索 id——退出前必须为空（§12.4）。
func (s *Store) HighValueUndisposed(ctx context.Context, runID string) ([]string, error) {
	var ids []string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `
			SELECT id FROM discoveries
			WHERE run_id=? AND high_value=1 AND state IN (?,?) ORDER BY created_at`,
			runID, DiscOpen, DiscFollowing)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				return e
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

// ReadyToExit 报告某 Run 是否所有高价值 Discovery 都已处置（§12.4 退出前置条件之一）。
func (s *Store) ReadyToExit(ctx context.Context, runID string) (bool, error) {
	ids, err := s.HighValueUndisposed(ctx, runID)
	return len(ids) == 0, err
}
