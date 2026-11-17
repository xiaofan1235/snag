package experiments

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"pentagent/internal/domain"
)

// race.go —— 受控竞态实验（§44.2），解串行化张力。竞态改的是**调度的串行约束**，不是**执法**：
// 独立 risk_class=concurrent_write，默认 deny、须显式审批；仅单端点/单身份/有界并发；**每个并发请求仍
// 逐一过 Policy Proxy（Scope/授权窗/Fencing/各消耗一个 Request Capability）**——只豁免「同端点写串行」
// 这一条调度规则，其它红线一条不豁免。副作用进对账（RECONCILING）。

const (
	RaceProposed = "PROPOSED"
	RaceApproved = "APPROVED"
	RaceDone     = "DONE"
	RaceDenied   = "DENIED"
)

// DefaultConcurrencyCap 并发度硬上限（配置上限，§44.2）。
const DefaultConcurrencyCap = 50

var (
	// ErrNeedsApproval —— 竞态实验未经人工审批（risk_class=concurrent_write 默认 deny，§9.6）。
	ErrNeedsApproval = &domain.DomainError{Code: "NEEDS_APPROVAL", Msg: "race experiment requires explicit approval (§44.2)"}
	// ErrConcurrencyExceeded —— 并发度超上限。
	ErrConcurrencyExceeded = &domain.DomainError{Code: "CONCURRENCY_EXCEEDED", Msg: "race concurrency exceeds cap"}
)

// RaceSpec 是竞态实验规格（§44.2）。
type RaceSpec struct {
	HypothesisID string
	EndpointID   string
	IdentityID   string
	Concurrency  int
	SyncStrategy string // last_byte | batch
	MaxTotal     int
	Assertion    string
}

// CreateRace 持久化竞态实验（state=PROPOSED，默认须审批）。并发度超上限直接拒。
func (s *Store) CreateRace(ctx context.Context, runID string, spec RaceSpec, now time.Time) (string, error) {
	if spec.Concurrency <= 0 || spec.Concurrency > DefaultConcurrencyCap {
		return "", ErrConcurrencyExceeded
	}
	if spec.SyncStrategy == "" {
		spec.SyncStrategy = "batch"
	}
	id := string(domain.NewID())
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO race_experiments (id, run_id, hypothesis_id, endpoint_id, identity_id, concurrency,
			  sync_strategy, max_total_requests, requires_approval, approval_state, risk_class, assertion,
			  state, success_count, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, runID, nz(spec.HypothesisID), nz(spec.EndpointID), nz(spec.IdentityID), spec.Concurrency,
			spec.SyncStrategy, spec.MaxTotal, 1, RaceProposed, "concurrent_write", nz(spec.Assertion),
			RaceProposed, 0, ts(now), ts(now))
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// Approve 人工审批通过（PROPOSED→APPROVED）。
func (s *Store) Approve(ctx context.Context, raceID string, now time.Time) error {
	return s.w.Durable(ctx, func(tx *sql.Tx) error {
		r, e := tx.ExecContext(ctx,
			`UPDATE race_experiments SET approval_state='APPROVED', state=?, updated_at=? WHERE id=? AND state=?`,
			RaceApproved, ts(now), raceID, RaceProposed)
		if e != nil {
			return e
		}
		if n, _ := r.RowsAffected(); n == 0 {
			return domain.Errorf(domain.ErrConflict, "race not in PROPOSED")
		}
		return nil
	})
}

// PerRequestCheck 是每个并发请求的逐请求执法（Proxy 的 Scope/授权窗/Fencing/Capability）。
// 返回该请求是否被放行；竞态只豁免串行，不豁免这里的执法。
type PerRequestCheck func(reqIndex int) (allowed bool, reason string)

// FireFunc 对一个被放行的请求实际发包，返回是否「成功」（按 assertion，如创建成功/扣款成功）。
type FireFunc func(reqIndex int) bool

// RaceOutcome 是竞态执行结果。
type RaceOutcome struct {
	Approved     bool
	Dispatched   int // 实际发出的请求数（通过逐请求执法的）
	Denied       int // 被逐请求执法拒绝的（执法未豁免）
	SuccessCount int // 成功次数（> 1 即命中竞态，assertion）
}

// RunRace 执行竞态：仅当已审批才跑；并发 concurrency 个请求，**每个仍过 PerRequestCheck 执法**（被拒的
// 不发包、计入 Denied），通过的并发发包（豁免串行）。统计成功次数。未审批返回 ErrNeedsApproval。
func (s *Store) RunRace(ctx context.Context, raceID string, check PerRequestCheck, fire FireFunc, now time.Time) (RaceOutcome, error) {
	var out RaceOutcome
	var concurrency, maxTotal int
	var state string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT concurrency, max_total_requests, state FROM race_experiments WHERE id=?`, raceID).
			Scan(&concurrency, &maxTotal, &state)
	})
	if err == sql.ErrNoRows {
		return out, domain.Errorf(domain.ErrNotFound, "race "+raceID)
	}
	if err != nil {
		return out, err
	}
	if state != RaceApproved {
		return out, ErrNeedsApproval // 默认 deny，未审批不跑
	}
	n := concurrency
	if maxTotal > 0 && n > maxTotal {
		n = maxTotal
	}

	// 逐请求执法（可并发，但每个独立过 check）。
	type res struct {
		allowed bool
		success bool
	}
	results := make([]res, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			allowed, _ := check(i) // 执法不豁免：Scope/Fencing/Capability 逐请求校验
			if !allowed {
				results[i] = res{allowed: false}
				return
			}
			results[i] = res{allowed: true, success: fire(i)} // 通过执法者并发发包（豁免串行）
		}(i)
	}
	wg.Wait()

	for _, r := range results {
		if !r.allowed {
			out.Denied++
			continue
		}
		out.Dispatched++
		if r.success {
			out.SuccessCount++
		}
	}
	out.Approved = true

	// 回写成功计数 + 置 DONE（副作用对账在执行面 RECONCILING 处理）。
	_ = s.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`UPDATE race_experiments SET success_count=?, state=?, updated_at=? WHERE id=?`,
			out.SuccessCount, RaceDone, ts(now), raceID)
		return e
	})
	return out, nil
}
