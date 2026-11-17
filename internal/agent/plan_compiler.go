package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"pentagent/api/schemas"
	"pentagent/internal/canon"
	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// plan_compiler.go —— PlanCompiler（§6.4）：模型提案→安全落库的确定性闸门，拦幻觉计划。
//
// 校验 PlanPatch（Schema/Snapshot 过期/Job 类型/依赖成环/深度/预算/指纹去重/最大新增 Job）→ 转数据库
// 事务。原子性（§6.4）：Snapshot 检查与 Patch 落库**同一事务 CAS**——
//   UPDATE runs SET current_snapshot_revision=current_snapshot_revision+1 WHERE id=? AND current_snapshot_revision=?
// 影响 0 行即过期拒绝（置 strategy_dirty 当前规划结束后重排），绝不「先检查后提交」两段式。
// patch_id 幂等；重复提交返回原结果。

// 编译结果状态（对应 plan_patches.state）。
const (
	PatchApplied         = "APPLIED"
	PatchRejectedStale   = "REJECTED_STALE"
	PatchRejectedInvalid = "REJECTED_INVALID"
)

// §6.6 合法 Job 类型。
var validJobKinds = map[string]bool{
	"DISCOVER": true, "MAP": true, "BASELINE": true, "ANALYZE": true, "EXPERIMENT": true,
	"VERIFY": true, "CHAIN": true, "FOLLOWUP": true, "CLEANUP": true,
}

// CompileResult 是一次编译的结果。
type CompileResult struct {
	PatchID       string
	Status        string
	CreatedJobIDs []string
	RejectReason  string
	Idempotent    bool // true 表示 patch_id 已存在，返回原结果
}

// Applied 报告 patch 是否成功落库。
func (r CompileResult) Applied() bool { return r.Status == PatchApplied }

// createJob 是一个 create_job 操作的载荷。
type createJob struct {
	TempID      string          `json:"temp_id"` // patch 内引用用临时 id
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	Spec        json.RawMessage `json:"spec"`
	ParentJobID string          `json:"parent_job_id"` // 既有 Job（用于深度）
	DependsOn   []string        `json:"depends_on"`    // temp_id 或既有 job id
	Priority    int             `json:"priority"`
}

// PlanCompiler 确定性编译 PlanPatch。
type PlanCompiler struct {
	w        *store.Writer
	schema   []byte
	maxJobs  int
	maxDepth int
}

// NewPlanCompiler 构造（默认单 Patch ≤8 Job、深度 ≤3，§6.4）。
func NewPlanCompiler(w *store.Writer) (*PlanCompiler, error) {
	sb, err := schemas.FS.ReadFile(schemas.PlanPatch)
	if err != nil {
		return nil, err
	}
	return &PlanCompiler{w: w, schema: sb, maxJobs: 8, maxDepth: 3}, nil
}

