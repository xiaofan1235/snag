// Package agent 是自主 Agent Runtime（M3）。本文件实现模型预算账本（§7.5 / §31 A4）。
//
// 预算要「预留 + 结算」，且预留本身是状态机：RESERVED→IN_FLIGHT→SUCCEEDED/UNKNOWN→SETTLED。
// 为什么：多个 Agent 并发调用若不先原子预留，可能一起突破预算；崩溃后若自动释放 IN_FLIGHT/UNKNOWN
// 的预留，会「既泄漏预算又重复扣费」（Provider 已计费但 Core 崩了）。故崩溃后 UNKNOWN 按最坏成本
// 保留至对账确认，绝不自动释放（§22.6「无 UNKNOWN 预算预留泄漏」）。
//
// 金额一律整数 micro_usd、Token 整数，全程不碰 float（§31 C7）。
package agent

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// 预留状态（§7.5）。
const (
	ResReserved  = "RESERVED"
	ResInFlight  = "IN_FLIGHT"
	ResSucceeded = "SUCCEEDED"
	ResUnknown   = "UNKNOWN"
	ResSettled   = "SETTLED"
)

// 结算原因。
const (
	SettleSucceeded       = "succeeded"        // 正常结算
	SettleUnknownReleased = "unknown_released" // 对账确认未计费 → 释放
	SettleUnknownBilled   = "unknown_billed"   // 对账确认已计费 → 按实际结算
)

// ErrBudgetExceeded —— 预留会突破预算上限（含对 finalization_reserve 的保护）。
var ErrBudgetExceeded = &domain.DomainError{Code: "BUDGET_EXCEEDED", Msg: "reservation would exceed run budget"}

// Pricing 是可配置、带版本的计费表（§31 A4）。单价为「每百万 Token 的 micro_usd」，
// 成本按整数运算后除以 1e6，全程不碰 float。Version 空 → 视为「估算/未知」。
type Pricing struct {
	Version           string
	InputPerMTok      int64 // input token 单价（micro_usd / 1M tok）
	OutputPerMTok     int64
	CacheWritePerMTok int64
	CacheReadPerMTok  int64
	ReasoningPerMTok  int64
}

// Estimated 报告该定价是否为「无价估算」（版本空）。
func (p Pricing) Estimated() bool { return p.Version == "" }

// TokenUsage 是多维 Token 计数（§31 A4：input/output/cache-write/cache-read/reasoning 分列）。
type TokenUsage struct {
	Input      int
	Output     int
	CacheWrite int
	CacheRead  int
	Reasoning  int
}

// Cost 按 pricing 计算这笔用量的 micro_usd（整数；分维度累加后统一 /1e6）。
func (p Pricing) Cost(u TokenUsage) int64 {
	micro := int64(u.Input)*p.InputPerMTok +
		int64(u.Output)*p.OutputPerMTok +
		int64(u.CacheWrite)*p.CacheWritePerMTok +
		int64(u.CacheRead)*p.CacheReadPerMTok +
		int64(u.Reasoning)*p.ReasoningPerMTok
	return micro / 1_000_000
}

// Limits 是 Run 级预算维度（§7.5）。金额 micro_usd、Token 整数。0 表示该维度不设限。
type Limits struct {
	TotalMicroUSD       int64 // Run 总费用上限
	FinalizationReserve int64 // 从 Total 划出的只读收尾额度：普通 Executor 耗尽后仍可产 PARTIAL（§7.5）
	MaxOutputPerCall    int   // 单模型调用输出 Token 上限
}

// Reservation 是一条预留的快照。
type Reservation struct {
	ID                string
	RunID             string
	JobID             string
	ProviderRequestID string
	State             string
	SettleReason      string
	WorstMicroUSD     int64
	ReservedIn        int
	ReservedOut       int
	Finalization      bool
	Actual            TokenUsage
	SettledMicroUSD   int64
	PricingVersion    string
}

// ReserveRequest 是一次预留请求。
type ReserveRequest struct {
	RunID             string
	JobID             string
	ProviderRequestID string // 幂等键（空则不去重）
	EstInputTokens    int    // 预估输入 Token
	MaxOutputTokens   int    // 本次调用 max_output
	Pricing           Pricing
	Finalization      bool // true 则动用收尾额度（仅限最终复盘/Verifier/报告，§7.5）
}

// Ledger 是预算账本，所有读写经单写者 Store（Durable tx 串行化 → 原子 check-and-reserve）。
type Ledger struct{ w *store.Writer }

