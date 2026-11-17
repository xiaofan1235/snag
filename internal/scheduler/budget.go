package scheduler

// budget.go —— 调度前置过滤门（§6.8「先过滤，再评分」）。
//
// 过滤维度（§6.8）：Scope / Policy / Dependency / Identity / Budget / Approval / Rate。
// 本文件用可注入的门（Gate）组合实现过滤，Budget 门单列（预算账本在 internal/agent，经接口注入避免环依赖）。
// 不 eligible 的 Job 根本不进评分队列；风险动作不因分高绕过红线（红线执法在 PDP/Permit/Proxy）。

// Gate 是一个过滤门：返回是否通过及（不通过时的）原因。
type Gate struct {
	Name string
	Pass func(jobID string) (bool, string)
}

// Filters 是有序过滤门链（§6.8）。
type Filters struct {
	gates []Gate
}

// NewFilters 构造。
func NewFilters(gates ...Gate) *Filters { return &Filters{gates: gates} }

// Add 追加一个门。
func (f *Filters) Add(g Gate) *Filters { f.gates = append(f.gates, g); return f }

// Eligible 依次过门：全过返回 (true,"")；任一不过返回 (false, "门名:原因")，短路。
func (f *Filters) Eligible(jobID string) (bool, string) {
	for _, g := range f.gates {
		ok, reason := g.Pass(jobID)
		if !ok {
			return false, g.Name + ":" + reason
		}
	}
	return true, ""
}

// BudgetSource 由持有预算账本者实现（internal/agent.Ledger 的适配），避免 scheduler→agent 环依赖。
type BudgetSource interface {
	// RemainingMicroUSD 返回某 Run 普通 Executor 可用余额（micro_usd）。
	RemainingMicroUSD(runID string) int64
}

// BudgetGate 构造一个「预算未耗尽」过滤门（§6.8 Budget 过滤）。jobRun 把 jobID 映射到 runID。
func BudgetGate(src BudgetSource, jobRun func(jobID string) string) Gate {
	return Gate{
		Name: "budget",
		Pass: func(jobID string) (bool, string) {
			if src.RemainingMicroUSD(jobRun(jobID)) <= 0 {
				return false, "run budget exhausted"
			}
			return true, ""
		},
	}
}
