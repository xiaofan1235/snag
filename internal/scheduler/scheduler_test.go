package scheduler

import "testing"

// 评分纯函数：收益正向、成本/失败负向、age 防饿死。
func TestScorePureAndDeterministic(t *testing.T) {
	w := DefaultWeights()
	f := ScoreFactors{PotentialImpact: 0.9, ExpectedInformation: 0.5, Cost: 0.3, FailureHistory: 0.2}
	a, b := Score(f, w), Score(f, w)
	if a != b {
		t.Fatal("评分应确定性")
	}
	// 成本上升 → 分数下降。
	f2 := f
	f2.Cost = 0.9
	if Score(f2, w) >= Score(f, w) {
		t.Fatal("成本上升应降低分数")
	}
}

// 防饿死：其余相同、age 更大的 Job 分数更高（age 只影响 eligible 顺序，§6.8）。
func TestAgeAntiStarvation(t *testing.T) {
	w := DefaultWeights()
	base := ScoreFactors{PotentialImpact: 0.5}
	young, old := base, base
	old.Age = 5
	if Score(old, w) <= Score(young, w) {
		t.Fatal("age 更大应评分更高，防饿死")
	}
}

// 去重：全新→放行；同指纹异载荷→软降权（惩罚增长）；字节级全重复→硬拒（§6.9/A6）。
func TestDedupeSoftAndHard(t *testing.T) {
	d := NewDeduper()
	fp := ActionFingerprint{NormalizedEndpoint: "/api/u", Identity: "idn-1", AttackType: "idor", Parameter: "id", NormalizedPayloadClass: "int"}

	if dec, _ := d.Check(fp, []byte("id=1")); dec != DedupeAllow {
		t.Fatalf("首个动作应放行，实得 %v", dec)
	}
	dec, pen := d.Check(fp, []byte("id=2")) // 同指纹异载荷
	if dec != DedupeSoftDownweight || pen != 1 {
		t.Fatalf("同指纹重复应软降权 penalty=1，实得 %v/%v", dec, pen)
	}
	dec, pen = d.Check(fp, []byte("id=3"))
	if dec != DedupeSoftDownweight || pen != 2 {
		t.Fatalf("软降权惩罚应随次数增长到 2，实得 %v/%v", dec, pen)
	}
	// 字节级全重复（同指纹 + 同载荷 id=2）→ 硬拒。
	if dec, _ := d.Check(fp, []byte("id=2")); dec != DedupeHardReject {
		t.Fatalf("字节级全重复应硬拒，实得 %v", dec)
	}
}

// 停滞 → 泊车 → Worker 转向其它活跃方向（§32.5 转向阶梯）。
func TestStallParkAndPivot(t *testing.T) {
	pt := NewProgressTracker(3)
	pt.SetScore("dirA", 10)
	pt.SetScore("dirB", 5)
	for i := 0; i < 3; i++ {
		pt.RecordAction("dirA") // 3 步无进展
	}
	if !pt.Stalled("dirA") {
		t.Fatal("连续 3 步无进展应判停滞")
	}
	pt.Park("dirA", ParkExhausted)

	sel := NewSelector(1.0, 0, nil)
	sel.current = "dirA"
	pick := sel.Pick(pt.ActiveScores()) // dirA 已泊车、不在活跃集 → 转向 dirB
	if pick != "dirB" {
		t.Fatalf("停滞泊车后 Worker 应转向 dirB，实得 %q", pick)
	}
}

// insight 算进展：安静思考、连点成线不被误判为卡住（§31 A6）。
func TestInsightCountsAsProgress(t *testing.T) {
	pt := NewProgressTracker(3)
	pt.RecordAction("dirA")
	pt.RecordAction("dirA")
	pt.RecordProgress("dirA") // 产出新假设 = 进展，清零无进展计数
	pt.RecordAction("dirA")
	if pt.Stalled("dirA") {
		t.Fatal("有进展（insight）不应判停滞")
	}
}

// 滞回：切主攻方向需分差超阈值，防抖动（§1⑤）。
func TestSelectorHysteresis(t *testing.T) {
	sel := NewSelector(3.0, 0, nil)
	sel.current = "A"
	if got := sel.Pick(map[string]float64{"A": 10, "B": 12}); got != "A" {
		t.Fatalf("分差 2≤阈值 3 不应切换，实得 %q", got)
	}
	if got := sel.Pick(map[string]float64{"A": 10, "B": 14}); got != "B" {
		t.Fatalf("分差 4>阈值 3 应切到 B，实得 %q", got)
	}
}

// ε 探索：以 ε 概率随机探索非最高分方向，防隧道视野（§A7）。
func TestSelectorEpsilonExploration(t *testing.T) {
	seq := []float64{0.0, 0.0} // 第一次<ε→探索；第二次决定 idx=0
	i := 0
	rng := func() float64 { v := seq[i]; i++; return v }
	sel := NewSelector(1.0, 0.5, rng)
	pick := sel.Pick(map[string]float64{"A": 1, "B": 100}) // B 最高，但探索选 keys[0]=A
	if pick != "A" {
		t.Fatalf("ε 探索应选非最高分方向 A，实得 %q", pick)
	}
}

type fakeBudget struct{ rem map[string]int64 }

func (f fakeBudget) RemainingMicroUSD(runID string) int64 { return f.rem[runID] }

// 先过滤：预算耗尽的 Run 的 Job 不 eligible（§6.8）。
func TestBudgetGateFilters(t *testing.T) {
	src := fakeBudget{rem: map[string]int64{"run-1": 0, "run-2": 500}}
	jobRun := func(j string) string {
		return map[string]string{"j1": "run-1", "j2": "run-2"}[j]
	}
	f := NewFilters(BudgetGate(src, jobRun))
	if ok, reason := f.Eligible("j1"); ok || reason == "" {
		t.Fatalf("预算耗尽的 run-1 的 j1 应不 eligible，实得 ok=%v reason=%q", ok, reason)
	}
	if ok, _ := f.Eligible("j2"); !ok {
		t.Fatal("预算充足的 run-2 的 j2 应 eligible")
	}
}
