module pentagent

// Snag 控制面 monorepo。
// domain 层零外部依赖（§3、Day 01）；默认构建（无 build tag）不编译任何外部驱动，
// UUIDv7 为自实现（internal/domain/ids.go）。
// SQLite 驱动 modernc.org/sqlite（v1.38.2，纯 Go）及其依赖仅在 build tag
// `sqlite_modernc` 下编入（见 internal/store/sqlite/driver_modernc.go），
// 故列于 require 但默认 go build ./... 不参与编译。
// 该驱动要求 go 1.23，是本模块 go 指令为 1.23 的唯一原因（Day 01 文档写 1.22+，此处微调记录在案）。
go 1.23.0

require (
	github.com/gowebpki/jcs v1.0.2
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	go.yaml.in/yaml/v3 v3.0.5
	modernc.org/sqlite v1.38.2
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	modernc.org/libc v1.66.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)
