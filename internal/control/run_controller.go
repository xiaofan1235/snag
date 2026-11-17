// Package control 实现 Run/Job/Action 的状态机与控制器（M1）。
//
// Run 状态机是**完整迁移表**（states.yaml → run_sm.gen.go，§1④ 代码生成）：任意 (状态,事件)
// 要么是已登记迁移，要么被显式拒绝，绝不 panic、不卡原态（§5.1）。
package control

//go:generate go run gen.go

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// Controller 驱动 Run 状态迁移。所有迁移走单写者 durable 连接的条件更新（§31 C1）。
type Controller struct {
	w *store.Writer
}

// NewController 用单写者构造控制器。
func NewController(w *store.Writer) *Controller { return &Controller{w: w} }

// EnsureWorkspace 幂等建一个 workspace（CLI/API 的默认工作区用）。
func (c *Controller) EnsureWorkspace(ctx context.Context, id, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return c.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO workspaces (id, name, memory_project_key, created_at) VALUES (?,?,?,?)`,
			id, name, "mpk-"+id, now)
		return e
	})
}

// CreateRun 在 workspace 下新建一个 DRAFT Run（Run 状态机初态），返回其 id。
func (c *Controller) CreateRun(ctx context.Context, workspaceID, name, authDigest string) (domain.ID, error) {
	id := domain.NewID()
	now := time.Now().UTC().Format(time.RFC3339)
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`INSERT INTO runs (id, workspace_id, name, state, authorization_scope_digest, created_at)
			 VALUES (?,?,?,?,?,?)`,
			string(id), workspaceID, name, string(domain.RunDraft), authDigest, now)
		return e
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// Apply 对 runID 施加事件 ev：
//   - 在一个 durable 事务内读当前 (state, revision)；
//   - 按状态机算下一态；非法迁移返回 domain.ErrInvalid（不改库、不 panic）；
//   - 条件更新 UPDATE ... WHERE id=? AND revision=?，命中 0 行返回 ErrConflict；
//   - 成功则 revision+1，返回新状态。
//
// 读与写在同一 durable 事务内原子完成；配合单写者串行，天然无并发写竞争（§31 C2）。
func (c *Controller) Apply(ctx context.Context, runID domain.ID, ev Event) (domain.RunState, error) {
	var next domain.RunState
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		var cur string
		var rev int64
		row := tx.QueryRowContext(ctx, `SELECT state, revision FROM runs WHERE id=?`, string(runID))
		if e := row.Scan(&cur, &rev); e != nil {
			if e == sql.ErrNoRows {
				return domain.ErrNotFound
			}
			return e
		}
		to, ok := Next(domain.RunState(cur), ev)
		if !ok {
			return domain.Errorf(domain.ErrInvalid,
				fmt.Sprintf("run %s: 状态 %s 不接受事件 %s", runID, cur, ev))
		}
		res, e := tx.ExecContext(ctx,
			`UPDATE runs SET state=?, revision=revision+1 WHERE id=? AND revision=?`,
			string(to), string(runID), rev)
		if e != nil {
			return e
		}
		n, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			return domain.ErrConflict
		}
		next = to
		return nil
	})
	return next, err
}
