package store

import (
	"database/sql"
	"fmt"

	"pentagent/internal/store/sqlite"
)

// reader.go —— 只读连接池（§31 C2：读写分离）。
//
// WAL 允许多个读者与单个写者并发，故读走独立连接池（可多连接），不与写路径争锁。
// query_only=1 从连接层禁止意外写入。

// Reader 是只读连接池。
type Reader struct {
	db *sql.DB
}

// OpenReader 打开只读连接池。maxConns 控制并发读连接数（WAL 下可 >1）。
func OpenReader(path string, maxConns int) (*Reader, error) {
	if maxConns < 1 {
		maxConns = 4
	}
	// 只读连接不设 journal_mode——库的 WAL 由写连接设定（文件级），只读连接改不了也不需要（Q-14）。
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"+
			"&_pragma=query_only(1)&_pragma=temp_store(MEMORY)",
		path,
	)
	db, err := sql.Open(sqlite.DriverName, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxConns)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Reader{db: db}, nil
}

// DB 返回底层只读 *sql.DB。
func (r *Reader) DB() *sql.DB { return r.db }

// Close 关闭只读池。
func (r *Reader) Close() error { return r.db.Close() }
