package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"pentagent/internal/store/sqlite"
)

// writer.go —— 单写者 Store（§31 C1 每次提交持久性分级、§31 C2 单写者）。
//
// 为什么单写者：SQLite(WAL) 同一时刻只允许一个写者，多 goroutine 并发写会 SQLITE_BUSY。
// 所有写都经**一个 writer goroutine 串行化**，从根上消除写冲突（§31 C2）。
//
// 为什么两条写连接（§31 C1 持久性分级）：
//   - durable：synchronous=FULL。凡「提交必须先于外部副作用」的记录走它——
//     Permit 消费 + DISPATCHED、Receipt 终结、Evidence 元数据、Approval 消费、Policy 切换。
//     提交返回即已 fsync（断电也不丢），代价是每次 commit 一次 fsync。
//   - bulk：synchronous=NORMAL + group-commit。事件/Outbox/度量走它，用微批把多条写合并到
//     一个事务，降低事务与 fsync 开销；NORMAL 下 WAL 仅在 checkpoint 时 fsync。
//
// 两条连接各 MaxOpenConns(1)，且只被同一个 writer goroutine 顺序使用，故彼此也不并发。
// 读走独立只读池（reader.go；WAL 允许并发读）。

const maxBulkBatch = 128

var errWriterClosed = errors.New("store: writer 已关闭")

// Writer 是串行化的写入器，持有 durable/bulk 两条写连接。
type Writer struct {
	durable *sql.DB
	bulk    *sql.DB

	cmds      chan writeCmd
	done      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
	started   atomic.Bool // Q-13：启动后禁止直接触碰 durable 连接（Migrate/DurableDBForTest）

	ckInterval time.Duration
}

type writeCmd struct {
	durable bool
	fn      func(*sql.Tx) error
	result  chan error
}

// OpenWriter 打开 durable/bulk 两条写连接（不同 synchronous），尚未启动 writer goroutine。
// 需先 Migrate()（如需）再 Start()。驱动经 build tag sqlite_modernc 接入（见 sqlite 包）。
func OpenWriter(path string) (*Writer, error) {
	durable, err := openWriteConn(path, "FULL")
	if err != nil {
		return nil, fmt.Errorf("open durable: %w", err)
	}
	bulk, err := openWriteConn(path, "NORMAL")
	if err != nil {
		_ = durable.Close()
		return nil, fmt.Errorf("open bulk: %w", err)
	}
	return &Writer{
		durable:    durable,
		bulk:       bulk,
		cmds:       make(chan writeCmd, 256),
		done:       make(chan struct{}),
		ckInterval: 30 * time.Second,
	}, nil
}

// openWriteConn 打开一条写连接：WAL + 指定 synchronous + 单连接串行。
func openWriteConn(path, synchronous string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"+
			"&_pragma=synchronous(%s)&_pragma=temp_store(MEMORY)",
		path, synchronous,
	)
	db, err := sql.Open(sqlite.DriverName, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // 单连接：写路径串行
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Migrate 在 durable 连接上应用迁移（启动前的一次性设置）。
// Q-13：Start() 后 durable 连接归 writer goroutine 独占，此时再调会数据竞态——故加运行期守卫。
func (w *Writer) Migrate() error {
	if w.started.Load() {
		return errors.New("store: Migrate 必须在 Start() 之前调用")
	}
	return Migrate(w.durable)
}

// Start 启动单写 goroutine。幂等：重复调用无副作用。
func (w *Writer) Start() {
	if w.started.Swap(true) {
		return
	}
	w.wg.Add(1)
	go w.loop()
}

// Durable 在 durable 连接上跑一个事务（提交即 fsync）。每个 durable 命令独立事务、独立提交边界。
func (w *Writer) Durable(ctx context.Context, fn func(*sql.Tx) error) error {
	return w.submit(ctx, true, fn)
}

// Bulk 在 bulk 连接上跑一个事务；writer 会把相邻的多个 bulk 命令 group-commit 到一个事务。
func (w *Writer) Bulk(ctx context.Context, fn func(*sql.Tx) error) error {
	return w.submit(ctx, false, fn)
}

func (w *Writer) submit(ctx context.Context, durable bool, fn func(*sql.Tx) error) error {
	// 已关闭则立刻拒绝，不往缓冲 cmds 投递（否则命令会被孤立，writer 已退出无人处理）。
	select {
	case <-w.done:
		return errWriterClosed
	default:
	}

	res := make(chan error, 1)
	cmd := writeCmd{durable: durable, fn: fn, result: res}
	select {
	case w.cmds <- cmd:
	case <-ctx.Done():
		return ctx.Err()
	case <-w.done:
		return errWriterClosed
	}

	// 等结果；若 writer 正在关闭，不无限等——优先取已就绪的真实结果，否则按关闭处理。
	select {
	case err := <-res:
		return err
	case <-ctx.Done():
		return ctx.Err() // 命令可能仍会被执行；调用方据此重试需保证幂等
	case <-w.done:
		select {
		case err := <-res:
			return err
		default:
			return errWriterClosed
		}
	}
}

func (w *Writer) loop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.ckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.done:
			w.drain()
			return
		case <-ticker.C:
			w.checkpoint()
		case c := <-w.cmds:
			w.handle(c)
		}
	}
}

