package errors

import (
	"testing"
)

func TestNew(t *testing.T) {
	err := New(ErrCodeNotFound, "资源不存在")

	if err.Code != ErrCodeNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeNotFound, err.Code)
	}

	if err.Message != "资源不存在" {
		t.Errorf("Expected message '资源不存在', got %s", err.Message)
	}

	if err.HTTPStatus != 404 {
		t.Errorf("Expected HTTP status 404, got %d", err.HTTPStatus)
	}
}

func TestWrap(t *testing.T) {
	originalErr := &ValidationError{Field: "id", Message: "ID is required"}
	wrappedErr := Wrap(originalErr, ErrCodeInvalidInput, "输入无效")

	if wrappedErr.Code != ErrCodeInvalidInput {
		t.Errorf("Expected code %s, got %s", ErrCodeInvalidInput, wrappedErr.Code)
	}

	if wrappedErr.Message != "输入无效" {
		t.Errorf("Expected message '输入无效', got %s", wrappedErr.Message)
	}

	if wrappedErr.Cause != originalErr {
		t.Error("Expected cause to be original error")
	}
}

func TestError_Error(t *testing.T) {
	err := New(ErrCodeNotFound, "资源不存在")
	expected := "NOT_FOUND: 资源不存在"

	if err.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, err.Error())
	}
}

func TestError_Unwrap(t *testing.T) {
	originalErr := &ValidationError{Field: "id", Message: "ID is required"}
	wrappedErr := Wrap(originalErr, ErrCodeInvalidInput, "输入无效")

	unwrapped := wrappedErr.Unwrap()
	if unwrapped != originalErr {
		t.Error("Expected unwrapped error to be original error")
	}
}

func TestIs(t *testing.T) {
	err := New(ErrCodeNotFound, "资源不存在")

	if !Is(err, ErrCodeNotFound) {
		t.Error("Expected Is to return true for NOT_FOUND")
	}

	if Is(err, ErrCodeInternal) {
		t.Error("Expected Is to return false for INTERNAL_ERROR")
	}
}

func TestGetCode(t *testing.T) {
	err := New(ErrCodeNotFound, "资源不存在")

	code := GetCode(err)
	if code != ErrCodeNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeNotFound, code)
	}
}

func TestGetHTTPStatus(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected int
	}{
		{ErrCodeInvalidInput, 400},
		{ErrCodeNotFound, 404},
		{ErrCodeConflict, 409},
		{ErrCodeUnauthorized, 401},
		{ErrCodeForbidden, 403},
		{ErrCodeTimeout, 408},
		{ErrCodeInternal, 500},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			err := New(tt.code, "test")
			status := GetHTTPStatus(err)

			if status != tt.expected {
				t.Errorf("Expected HTTP status %d, got %d", tt.expected, status)
			}
		})
	}
}

func TestWithDetails(t *testing.T) {
	err := New(ErrCodeNotFound, "资源不存在")
	err = err.WithDetails(map[string]string{"resource": "task"})

	if err.Details == nil {
		t.Error("Expected details to be set")
	}
}

func TestWithHTTPStatus(t *testing.T) {
	err := New(ErrCodeNotFound, "资源不存在")
	err = err.WithHTTPStatus(410)

	if err.HTTPStatus != 410 {
		t.Errorf("Expected HTTP status 410, got %d", err.HTTPStatus)
	}
}

func TestWrapTaskNotFound(t *testing.T) {
	err := WrapTaskNotFound("task-1")

	if err.Code != ErrCodeTaskNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeTaskNotFound, err.Code)
	}

	if err.HTTPStatus != 404 {
		t.Errorf("Expected HTTP status 404, got %d", err.HTTPStatus)
	}
}

func TestWrapSessionNotFound(t *testing.T) {
	err := WrapSessionNotFound("session-1")

	if err.Code != ErrCodeSessionNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeSessionNotFound, err.Code)
	}
}

func TestWrapKnowledgeNotFound(t *testing.T) {
	err := WrapKnowledgeNotFound("knowledge-1")

	if err.Code != ErrCodeKnowledgeNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeKnowledgeNotFound, err.Code)
	}
}

func TestWrapPatternNotFound(t *testing.T) {
	err := WrapPatternNotFound("pattern-1")

	if err.Code != ErrCodePatternNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodePatternNotFound, err.Code)
	}
}

func TestWrapAdapterNotFound(t *testing.T) {
	err := WrapAdapterNotFound("adapter-1")

	if err.Code != ErrCodeAdapterNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeAdapterNotFound, err.Code)
	}
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
