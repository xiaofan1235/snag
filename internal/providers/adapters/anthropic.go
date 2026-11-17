// Package adapters 把各家 Provider 适配到 providers.Provider 接口（§7.4）。
//
// 本文件是面向 Claude Messages API 的适配器，用原生 tool-calling 实现结构化输出（§31 A1）：
// 强制 tool_choice 指向角色的 emit 工具，input_schema 即角色契约。
//
// **有据偏离（网络关闭）**：本环境出网受限且无法拉取官方 SDK，故用 stdlib net/http 直接构造
// Messages API 请求/解析响应——结构与分类忠实于 §7.4，但**无法在本环境联网实测**。测试覆盖由
// providers.MockProvider 承载（providers 包内，离线确定性）。真实端点联调留待具备网络的 CI/部署。
// 真·SSE 流式对齐 Day 40（SSE 通道）；本版 StreamGenerate 以非流式回退满足接口。
package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"pentagent/internal/providers"
)

// Anthropic 是 Claude Messages API 适配器。
type Anthropic struct {
	APIKey     string
	BaseURL    string // 默认 https://api.anthropic.com
	Version    string // anthropic-version 头
	HTTP       *http.Client
	DefaultMax int // 未指定 MaxOutput 时的 max_tokens

	mu       sync.Mutex
	inflight map[string]context.CancelFunc // requestID -> cancel（供 Cancel 最佳努力）
}

// NewAnthropic 构造适配器。
func NewAnthropic(apiKey string) *Anthropic {
	return &Anthropic{
		APIKey:     apiKey,
		BaseURL:    "https://api.anthropic.com",
		Version:    "2023-06-01",
		HTTP:       &http.Client{Timeout: 120 * time.Second},
		DefaultMax: 4096,
		inflight:   map[string]context.CancelFunc{},
	}
}

var _ providers.Provider = (*Anthropic)(nil)

// GetCapabilities 声明 Claude 能力。
func (a *Anthropic) GetCapabilities() providers.Capabilities {
	return providers.Capabilities{
		ToolCalling:      true,
		StructuredOutput: true,
		Streaming:        true,
		PromptCaching:    true,
		TokenCounting:    true,
		MaxContext:       200000,
		MaxOutput:        8192,
	}
}

// ---- 请求/响应 线格式 ----

type wireTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type wireMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type wireReq struct {
	Model      string     `json:"model"`
	MaxTokens  int        `json:"max_tokens"`
	System     string     `json:"system,omitempty"`
	Messages   []wireMsg  `json:"messages"`
	Tools      []wireTool `json:"tools,omitempty"`
	ToolChoice any        `json:"tool_choice,omitempty"`
}

type wireContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type wireUsage struct {
	InputTokens          int `json:"input_tokens"`
	OutputTokens         int `json:"output_tokens"`
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
	CacheCreationTokens  int `json:"cache_creation_input_tokens"`
}

type wireResp struct {
	ID         string        `json:"id"`
	StopReason string        `json:"stop_reason"`
	Content    []wireContent `json:"content"`
	Usage      wireUsage     `json:"usage"`
}

