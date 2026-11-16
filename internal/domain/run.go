package domain

import "time"

// run.go —— Run 聚合（§4.1 runs）。
//
// 一次完整测试任务。Run 不是 Agent Session：一个 Run 内可产生数百个 Agent Session（§4.1）。
//
// 关键不变量：
//   - Revision 单调递增：所有状态迁移用乐观锁 UPDATE ... WHERE id=? AND revision=?，
//     命中 0 行即冲突（返回 ErrConflict），重读重试（§4.1）。
//   - AuthorizationScopeDigest 创建后不可变；它是「授权目标指纹」（规范化 Host 集合哈希），
//     也是 Workspace 内历史 Run 记忆共享的前提（§4.1）。
//   - State 是字符串枚举（RunState），不是 int。
type Run struct {
	ID                       ID         `json:"id"`
	WorkspaceID              ID         `json:"workspace_id"`
	Name                     string     `json:"name"`
	State                    RunState   `json:"state"`
	Revision                 int64      `json:"revision"` // 乐观锁版本，单调递增
	CurrentPolicyRevision    int64      `json:"current_policy_revision"`
	CurrentStrategyEpoch     int64      `json:"current_strategy_epoch"`
	CurrentSnapshotRevision  int64      `json:"current_snapshot_revision"`
	StrategyDirty            bool       `json:"strategy_dirty"`             // 置位即在当前规划结束后立即重规划（§4.1）
	AuthorizationScopeDigest string     `json:"authorization_scope_digest"` // 创建后不可变
	AuthorizationReference   string     `json:"authorization_reference"`
	CreatedBy                string     `json:"created_by"`
	CreatedAt                time.Time  `json:"created_at"`
	StartedAt                *time.Time `json:"started_at,omitempty"`
	EndedAt                  *time.Time `json:"ended_at,omitempty"`
	TerminalReason           string     `json:"terminal_reason,omitempty"`
	CancelCode               CancelCode `json:"cancel_code,omitempty"`
	CancelActor              string     `json:"cancel_actor,omitempty"`
	CancelDetail             string     `json:"cancel_detail,omitempty"`
	LastError                string     `json:"last_error,omitempty"`
}

// NewRun 造一个新 Run，初态 DRAFT、Revision=1。
// scopeDigest 为授权目标指纹，一经设定不可变（后续迁移不得改写该字段）。
func NewRun(workspaceID ID, name, createdBy, scopeDigest string) *Run {
	return &Run{
		ID:                       NewID(),
		WorkspaceID:              workspaceID,
		Name:                     name,
		State:                    RunDraft,
		Revision:                 1,
		AuthorizationScopeDigest: scopeDigest,
		CreatedBy:                createdBy,
		CreatedAt:                time.Now().UTC(),
	}
}
