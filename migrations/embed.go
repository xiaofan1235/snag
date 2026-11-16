// Package migrations 把 Snag 的 SQL 迁移文件嵌入二进制，供 internal/store 的内置迁移器读取。
//
// 这里只放 .sql 与本嵌入声明；goose CLI 会忽略 .go 文件，所以 migrations/ 目录同时满足
// 「goose -dir migrations」与「内置迁移器」两种用法（Day 02 无网适配）。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
