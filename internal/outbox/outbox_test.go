package outbox_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/events"
	"pentagent/internal/outbox"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// outbox_test.go —— Day 10 §7：先存后发验收。
// 命令：go test -tags sqlite_modernc ./internal/outbox/...（无驱动自动 SKIP）。
//
// 覆盖 DoD（§6）+ §0.5：
//   - 事务回滚 → 无 outbox 消息、无事件；
//   - 提交后、且仅在 Drain（提交后发布）后，消息才出现在 Bus；
//   - publisher 失败 → 退避重试、消息不丢；成功后幂等不重发。

func openOutboxDB(t *testing.T) (*store.Writer, *store.Reader, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "ob.db")
	w, err := store.OpenWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	rd, err := store.OpenReader(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rd.Close() })
	return w, rd, context.Background()
}

func seedRun(t *testing.T, w *store.Writer, ctx context.Context) {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			"ws-1", "acme", "mpk-1", ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT OR IGNORE INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
			VALUES (?,?,?,?,?,?)`, "run-1", "ws-1", "r", "RUNNING", "digest", ts)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// flakyBus 前 failTimes 次 Publish 失败，之后成功；记录成功投递的消息。
type flakyBus struct {
	failTimes int
	calls     int
	delivered []outbox.Message
}

func (b *flakyBus) Publish(_ context.Context, m outbox.Message) error {
	b.calls++
	if b.calls <= b.failTimes {
		return errors.New("sink down")
	}
	b.delivered = append(b.delivered, m)
	return nil
}

// 先存后发：同事务写事件 + enqueue；提交后 Drain 才发布；提交前/未 Drain 时 Bus 为空。
func TestStoreThenPublish(t *testing.T) {
	w, rd, ctx := openOutboxDB(t)
	seedRun(t, w, ctx)
	bus := outbox.NewMemBus()
	pub := outbox.NewPublisher(w, rd, bus)

	// 领域事务：写事件 + enqueue 消息（原子）。
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := events.Append(ctx, tx, events.Event{RunID: "run-1", Type: "run.updated", EntityType: "run", EntityID: "run-1", EntityVersion: 2}); e != nil {
			return e
		}
		_, e := outbox.Enqueue(ctx, tx, "run-1", "run.updated", `{"state":"RUNNING"}`)
		return e
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// 提交成功但尚未 Drain → Bus 为空（发布只在提交后的 publisher 环节发生）。
	if bus.Len() != 0 {
		t.Fatalf("Drain 前 Bus 应为空，得 %d", bus.Len())
	}

	// Drain（提交后发布）→ 消息到 Bus。
	n, err := pub.Drain(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || bus.Len() != 1 {
		t.Fatalf("Drain 应发布 1 条，得 n=%d bus=%d", n, bus.Len())
	}
	if msg := bus.Messages()[0]; msg.Topic != "run.updated" || msg.RunID != "run-1" {
		t.Errorf("消息内容不符：%+v", msg)
	}

	// 再 Drain：已发，幂等为 0。
	n2, err := pub.Drain(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 || bus.Len() != 1 {
		t.Errorf("重复 Drain 应幂等（0 条、Bus 不变），得 n=%d bus=%d", n2, bus.Len())
	}
}

// 事务回滚 → 无 outbox 消息、无事件。
func TestRollbackNoMessage(t *testing.T) {
	w, rd, ctx := openOutboxDB(t)
	seedRun(t, w, ctx)

	boom := errors.New("boom")
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		_, _ = events.Append(ctx, tx, events.Event{RunID: "run-1", Type: "run.updated", EntityType: "run", EntityID: "run-1"})
		_, _ = outbox.Enqueue(ctx, tx, "run-1", "run.updated", `{}`)
		return boom
	})
	var nOut, nEv int
	if e := rd.DB().QueryRow(`SELECT COUNT(1) FROM outbox`).Scan(&nOut); e != nil {
		t.Fatal(e)
	}
	if e := rd.DB().QueryRow(`SELECT COUNT(1) FROM events`).Scan(&nEv); e != nil {
		t.Fatal(e)
	}
	if nOut != 0 || nEv != 0 {
		t.Fatalf("回滚后应无消息无事件，得 outbox=%d events=%d", nOut, nEv)
	}
}

// publisher 失败 → 退避重试、消息不丢；退避未到不重投；到点后成功。
func TestPublisherRetryNoLoss(t *testing.T) {
	w, rd, ctx := openOutboxDB(t)
	seedRun(t, w, ctx)
	bus := &flakyBus{failTimes: 2} // 前两次失败
	// 固定退避 1 分钟，便于用注入 now 精确控制。
	pub := outbox.NewPublisher(w, rd, bus, outbox.WithBackoff(func(int) time.Duration { return time.Minute }))

	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := outbox.Enqueue(ctx, tx, "run-1", "job.updated", `{"n":1}`)
		return e
	}); err != nil {
		t.Fatal(err)
	}

	t0 := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	// 第 1 次 Drain：失败 → 0 发布，消息仍在（未丢），attempts=1、next_attempt_at=t0+1m。
	if n, _ := pub.Drain(ctx, t0); n != 0 {
		t.Fatalf("首投失败应 0 发布，得 %d", n)
	}
	assertUnpublished(t, rd, 1)

	// 退避未到（t0+30s）：不重投。
	if n, _ := pub.Drain(ctx, t0.Add(30*time.Second)); n != 0 {
		t.Fatalf("退避未到不应重投，得 %d", n)
	}
	assertUnpublished(t, rd, 1) // attempts 仍 1，没动

	// 到点（t0+61s）：第 2 次投递仍失败（flaky failTimes=2）→ attempts=2、next=t0+61s+1m。
	if n, _ := pub.Drain(ctx, t0.Add(61*time.Second)); n != 0 {
		t.Fatalf("第 2 投仍失败应 0，得 %d", n)
	}
	assertUnpublished(t, rd, 2)

	// 再到点：第 3 次成功 → 发布，消息标记已发、不丢。
	if n, _ := pub.Drain(ctx, t0.Add(200*time.Second)); n != 1 {
		t.Fatalf("第 3 投应成功发布 1，得 %d", n)
	}
	var published int
	if e := rd.DB().QueryRow(`SELECT published FROM outbox WHERE run_id=?`, "run-1").Scan(&published); e != nil {
		t.Fatal(e)
	}
	if published != 1 {
		t.Errorf("成功后应标 published=1，得 %d", published)
	}
	if len(bus.delivered) != 1 {
		t.Errorf("应恰好投递 1 条（不丢不重），得 %d", len(bus.delivered))
	}
}

func assertUnpublished(t *testing.T, rd *store.Reader, wantAttempts int) {
	t.Helper()
	var published, attempts int
	if e := rd.DB().QueryRow(`SELECT published, attempts FROM outbox WHERE run_id=?`, "run-1").
		Scan(&published, &attempts); e != nil {
		t.Fatal(e)
	}
	if published != 0 {
		t.Errorf("消息不应已发（不丢），published=%d", published)
	}
	if attempts != wantAttempts {
		t.Errorf("attempts 应为 %d，得 %d", wantAttempts, attempts)
	}
}
