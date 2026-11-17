// Package providers 是模型 Provider 抽象（§7.4）+ 每角色一次原生 tool-call 的结构化输出（§31 A1）。
//
// 设计立场：模型**只提案**，且提案一定合 schema——每个角色经一次 provider 原生 tool-call 输出，
// 工具入参 schema 即该角色的结构化契约（PlanPatch/ActionProposal/Verdict，Day 04 冻结）。约束解码
// （原生 tool-calling / GBNF）是主路，「修复一次」退为兜底（§31 A1）。本包只负责「拿到合 schema 的
// 结构化输出」，不碰库、不执行动作——落库由 PlanCompiler（§6.4），执行由 Executor。
package providers

import (
	"context"
	"encoding/json"
	"time"
)

// Provider 是模型能力的统一内部接口（§7.4）。各家 Provider 由 adapters 适配到此接口。
type Provider interface {
	// Generate 单次生成。若 req.Tool 非空，强制一次该工具的原生 tool-call（结构化输出）。
	Generate(ctx context.Context, req Request) (*Response, error)
	// StreamGenerate 流式生成；chunk 顺序投递，末个 Done=true 携带最终 Usage/StopReason。
	StreamGenerate(ctx context.Context, req Request) (<-chan StreamChunk, error)
	// CountTokens 估算/精算请求的输入 token 数（能力见 Capabilities.TokenCounting）。
	CountTokens(ctx context.Context, req Request) (int, error)
	// GetCapabilities 返回该 Provider/模型的能力声明。
	GetCapabilities() Capabilities
	// Cancel 尽力取消一个在途请求（按 provider_request_id，§7.5）。无状态补全下等价于取消其 ctx。
	Cancel(ctx context.Context, requestID string) error
}

// Capabilities 是 Provider 能力声明（§7.4）。
type Capabilities struct {
	ToolCalling      bool
	StructuredOutput bool
	Streaming        bool
	PromptCaching    bool
	TokenCounting    bool
	MaxContext       int
	MaxOutput        int
}

// Message 是一条对话消息。Role ∈ {user, assistant}；系统提示走 Request.System。
type Message struct {
	Role    string
	Content string
}

// Tool 是「角色 emit 工具」：强制模型经此一次 tool-call 产出结构化结果。
// InputSchema 为该角色的冻结契约 JSON Schema（Day 04），模型入参即结构化输出。
type Tool struct {
	Name        string          // 例：emit_plan_patch / emit_action_proposal / emit_verdict
	Description string          // 给模型的工具说明
	InputSchema json.RawMessage // input_schema = PlanPatch/ActionProposal/Verdict 的 schema
}

// Request 是一次生成请求。
type Request struct {
	RequestID string    // 幂等 + Cancel 用（§7.5 provider_request_id）；空则由 adapter 生成
	Role      string    // strategist / executor / verifier（取 Profile）
	Model     string    // 具体模型名（由 Registry Profile 解析填充）
	System    string    // 系统提示
	Messages  []Message // 对话历史
	Tool      *Tool     // 非空则强制一次原生 tool-call（tool_choice=该工具）
	MaxOutput int       // 最大输出 token
	Effort    string    // 推理档位（low/medium/high），由 adapter 翻译
}

// StopReason 是生成终止原因（跨 Provider 归一）。
type StopReason string

const (
	StopToolUse   StopReason = "tool_use"   // 正常产出了 tool-call（结构化输出就绪）
	StopEndTurn   StopReason = "end_turn"   // 普通文本结束（无强制工具时）
	StopMaxTokens StopReason = "max_tokens" // 输出被截断（§7.4：不执行工具，重新生成）
	StopRefusal   StopReason = "refusal"    // 模型安全拒绝（§7.4：当前提案停止，不换 Provider 绕过）
)

// Response 是一次生成结果。
type Response struct {
	RequestID  string
	ToolName   string          // 命中的工具名（强制 tool-call 时 == req.Tool.Name）
	ToolInput  json.RawMessage // 结构化输出（须经 req.Tool.InputSchema 校验）
	Text       string          // 无工具时的文本输出
	StopReason StopReason
	Usage      Usage
}

// StreamChunk 是流式增量。
type StreamChunk struct {
	TextDelta      string
	ToolInputDelta string
	Done           bool
	StopReason     StopReason
	Usage          Usage // 仅 Done=true 时有效
	Err            error // 非 nil 表示流中断（通常为 *ProviderError）
}

// FaultClass 是 Provider 故障分类（§7.4），决定上层行为。
type FaultClass int

const (
	FaultNone            FaultClass = iota
	FaultNetwork                    // 网络断开：有界重试、指数退避
	FaultRateLimited                // 429：按 Retry-After 等待
	FaultServer                     // 5xx：有界重试
	FaultContextOverflow            // 上下文超限：重建/压缩上下文
	FaultInvalidSchema              // 无效 Schema：最多一次修复请求
	FaultTruncated                  // 输出被截断：不执行工具，重新生成
	FaultQuotaExhausted             // 配额耗尽：Run 进入 BLOCKED/PARTIAL
	FaultSafetyRefusal              // 安全拒绝：当前提案停止，不自动换 Provider 绕过
	FaultModelSwitch                // 模型切换：从领域状态重建，不迁移私有上下文
)

func (f FaultClass) String() string {
	switch f {
	case FaultNetwork:
		return "network"
	case FaultRateLimited:
		return "rate_limited"
	case FaultServer:
		return "server_5xx"
	case FaultContextOverflow:
		return "context_overflow"
	case FaultInvalidSchema:
		return "invalid_schema"
	case FaultTruncated:
		return "truncated"
	case FaultQuotaExhausted:
		return "quota_exhausted"
	case FaultSafetyRefusal:
		return "safety_refusal"
	case FaultModelSwitch:
		return "model_switch"
	default:
		return "none"
	}
}

// Retryable 报告该类故障是否应由有界退避重试处理（§7.4）。安全拒绝/无效 Schema/截断/配额/上下文
// 超限均**不**走盲重试——各有专门处置，尤其安全拒绝绝不换 Provider 绕过。
func (f FaultClass) Retryable() bool {
	switch f {
	case FaultNetwork, FaultRateLimited, FaultServer:
		return true
	default:
		return false
	}
}

// ProviderError 携带故障分类，供上层按 §7.4 分类处置。
type ProviderError struct {
	Class      FaultClass
	RetryAfter time.Duration // 仅 429 有意义
	Err        error         // 底层错误
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return "provider error [" + e.Class.String() + "]: " + e.Err.Error()
	}
	return "provider error [" + e.Class.String() + "]"
}

func (e *ProviderError) Unwrap() error { return e.Err }

// Fault 构造一个带分类的 ProviderError。
func Fault(class FaultClass, err error) *ProviderError { return &ProviderError{Class: class, Err: err} }

// FaultOf 从任意 error 提取 FaultClass；非 ProviderError 视为 FaultNone。
func FaultOf(err error) FaultClass {
	if pe, ok := err.(*ProviderError); ok {
		return pe.Class
	}
	return FaultNone
}
