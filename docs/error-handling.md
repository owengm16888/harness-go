# Go 错误处理在 Harness Engineering 中的最佳实践

## 概述

本文档深入分析 Go 语言的错误处理哲学，并展示如何在 Harness Engineering 框架中应用这些最佳实践。

## 一、错误处理哲学

### 1.1 错误就是值

Go 语言的核心哲学：**错误就是值**，不是异常。

```go
// 不好 - 使用 panic
func divide(a, b int) int {
    if b == 0 {
        panic("division by zero")
    }
    return a / b
}

// 好 - 返回错误
func divide(a, b int) (int, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}
```

### 1.2 显式错误检查

```go
// 不好 - 忽略错误
result := doSomething()

// 好 - 显式检查
result, err := doSomething()
if err != nil {
    return err
}
```

## 二、错误类型

### 2.1 哨兵错误（Sentinel Error）

```go
// pkg/errors/errors.go

// 定义哨兵错误
var (
    ErrTaskNotFound      = errors.New("task not found")
    ErrSessionNotFound   = errors.New("session not found")
    ErrKnowledgeNotFound = errors.New("knowledge not found")
    ErrPatternNotFound   = errors.New("pattern not found")
    ErrUnauthorized      = errors.New("unauthorized")
    ErrForbidden         = errors.New("forbidden")
)

// 使用 errors.Is 匹配
func (m *TaskManager) GetTask(id string) (*Task, error) {
    task, err := m.store.GetTask(id)
    if err != nil {
        if errors.Is(err, ErrTaskNotFound) {
            return nil, fmt.Errorf("task %s not found: %w", id, err)
        }
        return nil, err
    }
    return task, nil
}
```

### 2.2 自定义错误类型

```go
// pkg/errors/errors.go

// 自定义错误类型
type Error struct {
    Code       ErrorCode
    Message    string
    Details    interface{}
    HTTPStatus int
    Cause      error
}

// 实现 error 接口
func (e *Error) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// 实现 Unwrap 接口
func (e *Error) Unwrap() error {
    return e.Cause
}

// 创建错误
func New(code ErrorCode, message string) *Error {
    return &Error{
        Code:       code,
        Message:    message,
        HTTPStatus: codeToHTTPStatus(code),
    }
}

// 包装错误
func Wrap(err error, code ErrorCode, message string) *Error {
    return &Error{
        Code:       code,
        Message:    message,
        HTTPStatus: codeToHTTPStatus(code),
        Cause:      err,
    }
}

// 使用 errors.As 匹配类型
func HandleError(err error) {
    var appErr *Error
    if errors.As(err, &appErr) {
        fmt.Printf("Error code: %s\n", appErr.Code)
        fmt.Printf("HTTP status: %d\n", appErr.HTTPStatus)
    }
}
```

## 三、错误包装链

### 3.1 错误包装

```go
// pkg/errors/errors.go

// 使用 fmt.Errorf 包装错误
func queryUser(id string) (*User, error) {
    user, err := db.Query("SELECT * FROM users WHERE id = ?", id)
    if err != nil {
        return nil, fmt.Errorf("query user %s: %w", id, err)
    }
    return user, nil
}

// 使用自定义 Wrap
func GetUser(id string) (*User, error) {
    user, err := queryUser(id)
    if err != nil {
        return nil, Wrap(err, ErrCodeStorageFailed, "failed to get user")
    }
    return user, nil
}
```

### 3.2 沿链匹配

```go
// pkg/errors/errors.go

// errors.Is - 沿链匹配值
func Is(err error, target error) bool {
    return errors.Is(err, target)
}

// errors.As - 沿链匹配类型
func As(err error, target interface{}) bool {
    return errors.As(err, target)
}

// 使用示例
func HandleError(err error) {
    // 匹配哨兵错误
    if errors.Is(err, ErrTaskNotFound) {
        fmt.Println("Task not found")
        return
    }
    
    // 匹配自定义错误类型
    var appErr *Error
    if errors.As(err, &appErr) {
        fmt.Printf("Error code: %s\n", appErr.Code)
        fmt.Printf("HTTP status: %d\n", appErr.HTTPStatus)
        return
    }
    
    // 未知错误
    fmt.Println("Unknown error")
}
```

## 四、Harness 中的错误处理

### 4.1 统一错误处理