// NewLedger 构造。
func NewLedger(w *store.Writer) *Ledger { return &Ledger{w: w} }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// worstCost 估算最坏成本：输入按预估、输出按 max_output 满额（§7.5 调用前按 input+max_output 预留）。
func worstCost(req ReserveRequest) int64 {
	return req.Pricing.Cost(TokenUsage{Input: req.EstInputTokens, Output: req.MaxOutputTokens})
}

// committedTx 在 tx 内计算 Run 已占用的 micro_usd，按是否 finalization 分列返回 (normal, final)。
// 活跃预留（RESERVED/IN_FLIGHT/UNKNOWN）计最坏成本；SETTLED 计真实结算成本。
func committedTx(ctx context.Context, tx *sql.Tx, runID string) (normal, final int64, err error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT finalization, state, worst_micro_usd, settled_micro_usd
		FROM budget_reservations WHERE run_id=?`, runID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var fin int
		var state string
		var worst, settled int64
		if err = rows.Scan(&fin, &state, &worst, &settled); err != nil {
			return 0, 0, err
		}
		var amt int64
		switch state {
		case ResReserved, ResInFlight, ResUnknown:
			amt = worst
		case ResSettled:
			amt = settled
		}
		if fin == 1 {
			final += amt
		} else {
			normal += amt
		}
	}
	return normal, final, rows.Err()
}

// getByPRID 在 tx 内按幂等键取已有预留；无则返回 (nil, nil)。
func getByPRID(ctx context.Context, tx *sql.Tx, prid string) (*Reservation, error) {
	if prid == "" {
		return nil, nil
	}
	r := &Reservation{}
	var fin int
	e := tx.QueryRowContext(ctx, `
		SELECT id, run_id, COALESCE(job_id,''), COALESCE(provider_request_id,''), state,
		       COALESCE(settle_reason,''), worst_micro_usd, reserved_in_tokens, reserved_out_tokens,
		       finalization, settled_micro_usd, COALESCE(pricing_version,'')
		FROM budget_reservations WHERE provider_request_id=?`, prid).
		Scan(&r.ID, &r.RunID, &r.JobID, &r.ProviderRequestID, &r.State, &r.SettleReason,
			&r.WorstMicroUSD, &r.ReservedIn, &r.ReservedOut, &fin, &r.SettledMicroUSD, &r.PricingVersion)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	r.Finalization = fin == 1
	return r, nil
}

// Reserve 原子预留（§7.5 调用前）：计算最坏成本，在同一 durable tx 内「查已占用 + 比上限 + 插入 RESERVED」。
// 幂等键命中已有预留 → 直接返回（防崩溃重试重复扣费）。超限返回 ErrBudgetExceeded。
func (l *Ledger) Reserve(ctx context.Context, req ReserveRequest, lim Limits, now time.Time) (Reservation, error) {
	var out Reservation
	worst := worstCost(req)
	err := l.w.Durable(ctx, func(tx *sql.Tx) error {
		// 幂等：同一 provider_request_id 不重复预留。
		if existing, e := getByPRID(ctx, tx, req.ProviderRequestID); e != nil {
			return e
		} else if existing != nil {
			out = *existing
			return nil
		}

		normal, final, e := committedTx(ctx, tx, req.RunID)
		if e != nil {
			return e
		}
		// 上限检查：普通预留不得吃进 finalization_reserve；收尾预留可用到 Total。
		if lim.TotalMicroUSD > 0 {
			if req.Finalization {
				if normal+final+worst > lim.TotalMicroUSD {
					return domain.Errorf(ErrBudgetExceeded, "finalization reserve exhausted")
				}
			} else {
				limitCap := lim.TotalMicroUSD - lim.FinalizationReserve
				if normal+worst > limitCap {
					return domain.Errorf(ErrBudgetExceeded, "run budget exhausted (reserve protected)")
				}
			}
		}

		id := string(domain.NewID())
		fin := 0
		if req.Finalization {
			fin = 1
		}
		tstr := ts(now)
		if _, e = tx.ExecContext(ctx, `
			INSERT INTO budget_reservations
			  (id, run_id, job_id, provider_request_id, state, worst_micro_usd,
			   reserved_in_tokens, reserved_out_tokens, finalization, pricing_version, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, req.RunID, nz(req.JobID), nz(req.ProviderRequestID), ResReserved, worst,
			req.EstInputTokens, req.MaxOutputTokens, fin, req.Pricing.Version, tstr, tstr); e != nil {
			return e
		}
		auditLedger(ctx, tx, req.RunID, "RESERVED", worst, id, tstr)
		out = Reservation{
			ID: id, RunID: req.RunID, JobID: req.JobID, ProviderRequestID: req.ProviderRequestID,
			State: ResReserved, WorstMicroUSD: worst, ReservedIn: req.EstInputTokens,
			ReservedOut: req.MaxOutputTokens, Finalization: req.Finalization, PricingVersion: req.Pricing.Version,
		}
		return nil
	})
	return out, err
}

