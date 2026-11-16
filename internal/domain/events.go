package domain

import (
	"encoding/json"
	"time"
)

// events.go —— Event 聚合（§4.1 events）。
//
// Run 内的事件日志，唯一约束 UNIQUE(run_id, sequence) 保证每个 Run 内序号不重复、可定序。
// 事件在事务提交后由 Outbox Publisher 推送，不能在数据库提交前向 UI 发布（§4.1）——
// 这样 UI 永远看不到「尚未落库」的状态，崩溃恢复后事件流也一致。
type Event struct {
	RunID         ID              `json:"run_id"`
	Sequence      int64           `json:"sequence"` // Run 内单调；(run_id, sequence) 唯一
	EventType     string          `json:"event_type"`
	EntityType    string          `json:"entity_type"`
	EntityID      ID              `json:"entity_id"`
	EntityVersion int64           `json:"entity_version"`
	PayloadJSON   json.RawMessage `json:"payload_json"`
	CreatedAt     time.Time       `json:"created_at"`
}
