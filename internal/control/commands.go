package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// commands.go —— 幂等命令账本（§15.7 / §4.2 commands·operations / §16.33）。
//
// 保证「网络超时重试不产生重复对象」：同一 (actor_id, idempotency_key) 的重试返回**原**
// Job/Approval/Operation 引用，不二次执行副作用（CLI/UI 都依赖）。只存 request_digest，
// 不存原始秘密请求体（§4.2/§1163）。异步命令建 operation（ACCEPTED/RUNNING/SUCCEEDED/FAILED）。
//
// 并发安全靠单写者串行：Receive 在一个 durable 事务里「查重→执行 fn→记账」，单写 goroutine
// 串行处理各命令，故先到者整事务提交后、后到者才查得到——SELECT-then-act 无竞争，
// PRIMARY KEY(actor_id, idempotency_key) 作末端兜底。

// ResourceRef 指向命令产出的对象。
type ResourceRef struct {
	Kind string    `json:"kind"` // "job"/"operation"/"approval"/"run"…
	ID   domain.ID `json:"id"`
}

// ReceiveResult 是 Receive 的产物。Replayed=true 表示命中既有命令、返回的是缓存结果（未重新执行 fn）。
type ReceiveResult struct {
	Refs     []ResourceRef
	Replayed bool
}

// CommandLedger 受理幂等命令、跟踪异步 operation。
type CommandLedger struct {
	w  *store.Writer
	rd *store.Reader
}

// NewCommandLedger 构造账本。rd 用于只读查询（GetOperation）。
func NewCommandLedger(w *store.Writer, rd *store.Reader) *CommandLedger {
	return &CommandLedger{w: w, rd: rd}
}

