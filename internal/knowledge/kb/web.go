package kb

import "context"

// web.go —— 联网知识检索（§36）+ 卡住即检索（§36.2）。
//
// 联网走 utility 通道（§36.1，与目标 Proxy 不同 Egress Channel，不混用 Scope）；结果**不可信、标参考**，
// 不能改 Scope/Policy/Coverage/Finding。地址/查询由 harness 发起，不由模型任意选第三方。
//
// 有据偏离（网络关闭）：真实 web_search 需出网，本环境不可得；故定义 WebSearcher 接口，离线由 stub 注入，
// 具备环境后接真实 utility 通道客户端。本文件实现的是「卡住触发检索」的确定性编排，与出网实现解耦。

// WebResult 是一条联网结果（不可信、仅参考）。
type WebResult struct {
	Title     string
	URL       string
	Snippet   string
	Untrusted bool // 恒 true：联网结果不可信
	Reference bool // 恒 true：只作参考，不作权威
}

// WebSearcher 是联网检索接口（utility 通道实现，可插拔）。
type WebSearcher interface {
	Search(ctx context.Context, query string) ([]WebResult, error)
}

// markReference 把结果统一标为不可信参考（§36/§46.3）。
func markReference(rs []WebResult) []WebResult {
	for i := range rs {
		rs[i].Untrusted = true
		rs[i].Reference = true
	}
	return rs
}

// WebSearch 经 utility 通道检索并统一标记为不可信参考。
func WebSearch(ctx context.Context, s WebSearcher, query string) ([]WebResult, error) {
	rs, err := s.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	return markReference(rs), nil
}

// StuckTrigger 判定是否「卡住即检索」（§36.2）：无进展步数达阈且本方向尚未检索过，则在进入 Reframe 前
// 主动联网检索注入。确定性，便于测试。
type StuckTrigger struct {
	Threshold int
}

// ShouldSearch 报告某方向是否应触发联网检索：无进展步数 ≥ 阈值且尚未搜过。
func (t StuckTrigger) ShouldSearch(noProgressSteps int, alreadySearched bool) bool {
	th := t.Threshold
	if th <= 0 {
		th = 3
	}
	return noProgressSteps >= th && !alreadySearched
}