```go
// pkg/errors/errors.go

// 错误代码
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
    ErrCodeTaskNotFound  ErrorCode = "TASK_NOT_FOUND"
    ErrCodeTaskInvalid   ErrorCode = "TASK_INVALID"
    ErrCodeTaskFailed    ErrorCode = "TASK_FAILED"
    ErrCodeTaskCancelled ErrorCode = "TASK_CANCELLED"
    
    // 会话错误
    ErrCodeSessionNotFound ErrorCode = "SESSION_NOT_FOUND"
    ErrCodeSessionExpired  ErrorCode = "SESSION_EXPIRED"
    
    // 知识错误
    ErrCodeKnowledgeNotFound ErrorCode = "KNOWLEDGE_NOT_FOUND"
    
    // 模式错误
    ErrCodePatternNotFound ErrorCode = "PATTERN_NOT_FOUND"
    ErrCodePatternMatchFailed ErrorCode = "PATTERN_MATCH_FAILED"
)

// 预定义错误
var (
    ErrInternal     = New(ErrCodeInternal, "internal server error")
    ErrInvalidInput = New(ErrCodeInvalidInput, "invalid input")
    ErrNotFound     = New(ErrCodeNotFound, "resource not found")
    ErrUnauthorized = New(ErrCodeUnauthorized, "unauthorized")
    ErrForbidden    = New(ErrCodeForbidden, "forbidden")
    ErrTimeout      = New(ErrCodeTimeout, "request timeout")
)

// 包装错误
func WrapTaskNotFound(id string) *Error {
    return &Error{
        Code:       ErrCodeTaskNotFound,
        Message:    fmt.Sprintf("task not found: %s", id),
        HTTPStatus: http.StatusNotFound,
    }
}

func WrapStorageFailed(err error) *Error {
    return Wrap(err, ErrCodeStorageFailed, "storage operation failed")
}
```

### 4.2 错误处理中间件

```go
// pkg/middleware/middleware.go

// 错误处理中间件
func ErrorHandler(log *logger.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if err := recover(); err != nil {
                    var appErr *errors.Error
                    
                    switch e := err.(type) {
                    case *errors.Error:
                        appErr = e
                    case error:
                        appErr = errors.Wrap(e, errors.ErrCodeInternal, "internal error")
                    default:
                        appErr = errors.New(errors.ErrCodeInternal, fmt.Sprintf("%v", err))
                    }
                    
                    log.WithFields(map[string]any{
                        "error":      appErr.Error(),
                        "code":       appErr.Code,
                        "request_id": r.Context().Value("request_id"),
                    }).Error("Request error")
                    
                    w.Header().Set("Content-Type", "application/json")
                    w.WriteHeader(appErr.HTTPStatus)
                    json.NewEncoder(w).Encode(map[string]interface{}{
                        "error": map[string]interface{}{
                            "code":    appErr.Code,
                            "message": appErr.Message,
                        },
                    })
                }
            }()
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### 4.3 业务逻辑中的错误处理

```go
// internal/core/task_manager.go

func (m *TaskManager) CreateTask(ctx context.Context, task Task) (*TaskState, error) {
    // 验证任务
    if err := m.validateTask(task); err != nil {
        return nil, errors.Wrap(err, errors.ErrCodeInvalidInput, "invalid task")
    }
    
    // 检查任务是否已存在
    existing, err := m.store.GetTask(ctx, task.ID)
    if err != nil && !errors.Is(err, errors.ErrTaskNotFound) {
        return nil, errors.WrapStorageFailed(err)
    }
    if existing != nil {
        return nil, errors.New(errors.ErrCodeConflict, "task already exists")
    }
    
    // 创建任务
    state := &TaskState{
        Task:      task,
        Status:    TaskStatusPending,
        CreatedAt: time.Now(),
    }
    
    if err := m.store.SaveTask(ctx, state); err != nil {
        return nil, errors.WrapStorageFailed(err)
    }
    
    return state, nil
}

