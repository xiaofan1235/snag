package scheduler

// priority.go —— Job 调度评分（§6.8）。纯函数、可测、确定性。
//
// 先过滤再评分（§6.8）：过滤由 Filters（budget.go/scheduler.go）把关，本文件只管「eligible Job 怎么排」。
// 评分是加权和：收益因子正向、成本/失败历史负向、age 正向防饿死（只影响 eligible Job 的顺序）。

// ScoreFactors 是 §6.8 的评分因子（均为已归一的无量纲量，建议 0..1，age 可 >1）。
type ScoreFactors struct {
	PotentialImpact     float64 // 潜在影响
	ExpectedInformation float64 // 预期信息增益
	ChainValue          float64 // 对攻击链的价值
	SurfaceCriticality  float64 // 业务关键度
	Novelty             float64 // 与历史动作的差异
	EvidenceConfidence  float64 // 前置证据质量
	Cost                float64 // 请求/Token/时间成本（负向）
	FailureHistory      float64 // 历史失败惩罚（负向）
	Age                 float64 // 等待时长，防低优先级饿死（正向）
}

// Weights 是各因子权重。
type Weights struct {
	PotentialImpact     float64
	ExpectedInformation float64
	ChainValue          float64
	SurfaceCriticality  float64
	Novelty             float64
	EvidenceConfidence  float64
	Cost                float64
	FailureHistory      float64
	Age                 float64
}

// DefaultWeights 给一组合理默认权重。
func DefaultWeights() Weights {
	return Weights{
		PotentialImpact: 1.0, ExpectedInformation: 0.8, ChainValue: 0.9, SurfaceCriticality: 0.7,
		Novelty: 0.6, EvidenceConfidence: 0.5, Cost: 0.8, FailureHistory: 0.7, Age: 0.3,
	}
}

// Score 计算 Job 的调度分：收益正向加权，Cost/FailureHistory 负向，Age 正向防饿死。
func Score(f ScoreFactors, w Weights) float64 {
	return w.PotentialImpact*f.PotentialImpact +
		w.ExpectedInformation*f.ExpectedInformation +
		w.ChainValue*f.ChainValue +
		w.SurfaceCriticality*f.SurfaceCriticality +
		w.Novelty*f.Novelty +
		w.EvidenceConfidence*f.EvidenceConfidence +
		w.Age*f.Age -
		w.Cost*f.Cost -
		w.FailureHistory*f.FailureHistory
}
