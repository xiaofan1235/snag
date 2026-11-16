module pentagent

// Snag 控制面 monorepo。首版只用标准库：domain 层零外部依赖（§3、Day 01）。
// UUIDv7 自实现（见 internal/domain/ids.go），不引入 github.com/google/uuid，
// 以便在无网络环境也能 go build / go test。
go 1.22