func (m *TaskManager) ExecuteTask(ctx context.Context, id string) (*Result, error) {
    // 获取任务
    state, err := m.GetTask(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // 检查任务状态
    if state.Status != TaskStatusPending {
        return nil, errors.New(errors.ErrCodeTaskInvalid, 
            fmt.Sprintf("task is not pending: %s", state.Status))
    }
    
    // 执行任务
    result, err := m.execute(ctx, state)
    if err != nil {
        // 更新任务状态为失败
        state.Status = TaskStatusFailed
        state.Error = err
        m.store.SaveTask(ctx, state)
        
        return nil, errors.Wrap(err, errors.ErrCodeTaskFailed, "task execution failed")
    }
    
    // 更新任务状态为完成
    state.Status = TaskStatusCompleted
    state.Result = result
    m.store.SaveTask(ctx, state)
    
    return result, nil
}
```

## 五、错误恢复

### 5.1 panic 和 recover

```go
// pkg/middleware/middleware.go

// 恢复 panic
func Recovery(log *logger.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if err := recover(); err != nil {
                    stack := debug.Stack()
                    
                    log.WithFields(map[string]any{
                        "error":       err,
                        "stack":       string(stack),
                        "method":      r.Method,
                        "path":        r.URL.Path,
                        "request_id":  r.Context().Value("request_id"),
                    }).Error("Panic recovered")
                    
                    http.Error(w, "Internal Server Error", 
                        http.StatusInternalServerError)
                }
            }()
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### 5.2 重试机制

```go
// pkg/utils/retry.go

type RetryConfig struct {
    MaxAttempts int
    Delay       time.Duration
    MaxDelay    time.Duration
    Multiplier  float64
}

func Retry(config RetryConfig, fn func() error) error {
    var lastErr error
    
    for attempt := 0; attempt < config.MaxAttempts; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // 计算延迟
        delay := config.Delay * time.Duration(math.Pow(config.Multiplier, float64(attempt)))
        if delay > config.MaxDelay {
            delay = config.MaxDelay
        }
        
        time.Sleep(delay)
    }
    
    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// 使用
err := Retry(RetryConfig{
    MaxAttempts: 3,
    Delay:       time.Second,
    MaxDelay:    10 * time.Second,
    Multiplier:  2,
}, func() error {
    return doSomething()
})
```

## 六、错误日志

### 6.1 结构化错误日志

```go
// pkg/logger/logger.go

func (l *Logger) LogError(err error, context map[string]any) {
    fields := map[string]any{
        "error": err.Error(),
    }
    
    // 提取自定义错误信息
    var appErr *errors.Error
    if errors.As(err, &appErr) {
        fields["error_code"] = appErr.Code
        fields["http_status"] = appErr.HTTPStatus
    }
    
    // 添加上下文
    for k, v := range context {
        fields[k] = v
    }
    
    l.WithFields(fields).Error("Operation failed")
}

// 使用
logger.LogError(err, map[string]any{
    "task_id": task.ID,
    "operation": "execute",
})
```

### 6.2 错误追踪

```go
// pkg/errors/trace.go

type ErrorTrace struct {
    Error     error
    Stack     string
    Timestamp time.Time
    Context   map[string]any
}

func CaptureError(err error, context map[string]any) *ErrorTrace {
    return &ErrorTrace{
        Error:     err,
        Stack:     string(debug.Stack()),
        Timestamp: time.Now(),
        Context:   context,
    }
}

func (t *ErrorTrace) Log(logger *logger.Logger) {
    logger.WithFields(map[string]any{
        "error":     t.Error.Error(),
        "stack":     t.Stack,
        "timestamp": t.Timestamp,
        "context":   t.Context,
    }).Error("Error captured")
}
```

## 七、最佳实践

### 7.1 错误处理原则

1. **错误就是值**：不要使用 panic，返回错误
2. **显式检查**：不要忽略错误
3. **尽早返回**：遇到错误立即返回
4. **包装错误**：添加上下文信息
5. **统一处理**：使用中间件统一处理错误

### 7.2 错误命名规范

```go
// 哨兵错误：Err + 名词
var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")

// 错误代码：ERR_ + 类别_ + 具体
const (
    ErrCodeTaskNotFound  ErrorCode = "ERR_TASK_NOT_FOUND"
    ErrCodeTaskFailed    ErrorCode = "ERR_TASK_FAILED"
    ErrCodeSessionExpired ErrorCode = "ERR_SESSION_EXPIRED"
)
```

### 7.3 错误包装规范

```go
// 包装格式：动词 + 名词 + %w
fmt.Errorf("query user %s: %w", id, err)
fmt.Errorf("execute task %s: %w", taskID, err)
fmt.Errorf("save knowledge %s: %w", entry.ID, err)
```

## 八、常见陷阱

### 8.1 错误比较

```go
// 不好 - 使用 == 比较
if err == ErrNotFound {
    // ...
}

// 好 - 使用 errors.Is 比较
if errors.Is(err, ErrNotFound) {
    // ...
}
```

### 8.2 错误类型断言

```go
// 不好 - 使用类型断言
if e, ok := err.(*Error); ok {
    // ...
}

// 好 - 使用 errors.As
var e *Error
if errors.As(err, &e) {
    // ...
}
```

### 8.3 忽略错误

```go
// 不好 - 忽略错误
doSomething()

// 好 - 检查错误
if err := doSomething(); err != nil {
    return err
}
```

## 九、总结

Go 错误处理在 Harness Engineering 中的应用：

| 概念 | 应用 |
|------|------|
| 错误就是值 | 返回错误，不使用 panic |
| 哨兵错误 | 定义预定义错误 |
| 自定义错误类型 | 携带更多信息 |
| 错误包装链 | 添加上下文 |
| errors.Is/As | 沿链匹配 |
| 错误恢复 | panic/recover |
| 重试机制 | 临时错误处理 |
| 结构化日志 | 错误记录和追踪 |

通过遵循这些最佳实践，可以构建出健壮、可维护的错误处理系统。
