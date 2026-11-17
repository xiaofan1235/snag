package outbox

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// Message 是一条从 outbox 取出的待发消息。
type Message struct {
	OutboxID    domain.ID
	RunID       domain.ID
	Topic       string
	PayloadJSON string
	Attempts    int // 本次投递前已失败的次数（0 = 首投）
}

// Bus 是事件汇。Day 10 用内存实现（MemBus）；Day 40 接 SSE。Publish 失败 → 该条退避重试。
type Bus interface {
	Publish(ctx context.Context, msg Message) error
}

// Publisher 在领域事务**提交后**扫 outbox 表、推送到 Bus、标记已发；失败指数退避（§4/§26.9）。
// bus.Publish 在任何 DB 事务之外调用——不在发布 I/O 期间占用单写者，发布失败也不回滚领域事务。
type Publisher struct {
	w         *store.Writer
	rd        *store.Reader
	bus       Bus
	backoff   func(attempt int) time.Duration
	batchSize int
}

// Option 配置 Publisher。
type Option func(*Publisher)

// WithBackoff 覆盖退避函数（入参为新的尝试计数，1 起）。
func WithBackoff(f func(attempt int) time.Duration) Option {
	return func(p *Publisher) { p.backoff = f }
}

// WithBatchSize 覆盖单次 Drain 的最大条数。
func WithBatchSize(n int) Option { return func(p *Publisher) { p.batchSize = n } }

// DefaultBackoff 指数退避：200ms·2^(attempt-1)，上限 30s。
func DefaultBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := 200 * time.Millisecond
	for i := 1; i < attempt && d < 30*time.Second; i++ {
		d *= 2
	}
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

// NewPublisher 构造 Publisher。rd 用于扫表（只读池），w 用于标记（单写者）。
func NewPublisher(w *store.Writer, rd *store.Reader, bus Bus, opts ...Option) *Publisher {
	p := &Publisher{w: w, rd: rd, bus: bus, backoff: DefaultBackoff, batchSize: 128}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Drain 扫一批「已到期的未发」消息，逐条推送：成功标 published，失败累计 attempts 并按退避排下次。
// now 由调用方注入（确定性测试 + 统一时钟源）。返回本次成功发布的条数。
//
// 流程特意分三段：① 只读池取批（随即关闭游标，不在发布期间占用读连接）；② 事务外 bus.Publish；
// ③ 单写者落标记。失败不影响其它条、不回滚领域事务（§26.9）；已 published 的行被 WHERE 守卫幂等跳过。
func (p *Publisher) Drain(ctx context.Context, now time.Time) (int, error) {
	nowStr := now.UTC().Format(time.RFC3339)

	rows, err := p.rd.DB().QueryContext(ctx,
		`SELECT id, run_id, topic, payload_json, attempts FROM outbox
		 WHERE published=0 AND (next_attempt_at IS NULL OR next_attempt_at<=?)
		 ORDER BY created_at, id LIMIT ?`, nowStr, p.batchSize)
	if err != nil {
		return 0, err
	}
	var batch []Message
	for rows.Next() {
		var m Message
		var oid, rid string
		if e := rows.Scan(&oid, &rid, &m.Topic, &m.PayloadJSON, &m.Attempts); e != nil {
			rows.Close()
			return 0, e
		}
		m.OutboxID = domain.ID(oid)
		m.RunID = domain.ID(rid)
		batch = append(batch, m)
	}
	if e := rows.Err(); e != nil {
		rows.Close()
		return 0, e
	}
	rows.Close()

	published := 0
	for _, m := range batch {
		if perr := p.bus.Publish(ctx, m); perr != nil {
			nextAt := now.Add(p.backoff(m.Attempts + 1)).UTC().Format(time.RFC3339)
			if e := p.markFailed(ctx, m.OutboxID, nextAt, perr.Error()); e != nil {
				return published, e
			}
			continue
		}
		if e := p.markPublished(ctx, m.OutboxID, nowStr); e != nil {
			return published, e
		}
		published++
	}
	return published, nil
}

func (p *Publisher) markPublished(ctx context.Context, id domain.ID, nowStr string) error {
	return p.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`UPDATE outbox SET published=1, published_at=? WHERE id=? AND published=0`, nowStr, string(id))
		return e
	})
}

func (p *Publisher) markFailed(ctx context.Context, id domain.ID, nextAt, errMsg string) error {
	return p.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`UPDATE outbox SET attempts=attempts+1, next_attempt_at=?, last_error=? WHERE id=? AND published=0`,
			nextAt, errMsg, string(id))
		return e
	})
}
