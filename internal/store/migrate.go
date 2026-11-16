package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"pentagent/migrations"
)

// migrate.go —— 内置的、goose 格式兼容的极简迁移器（Day 02 无网适配，替代 goose CLI）。
//
// 行为：按文件名排序遍历 migrations.FS 里的 *.sql，未应用的版本在**一个事务**内执行其
// `-- +goose Up` 段，并把版本记入 schema_migrations。已应用的跳过（幂等）。
// 装了 goose CLI 时，同样的文件也能被 `goose -dir migrations sqlite3 db up` 应用。

// Migrate 把所有尚未应用的迁移应用到 db。可重复调用（幂等）。
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	names, err := migrationFiles()
	if err != nil {
		return err
	}
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		applied, err := isApplied(db, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		raw, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return err
		}
		up, err := extractUp(string(raw))
		if err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if err := applyOne(db, version, up); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

func migrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func isApplied(db *sql.DB, version string) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version=?`, version).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func applyOne(db *sql.DB, version, upSQL string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, stmt := range splitStatements(upSQL) {
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`,
		version, nowRFC3339(),
	); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// extractUp 取 `-- +goose Up` 与 `-- +goose Down` 之间的内容。
// 缺 `-- +goose Up` 标记则报错，避免把整文件（含 Down 段）当 Up 执行（Q-2）。
func extractUp(s string) (string, error) {
	i := strings.Index(s, "-- +goose Up")
	if i < 0 {
		return "", fmt.Errorf("缺少 '-- +goose Up' 标记")
	}
	up := s[i+len("-- +goose Up"):]
	if j := strings.Index(up, "-- +goose Down"); j >= 0 {
		up = up[:j]
	}
	return up, nil
}

// splitStatements 先剥掉行内 -- 尾注（含整行注释）再按 ; 切分为单条语句。
// 先剥注释再切，避免尾注里的 ; 把语句切坏（Q-2）；本项目 DDL 无含 -- 的字符串字面量，
// 故按每行首个 -- 截断是安全的。
func splitStatements(block string) []string {
	var b strings.Builder
	for _, line := range strings.Split(block, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	var out []string
	for _, part := range strings.Split(b.String(), ";") {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}
