package providers

import (
	"context"
	"errors"
	"sync"
)

// mock.go —— 可编程 Mock Provider（§5.5 测试用）。按脚本逐次返回预设 Response 或 *ProviderError，
// 用于驱动结构化输出/修复一次/退避/安全拒绝等路径的确定性测试，也供下游模块（Day 24+）离线测试复用。

// MockStep 是一次调用的脚本项：Err 非空则该次返回错误，否则返回 Resp。
type MockStep struct {
	Resp *Response
	Err  error
}

// MockProvider 按 steps 顺序应答 Generate。超出脚本长度返回 errNoMoreSteps。
type MockProvider struct {
	caps  Capabilities
	mu    sync.Mutex
	steps []MockStep
	calls int
	seen  []Request       // 记录收到的请求（断言修复请求、tool_choice 等）
	canc  map[string]bool // 被 Cancel 的 requestID
}

var errNoMoreSteps = errors.New("mock provider: 脚本已用尽")

// NewMockProvider 构造。caps 为能力声明，steps 为逐次应答脚本。
func NewMockProvider(caps Capabilities, steps ...MockStep) *MockProvider {
	return &MockProvider{caps: caps, steps: steps, canc: map[string]bool{}}
}

// Generate 弹出下一个脚本项。
func (m *MockProvider) Generate(_ context.Context, req Request) (*Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen = append(m.seen, req)
	i := m.calls
	m.calls++
	if i >= len(m.steps) {
		return nil, errNoMoreSteps
	}
	s := m.steps[i]
	if s.Err != nil {
		return nil, s.Err
	}
	return s.Resp, nil
}

// StreamGenerate 最小实现：跑一次 Generate，把结果拆成「一个增量 + Done」两块投递。
func (m *MockProvider) StreamGenerate(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	resp, err := m.Generate(ctx, req)
	ch := make(chan StreamChunk, 2)
	if err != nil {
		ch <- StreamChunk{Done: true, Err: err}
		close(ch)
		return ch, nil
	}
	ch <- StreamChunk{TextDelta: resp.Text, ToolInputDelta: string(resp.ToolInput)}
	ch <- StreamChunk{Done: true, StopReason: resp.StopReason, Usage: resp.Usage}
	close(ch)
	return ch, nil
}

// CountTokens 粗略估算：每 4 字节约 1 token（仅测试占位）。
func (m *MockProvider) CountTokens(_ context.Context, req Request) (int, error) {
	n := len(req.System)
	for _, msg := range req.Messages {
		n += len(msg.Content)
	}
	return n/4 + 1, nil
}

// GetCapabilities 返回声明的能力。
func (m *MockProvider) GetCapabilities() Capabilities { return m.caps }

// Cancel 记录被取消的 requestID（无状态补全下即最佳努力）。
func (m *MockProvider) Cancel(_ context.Context, requestID string) error {
	m.mu.Lock()
	m.canc[requestID] = true
	m.mu.Unlock()
	return nil
}

// Calls 返回已发生的调用次数（测试断言「修复一次」= 2 次）。
func (m *MockProvider) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// Seen 返回收到的请求副本（断言修复请求追加了修复消息）。
func (m *MockProvider) Seen() []Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Request, len(m.seen))
	copy(out, m.seen)
	return out
}

// Canceled 报告某 requestID 是否被取消过。
func (m *MockProvider) Canceled(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.canc[id]
}
