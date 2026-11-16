package domain

// errors.go —— 领域错误（Day 01 §4）。
//
// 用稳定机器码 Code 表达错误类别，让上层（control/scheduler/api）按 Code 分支，
// 而不是匹配 error 文本——文本会漂移，Code 不会。配合 errors.Is 使用。

// DomainError 是带稳定机器码的领域错误。
type DomainError struct {
	Code string // 稳定机器码，如 "CONFLICT" / "NOT_FOUND" / "INVALID"
	Msg  string // 人类可读描述，可携带上下文
}

func (e *DomainError) Error() string {
	if e.Msg == "" {
		return e.Code
	}
	return e.Code + ": " + e.Msg
}

// Is 让 errors.Is 按 Code 匹配：errors.Is(err, ErrConflict) 对任何 Code=="CONFLICT" 的
// DomainError 都为真，即使它携带了不同的上下文 Msg。
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	return ok && t.Code == e.Code
}

// 预置的三类基础错误（Day 01 §4 指定）：
var (
	// ErrConflict —— 乐观锁冲突：UPDATE ... WHERE id=? AND revision=? 命中 0 行，需重读重试（§4.1）。
	ErrConflict = &DomainError{Code: "CONFLICT", Msg: "optimistic lock conflict, reread and retry"}
	// ErrNotFound —— 实体不存在。
	ErrNotFound = &DomainError{Code: "NOT_FOUND", Msg: "entity not found"}
	// ErrInvalid —— 参数/状态非法（构造或迁移前置条件不满足）。
	ErrInvalid = &DomainError{Code: "INVALID", Msg: "invalid argument or state"}
)

// Errorf 构造一个带上下文 Msg、但沿用既有 Code 的 DomainError，
// 便于 errors.Is 仍能按类别匹配：
//
//	return domain.Errorf(domain.ErrNotFound, "run %s", id)
func Errorf(base *DomainError, msg string) *DomainError {
	return &DomainError{Code: base.Code, Msg: msg}
}