// parseCreateJobs 从 patch 原文抽取 create_job 载荷（按出现顺序）。
func parseCreateJobs(raw json.RawMessage) ([]createJob, error) {
	var p struct {
		Operations []struct {
			Op  string          `json:"op"`
			Job json.RawMessage `json:"job"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	var jobs []createJob
	for _, o := range p.Operations {
		if o.Op != "create_job" {
			continue
		}
		var cj createJob
		if len(o.Job) > 0 {
			if err := json.Unmarshal(o.Job, &cj); err != nil {
				return nil, err
			}
		}
		jobs = append(jobs, cj)
	}
	return jobs, nil
}

// detectCycle 在 patch 内的 temp 依赖图上做环检测（DFS 三色）。
func detectCycle(jobs []createJob) bool {
	adj := map[string][]string{}
	present := map[string]bool{}
	for _, j := range jobs {
		if j.TempID != "" {
			present[j.TempID] = true
		}
	}
	for _, j := range jobs {
		for _, d := range j.DependsOn {
			if present[d] { // 只连 patch 内的 temp 依赖（既有 Job 已提交、不可能反向依赖新 Job）
				adj[j.TempID] = append(adj[j.TempID], d)
			}
		}
	}
	const white, gray, black = 0, 1, 2
	color := map[string]int{}
	var dfs func(string) bool
	dfs = func(n string) bool {
		color[n] = gray
		for _, m := range adj[n] {
			if color[m] == gray {
				return true
			}
			if color[m] == white && dfs(m) {
				return true
			}
		}
		color[n] = black
		return false
	}
	for id := range present {
		if color[id] == white && dfs(id) {
			return true
		}
	}
	return false
}

// specHash 计算 Job 指纹（kind + spec 的 JCS 摘要），用于去重。
func specHash(j createJob) string {
	obj := map[string]any{"kind": j.Kind}
	if len(j.Spec) > 0 {
		obj["spec"] = json.RawMessage(j.Spec)
	}
	h, _ := canon.Digest(obj)
	return h
}

// Compile 校验并落库一个 PlanPatch（§6.4）。
func (pc *PlanCompiler) Compile(ctx context.Context, runID, epochID, patchID string, patch ParsedPlanPatch, now time.Time) (CompileResult, error) {
	// 幂等：patch_id 已存在 → 返回原结果。
	if st, ok, err := pc.getPatch(ctx, patchID); err != nil {
		return CompileResult{}, err
	} else if ok {
		return CompileResult{PatchID: patchID, Status: st, Idempotent: true}, nil
	}

	// 纯确定性校验（无 DB 写）：Schema / 最大新增 Job / Job 类型 / 依赖成环。
	if err := canon.ValidateAgainst(pc.schema, patch.Raw); err != nil {
		return pc.recordInvalid(ctx, runID, epochID, patchID, patch, "schema: "+err.Error(), now)
	}
	jobs, err := parseCreateJobs(patch.Raw)
	if err != nil {
		return pc.recordInvalid(ctx, runID, epochID, patchID, patch, "parse: "+err.Error(), now)
	}
	if len(jobs) > pc.maxJobs {
		return pc.recordInvalid(ctx, runID, epochID, patchID, patch, "exceeds max new jobs (8)", now)
	}
	for _, j := range jobs {
		if !validJobKinds[j.Kind] {
			return pc.recordInvalid(ctx, runID, epochID, patchID, patch, "invalid job kind "+j.Kind, now)
		}
	}
	if detectCycle(jobs) {
		return pc.recordInvalid(ctx, runID, epochID, patchID, patch, "dependency cycle", now)
	}

	// 原子事务：深度读校验 → Snapshot CAS → 落库。
	var res CompileResult
	tstr := ts(now)
	err = pc.w.Durable(ctx, func(tx *sql.Tx) error {
		if st, ok := getPatchTx(ctx, tx, patchID); ok { // 锁内重查幂等
			res = CompileResult{PatchID: patchID, Status: st, Idempotent: true}
			return nil
		}
		// 深度 ≤ maxDepth（走既有父链）。
		for _, j := range jobs {
			if j.ParentJobID == "" {
				continue
			}
			d, e := jobDepthTx(ctx, tx, j.ParentJobID)
			if e != nil {
				return e
			}
			if d+1 > pc.maxDepth {
				res = CompileResult{PatchID: patchID, Status: PatchRejectedInvalid, RejectReason: "job depth exceeds 3"}
				return insertPatchTx(ctx, tx, patchID, runID, epochID, patch, PatchRejectedInvalid, tstr)
			}
		}

		// Snapshot CAS（原子闸门，§6.4）。
		r, e := tx.ExecContext(ctx,
			`UPDATE runs SET current_snapshot_revision=current_snapshot_revision+1 WHERE id=? AND current_snapshot_revision=?`,
			runID, patch.SnapshotRevision)
		if e != nil {
			return e
		}
		if n, _ := r.RowsAffected(); n == 0 {
			// 过期：拒绝 + 置 strategy_dirty（当前规划结束后重排，§6.4）。
			if _, e = tx.ExecContext(ctx, `UPDATE runs SET strategy_dirty=1 WHERE id=?`, runID); e != nil {
				return e
			}
			res = CompileResult{PatchID: patchID, Status: PatchRejectedStale, RejectReason: "snapshot revision stale"}
			return insertPatchTx(ctx, tx, patchID, runID, epochID, patch, PatchRejectedStale, tstr)
		}

		// 落库：建 Job（指纹去重跳过）+ 依赖 + plan_patches APPLIED。
		tempToID := map[string]string{}
		seenHash := map[string]bool{}
		var created []string
		for _, j := range jobs {
			hash := specHash(j)
			// 去重：patch 内重复 or run 内已存在同指纹 → 跳过创建。
			if seenHash[hash] {
				continue
			}
			exists, e := specHashExistsTx(ctx, tx, runID, hash)
			if e != nil {
				return e
			}
			if exists {
				seenHash[hash] = true
				continue
			}
			seenHash[hash] = true
			id := string(domain.NewID())
			state := string(domain.JobReady)
			if len(j.DependsOn) > 0 {
				state = string(domain.JobPending) // 有依赖 → 先 PENDING，依赖满足再 READY
			}
			if _, e = tx.ExecContext(ctx, `
				INSERT INTO jobs (id, run_id, parent_job_id, kind, title, spec_json, spec_hash, state, priority, created_at, updated_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
				id, runID, nz(j.ParentJobID), j.Kind, nz(j.Title), nz(string(j.Spec)), hash, state, j.Priority, tstr, tstr); e != nil {
				return e
			}
			if j.TempID != "" {
				tempToID[j.TempID] = id
			}
			created = append(created, id)
		}
		// 依赖边（temp→新 id 或既有 id）。
		for _, j := range jobs {
			jid, ok := tempToID[j.TempID]
			if !ok {
				continue // 该 job 被去重跳过，无边可建
			}
			for _, dep := range j.DependsOn {
				depID := dep
				if mapped, ok := tempToID[dep]; ok {
					depID = mapped
				}
				if _, e = tx.ExecContext(ctx,
					`INSERT OR IGNORE INTO job_dependencies (job_id, depends_on_job_id, kind) VALUES (?,?,?)`,
					jid, depID, "finish"); e != nil {
					return e
				}
			}
		}
		if e = insertPatchTx(ctx, tx, patchID, runID, epochID, patch, PatchApplied, tstr); e != nil {
			return e
		}
		res = CompileResult{PatchID: patchID, Status: PatchApplied, CreatedJobIDs: created}
		return nil
	})
	return res, err
}

