package outbox

import (
	"context"
	"sync"
)

// MemBus 是内存事件汇（Day 10 默认实现，Day 40 由 SSE 取代）。线程安全，按投递顺序累积消息。
// 它只收集、从不失败——失败重试路径的测试用自定义失败 Bus 覆盖。
type MemBus struct {
	mu   sync.Mutex
	msgs []Message
}

// NewMemBus 构造一个空 MemBus。
func NewMemBus() *MemBus { return &MemBus{} }

// Publish 记录一条消息。
func (b *MemBus) Publish(_ context.Context, msg Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, msg)
	return nil
}

// Messages 返回已发消息的快照副本。
func (b *MemBus) Messages() []Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Message, len(b.msgs))
	copy(out, b.msgs)
	return out
}

// Len 返回已发消息数。
func (b *MemBus) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.msgs)
}