type wireErrEnvelope struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Generate 发一次强制 tool-call 的生成请求。
func (a *Anthropic) Generate(ctx context.Context, req providers.Request) (*providers.Response, error) {
	reqID := req.RequestID
	if reqID == "" {
		reqID = "req-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	ctx, cancel := context.WithCancel(ctx)
	a.register(reqID, cancel)
	defer a.unregister(reqID)

	body := wireReq{
		Model:     req.Model,
		MaxTokens: req.MaxOutput,
		System:    req.System,
	}
	if body.MaxTokens <= 0 {
		body.MaxTokens = a.DefaultMax
	}
	for _, m := range req.Messages {
		role := m.Role
		if role == "" {
			role = "user"
		}
		body.Messages = append(body.Messages, wireMsg{Role: role, Content: m.Content})
	}
	if req.Tool != nil {
		body.Tools = []wireTool{{Name: req.Tool.Name, Description: req.Tool.Description, InputSchema: req.Tool.InputSchema}}
		body.ToolChoice = map[string]string{"type": "tool", "name": req.Tool.Name} // 强制单工具（结构化输出）
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, providers.Fault(providers.FaultNone, err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return nil, providers.Fault(providers.FaultNone, err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", a.APIKey)
	httpReq.Header.Set("anthropic-version", a.Version)

	resp, err := a.HTTP.Do(httpReq)
	if err != nil {
		return nil, providers.Fault(providers.FaultNetwork, err) // 网络断开：有界重试
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, classifyHTTP(resp, payload)
	}

	var wr wireResp
	if err := json.Unmarshal(payload, &wr); err != nil {
		return nil, providers.Fault(providers.FaultNone, fmt.Errorf("解析响应失败: %w", err))
	}
	out := &providers.Response{
		RequestID:  reqID,
		StopReason: mapStop(wr.StopReason),
		Usage: providers.Usage{
			InputTokens:      wr.Usage.InputTokens,
			OutputTokens:     wr.Usage.OutputTokens,
			CacheReadTokens:  wr.Usage.CacheReadInputTokens,
			CacheWriteTokens: wr.Usage.CacheCreationTokens,
		},
	}
	for _, c := range wr.Content {
		switch c.Type {
		case "tool_use":
			out.ToolName = c.Name
			out.ToolInput = c.Input
		case "text":
			out.Text += c.Text
		}
	}
	return out, nil
}

// mapStop 归一 stop_reason。
func mapStop(s string) providers.StopReason {
	switch s {
	case "tool_use":
		return providers.StopToolUse
	case "max_tokens":
		return providers.StopMaxTokens
	case "refusal":
		return providers.StopRefusal
	default:
		return providers.StopEndTurn
	}
}

// classifyHTTP 把非 200 响应映射为 §7.4 故障分类。
func classifyHTTP(resp *http.Response, payload []byte) error {
	var env wireErrEnvelope
	_ = json.Unmarshal(payload, &env)
	base := errors.New(resp.Status)
	if env.Error.Message != "" {
		base = fmt.Errorf("%s: %s", env.Error.Type, env.Error.Message)
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests: // 429
		pe := providers.Fault(providers.FaultRateLimited, base)
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				pe.RetryAfter = time.Duration(secs) * time.Second
			}
		}
		return pe
	case resp.StatusCode >= 500: // 5xx
		return providers.Fault(providers.FaultServer, base)
	case env.Error.Type == "overloaded_error":
		return providers.Fault(providers.FaultServer, base)
	case resp.StatusCode == http.StatusBadRequest && isContextOverflow(env.Error.Message):
		return providers.Fault(providers.FaultContextOverflow, base)
	default:
		// 4xx（鉴权/参数等）：不可重试、不换 Provider。
		return providers.Fault(providers.FaultNone, base)
	}
}

func isContextOverflow(msg string) bool {
	// 轻量判定：错误文案含 token/context 超限线索。
	for _, k := range []string{"context window", "maximum", "too long", "max_tokens", "prompt is too"} {
		if containsFold(msg, k) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexFold(s, sub) >= 0)
}

func indexFold(s, sub string) int {
	ls, lsub := len(s), len(sub)
	for i := 0; i+lsub <= ls; i++ {
		ok := true
		for j := 0; j < lsub; j++ {
			if lower(s[i+j]) != lower(sub[j]) {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

func lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// CountTokens 调 count_tokens 端点（结构化占位；离线不可实测）。
func (a *Anthropic) CountTokens(ctx context.Context, req providers.Request) (int, error) {
	// 真实实现 POST /v1/messages/count_tokens；离线环境下回退为粗略估算，避免联网。
	n := len(req.System)
	for _, m := range req.Messages {
		n += len(m.Content)
	}
	return n/4 + 1, nil
}

// StreamGenerate 非流式回退（真·SSE 对齐 Day 40）：跑一次 Generate，拆成一增量 + Done。
func (a *Anthropic) StreamGenerate(ctx context.Context, req providers.Request) (<-chan providers.StreamChunk, error) {
	ch := make(chan providers.StreamChunk, 2)
	resp, err := a.Generate(ctx, req)
	if err != nil {
		ch <- providers.StreamChunk{Done: true, Err: err}
		close(ch)
		return ch, nil
	}
	ch <- providers.StreamChunk{TextDelta: resp.Text, ToolInputDelta: string(resp.ToolInput)}
	ch <- providers.StreamChunk{Done: true, StopReason: resp.StopReason, Usage: resp.Usage}
	close(ch)
	return ch, nil
}

// Cancel 取消一个在途请求（最佳努力：取消其 ctx，§7.5）。
func (a *Anthropic) Cancel(_ context.Context, requestID string) error {
	a.mu.Lock()
	cancel := a.inflight[requestID]
	a.mu.Unlock()
	if cancel == nil {
		return fmt.Errorf("adapters: 无在途请求 %q", requestID)
	}
	cancel()
	return nil
}

func (a *Anthropic) register(id string, cancel context.CancelFunc) {
	a.mu.Lock()
	a.inflight[id] = cancel
	a.mu.Unlock()
}

func (a *Anthropic) unregister(id string) {
	a.mu.Lock()
	delete(a.inflight, id)
	a.mu.Unlock()
}
