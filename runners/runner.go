// Package runners 定义 Runner↔Core 的执行面接口（§25）。
//
// 【有据偏离】dev-plan 用 gRPC+Protobuf，但本环境无法引入 grpc/protobuf 依赖（网络受限）；
// 改为进程内 Go 接口，按 gRPC 形状设计（Action 入、Receipt 出），日后可平滑包 gRPC 传输层。
package runners

import (
	"context"

	"pentagent/internal/domain"
	"pentagent/internal/network"
)

// Action 是下发给 Runner 的一个已授权动作（经 Permit/Capability 绑定）。
type Action struct {
	ActionID         domain.ID
	CapabilityID     domain.ID
	URL              string
	Method           string
	IdempotencyClass domain.IdempotencyClass
}

// Receipt 是执行回执（Day 21 正式化对账与三方一致；此处先给执行结果的最小回执）。
type Receipt struct {
	ActionID       domain.ID
	Status         int    // 目标响应码（0=未发出）
	ResponseDigest string // 响应体 SHA-256
	ResponseBytes  int
	Blocked        bool // 被 Proxy 或门禁拦截
	BlockCode      string
	DurationMS     int64
	Error          string
}

// Runner 执行一个动作并回回执。实现须经 EgressLease（唯一出口 Proxy）访问目标，不得直连。
type Runner interface {
	Execute(ctx context.Context, a Action, lease network.EgressLease) (Receipt, error)
}
