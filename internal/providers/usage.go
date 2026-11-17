package providers

// usage.go —— token 用量（§7.5 预算维度的计量输入）。
//
// 本包只**计量**真实 Usage；预留+结算的预算状态机（§7.5 RESERVED→IN_FLIGHT→SETTLED）是后续
// 预算模块的职责，这里提供其所需的用量数据。

// Usage 是一次调用的 token 用量。
type Usage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int // prompt caching 命中读入（计费通常更低）
	CacheWriteTokens int // 写入缓存
}

// Total 返回输入+输出总 token。
func (u Usage) Total() int { return u.InputTokens + u.OutputTokens }

// Add 累加另一次用量（多轮/修复重试累计）。
func (u *Usage) Add(o Usage) {
	u.InputTokens += o.InputTokens
	u.OutputTokens += o.OutputTokens
	u.CacheReadTokens += o.CacheReadTokens
	u.CacheWriteTokens += o.CacheWriteTokens
}
