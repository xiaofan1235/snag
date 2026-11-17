package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"pentagent/internal/canon"
)

// structured.go —— 每角色「一次原生 tool-call 的结构化输出」+ §7.4 故障处置。
//
// 主路：强制一次原生 tool-call，工具入参 schema 即角色契约（Day 04 冻结），模型产出即合 schema。
// 兜底：产出未过 schema 校验 → **至多一次**修复请求（§31 A1「修复一次退为兜底」），仍不过 fail-closed。
// 退避：网络/429/5xx 走有界指数退避（§7.4）；安全拒绝/截断/配额/上下文超限各自处置，**绝不**盲重试、
// 更不自动换 Provider 绕过安全拒绝（§7.4）。

// RetryPolicy 有界退避参数（§7.4）。
type RetryPolicy struct {
	MaxAttempts int           // 含首次尝试
	BaseDelay   time.Duration // 指数退避基值
	MaxDelay    time.Duration // 单次退避上限（也用于夹断 Retry-After）
}

// DefaultRetryPolicy 合理默认。
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 4, BaseDelay: 200 * time.Millisecond, MaxDelay: 10 * time.Second}
}

func (p RetryPolicy) withDefaults() RetryPolicy {
	if p.MaxAttempts <= 0 {
		return DefaultRetryPolicy()
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 200 * time.Millisecond
	}
	return p
}

// delay 计算第 attempt 次失败后的等待：429 优先按 Retry-After，否则指数退避，均夹断 MaxDelay。
func (p RetryPolicy) delay(attempt int, err error) time.Duration {
	if pe, ok := err.(*ProviderError); ok && pe.Class == FaultRateLimited && pe.RetryAfter > 0 {
		d := pe.RetryAfter
		if p.MaxDelay > 0 && d > p.MaxDelay {
			d = p.MaxDelay
		}
		return d
	}
	shift := attempt - 1
	if shift > 20 {
		shift = 20
	}
	d := p.BaseDelay << shift
	if p.MaxDelay > 0 && d > p.MaxDelay {
		d = p.MaxDelay
	}
	return d
}

// Retrier 以有界退避调用 Provider.Generate。Sleep 可注入（测试用），nil 则 ctx-aware 真睡。
type Retrier struct {
	Policy RetryPolicy
	Sleep  func(ctx context.Context, d time.Duration) error
}

func ctxSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Call 调 Generate，对可重试故障（网络/429/5xx）有界退避重试；不可重试故障（安全拒绝/无效 Schema/
// 截断/配额/上下文超限）立即返回，**不换 Provider**（§7.4）。
func (rt Retrier) Call(ctx context.Context, p Provider, req Request) (*Response, error) {
	policy := rt.Policy.withDefaults()
	sleep := rt.Sleep
	if sleep == nil {
		sleep = ctxSleep
	}
	var lastErr error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		resp, err := p.Generate(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !FaultOf(err).Retryable() {
			return nil, err // 安全拒绝等：停在当前提案，不重试、不换 Provider
		}
		if attempt == policy.MaxAttempts {
			break
		}
		if serr := sleep(ctx, policy.delay(attempt, err)); serr != nil {
			return nil, serr
		}
	}
	return nil, lastErr
}

// checkStop 把 StopReason 翻成 §7.4 故障；正常（tool_use/end_turn）返回 nil。
func checkStop(resp *Response) error {
	switch resp.StopReason {
	case StopMaxTokens:
		return Fault(FaultTruncated, errors.New("输出被截断，不执行工具（§7.4）"))
	case StopRefusal:
		return Fault(FaultSafetyRefusal, errors.New("模型安全拒绝，当前提案停止（§7.4）"))
	}
	return nil
}

// validEmit 校验一次产出是否为「正确工具 + 合 schema 的入参」。
func validEmit(resp *Response, tool *Tool) error {
	if resp.ToolName != tool.Name {
		return fmt.Errorf("期望工具 %q，实际 %q", tool.Name, resp.ToolName)
	}
	if len(resp.ToolInput) == 0 {
		return errors.New("工具入参为空")
	}
	return canon.ValidateAgainst(tool.InputSchema, resp.ToolInput)
}

func repairPrompt(schemaErr error) string {
	return "上一次工具调用入参未通过 schema 校验：" + schemaErr.Error() +
		"。请严格按工具 input_schema 重新调用一次该工具，仅修正不合规处，不要新增 schema 未定义的字段。"
}

// withRepair 构造修复请求：保留原消息，追加一条修复指令（最小实现，不改原 Request）。
func withRepair(req Request, schemaErr error) Request {
	r := req
	r.Messages = append(append([]Message{}, req.Messages...), Message{Role: "user", Content: repairPrompt(schemaErr)})
	return r
}

// EmitStructured 让角色经一次原生 tool-call 产出结构化输出并按 Tool.InputSchema 校验；不过则发起
// **至多一次**修复请求，仍不过返回 FaultInvalidSchema（§31 A1）。截断/安全拒绝直接按 §7.4 返回，
// 不修复、不换 Provider。网络/429/5xx 的退避由 rt 承担（每次 emit 都经 rt.Call）。
// 返回合 schema 的结构化入参（json.RawMessage）与累计 Usage。
func EmitStructured(ctx context.Context, p Provider, req Request, rt Retrier) (json.RawMessage, Usage, error) {
	var usage Usage
	if req.Tool == nil {
		return nil, usage, errors.New("providers: EmitStructured 需要 req.Tool（emit 工具）")
	}

	// 第一次：原生 tool-call（主路）。
	resp, err := rt.Call(ctx, p, req)
	if err != nil {
		return nil, usage, err
	}
	usage.Add(resp.Usage)
	if serr := checkStop(resp); serr != nil {
		return nil, usage, serr // 截断/安全拒绝：不进修复
	}
	ve := validEmit(resp, req.Tool)
	if ve == nil {
		return resp.ToolInput, usage, nil
	}

	// 兜底：修复一次。
	resp2, err := rt.Call(ctx, p, withRepair(req, ve))
	if err != nil {
		return nil, usage, err
	}
	usage.Add(resp2.Usage)
	if serr := checkStop(resp2); serr != nil {
		return nil, usage, serr
	}
	if ve2 := validEmit(resp2, req.Tool); ve2 != nil {
		return nil, usage, Fault(FaultInvalidSchema, fmt.Errorf("修复后仍不合 schema: %w", ve2))
	}
	return resp2.ToolInput, usage, nil
}