// MarkInFlight 置 RESERVED→IN_FLIGHT（调用真正发出）。
func (l *Ledger) MarkInFlight(ctx context.Context, resID string, now time.Time) error {
	return l.transition(ctx, resID, []string{ResReserved}, ResInFlight, "", now)
}

// Settle 结算（IN_FLIGHT/RESERVED→SETTLED，经 SUCCEEDED 概念态）：按真实多维 Usage 记账，
// 释放「最坏预留 - 真实成本」的差额（committed 随之下降）。
func (l *Ledger) Settle(ctx context.Context, resID string, actual TokenUsage, pricing Pricing, now time.Time) error {
	settled := pricing.Cost(actual)
	return l.w.Durable(ctx, func(tx *sql.Tx) error {
		state, runID, err := lockState(ctx, tx, resID)
		if err != nil {
			return err
		}
		if state != ResInFlight && state != ResReserved {
			return domain.Errorf(domain.ErrConflict, "settle from non-active state "+state)
		}
		tstr := ts(now)
		if _, err = tx.ExecContext(ctx, `
			UPDATE budget_reservations SET state=?, settle_reason=?, settled_micro_usd=?,
			  actual_in_tokens=?, actual_out_tokens=?, actual_cache_write=?, actual_cache_read=?,
			  actual_reasoning=?, pricing_version=?, updated_at=?
			WHERE id=?`,
			ResSettled, SettleSucceeded, settled, actual.Input, actual.Output, actual.CacheWrite,
			actual.CacheRead, actual.Reasoning, pricing.Version, tstr, resID); err != nil {
			return err
		}
		auditLedger(ctx, tx, runID, "SETTLED", settled, resID, tstr)
		return nil
	})
}

// MarkUnknown 置 IN_FLIGHT→UNKNOWN（崩溃/超时，不知是否计费）。**不释放预留**——按最坏保留至对账（§7.5）。
func (l *Ledger) MarkUnknown(ctx context.Context, resID string, now time.Time) error {
	return l.transition(ctx, resID, []string{ResInFlight, ResReserved}, ResUnknown, "", now)
}

// SettleUnknown 对账后收束 UNKNOWN→SETTLED。billed=false：确认未计费 → 释放全部预留（结算 0）；
// billed=true：确认已计费 → 按真实 Usage 结算。
func (l *Ledger) SettleUnknown(ctx context.Context, resID string, billed bool, actual TokenUsage, pricing Pricing, now time.Time) error {
	return l.w.Durable(ctx, func(tx *sql.Tx) error {
		state, runID, err := lockState(ctx, tx, resID)
		if err != nil {
			return err
		}
		if state != ResUnknown {
			return domain.Errorf(domain.ErrConflict, "settle-unknown from non-UNKNOWN state "+state)
		}
		reason := SettleUnknownReleased
		var settled int64
		var u TokenUsage
		if billed {
			reason, u, settled = SettleUnknownBilled, actual, pricing.Cost(actual)
		}
		tstr := ts(now)
		if _, err = tx.ExecContext(ctx, `
			UPDATE budget_reservations SET state=?, settle_reason=?, settled_micro_usd=?,
			  actual_in_tokens=?, actual_out_tokens=?, actual_cache_write=?, actual_cache_read=?,
			  actual_reasoning=?, updated_at=? WHERE id=?`,
			ResSettled, reason, settled, u.Input, u.Output, u.CacheWrite, u.CacheRead, u.Reasoning,
			tstr, resID); err != nil {
			return err
		}
		auditLedger(ctx, tx, runID, "RELEASED", settled, resID, tstr)
		return nil
	})
}