// Receive 幂等受理命令（§15.7）：
//   - 命中 (actorID, idemKey)：返回原 resource_refs，Replayed=true，**不**执行 fn；
//   - 未命中：在**同一事务**内执行 fn 创建对象、记一行 commands；fn 失败 → 整事务回滚
//     （无命令行、无对象）→ 后续重试会真正重新执行；
//   - 同 key 但 request_digest 不同 → ErrConflict（key 复用于不同请求体，稳定错误）。
//
// fn 收到事务 tx，应在其中创建对象并返回其 refs；一切副作用与命令记账同事务原子落库。
func (c *CommandLedger) Receive(ctx context.Context, actorID, idemKey, reqDigest string,
	fn func(tx *sql.Tx) ([]ResourceRef, error)) (ReceiveResult, error) {
	var out ReceiveResult
	err := c.w.Durable(ctx, func(tx *sql.Tx) error {
		var digest sql.NullString
		var refsJSON sql.NullString
		e := tx.QueryRowContext(ctx,
			`SELECT request_digest, resource_refs FROM commands WHERE actor_id=? AND idempotency_key=?`,
			actorID, idemKey).Scan(&digest, &refsJSON)
		if e == nil {
			// 命中：校验 digest 一致后回放原结果。
			if digest.String != reqDigest {
				return domain.Errorf(domain.ErrConflict,
					fmt.Sprintf("idempotency key %q 被复用于不同请求（digest 不符）", idemKey))
			}
			refs, de := decodeRefs(refsJSON.String)
			if de != nil {
				return de
			}
			out = ReceiveResult{Refs: refs, Replayed: true}
			return nil
		}
		if e != sql.ErrNoRows {
			return e
		}
		// 未命中：执行 fn（创建对象），再记账——同事务，失败则一起回滚。
		refs, fe := fn(tx)
		if fe != nil {
			return fe
		}
		enc, me := json.Marshal(refs)
		if me != nil {
			return me
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if _, ie := tx.ExecContext(ctx,
			`INSERT INTO commands (actor_id, idempotency_key, request_digest, state, resource_refs, created_at)
			 VALUES (?,?,?,?,?,?)`,
			actorID, idemKey, reqDigest, "ACCEPTED", string(enc), now); ie != nil {
			return ie
		}
		out = ReceiveResult{Refs: refs, Replayed: false}
		return nil
	})
	if err != nil {
		return ReceiveResult{}, err
	}
	return out, nil
}

func decodeRefs(s string) ([]ResourceRef, error) {
	if s == "" {
		return nil, nil
	}
	var refs []ResourceRef
	if err := json.Unmarshal([]byte(s), &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

// revisionTables 是 GuardRevision 允许的表白名单（表名不能参数化，白名单防注入）。
var revisionTables = map[string]bool{"jobs": true, "runs": true}

// GuardRevision 乐观并发校验（§15.7 expected_revision）：目标对象当前 revision != expected
// → ErrConflict（稳定错误码）。供 mutate 类命令的 fn 在事务内调用。
func GuardRevision(ctx context.Context, tx *sql.Tx, table string, id domain.ID, expected int64) error {
	if !revisionTables[table] {
		return domain.Errorf(domain.ErrInvalid, fmt.Sprintf("GuardRevision: 未知表 %q", table))
	}
	var cur int64
	e := tx.QueryRowContext(ctx, `SELECT revision FROM `+table+` WHERE id=?`, string(id)).Scan(&cur)
	if e == sql.ErrNoRows {
		return domain.ErrNotFound
	}
	if e != nil {
		return e
	}
	if cur != expected {
		return domain.Errorf(domain.ErrConflict,
			fmt.Sprintf("%s %s: expected_revision=%d 与当前 %d 冲突", table, id, expected, cur))
	}
	return nil
}

// --- operations（异步命令跟踪，§4.2） ---

// OperationState 是异步 operation 的状态。
type OperationState string

const (
	OpAccepted  OperationState = "ACCEPTED"
	OpRunning   OperationState = "RUNNING"
	OpSucceeded OperationState = "SUCCEEDED"
	OpFailed    OperationState = "FAILED"
)

var opTerminal = map[OperationState]bool{OpSucceeded: true, OpFailed: true}

// Operation 是一条异步命令的跟踪记录。
type Operation struct {
	ID        domain.ID
	RunID     domain.ID
	Kind      string
	State     OperationState
	Refs      []ResourceRef
	CreatedAt string
	UpdatedAt string
}

// CreateOperationTx 在事务内建一条 ACCEPTED operation，返回其 id。可在 Receive 的 fn 内调用，
// 使「建 operation + 记命令」同事务原子（异步命令的幂等入口）。
func CreateOperationTx(ctx context.Context, tx *sql.Tx, runID domain.ID, kind string) (domain.ID, error) {
	id := domain.NewID()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, e := tx.ExecContext(ctx,
		`INSERT INTO operations (id, run_id, kind, state, created_at, updated_at) VALUES (?,?,?,?,?,?)`,
		string(id), string(runID), kind, string(OpAccepted), now, now); e != nil {
		return "", e
	}
	return id, nil
}

// Transition 推进 operation 状态并写 refs（终态不可再变）。供异步 worker 调用。
func (c *CommandLedger) Transition(ctx context.Context, opID domain.ID, to OperationState, refs []ResourceRef) error {
	enc, me := json.Marshal(refs)
	if me != nil {
		return me
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return c.w.Durable(ctx, func(tx *sql.Tx) error {
		var st string
		if e := tx.QueryRowContext(ctx, `SELECT state FROM operations WHERE id=?`, string(opID)).Scan(&st); e != nil {
			if e == sql.ErrNoRows {
				return domain.ErrNotFound
			}
			return e
		}
		if opTerminal[OperationState(st)] {
			return domain.Errorf(domain.ErrConflict, fmt.Sprintf("operation %s 已终态 %s，不可再变", opID, st))
		}
		var refsArg any
		if len(refs) > 0 {
			refsArg = string(enc)
		}
		_, e := tx.ExecContext(ctx,
			`UPDATE operations SET state=?, resource_refs=COALESCE(?, resource_refs), updated_at=? WHERE id=?`,
			string(to), refsArg, now, string(opID))
		return e
	})
}

// GetOperation 查询 operation 当前状态（只读池），供 GET /v1/operations/{id}。
func (c *CommandLedger) GetOperation(ctx context.Context, opID domain.ID) (Operation, error) {
	var op Operation
	var st string
	var refsJSON sql.NullString
	e := c.rd.DB().QueryRowContext(ctx,
		`SELECT id, run_id, kind, state, resource_refs, created_at, updated_at FROM operations WHERE id=?`,
		string(opID)).Scan(&op.ID, &op.RunID, &op.Kind, &st, &refsJSON, &op.CreatedAt, &op.UpdatedAt)
	if e == sql.ErrNoRows {
		return Operation{}, domain.ErrNotFound
	}
	if e != nil {
		return Operation{}, e
	}
	op.State = OperationState(st)
	refs, de := decodeRefs(refsJSON.String)
	if de != nil {
		return Operation{}, de
	}
	op.Refs = refs
	return op, nil
}
