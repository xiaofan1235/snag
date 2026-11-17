package execution_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/execution"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// permit_test.go —— Day 17 §7：两层 Permit 验收。
// 命令：go test -race -tags sqlite_modernc ./internal/execution/...（无驱动自动 SKIP）。
//
// 覆盖 DoD（§6）+ §4.3 #4 + §9.5 + §22.4「Permit 重放」：
//   - 一个 Permit 只能消费一次（并发消费只 1 成功）；重放/越代/过期/Run 不可执行/策略被取代均拒；
//   - 消费成功 ⇒ Action AUTHORIZED→DISPATCHED（同事务）；RevokeAttemptPermits 撤销未消费 Permit；
//   - Request Capability 超 max_subrequests 即中断。

func openPermitDB(t *testing.T) (*store.Writer, *store.Reader, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "permit.db")
	w, err := store.OpenWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	rd, err := store.OpenReader(path, 6)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rd.Close() })
	return w, rd, context.Background()
}

// seedPermitScene 造 run(RUNNING)+job(gen)+attempt(活跃,gen)+action(AUTHORIZED)。返回 ids。
func seedPermitScene(t *testing.T, w *store.Writer, ctx context.Context, gen int64) (runID, jobID, attemptID, actionID domain.ID) {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	runID, jobID, attemptID, actionID = "run-1", "job-1", "att-1", "act-1"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`, "ws-1", "a", "mpk", ts); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at) VALUES (?,?,?,?,?,?)`, string(runID), "ws-1", "r", "RUNNING", "d", ts); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO jobs (id, run_id, kind, state, generation, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`, string(jobID), string(runID), "k", "RUNNING", gen, ts, ts); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO attempts (id, job_id, generation, fencing_token, state, started_at) VALUES (?,?,?,?,?,?)`, string(attemptID), string(jobID), gen, 1, string(domain.AttemptRunning), ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO actions (id, run_id, job_id, attempt_id, tool_name, state, identity_id) VALUES (?,?,?,?,?,?,?)`, string(actionID), string(runID), string(jobID), string(attemptID), "http", string(domain.ActionAuthorized), "idn-1")
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return
}

func issuePermit(t *testing.T, pm *execution.Permits, ctx context.Context, runID, actionID, attemptID domain.ID, ttl time.Duration) domain.ID {
	t.Helper()
	id, err := pm.IssueActionPermit(ctx, execution.ActionPermitGrant{
		RunID: runID, ActionID: actionID, AttemptID: attemptID, FencingToken: 1, PolicyRevision: 0, IdentityID: "idn-1", TTL: ttl,
	})
	if err != nil {
		t.Fatalf("签发 permit: %v", err)
	}
	return id
}

// 并发消费同一 Permit：恰 1 成功，其余 ErrConflict；成功后 Action=DISPATCHED、Permit=CONSUMED。
func TestPermitConsumeOnceConcurrent(t *testing.T) {
	w, rd, ctx := openPermitDB(t)
	runID, _, attemptID, actionID := seedPermitScene(t, w, ctx, 1)
	pm := execution.NewPermits(w)
	permitID := issuePermit(t, pm, ctx, runID, actionID, attemptID, time.Hour)

	const N = 16
	var wg sync.WaitGroup
	errs := make([]error, N)
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) { defer wg.Done(); errs[i] = pm.ConsumeActionPermit(ctx, permitID, time.Now()) }(i)
	}
	wg.Wait()

	ok := 0
	for _, e := range errs {
		if e == nil {
			ok++
		} else if !errors.Is(e, domain.ErrConflict) {
			t.Errorf("落败应 ErrConflict，得 %v", e)
		}
	}
	if ok != 1 {
		t.Fatalf("并发消费应恰 1 成功，得 %d", ok)
	}
	var ps, as string
	rd.DB().QueryRow(`SELECT state FROM action_permits WHERE id=?`, string(permitID)).Scan(&ps)
	rd.DB().QueryRow(`SELECT state FROM actions WHERE id=?`, string(actionID)).Scan(&as)
	if ps != "CONSUMED" || as != "DISPATCHED" {
		t.Errorf("应 Permit=CONSUMED/Action=DISPATCHED，得 %s/%s", ps, as)
	}
}

// 重放：消费成功后再消费 → 拒绝。
func TestPermitReplayRejected(t *testing.T) {
	w, _, ctx := openPermitDB(t)
	runID, _, attemptID, actionID := seedPermitScene(t, w, ctx, 1)
	pm := execution.NewPermits(w)
	permitID := issuePermit(t, pm, ctx, runID, actionID, attemptID, time.Hour)
	if e := pm.ConsumeActionPermit(ctx, permitID, time.Now()); e != nil {
		t.Fatalf("首次消费应成功: %v", e)
	}
	if e := pm.ConsumeActionPermit(ctx, permitID, time.Now()); !errors.Is(e, domain.ErrConflict) {
		t.Errorf("重放消费应 ErrConflict，得 %v", e)
	}
}

// 越代：Job 增代后旧 Permit 消费被拒。
func TestPermitStaleGenerationRejected(t *testing.T) {
	w, ctx := mustWriter(t)
	runID, jobID, attemptID, actionID := seedPermitScene(t, w, ctx, 1)
	pm := execution.NewPermits(w)
	permitID := issuePermit(t, pm, ctx, runID, actionID, attemptID, time.Hour)
	// 模拟重新认领：Job generation → 2（attempt 仍 gen 1）。
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`UPDATE jobs SET generation=2 WHERE id=?`, string(jobID))
		return e
	})
	if e := pm.ConsumeActionPermit(ctx, permitID, time.Now()); !errors.Is(e, domain.ErrConflict) {
		t.Errorf("越代消费应 ErrConflict，得 %v", e)
	}
}

// 过期：TTL 过后消费被拒。
func TestPermitExpiredRejected(t *testing.T) {
	w, ctx := mustWriter(t)
	runID, _, attemptID, actionID := seedPermitScene(t, w, ctx, 1)
	pm := execution.NewPermits(w)
	permitID := issuePermit(t, pm, ctx, runID, actionID, attemptID, time.Second)
	later := time.Now().Add(2 * time.Second)
	if e := pm.ConsumeActionPermit(ctx, permitID, later); !errors.Is(e, domain.ErrConflict) {
		t.Errorf("过期消费应 ErrConflict，得 %v", e)
	}
}

// Run 非可执行（PAUSED）→ 消费被拒。
func TestPermitRunNotExecutableRejected(t *testing.T) {
	w, ctx := mustWriter(t)
	runID, _, attemptID, actionID := seedPermitScene(t, w, ctx, 1)
	pm := execution.NewPermits(w)
	permitID := issuePermit(t, pm, ctx, runID, actionID, attemptID, time.Hour)
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`UPDATE runs SET state='PAUSED' WHERE id=?`, string(runID))
		return e
	})
	if e := pm.ConsumeActionPermit(ctx, permitID, time.Now()); !errors.Is(e, domain.ErrConflict) {
		t.Errorf("Run 非可执行消费应 ErrConflict，得 %v", e)
	}
}

// 策略被取代 → 消费被拒。
func TestPermitPolicySupersededRejected(t *testing.T) {
	w, ctx := mustWriter(t)
	runID, _, attemptID, actionID := seedPermitScene(t, w, ctx, 1)
	pm := execution.NewPermits(w)
	permitID := issuePermit(t, pm, ctx, runID, actionID, attemptID, time.Hour) // policy_revision=0
	// 出现更新的策略修订。
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO policy_revisions (id, run_id, revision, canonical_json, digest, created_at) VALUES (?,?,?,?,?,?)`,
			"pr-1", string(runID), 1, "{}", "d", "2026-11-17T00:00:00Z")
		return e
	})
	if e := pm.ConsumeActionPermit(ctx, permitID, time.Now()); !errors.Is(e, domain.ErrConflict) {
		t.Errorf("策略被取代消费应 ErrConflict，得 %v", e)
	}
}

