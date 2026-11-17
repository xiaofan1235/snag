// Package events 实现事件日志的事务内写入（§4.2 events、§0.5 崩溃一致性、§15.6）。
//
// 事件是 SSE / 审计 / 五方对账的事实来源。写入是**纯事务内操作**（Append 接收调用方的 *sql.Tx），
// 要与某领域状态变更原子，就把事件写进那次变更的同一事务——提交后才由 outbox.Publisher 向 UI
// 发布（§4.2「事件在事务提交后由 Outbox Publisher 推送，不能在数据库提交前向 UI 发布」）。
//
// 两级序号（§3151）：run 内单调 sequence（SSE 补发游标 event_cursor）+ 全局单调 global_sequence
// （全局流排序）。两者均在事务内取 MAX+1；单写者串行化所有写（durable/bulk 同一 goroutine），
// 故 MAX+1 无并发竞争、无空洞。
package events

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
)

// Event 是一条待写入的领域事件（§4.2 events 字段）。
type Event struct {
	RunID         domain.ID
	Type          string // event_type，取 §15.6 注册表里的事件名
	EntityType    string // 聚合类型，如 "run"/"job"/"action"
	EntityID      domain.ID
	EntityVersion int64 // 直接用聚合 revision（§4.3 #11）
	PayloadJSON   string
}

// Appended 是 Append 的产物：该事件拿到的 run 内 sequence 与全局 global_sequence。
type Appended struct {
	Sequence       int64
	GlobalSequence int64
}

// Append 在给定事务 tx 内写一条事件：run 内 sequence=MAX+1、全局 global_sequence=MAX+1。
// tx 由调用方给定——与领域状态变更同事务传入即获得「提交失败则无事件」的原子性（§0.5）。
// 不负责发布：发布由 outbox.Publisher 在提交后完成。
func Append(ctx context.Context, tx *sql.Tx, e Event) (Appended, error) {
	var runSeq, gSeq int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sequence),0) FROM events WHERE run_id=?`, string(e.RunID)).Scan(&runSeq); err != nil {
		return Appended{}, err
	}
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(global_sequence),0) FROM events`).Scan(&gSeq); err != nil {
		return Appended{}, err
	}
	runSeq++
	gSeq++
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO events
		 (run_id, sequence, global_sequence, event_type, entity_type, entity_id, entity_version, payload_json, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		string(e.RunID), runSeq, gSeq, e.Type, e.EntityType, string(e.EntityID), e.EntityVersion, e.PayloadJSON, now,
	); err != nil {
		return Appended{}, err
	}
	return Appended{Sequence: runSeq, GlobalSequence: gSeq}, nil
}