// handle 处理一个命令：durable 独立提交；bulk 则尽量把当前可取的 bulk 命令合并一批。
func (w *Writer) handle(c writeCmd) {
	if c.durable {
		c.result <- w.runDurable(c.fn)
		return
	}
	batch := []writeCmd{c}
	for len(batch) < maxBulkBatch {
		select {
		case c2 := <-w.cmds:
			if c2.durable {
				// 不能把 durable 混进 bulk 批：先刷掉当前批，再独立跑 durable。
				w.flushBulk(batch)
				c2.result <- w.runDurable(c2.fn)
				return
			}
			batch = append(batch, c2)
		default:
			w.flushBulk(batch)
			return
		}
	}
	w.flushBulk(batch)
}

// runDurable 在 durable 连接上单条事务提交（synchronous=FULL → 返回即 fsync）。
func (w *Writer) runDurable(fn func(*sql.Tx) error) error {
	tx, err := w.durable.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// flushBulk 把一批 bulk 命令合并到**一个事务**提交，用 SAVEPOINT 做逐命令错误隔离：
// 某条失败只回滚它自己，不牵连同批其它命令（group-commit 正确性关键）。
func (w *Writer) flushBulk(batch []writeCmd) {
	if len(batch) == 0 {
		return
	}
	tx, err := w.bulk.Begin()
	if err != nil {
		for _, c := range batch {
			c.result <- err
		}
		return
	}
	results := make([]error, len(batch))
	for i, c := range batch {
		sp := fmt.Sprintf("sp%d", i)
		if _, e := tx.Exec("SAVEPOINT " + sp); e != nil {
			results[i] = e
			continue
		}
		if e := c.fn(tx); e != nil {
			_, _ = tx.Exec("ROLLBACK TO " + sp)
			_, _ = tx.Exec("RELEASE " + sp)
			results[i] = e
			continue
		}
		if _, e := tx.Exec("RELEASE " + sp); e != nil {
			results[i] = e
		}
	}
	if e := tx.Commit(); e != nil {
		for i := range results {
			if results[i] == nil {
				results[i] = e
			}
		}
	}
	for i, c := range batch {
		c.result <- results[i]
	}
}

// drain 关闭时把残留命令执行完（不丢未决写）。
func (w *Writer) drain() {
	for {
		select {
		case c := <-w.cmds:
			if c.durable {
				c.result <- w.runDurable(c.fn)
			} else {
				w.flushBulk([]writeCmd{c})
			}
		default:
			return
		}
	}
}

// checkpoint 做一次 WAL 截断 checkpoint（在 writer goroutine 内，串行安全）。
// best-effort（Q-14）：有读者占用时 TRUNCATE 可能返回 busy，忽略即可，下次再截断。
func (w *Writer) checkpoint() {
	_, _ = w.durable.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
}

// Close 停止 writer goroutine（先把残留命令跑完）并关闭两条写连接。幂等、可重复调用。
func (w *Writer) Close() error {
	w.closeOnce.Do(func() {
		close(w.done)
		w.wg.Wait()
		err1 := w.durable.Close()
		err2 := w.bulk.Close()
		if err1 != nil {
			w.closeErr = err1
		} else {
			w.closeErr = err2
		}
	})
	return w.closeErr
}

// DurableDBForTest 暴露 durable 连接，仅供 Start() 之前的测试只读核对；正常写一律走 Durable/Bulk。
// 命名即约束（Q-13）：Start() 后严禁使用，否则与 writer goroutine 争用同一连接。
func (w *Writer) DurableDBForTest() *sql.DB {
	if w.started.Load() {
		panic("store: DurableDBForTest 在 Start() 之后被调用（会与 writer 争用连接）")
	}
	return w.durable
}