// 撤销某 Attempt 的未消费 Permit。
func TestRevokeAttemptPermits(t *testing.T) {
	w, _, ctx := openPermitDB(t)
	runID, _, attemptID, actionID := seedPermitScene(t, w, ctx, 1)
	pm := execution.NewPermits(w)
	_ = issuePermit(t, pm, ctx, runID, actionID, attemptID, time.Hour)
	n, err := pm.RevokeAttemptPermits(ctx, attemptID, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("应撤销 1 个，得 n=%d err=%v", n, err)
	}
	// 撤销后消费被拒。
	permitID := issuePermit(t, pm, ctx, runID, actionID, attemptID, time.Hour)
	pm.RevokeAttemptPermits(ctx, attemptID, time.Now())
	if e := pm.ConsumeActionPermit(ctx, permitID, time.Now()); !errors.Is(e, domain.ErrConflict) {
		t.Errorf("撤销后消费应 ErrConflict，得 %v", e)
	}
}

// Request Capability：消耗到 max_subrequests 即中断。
func TestRequestCapabilityLimit(t *testing.T) {
	w, _, ctx := openPermitDB(t)
	runID, _, attemptID, actionID := seedPermitScene(t, w, ctx, 1)
	pm := execution.NewPermits(w)
	permitID := issuePermit(t, pm, ctx, runID, actionID, attemptID, time.Hour)
	capID, err := pm.IssueRequestCapability(ctx, execution.RequestCapabilityGrant{
		PermitID: permitID, ActionID: actionID, RunID: runID, AttemptID: attemptID, MaxSubrequests: 3, RequestKind: "browser",
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if e := pm.ConsumeSubrequest(ctx, capID); e != nil {
			t.Fatalf("第 %d 个子请求应成功: %v", i+1, e)
		}
	}
	if e := pm.ConsumeSubrequest(ctx, capID); !errors.Is(e, domain.ErrConflict) {
		t.Errorf("超 max_subrequests 应中断 ErrConflict，得 %v", e)
	}
}

// mustWriter 开库（跳过无驱动）。
func mustWriter(t *testing.T) (*store.Writer, context.Context) {
	w, _, ctx := openPermitDB(t)
	return w, ctx
}