// recordInvalid 记一条 REJECTED_INVALID（独立小事务），返回结果。
func (pc *PlanCompiler) recordInvalid(ctx context.Context, runID, epochID, patchID string, patch ParsedPlanPatch, reason string, now time.Time) (CompileResult, error) {
	err := pc.w.Durable(ctx, func(tx *sql.Tx) error {
		return insertPatchTx(ctx, tx, patchID, runID, epochID, patch, PatchRejectedInvalid, ts(now))
	})
	return CompileResult{PatchID: patchID, Status: PatchRejectedInvalid, RejectReason: reason}, err
}

func insertPatchTx(ctx context.Context, tx *sql.Tx, patchID, runID, epochID string, patch ParsedPlanPatch, state, tstr string) error {
	applied := ""
	if state == PatchApplied {
		applied = tstr
	}
	_, e := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO plan_patches (patch_id, run_id, epoch_id, snapshot_revision, operations_json, state, applied_at)
		VALUES (?,?,?,?,?,?,?)`,
		patchID, runID, epochID, patch.SnapshotRevision, string(patch.Raw), state, nz(applied))
	return e
}

func getPatchTx(ctx context.Context, tx *sql.Tx, patchID string) (string, bool) {
	var st string
	e := tx.QueryRowContext(ctx, `SELECT state FROM plan_patches WHERE patch_id=?`, patchID).Scan(&st)
	if e != nil {
		return "", false
	}
	return st, true
}

func (pc *PlanCompiler) getPatch(ctx context.Context, patchID string) (string, bool, error) {
	var st string
	var found bool
	err := pc.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT state FROM plan_patches WHERE patch_id=?`, patchID).Scan(&st)
		if e == sql.ErrNoRows {
			return nil
		}
		if e != nil {
			return e
		}
		found = true
		return nil
	})
	return st, found, err
}

// jobDepthTx 返回某 Job 的深度（根=1），走 parent_job_id 父链。
func jobDepthTx(ctx context.Context, tx *sql.Tx, jobID string) (int, error) {
	depth := 0
	cur := jobID
	for i := 0; i < 100 && cur != ""; i++ {
		var parent sql.NullString
		e := tx.QueryRowContext(ctx, `SELECT parent_job_id FROM jobs WHERE id=?`, cur).Scan(&parent)
		if e == sql.ErrNoRows {
			break
		}
		if e != nil {
			return 0, e
		}
		depth++
		if !parent.Valid {
			break
		}
		cur = parent.String
	}
	return depth, nil
}

func specHashExistsTx(ctx context.Context, tx *sql.Tx, runID, hash string) (bool, error) {
	var n int
	e := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE run_id=? AND spec_hash=?`, runID, hash).Scan(&n)
	return n > 0, e
}

// ----- 战役计划（§32.4）：跨 Epoch 稳定的方向树，Strategist 每 Epoch 精修不重建 -----

// UpsertCampaignPlan 写入/更新某 Run 的战役计划（方向树 JSON + JCS 摘要）。
func (pc *PlanCompiler) UpsertCampaignPlan(ctx context.Context, runID, epochID string, planJSON json.RawMessage, now time.Time) (string, error) {
	digest, _ := canon.DigestJSON(planJSON)
	id := string(domain.NewID())
	err := pc.w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO campaign_plan (id, run_id, epoch_id, plan_json, digest, created_at)
			VALUES (?,?,?,?,?,?)`,
			id, runID, nz(epochID), string(planJSON), digest, ts(now))
		return e
	})
	return digest, err
}

// LatestCampaignPlan 读某 Run 最新战役计划（plan_json, digest）。
func (pc *PlanCompiler) LatestCampaignPlan(ctx context.Context, runID string) (json.RawMessage, string, error) {
	var plan, digest string
	err := pc.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx,
			`SELECT plan_json, COALESCE(digest,'') FROM campaign_plan WHERE run_id=? ORDER BY created_at DESC, id DESC LIMIT 1`, runID).
			Scan(&plan, &digest)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "no campaign plan for run "+runID)
		}
		return e
	})
	if err != nil {
		return nil, "", err
	}
	return json.RawMessage(plan), digest, nil
}
