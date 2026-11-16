package store

import (
	"context"
	"database/sql"

	"pentagent/internal/domain"
)

// store_workspace.go —— workspaces 的插入/读取（§4.1 workspaces）。

const workspaceColumns = `id, name, memory_project_key, COALESCE(data_classification,''),
	retention_policy_json, created_at, archived_at`

// InsertWorkspace 插入一个新 Workspace（memory_project_key 唯一，由服务端注入）。
func (s *Store) InsertWorkspace(ctx context.Context, w *domain.Workspace) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO workspaces (
		id, name, memory_project_key, data_classification, retention_policy_json,
		created_at, archived_at
	) VALUES (?,?,?,?,?, ?,?)`,
		string(w.ID), w.Name, w.MemoryProjectKey, string(w.DataClassification),
		dbNullJSON(w.RetentionPolicyJSON), dbT(w.CreatedAt), dbNullT(w.ArchivedAt),
	)
	return err
}

// GetWorkspace 按 id 读取，未找到返回 domain.ErrNotFound。
func (s *Store) GetWorkspace(ctx context.Context, id domain.ID) (*domain.Workspace, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+workspaceColumns+` FROM workspaces WHERE id=?`, string(id))
	var (
		w              domain.Workspace
		idStr, dataCls string
		retention      sql.NullString
		createdAt      string
		archivedAt     sql.NullString
	)
	err := row.Scan(&idStr, &w.Name, &w.MemoryProjectKey, &dataCls, &retention, &createdAt, &archivedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	w.ID = domain.ID(idStr)
	w.DataClassification = domain.DataClass(dataCls)
	w.RetentionPolicyJSON = nullJSON(retention)
	w.CreatedAt = parseT(createdAt)
	w.ArchivedAt = nullT(archivedAt)
	return &w, nil
}
