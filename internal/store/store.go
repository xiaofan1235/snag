// Package store 是**唯一**直接访问 SQLite 的地方（§0.2 单向约束）：上层只调 Store 接口，
// 不碰 database/sql。Day 02 为手写查询层（有意不使用 sqlc，理由见本目录 README.md）。
package store

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
)

// Store 封装一个 *sql.DB 并提供类型安全的领域查询。
type Store struct {
	db *sql.DB
}

// New 用一个已打开的 *sql.DB 构造 Store（连接与 PRAGMA 由 store/sqlite.Open 负责）。
func New(db *sql.DB) *Store { return &Store{db: db} }

// DB 暴露底层连接（给迁移器 / 测试用）。
func (s *Store) DB() *sql.DB { return s.db }

// --- 时间 / 布尔 / 空值助手：统一以 RFC3339Nano 文本存时间 ---

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func dbT(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func dbNullT(t *time.Time) any {
	if t == nil {
		return nil
	}
	return dbT(*t)
}

func parseT(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func nullT(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t := parseT(ns.String)
	return &t
}

func dbBool(b bool) int {
	if b {
		return 1
	}
	return 0
}

// rowScanner 抽象 *sql.Row 与 *sql.Rows 的 Scan，便于单条/多条复用同一扫描逻辑。
type rowScanner interface {
	Scan(dest ...any) error
}

// ---------- runs ----------

const runColumns = `id, workspace_id, name, state, revision,
	current_policy_revision, current_strategy_epoch, current_snapshot_revision,
	strategy_dirty, authorization_scope_digest,
	COALESCE(authorization_reference,''), COALESCE(created_by,''),
	created_at, started_at, ended_at,
	COALESCE(terminal_reason,''), COALESCE(cancel_code,''),
	COALESCE(cancel_actor,''), COALESCE(cancel_detail,''), COALESCE(last_error,'')`

// InsertRun 插入一个新 Run。调用方应已用 domain.NewRun 构造（state=DRAFT, revision=1）。
func (s *Store) InsertRun(ctx context.Context, r *domain.Run) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO runs (
		id, workspace_id, name, state, revision,
		current_policy_revision, current_strategy_epoch, current_snapshot_revision,
		strategy_dirty, authorization_scope_digest, authorization_reference, created_by,
		created_at, started_at, ended_at,
		terminal_reason, cancel_code, cancel_actor, cancel_detail, last_error
	) VALUES (?,?,?,?,?, ?,?,?, ?,?,?,?, ?,?,?, ?,?,?,?,?)`,
		string(r.ID), string(r.WorkspaceID), r.Name, string(r.State), r.Revision,
		r.CurrentPolicyRevision, r.CurrentStrategyEpoch, r.CurrentSnapshotRevision,
		dbBool(r.StrategyDirty), r.AuthorizationScopeDigest, r.AuthorizationReference, r.CreatedBy,
		dbT(r.CreatedAt), dbNullT(r.StartedAt), dbNullT(r.EndedAt),
		r.TerminalReason, string(r.CancelCode), r.CancelActor, r.CancelDetail, r.LastError,
	)
	return err
}

// GetRun 按 id 读取，未找到返回 domain.ErrNotFound。
func (s *Store) GetRun(ctx context.Context, id domain.ID) (*domain.Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+runColumns+` FROM runs WHERE id=?`, string(id))
	return scanRun(row)
}

func scanRun(sc rowScanner) (*domain.Run, error) {
	var (
		r                           domain.Run
		id, wsid, state, cancelCode string
		createdAt                   string
		startedAt, endedAt          sql.NullString
		strategyDirty               int
	)
	err := sc.Scan(
		&id, &wsid, &r.Name, &state, &r.Revision,
		&r.CurrentPolicyRevision, &r.CurrentStrategyEpoch, &r.CurrentSnapshotRevision,
		&strategyDirty, &r.AuthorizationScopeDigest, &r.AuthorizationReference, &r.CreatedBy,
		&createdAt, &startedAt, &endedAt,
		&r.TerminalReason, &cancelCode, &r.CancelActor, &r.CancelDetail, &r.LastError,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.ID = domain.ID(id)
	r.WorkspaceID = domain.ID(wsid)
	r.State = domain.RunState(state)
	r.CancelCode = domain.CancelCode(cancelCode)
	r.StrategyDirty = strategyDirty != 0
	r.CreatedAt = parseT(createdAt)
	r.StartedAt = nullT(startedAt)
	r.EndedAt = nullT(endedAt)
	return &r, nil
}

// UpdateRunState 以乐观锁迁移 Run 状态：UPDATE ... WHERE id=? AND revision=expectedRev。
// 命中 0 行即冲突，返回 domain.ErrConflict（调用方重读重试）。成功则 revision=expectedRev+1。
// 返回新的 revision。
func (s *Store) UpdateRunState(ctx context.Context, id domain.ID, expectedRev int64, newState domain.RunState) (int64, error) {
	newRev := expectedRev + 1
	res, err := s.db.ExecContext(ctx,
		`UPDATE runs SET state=?, revision=? WHERE id=? AND revision=?`,
		string(newState), newRev, string(id), expectedRev,
	)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, domain.ErrConflict
	}
	return newRev, nil
}

// MigrateUp 把内置迁移应用到本 Store 的库（便捷封装）。
func (s *Store) MigrateUp() error { return Migrate(s.db) }
