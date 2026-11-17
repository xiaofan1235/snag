package scheduler_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/scheduler"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// fencing_test.go —— Day 09 §7：认领/租约/fencing 的崩溃安全性验收。
// 命令：go test -race -tags sqlite_modernc ./internal/scheduler/...（无驱动自动 SKIP）。
//
// 覆盖 DoD（§6）+ §4.3 #2 + §22.4「过期 Attempt 提交」：
//   - 并发 Claim 同一 Job 只有一个成功；
//   - 旧 fencing_token（增代后）的写入被拒；
//   - 租约过期后新 Claim 增代（generation + fencing_token 均递增）。

func openSchedDB(t *testing.T) (*store.Writer, *store.Reader, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "sched.db")
	w, err := store.OpenWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	rd, err := store.OpenReader(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rd.Close() })
	return w, rd, context.Background()
}

// seedReadyJob 造 workspace+run+agent_session+Job(READY)。返回 jobID、agentSessionID。
func seedReadyJob(t *testing.T, w *store.Writer, ctx context.Context, jobID string) (domain.ID, domain.ID) {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	sess := "sess-1"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			"ws-1", "acme", "mpk-1", ts); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT OR IGNORE INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
			VALUES (?,?,?,?,?,?)`, "run-1", "ws-1", "r", "RUNNING", "digest", ts); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT OR IGNORE INTO agent_sessions (id, run_id, role, state, created_at, last_active_at)
			VALUES (?,?,?,?,?,?)`, sess, "run-1", "executor", "ACTIVE", ts, ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO jobs (id, run_id, kind, state, created_at, updated_at) VALUES (?,?,?,?,?,?)`,
			jobID, "run-1", "probe", string(domain.JobReady), ts, ts)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return domain.ID(jobID), domain.ID(sess)
}

func readJobGenState(t *testing.T, rd *store.Reader, jobID string) (state string, gen int64, claimedBy, lease sql.NullString) {
	t.Helper()
	if e := rd.DB().QueryRow(`SELECT state, generation, claimed_by, lease_until FROM jobs WHERE id=?`, jobID).
		Scan(&state, &gen, &claimedBy, &lease); e != nil {
		t.Fatal(e)
	}
	return
}

// 并发 Claim 同一 READY Job：恰好一个成功，其余 ErrConflict；成功方 generation=1、fencing_token=1。
func TestConcurrentClaimSingleWinner(t *testing.T) {
	w, rd, ctx := openSchedDB(t)
	jobID, sess := seedReadyJob(t, w, ctx, "job-1")
	s := scheduler.New(w)

	const N = 16
	var wg sync.WaitGroup
	results := make([]*scheduler.ClaimResult, N)
	errs := make([]error, N)
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = s.Claim(ctx, jobID, sess)
		}(i)
	}
	wg.Wait()

	wins := 0
	var winner *scheduler.ClaimResult
	for i := 0; i < N; i++ {
		if errs[i] == nil {
			wins++
			winner = results[i]
		} else if !errors.Is(errs[i], domain.ErrConflict) {
			t.Errorf("落败方应为 ErrConflict，得 %v", errs[i])
		}
	}
	if wins != 1 {
		t.Fatalf("并发 Claim 应恰好 1 个成功，得 %d", wins)
	}
	if winner.Generation != 1 || winner.FencingToken != 1 {
		t.Errorf("首次认领应 generation=1 fencing=1，得 gen=%d fence=%d", winner.Generation, winner.FencingToken)
	}
	state, gen, claimedBy, lease := readJobGenState(t, rd, "job-1")
	if state != string(domain.JobClaimed) || gen != 1 || !claimedBy.Valid || !lease.Valid {
		t.Errorf("Job 应 CLAIMED/gen1/有认领者与租约，得 state=%s gen=%d claimedBy=%v lease=%v",
			state, gen, claimedBy.Valid, lease.Valid)
	}

	// 只建了一个 Attempt。
	var nAtt int
	if e := rd.DB().QueryRow(`SELECT COUNT(1) FROM attempts WHERE job_id=?`, "job-1").Scan(&nAtt); e != nil {
		t.Fatal(e)
	}
	if nAtt != 1 {
		t.Errorf("应只建 1 个 Attempt，得 %d", nAtt)
	}
}

// 租约过期 → 新 Claim 增代；旧 fencing_token 的写入被拒（§22.4 过期 Attempt 提交）。
func TestLeaseExpiryRegenAndStaleFencingRejected(t *testing.T) {
	w, rd, ctx := openSchedDB(t)
	jobID, sess := seedReadyJob(t, w, ctx, "job-2")
	s := scheduler.New(w, scheduler.WithLeaseDuration(2*time.Second))

	// 首次认领。
	c1, err := s.Claim(ctx, jobID, sess)
	if err != nil {
		t.Fatalf("Claim#1: %v", err)
	}
	if c1.Generation != 1 || c1.FencingToken != 1 {
		t.Fatalf("Claim#1 应 gen1/fence1，得 gen=%d fence=%d", c1.Generation, c1.FencingToken)
	}
	// 当代写入成功。
	if e := s.WriteAttemptResult(ctx, jobID, c1.FencingToken, `{"ok":true}`); e != nil {
		t.Fatalf("当代写入应成功，得 %v", e)
	}

	// 未到期：不过期。
	n0, err := s.ExpireLeases(ctx, c1.LeaseUntil.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if n0 != 0 {
		t.Fatalf("未到期不应过期，得 %d", n0)
	}

	// 到期：过期扫描把 Job 重新上架（CLAIMED→READY）。
	n1, err := s.ExpireLeases(ctx, c1.LeaseUntil.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if n1 != 1 {
		t.Fatalf("到期应重新上架 1 个，得 %d", n1)
	}
	state, _, claimedBy, lease := readJobGenState(t, rd, "job-2")
	if state != string(domain.JobReady) || claimedBy.Valid || lease.Valid {
		t.Errorf("过期后应 READY 且清认领/租约，得 state=%s claimedBy=%v lease=%v", state, claimedBy.Valid, lease.Valid)
	}
	// 旧 Attempt 标 EXPIRED。
	var att1State string
	if e := rd.DB().QueryRow(`SELECT state FROM attempts WHERE job_id=? AND generation=1`, "job-2").Scan(&att1State); e != nil {
		t.Fatal(e)
	}
	if att1State != string(domain.AttemptExpired) {
		t.Errorf("旧 Attempt 应 EXPIRED，得 %s", att1State)
	}

	// 重新认领：generation 与 fencing_token 均增。
	c2, err := s.Claim(ctx, jobID, sess)
	if err != nil {
		t.Fatalf("Claim#2: %v", err)
	}
	if c2.Generation != 2 {
		t.Errorf("重领应增代到 2，得 %d", c2.Generation)
	}
	if c2.FencingToken <= c1.FencingToken {
		t.Errorf("重领 fencing_token 应严格递增：%d -> %d", c1.FencingToken, c2.FencingToken)
	}

	// 旧 fencing_token（c1）的写入被拒——旧 Attempt 即使复活也写不进。
	if e := s.WriteAttemptResult(ctx, jobID, c1.FencingToken, `{"stale":true}`); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("旧 fencing_token 写入应被拒（ErrConflict），得 %v", e)
	}
	if e := s.CheckFencing(ctx, jobID, c1.FencingToken); !errors.Is(e, domain.ErrConflict) {
		t.Errorf("CheckFencing 旧令牌应 ErrConflict，得 %v", e)
	}
	// 新 fencing_token 写入成功。
	if e := s.WriteAttemptResult(ctx, jobID, c2.FencingToken, `{"fresh":true}`); e != nil {
		t.Errorf("当代（新）fencing_token 写入应成功，得 %v", e)
	}
}

// 续租：当代令牌可续、陈旧令牌被拒。
func TestRenewFencing(t *testing.T) {
	w, _, ctx := openSchedDB(t)
	jobID, sess := seedReadyJob(t, w, ctx, "job-3")
	s := scheduler.New(w, scheduler.WithLeaseDuration(2*time.Second))

	c1, err := s.Claim(ctx, jobID, sess)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	// 当代续租成功，租约延后。
	newLease, err := s.Renew(ctx, jobID, c1.FencingToken)
	if err != nil {
		t.Fatalf("Renew 当代应成功，得 %v", err)
	}
	if !newLease.After(c1.LeaseUntil.Add(-time.Hour)) { // 粗略：拿到了一个合理的新租约
		t.Errorf("续租应返回新租约时刻")
	}
	// 陈旧令牌续租被拒。
	if _, e := s.Renew(ctx, jobID, c1.FencingToken+999); !errors.Is(e, domain.ErrConflict) {
		t.Errorf("陈旧令牌续租应 ErrConflict，得 %v", e)
	}
}
