package agent

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"sync"
	"time"

	"pentagent/api/schemas"
	"pentagent/internal/canon"
	"pentagent/internal/domain"
	"pentagent/internal/providers"
)

// strategist.go —— Run Strategist 慢循环（§6.1-6.3 规划 / §6.5 触发器 / §32 统一指挥层）。
//
// Strategist 吃不可变 StrategySnapshot（摘要，不读整库/不读全部原始响应），经 Provider 原生 tool-call
// 产出 PlanPatch（只提案、不碰库——落库是 Day 29 PlanCompiler 的事）。op 限定 §6.3 白名单；模型**不能**
// 经 PlanPatch 改 Scope/账号/并发/定级/Evidence、直接标 DONE 或结束 Run（这些根本不在 op 枚举里）。

// §6.3 允许的 PlanPatch 操作白名单（与冻结 planpatch.schema.json 的 op 枚举一致）。
var allowedPlanOps = map[string]bool{
	"upsert_hypothesis":       true,
	"create_job":              true,
	"cancel_pending_job":      true,
	"reprioritize_job":        true,
	"mark_hypothesis_blocked": true,
	"request_approval":        true,
	"request_reframe":         true,
	"close_epoch":             true,
}

// ErrForbiddenOp —— PlanPatch 含不在 §6.3 白名单的操作（纵深防御，schema 已先拦）。
var ErrForbiddenOp = &domain.DomainError{Code: "FORBIDDEN_OP", Msg: "PlanPatch op not permitted (§6.3)"}

// ErrStaleSnapshot —— PlanPatch 的 snapshot_revision 与规划所用快照不一致（过期规划）。
var ErrStaleSnapshot = &domain.DomainError{Code: "STALE_SNAPSHOT", Msg: "plan patch snapshot revision mismatch"}

// StrategySnapshot 是不可变规划输入（§6.2）。子摘要用 json.RawMessage，调用方传结构化摘要。
type StrategySnapshot struct {
	RunID                 string          `json:"run_id"`
	SnapshotRevision      int             `json:"snapshot_revision"`
	PolicyDigest          string          `json:"policy_digest"`
	AssetSummary          json.RawMessage `json:"asset_summary,omitempty"`
	TechnologySummary     json.RawMessage `json:"technology_summary,omitempty"`
	ActiveJobs            json.RawMessage `json:"active_jobs,omitempty"`
	RecentFailures        json.RawMessage `json:"recent_failures,omitempty"`
	OpenHypotheses        json.RawMessage `json:"open_hypotheses,omitempty"`
	HighValueDiscoveries  json.RawMessage `json:"high_value_discoveries,omitempty"`
	CoverageSummary       json.RawMessage `json:"coverage_summary,omitempty"`
	FindingSummary        json.RawMessage `json:"finding_summary,omitempty"`
	BudgetSummary         json.RawMessage `json:"budget_summary,omitempty"`
	RecentProgressSignals json.RawMessage `json:"recent_progress_signals,omitempty"`
	OpenApprovals         json.RawMessage `json:"open_approvals,omitempty"`
	ResidualRisks         json.RawMessage `json:"residual_risks,omitempty"`
}

// Digest 返回快照的 JCS 规范化摘要（snapshot_digest，§6.2）。
func (s StrategySnapshot) Digest() (string, error) { return canon.Digest(s) }

func (s StrategySnapshot) marshal() []byte { b, _ := json.Marshal(s); return b }

// ParsedPlanPatch 是校验通过的 PlanPatch（Raw 供 Day 29 PlanCompiler 落库）。
type ParsedPlanPatch struct {
	SnapshotRevision int
	EpochReason      string
	Ops              []string
	Raw              json.RawMessage
}

// Strategist 产出 PlanPatch（只提案）。
type Strategist struct {
	provider providers.Provider
	retrier  providers.Retrier
	schema   []byte
	model    string
}

// NewStrategist 构造。加载冻结的 planpatch schema 作为 emit 工具入参契约。
func NewStrategist(p providers.Provider, model string) (*Strategist, error) {
	sb, err := schemas.FS.ReadFile(schemas.PlanPatch)
	if err != nil {
		return nil, err
	}
	return &Strategist{provider: p, schema: sb, model: model}, nil
}

// strategistSystem 组装 Strategist 系统提示（稳定前缀 + §6.3 约束提醒）。
func strategistSystem() string {
	return stablePrefix(RoleStrategist) + "\n" +
		"你只能输出 PlanPatch（operations 限 §6.3 白名单）；不得改 Scope/账号/并发/定级/Evidence，" +
		"不得直接把 Job 标 DONE 或结束 Run。snapshot_revision 必须原样回填。"
}

