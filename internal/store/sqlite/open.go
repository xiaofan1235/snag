// Package sqlite 负责 SQLite 连接管理（driver 接入与连接参数）。
// 这是唯一设置 SQLite 连接级 PRAGMA 的地方。
package sqlite

import (
	"database/sql"
	"fmt"
)

// DriverName 是 database/sql 注册的驱动名。modernc.org/sqlite 注册名为 "sqlite"。
// 驱动本身通过 build tag `sqlite_modernc` 以空白导入接入（见 driver_modernc.go），
// 默认构建不含它，故 go build ./... 零外部依赖；接入后 sql.Open 才能真正连库。
const DriverName = "sqlite"

// Open 打开一个 SQLite 库并设好连接级 PRAGMA：
//   - journal_mode=WAL：读写并发更好（§2.1）；
//   - foreign_keys=1：运行期强制外键（迁移建表顺序已排好，开启亦不冲突）；
//   - busy_timeout=5000：遇写锁最多等 5s，减少 SQLITE_BUSY。
//
// 连接数暂设 1（单写者）。Day 06 再细化为 durable/bulk 双连接（§6.x）。
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
		path,
	)
	db, err := sql.Open(DriverName, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite (%s): %w", DriverName, err)
	}
	return db, nil
}

// Registered 报告 database/sql 是否已注册名为 DriverName 的驱动。
// 测试据此决定是否跳过需要真库的用例（无网环境下驱动未接入时跳过）。
func Registered() bool {
	for _, d := range sql.Drivers() {
		if d == DriverName {
			return true
		}
	}
	return false
}