// transition 是通用状态迁移：仅当当前状态在 from 集合内才迁到 to，否则 ErrConflict（fail-closed）。
func (l *Ledger) transition(ctx context.Context, resID string, from []string, to, reason string, now time.Time) error {
	return l.w.Durable(ctx, func(tx *sql.Tx) error {
		state, _, err := lockState(ctx, tx, resID)
		if err != nil {
			return err
		}
		ok := false
		for _, f := range from {
			if state == f {
				ok = true
				break
			}
		}
		if !ok {
			return domain.Errorf(domain.ErrConflict, "invalid transition from "+state+" to "+to)
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE budget_reservations SET state=?, updated_at=? WHERE id=?`, to, ts(now), resID)
		return err
	})
}

// lockState 读一条预留的当前状态与 run_id（tx 内）。
func lockState(ctx context.Context, tx *sql.Tx, resID string) (state, runID string, err error) {
	e := tx.QueryRowContext(ctx,
		`SELECT state, run_id FROM budget_reservations WHERE id=?`, resID).Scan(&state, &runID)
	if e == sql.ErrNoRows {
		return "", "", domain.Errorf(domain.ErrNotFound, "reservation "+resID)
	}
	return state, runID, e
}

// Committed 返回 Run 当前已占用 micro_usd（普通 + 收尾）。
func (l *Ledger) Committed(ctx context.Context, runID string) (int64, error) {
	var normal, final int64
	err := l.w.Durable(ctx, func(tx *sql.Tx) error {
		var e error
		normal, final, e = committedTx(ctx, tx, runID)
		return e
	})
	return normal + final, err
}

// Available 返回普通 Executor 的剩余可用 micro_usd（扣除 finalization_reserve 与已占用普通额度）。
// 可能为负（表示已耗尽）。
func (l *Ledger) Available(ctx context.Context, runID string, lim Limits) (int64, error) {
	var normal int64
	err := l.w.Durable(ctx, func(tx *sql.Tx) error {
		var e error
		normal, _, e = committedTx(ctx, tx, runID)
		return e
	})
	if err != nil {
		return 0, err
	}
	return lim.TotalMicroUSD - lim.FinalizationReserve - normal, nil
}

// Get 取一条预留快照。
func (l *Ledger) Get(ctx context.Context, resID string) (Reservation, error) {
	var r Reservation
	err := l.w.Durable(ctx, func(tx *sql.Tx) error {
		var fin int
		e := tx.QueryRowContext(ctx, `
			SELECT id, run_id, COALESCE(job_id,''), COALESCE(provider_request_id,''), state,
			       COALESCE(settle_reason,''), worst_micro_usd, reserved_in_tokens, reserved_out_tokens,
			       finalization, settled_micro_usd, COALESCE(pricing_version,''),
			       actual_in_tokens, actual_out_tokens, actual_cache_write, actual_cache_read, actual_reasoning
			FROM budget_reservations WHERE id=?`, resID).
			Scan(&r.ID, &r.RunID, &r.JobID, &r.ProviderRequestID, &r.State, &r.SettleReason,
				&r.WorstMicroUSD, &r.ReservedIn, &r.ReservedOut, &fin, &r.SettledMicroUSD, &r.PricingVersion,
				&r.Actual.Input, &r.Actual.Output, &r.Actual.CacheWrite, &r.Actual.CacheRead, &r.Actual.Reasoning)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "reservation "+resID)
		}
		r.Finalization = fin == 1
		return e
	})
	return r, err
}

// ListUnsettled 列出 Run 下所有未结算（非 SETTLED）预留 id，供对账（§22.6 清零 UNKNOWN 泄漏）。
func (l *Ledger) ListUnsettled(ctx context.Context, runID string) ([]Reservation, error) {
	var out []Reservation
	err := l.w.Durable(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `
			SELECT id, state, finalization, worst_micro_usd, COALESCE(provider_request_id,'')
			FROM budget_reservations WHERE run_id=? AND state!=? ORDER BY created_at`, runID, ResSettled)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var r Reservation
			var fin int
			if e = rows.Scan(&r.ID, &r.State, &fin, &r.WorstMicroUSD, &r.ProviderRequestID); e != nil {
				return e
			}
			r.RunID, r.Finalization = runID, fin == 1
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

// auditLedger 往 budget_ledger 追加一条审计流水（best-effort；失败不回滚预留主事务，仅审计缺条）。
func auditLedger(ctx context.Context, tx *sql.Tx, runID, kind string, amount int64, refID, tstr string) {
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO budget_ledger (id, run_id, scope, kind, amount, unit, ref_type, ref_id, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		string(domain.NewID()), runID, "run", kind, amount, "micro_usd", "budget_reservation", refID, tstr)
}

// nz 把空串转成 NULL 存储（保持 provider_request_id 的部分唯一索引语义）。
func nz(s string) any {
	if s == "" {
		return nil
	}
	return s
}
