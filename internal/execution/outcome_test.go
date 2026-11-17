package execution_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/execution"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// outcome_test.go —— Day 21 §7：Receipt + 对账 + OUTCOME_UNKNOWN 闭环（§5.3）。
// 命令：go test -tags sqlite_modernc ./internal/execution/...（无驱动自动 SKIP）。

func openOutcomeDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("需 -tags sqlite_modernc")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "oc.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	return w, context.Background()
}

func seedAction(t *testing.T, w *store.Writer, ctx context.Context, actionID, state string) {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES ('ws','a','m',?)`, ts)
		tx.Exec(`INSERT OR IGNORE INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES ('run-1','ws','r','RUNNING','d',?)`, ts)
		tx.Exec(`INSERT OR IGNORE INTO jobs (id,run_id,kind,state,generation,created_at,updated_at) VALUES ('job-1','run-1','k','RUNNING',1,?,?)`, ts, ts)
		tx.Exec(`INSERT OR IGNORE INTO attempts (id,job_id,generation,fencing_token,state,started_at) VALUES ('att-1','job-1',1,1,'RUNNING',?)`, ts)
		_, e := tx.Exec(`INSERT INTO actions (id,run_id,job_id,attempt_id,tool_name,state,identity_id) VALUES (?,?,?,?,?,?,?)`,
			actionID, "run-1", "job-1", "att-1", "http", state, "idn-1")
		return e
	}); err != nil {
		t.Fatalf("seed action: %v", err)
	}
}

func writeReceipt(t *testing.T, w *store.Writer, ctx context.Context, actionID string) {
	t.Helper()
	r := execution.NewReceipts(w)
	if _, err := r.Write(ctx, execution.ReceiptInput{
		ActionID: domain.ID(actionID), RunID: "run-1", AttemptID: "att-1", FencingToken: 1,
		IdentityID: "idn-1", StatusCode: 200, Outcome: domain.ActionExecuted,
		ResponseDigest: "abc", Bytes: 10, StartedAt: time.Now(), CompletedAt: time.Now(),
	}); err != nil {
		t.Fatalf("写回执: %v", err)
	}
}

func insertEvidence(t *testing.T, w *store.Writer, ctx context.Context, actionID, blobPath string) {
	t.Helper()
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO evidence (id,run_id,action_id,kind,plain_sha256,blob_path,size_bytes,captured_at) VALUES (?,?,?,?,?,?,?,?)`,
			domain.NewID(), "run-1", actionID, "http_exchange", "sha-"+actionID, nilIfEmpty(blobPath), 10, "2026-11-17T00:00:00Z")
		return e
	})
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func actionState(t *testing.T, w *store.Writer, ctx context.Context, actionID string) string {
	t.Helper()
	var st string
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT state FROM actions WHERE id=?`, actionID).Scan(&st)
	})
	return st
}

// 正常：EXECUTED + 终结回执 + 证据(带 blob) → RecordAction → RECORDED。
func TestRecordActionHappy(t *testing.T) {
	w, ctx := openOutcomeDB(t)
	seedAction(t, w, ctx, "act-1", "EXECUTED")
	writeReceipt(t, w, ctx, "act-1")
	insertEvidence(t, w, ctx, "act-1", "ab/cd/blob")
	r := execution.NewReceipts(w)

	done, reason, err := r.RecordAction(ctx, "act-1")
	if err != nil || !done {
		t.Fatalf("应进 RECORDED，得 done=%v reason=%s err=%v", done, reason, err)
	}
	if s := actionState(t, w, ctx, "act-1"); s != "RECORDED" {
		t.Errorf("应 RECORDED，得 %s", s)
	}
}

// 缺证据（Receipt-only）→ 不进 RECORDED（exchange_incomplete）；对账 → OUTCOME_UNKNOWN。
func TestReceiptOnlyOrphan(t *testing.T) {
	w, ctx := openOutcomeDB(t)
	seedAction(t, w, ctx, "act-1", "EXECUTED")
	writeReceipt(t, w, ctx, "act-1") // 无 evidence
	r := execution.NewReceipts(w)

	done, reason, _ := r.RecordAction(ctx, "act-1")
	if done || reason != "exchange_incomplete" {
		t.Fatalf("缺证据不应 RECORDED，得 done=%v reason=%s", done, reason)
	}
	rec := execution.NewOutcomeReconciler(w)
	rep, err := rec.ReconcileOrphans(ctx)
	if err != nil || rep.ReceiptOnly != 1 {
		t.Fatalf("应 ReceiptOnly=1，得 %+v err=%v", rep, err)
	}
	if s := actionState(t, w, ctx, "act-1"); s != "OUTCOME_UNKNOWN" {
		t.Errorf("Receipt-only 孤儿应 OUTCOME_UNKNOWN，得 %s", s)
	}
}

// 缺回执（Evidence-only）→ 不进 RECORDED；对账 → OUTCOME_UNKNOWN。
func TestEvidenceOnlyOrphan(t *testing.T) {
	w, ctx := openOutcomeDB(t)
	seedAction(t, w, ctx, "act-1", "DISPATCHED")
	insertEvidence(t, w, ctx, "act-1", "ab/cd/blob") // 无回执
	r := execution.NewReceipts(w)

	done, reason, _ := r.RecordAction(ctx, "act-1")
	if done || reason != "receipt_not_finalized" {
		t.Fatalf("缺回执不应 RECORDED，得 done=%v reason=%s", done, reason)
	}
	rec := execution.NewOutcomeReconciler(w)
	rep, _ := rec.ReconcileOrphans(ctx)
	if rep.EvidenceOnly != 1 {
		t.Fatalf("应 EvidenceOnly=1，得 %+v", rep)
	}
	if s := actionState(t, w, ctx, "act-1"); s != "OUTCOME_UNKNOWN" {
		t.Errorf("Evidence-only 孤儿应 OUTCOME_UNKNOWN，得 %s", s)
	}
}

// 缺 Blob（证据元数据在但 blob_path 空）→ 不进 RECORDED（blob_not_persisted）。
func TestMissingBlobBlocksRecord(t *testing.T) {
	w, ctx := openOutcomeDB(t)
	seedAction(t, w, ctx, "act-1", "EXECUTED")
	writeReceipt(t, w, ctx, "act-1")
	insertEvidence(t, w, ctx, "act-1", "") // blob_path 空
	r := execution.NewReceipts(w)

	done, reason, _ := r.RecordAction(ctx, "act-1")
	if done || reason != "blob_not_persisted" {
		t.Fatalf("缺 blob 不应 RECORDED，得 done=%v reason=%s", done, reason)
	}
}

// 对账不动「两者齐备」的动作（留给 RecordAction）。
func TestReconcileLeavesPaired(t *testing.T) {
	w, ctx := openOutcomeDB(t)
	seedAction(t, w, ctx, "act-1", "EXECUTED")
	writeReceipt(t, w, ctx, "act-1")
	insertEvidence(t, w, ctx, "act-1", "ab/cd/blob")
	rec := execution.NewOutcomeReconciler(w)
	rep, _ := rec.ReconcileOrphans(ctx)
	if rep.Paired != 1 || rep.ReceiptOnly != 0 || rep.EvidenceOnly != 0 {
		t.Errorf("齐备动作应 Paired=1 不被改，得 %+v", rep)
	}
	if s := actionState(t, w, ctx, "act-1"); s != "EXECUTED" {
		t.Errorf("齐备动作应留 EXECUTED，得 %s", s)
	}
}
