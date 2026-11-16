package domain

import (
	"encoding/json"
	"time"
)

// workspace.go —— Workspace 聚合（§4.1 workspaces、§4.0 多项目与工作区隔离）。
//
// Workspace 是「同一授权工作目标」的历史分组与记忆隔离边界：
//   - 它不是网络 Scope，也不替代每个 Run 的授权确认（§4.1 约束）；
//   - 跨 Run 记忆只在同一 Workspace 内、且 Scope Digest 完全相同时才共享（§4.1 runs 约束）。
type Workspace struct {
	ID                  ID              `json:"id"`
	Name                string          `json:"name"`
	MemoryProjectKey    string          `json:"memory_project_key"` // 服务端生成且唯一，不接受模型指定
	DataClassification  DataClass       `json:"data_classification"`
	RetentionPolicyJSON json.RawMessage `json:"retention_policy_json"` // 保留/清除策略
	CreatedAt           time.Time       `json:"created_at"`
	ArchivedAt          *time.Time      `json:"archived_at,omitempty"` // 归档不删除 Evidence/Memory
}

// NewWorkspace 造一个新 Workspace。memory_project_key 由调用方（服务端）注入，保证唯一。
func NewWorkspace(name, memoryProjectKey string) *Workspace {
	return &Workspace{
		ID:               NewID(),
		Name:             name,
		MemoryProjectKey: memoryProjectKey,
		CreatedAt:        time.Now().UTC(),
	}
}
