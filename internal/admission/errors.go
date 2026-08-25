package admission

import "fmt"

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

func invalid(message string) error { return &Error{Code: "invalid_input", Message: message} }
func notFound(entity, id string) error {
	return &Error{Code: "not_found", Message: fmt.Sprintf("%s %s 不存在", entity, id)}
}
func stateConflict(message string) error { return &Error{Code: "state_conflict", Message: message} }
func versionConflict(got, want uint64) error {
	return &Error{Code: "version_conflict", Message: fmt.Sprintf("expectedVersion=%d，当前 version=%d", got, want)}
}
func idempotencyConflict() error {
	return &Error{Code: "idempotency_conflict", Message: "idempotencyKey 已用于其他操作"}
}

func ErrorCode(err error) string {
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return "internal_error"
}
