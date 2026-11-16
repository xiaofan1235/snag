//go:build sqlite_modernc

// 接入纯 Go 的 SQLite 驱动 modernc.org/sqlite（注册名 "sqlite"）。
//
// 默认构建**不含**本文件（受 build tag 保护），因此 go build ./... 无需任何外部依赖。
// 启用真实数据库的步骤（需要网络）：
//  1. go get modernc.org/sqlite
//  2. go build -tags sqlite_modernc ./...   或   go test -tags sqlite_modernc ./...
package sqlite

import _ "modernc.org/sqlite"
