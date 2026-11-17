// Package outbox 实现"先存后发"（§53.2）：待发消息在领域事务内 enqueue 到 outbox 表，
// 事务提交后由 Publisher 扫表推送到事件汇（Bus），成功标记已发、失败指数退避重试。
//
// 关键不变量（§4.2/§0.5）：消息与领域状态变更**同事务落库**——提交失败则无消息（不会误发
// 一个未真正发生的变更）；提交成功后消息必被投递（至少一次）。发布在提交后进行，决不在提交前。
package outbox

import (
	"context"
	"database/sql"
	"time"

	"pentagent/internal/domain"
)

// Enqueue 在事务 tx 内投一条待发消息。与领域变更同事务传入 tx 即获得"先存后发"的原子性。
// 返回 outbox 行 id。
func Enqueue(ctx context.Context, tx *sql.Tx, runID domain.ID, topic, payloadJSON string) (domain.ID, error) {
	id := domain.NewID()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO outbox (id, run_id, topic, payload_json, published, attempts, created_at)
		 VALUES (?,?,?,?,0,0,?)`,
		string(id), string(runID), topic, payloadJSON, now); err != nil {
		return "", err
	}
	return id, nil
}