// Plan 基于快照产出校验通过的 PlanPatch（§6.3）。只提案、不碰库。
func (s *Strategist) Plan(ctx context.Context, snap StrategySnapshot) (ParsedPlanPatch, error) {
	tool := &providers.Tool{
		Name: "emit_plan_patch", Description: "基于 StrategySnapshot 输出一个 PlanPatch（§6.3）", InputSchema: s.schema,
	}
	req := providers.Request{
		RequestID: "strat-" + snap.RunID + "-" + strconv.Itoa(snap.SnapshotRevision),
		Role:      "strategist", Model: s.model,
		System: strategistSystem(),
		Messages: []providers.Message{{
			Role: "user",
			Content: "当前 StrategySnapshot（仅摘要，不含整库）：\n" + string(snap.marshal()) +
				"\n\n请基于快照提案下一步计划。snapshot_revision 必须回填为 " + strconv.Itoa(snap.SnapshotRevision) + "。",
		}},
		Tool: tool, MaxOutput: 2048,
	}
	raw, _, err := providers.EmitStructured(ctx, s.provider, req, s.retrier)
	if err != nil {
		return ParsedPlanPatch{}, err
	}
	return parsePlanPatch(raw, snap.SnapshotRevision)
}

// parsePlanPatch 解析并校验：snapshot_revision 一致（防过期）+ op 全在白名单（纵深防御，§6.3）。
func parsePlanPatch(raw json.RawMessage, wantRev int) (ParsedPlanPatch, error) {
	var p struct {
		SnapshotRevision int    `json:"snapshot_revision"`
		EpochReason      string `json:"epoch_reason"`
		Operations       []struct {
			Op string `json:"op"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return ParsedPlanPatch{}, domain.Errorf(domain.ErrInvalid, "bad plan patch json: "+err.Error())
	}
	if p.SnapshotRevision != wantRev {
		return ParsedPlanPatch{}, domain.Errorf(ErrStaleSnapshot,
			"patch rev "+strconv.Itoa(p.SnapshotRevision)+" != snapshot rev "+strconv.Itoa(wantRev))
	}
	ops := make([]string, 0, len(p.Operations))
	for _, o := range p.Operations {
		if !allowedPlanOps[o.Op] {
			return ParsedPlanPatch{}, domain.Errorf(ErrForbiddenOp, "op "+o.Op)
		}
		ops = append(ops, o.Op)
	}
	return ParsedPlanPatch{SnapshotRevision: p.SnapshotRevision, EpochReason: p.EpochReason, Ops: ops, Raw: raw}, nil
}

// ----- 触发器（§6.5）：3-5s Debounce + 单飞 + 事件合并 + Snapshot Revision 防过期 -----

// TriggerGate 为每个 Run 做规划触发的 Debounce + 单飞 + 事件合并（§6.5）。
type TriggerGate struct {
	mu       sync.Mutex
	debounce time.Duration
	runs     map[string]*gateRun
}

type gateRun struct {
	inFlight   bool
	dirty      bool // 规划期间有新事件 → 结束后重排（§6.4 dirty 重排）
	hasPending bool
	lastFire   time.Time
	events     map[string]struct{}
}

// NewTriggerGate 构造。debounce<=0 用默认 3s。
func NewTriggerGate(debounce time.Duration) *TriggerGate {
	if debounce <= 0 {
		debounce = 3 * time.Second
	}
	return &TriggerGate{debounce: debounce, runs: map[string]*gateRun{}}
}

// Fire 记一个触发事件（§6.5 的 run.started/surface.revision_changed/... 之一）。合并重复；规划在途则置 dirty。
func (g *TriggerGate) Fire(runID, event string, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.runs[runID]
	if r == nil {
		r = &gateRun{events: map[string]struct{}{}}
		g.runs[runID] = r
	}
	r.events[event] = struct{}{}
	r.hasPending = true
	r.lastFire = now
	if r.inFlight {
		r.dirty = true
	}
}

// Claim 尝试领取一次规划：无在途（单飞）+ 有待处理 + 已过 debounce 静默期 → 领取并返回合并后的事件集。
// 否则返回 (nil,false)。领取后须在规划结束调 Release。
func (g *TriggerGate) Claim(runID string, now time.Time) ([]string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.runs[runID]
	if r == nil || r.inFlight || !r.hasPending {
		return nil, false
	}
	if now.Sub(r.lastFire) < g.debounce {
		return nil, false // 静默期未到，继续 Debounce
	}
	events := make([]string, 0, len(r.events))
	for e := range r.events {
		events = append(events, e)
	}
	sort.Strings(events)
	r.events = map[string]struct{}{}
	r.hasPending = false
	r.inFlight = true
	return events, true
}

// Release 结束单飞。若规划期间有新事件（dirty）→ 保留待处理，下次 Claim 重排（§6.4）。
func (g *TriggerGate) Release(runID string, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.runs[runID]
	if r == nil {
		return
	}
	r.inFlight = false
	if r.dirty {
		r.hasPending = true
		r.dirty = false
		r.lastFire = now // 为重排重置静默窗口
	}
}
