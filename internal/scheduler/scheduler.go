package scheduler

import "sync"

// scheduler.go —— 无进展检测 + 转向阶梯 + 方向泊车（§6.9/§32.5）+ 评分滞回与 ε 探索（§1⑤/A7）。
//
// 停滞判定（§32.5）：动作/新信息比趋 0 = 语义循环/停滞；「进展」含 insight（新可验证假设也算，§31 A6），
// 故「安静地思考、连点成线」不会被误判为卡住。转向阶梯：Job 收尾 → 方向降权/泊车（Worker 流向高分方向）
// → Epoch 切换 → 连续 3 Epoch 无进展 Reframe。泊车不是杀死：记停滞原因，匹配的新事实到达时自动唤醒。

// DirState 方向状态。
type DirState int

const (
	DirActive DirState = iota
	DirParked
)

// ParkReason 泊车原因（§32.5）。
type ParkReason string

const (
	ParkNeedsIdentity     ParkReason = "needs_identity"
	ParkNeedsPrecondition ParkReason = "needs_precondition"
	ParkExhausted         ParkReason = "exhausted"
)

type dirProgress struct {
	actions         int
	newInfo         int
	noProgressSteps int
	state           DirState
	parkReason      ParkReason
	score           float64
}

// ProgressTracker 按方向跟踪进展并驱动转向（§32.5）。线程安全。
type ProgressTracker struct {
	mu            sync.Mutex
	dirs          map[string]*dirProgress
	dirStallSteps int // 方向连续无进展步数阈值 → 停滞
}

// NewProgressTracker 构造。dirStallSteps<=0 用默认 4。
func NewProgressTracker(dirStallSteps int) *ProgressTracker {
	if dirStallSteps <= 0 {
		dirStallSteps = 4
	}
	return &ProgressTracker{dirs: map[string]*dirProgress{}, dirStallSteps: dirStallSteps}
}

func (p *ProgressTracker) dir(d string) *dirProgress {
	g := p.dirs[d]
	if g == nil {
		g = &dirProgress{state: DirActive}
		p.dirs[d] = g
	}
	return g
}

// RecordAction 记一次该方向的动作（暂增无进展计数，等 RecordProgress 清零）。
func (p *ProgressTracker) RecordAction(d string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	g := p.dir(d)
	g.actions++
	g.noProgressSteps++
}

// RecordProgress 记一次有效进展（§6.9：新接口/参数/身份/Observation/证实或证伪假设/新假设 insight…均算）。
func (p *ProgressTracker) RecordProgress(d string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	g := p.dir(d)
	g.newInfo++
	g.noProgressSteps = 0
}

// Stalled 报告某方向是否停滞：连续无进展步数达阈，或有动作却零新信息（动作/新信息比趋 0）。
func (p *ProgressTracker) Stalled(d string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	g := p.dir(d)
	if g.noProgressSteps >= p.dirStallSteps {
		return true
	}
	return g.actions >= p.dirStallSteps && g.newInfo == 0
}

// Park 泊车一个停滞方向，记原因（§32.5：不是杀死）。
func (p *ProgressTracker) Park(d string, reason ParkReason) {
	p.mu.Lock()
	defer p.mu.Unlock()
	g := p.dir(d)
	g.state = DirParked
	g.parkReason = reason
}

// Reactivate 唤醒泊车方向（新事实到达且匹配泊车原因时），清零无进展计数重新投入。
func (p *ProgressTracker) Reactivate(d string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	g := p.dir(d)
	g.state = DirActive
	g.noProgressSteps = 0
}

// SetScore 设置方向评分（供选择器 ε/滞回用）。
func (p *ProgressTracker) SetScore(d string, score float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dir(d).score = score
}

// ActiveScores 返回所有 ACTIVE 方向的评分快照（泊车方向不参与 Worker 分配）。
func (p *ProgressTracker) ActiveScores() map[string]float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]float64{}
	for d, g := range p.dirs {
		if g.state == DirActive {
			out[d] = g.score
		}
	}
	return out
}

// Selector 选择主攻方向：评分滞回（切主攻需分差超阈值，§1⑤）+ ε 探索（防隧道视野，§A7）。
type Selector struct {
	hysteresis float64
	epsilon    float64
	rng        func() float64 // [0,1)，可注入；nil 则无探索
	current    string
}

// NewSelector 构造。rng 返回 [0,1)，nil 关闭 ε 探索（确定性）。
func NewSelector(hysteresis, epsilon float64, rng func() float64) *Selector {
	return &Selector{hysteresis: hysteresis, epsilon: epsilon, rng: rng}
}

// Current 返回当前主攻方向。
func (s *Selector) Current() string { return s.current }

// Pick 从 ACTIVE 方向评分中选主攻：以 ε 概率随机探索；否则取最高分，但仅当超过当前方向分差阈值才切换
// （滞回防抖动）。当前方向不在候选（已泊车）时无条件切到最高分。返回选中方向（空集返回 ""）。
func (s *Selector) Pick(scores map[string]float64) string {
	if len(scores) == 0 {
		s.current = ""
		return ""
	}
	// ε 探索：随机选一个（确定性顺序 + rng 决定是否探索、选谁）。
	keys := sortedKeys(scores)
	if s.rng != nil && s.epsilon > 0 && s.rng() < s.epsilon {
		idx := int(s.rng() * float64(len(keys)))
		if idx >= len(keys) {
			idx = len(keys) - 1
		}
		s.current = keys[idx]
		return s.current
	}
	// 取最高分（同分按 key 稳定）。
	top := keys[0]
	for _, k := range keys {
		if scores[k] > scores[top] {
			top = k
		}
	}
	// 滞回：当前方向仍在候选时，挑战者须超过当前分差阈值才切换。
	if cur, ok := scores[s.current]; ok && s.current != "" {
		if scores[top]-cur <= s.hysteresis {
			return s.current // 分差不够，不抖动
		}
	}
	s.current = top
	return top
}

func sortedKeys(m map[string]float64) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	// 简单插入排序（方向数很小），保证确定性。
	for i := 1; i < len(ks); i++ {
		for j := i; j > 0 && ks[j-1] > ks[j]; j-- {
			ks[j-1], ks[j] = ks[j], ks[j-1]
		}
	}
	return ks
}
