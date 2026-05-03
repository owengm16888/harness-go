package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode 错误代码
type ErrorCode string

const (
	// 通用错误
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeConflict     ErrorCode = "CONFLICT"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"

	// 任务错误
	ErrCodeTaskNotFound      ErrorCode = "TASK_NOT_FOUND"
	ErrCodeTaskInvalid       ErrorCode = "TASK_INVALID"
	ErrCodeTaskFailed        ErrorCode = "TASK_FAILED"
	ErrCodeTaskCancelled     ErrorCode = "TASK_CANCELLED"
	ErrCodeTaskAlreadyExists ErrorCode = "TASK_ALREADY_EXISTS"

	// 会话错误
	ErrCodeSessionNotFound      ErrorCode = "SESSION_NOT_FOUND"
	ErrCodeSessionExpired       ErrorCode = "SESSION_EXPIRED"
	ErrCodeSessionAlreadyExists ErrorCode = "SESSION_ALREADY_EXISTS"

	// 知识错误
	ErrCodeKnowledgeNotFound      ErrorCode = "KNOWLEDGE_NOT_FOUND"
	ErrCodeKnowledgeAlreadyExists ErrorCode = "KNOWLEDGE_ALREADY_EXISTS"

	// 模式错误
	ErrCodePatternNotFound      ErrorCode = "PATTERN_NOT_FOUND"
	ErrCodePatternAlreadyExists ErrorCode = "PATTERN_ALREADY_EXISTS"
	ErrCodePatternMatchFailed   ErrorCode = "PATTERN_MATCH_FAILED"

	// 适配器错误
	ErrCodeAdapterNotFound      ErrorCode = "ADAPTER_NOT_FOUND"
	ErrCodeAdapterInitFailed    ErrorCode = "ADAPTER_INIT_FAILED"
	ErrCodeAdapterExecuteFailed ErrorCode = "ADAPTER_EXECUTE_FAILED"

	// 存储错误
	ErrCodeStorageFailed   ErrorCode = "STORAGE_FAILED"
	ErrCodeStorageNotFound ErrorCode = "STORAGE_NOT_FOUND"
	ErrCodeStorageConflict ErrorCode = "STORAGE_CONFLICT"

	// 反馈错误
	ErrCodeFeedbackFailed   ErrorCode = "FEEDBACK_FAILED"
	ErrCodeValidationFailed ErrorCode = "VALIDATION_FAILED"
	ErrCodeFixFailed        ErrorCode = "FIX_FAILED"
)

// Error 自定义错误
type Error struct {
	Code       ErrorCode   `json:"code"`
	Message    string      `json:"message"`
	Details    interface{} `json:"details,omitempty"`
	HTTPStatus int         `json:"-"`
	Cause      error       `json:"-"`
}

// Error 实现 error 接口
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 返回原因错误
func (e *Error) Unwrap() error {
	return e.Cause
}

// New 创建新错误
func New(code ErrorCode, message string) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: codeToHTTPStatus(code),
	}
}

// Wrap 包装错误
func Wrap(err error, code ErrorCode, message string) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: codeToHTTPStatus(code),
		Cause:      err,
	}
}

// WithDetails 添加详细信息
func (e *Error) WithDetails(details interface{}) *Error {
	e.Details = details
	return e
}

// WithHTTPStatus 设置 HTTP 状态码
func (e *Error) WithHTTPStatus(status int) *Error {
	e.HTTPStatus = status
	return e
}

// Is 检查错误类型
func Is(err error, code ErrorCode) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == code
	}
	return false
}

// GetCode 获取错误代码
func GetCode(err error) ErrorCode {
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return ErrCodeInternal
}

// GetHTTPStatus 获取 HTTP 状态码
func GetHTTPStatus(err error) int {
	if e, ok := err.(*Error); ok {
		return e.HTTPStatus
	}
	return http.StatusInternalServerError
}

// codeToHTTPStatus 错误代码转 HTTP 状态码
func codeToHTTPStatus(code ErrorCode) int {
	switch code {
	case ErrCodeInvalidInput, ErrCodeTaskInvalid:
		return http.StatusBadRequest
	case ErrCodeNotFound, ErrCodeTaskNotFound, ErrCodeSessionNotFound,
		ErrCodeKnowledgeNotFound, ErrCodePatternNotFound, ErrCodeAdapterNotFound:
		return http.StatusNotFound
	case ErrCodeConflict, ErrCodeTaskAlreadyExists, ErrCodeSessionAlreadyExists,
		ErrCodeKnowledgeAlreadyExists, ErrCodePatternAlreadyExists:
		return http.StatusConflict
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeTimeout:
		return http.StatusRequestTimeout
	default:
		return http.StatusInternalServerError
	}
}

// 预定义错误
var (
	ErrInternal     = New(ErrCodeInternal, "内部服务器错误")
	ErrInvalidInput = New(ErrCodeInvalidInput, "输入无效")
	ErrNotFound     = New(ErrCodeNotFound, "资源不存在")
	ErrConflict     = New(ErrCodeConflict, "资源冲突")
	ErrUnauthorized = New(ErrCodeUnauthorized, "未授权")
	ErrForbidden    = New(ErrCodeForbidden, "禁止访问")
	ErrTimeout      = New(ErrCodeTimeout, "请求超时")
)

// WrapTaskNotFound 包装任务不存在错误
func WrapTaskNotFound(id string) *Error {
	return &Error{
		Code:       ErrCodeTaskNotFound,
		Message:    fmt.Sprintf("任务不存在: %s", id),
		HTTPStatus: http.StatusNotFound,
	}
}

// WrapSessionNotFound 包装会话不存在错误
func WrapSessionNotFound(id string) *Error {
	return &Error{
		Code:       ErrCodeSessionNotFound,
		Message:    fmt.Sprintf("会话不存在: %s", id),
		HTTPStatus: http.StatusNotFound,
	}
}

// WrapKnowledgeNotFound 包装知识不存在错误
func WrapKnowledgeNotFound(id string) *Error {
	return &Error{
		Code:       ErrCodeKnowledgeNotFound,
		Message:    fmt.Sprintf("知识不存在: %s", id),
		HTTPStatus: http.StatusNotFound,
	}
}

// WrapPatternNotFound 包装模式不存在错误
func WrapPatternNotFound(id string) *Error {
	return &Error{
		Code:       ErrCodePatternNotFound,
		Message:    fmt.Sprintf("模式不存在: %s", id),
		HTTPStatus: http.StatusNotFound,
	}
}

// WrapAdapterNotFound 包装适配器不存在错误
func WrapAdapterNotFound(name string) *Error {
	return &Error{
		Code:       ErrCodeAdapterNotFound,
		Message:    fmt.Sprintf("适配器不存在: %s", name),
		HTTPStatus: http.StatusNotFound,
	}
}

// WrapStorageFailed 包装存储失败错误
func WrapStorageFailed(err error) *Error {
	return &Error{
		Code:       ErrCodeStorageFailed,
		Message:    "存储操作失败",
		HTTPStatus: http.StatusInternalServerError,
		Cause:      err,
	}
}

// WrapValidationFailed 包装验证失败错误
func WrapValidationFailed(err error) *Error {
	return &Error{
		Code:       ErrCodeValidationFailed,
		Message:    "验证失败",
		HTTPStatus: http.StatusBadRequest,
		Cause:      err,
	}
}
