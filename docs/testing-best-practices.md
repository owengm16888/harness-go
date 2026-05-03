# Go 测试最佳实践在 Harness Engineering 中的应用

## 概述

本文档深入分析 Go 语言的测试最佳实践，并展示如何在 Harness Engineering 框架中应用这些模式。

## 一、表驱动测试

### 1.1 基本用法

**面试要点**：使用结构体切片定义测试用例。

```go
// internal/core/task_manager_test.go

func TestTaskManager_CreateTask(t *testing.T) {
    tests := []struct {
        name    string
        task    Task
        wantErr bool
        errCode errors.ErrorCode
    }{
        {
            name: "valid task",
            task: Task{
                ID:          "test-1",
                Type:        "implement",
                Description: "Test task",
            },
            wantErr: false,
        },
        {
            name: "missing ID",
            task: Task{
                Type:        "implement",
                Description: "Test task",
            },
            wantErr: true,
            errCode: errors.ErrCodeInvalidInput,
        },
        {
            name: "missing Type",
            task: Task{
                ID:          "test-2",
                Description: "Test task",
            },
            wantErr: true,
            errCode: errors.ErrCodeInvalidInput,
        },
        {
            name: "duplicate ID",
            task: Task{
                ID:          "existing-task",
                Type:        "implement",
                Description: "Test task",
            },
            wantErr: true,
            errCode: errors.ErrCodeConflict,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tm := NewTaskManager(nil, nil)
            
            _, err := tm.CreateTask(context.Background(), tt.task)
            
            if tt.wantErr {
                if err == nil {
                    t.Error("Expected error, got nil")
                    return
                }
                
                if tt.errCode != "" {
                    var appErr *errors.Error
                    if !errors.As(err, &appErr) {
                        t.Errorf("Expected *errors.Error, got %T", err)
                        return
                    }
                    
                    if appErr.Code != tt.errCode {
                        t.Errorf("Expected error code %s, got %s", tt.errCode, appErr.Code)
                    }
                }
            } else {
                if err != nil {
                    t.Errorf("Unexpected error: %v", err)
                }
            }
        })
    }
}
```

### 1.2 子测试

```go
// internal/core/task_manager_test.go

func TestTaskManager_ExecuteTask(t *testing.T) {
    t.Run("success", func(t *testing.T) {
        // 测试成功执行
    })
    
    t.Run("task not found", func(t *testing.T) {
        // 测试任务不存在
    })
    
    t.Run("task already running", func(t *testing.T) {
        // 测试任务已在运行
    })
}
```

## 二、基准测试

### 2.1 基本基准测试

**面试要点**：使用 `testing.B` 进行性能测试。

```go
// pkg/cache/cache_test.go

func BenchmarkCache_Get(b *testing.B) {
    cache := NewMemoryCache(1000, time.Minute)
    cache.Set("key", "value", time.Minute)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cache.Get("key")
    }
}

func BenchmarkCache_Set(b *testing.B) {
    cache := NewMemoryCache(1000, time.Minute)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cache.Set("key", "value", time.Minute)
    }
}

// 运行: go test -bench=. -benchmem
```

### 2.2 并行基准测试

```go
// pkg/cache/cache_test.go

func BenchmarkCache_GetParallel(b *testing.B) {
    cache := NewMemoryCache(1000, time.Minute)
    cache.Set("key", "value", time.Minute)
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            cache.Get("key")
        }
    })
}
```

## 三、模糊测试

### 3.1 基本模糊测试

**面试要点**：使用 `testing.F` 进行模糊测试。

```go
// pkg/utils/utils_test.go

func FuzzParseJSON(f *testing.F) {
    f.Add([]byte(`{"key":"value"}`))
    f.Add([]byte(`{"key":123}`))
    f.Add([]byte(`[1,2,3]`))
    
    f.Fuzz(func(t *testing.T, data []byte) {
        // 不应该 panic
        ParseJSON(data)
    })
}

// 运行: go test -fuzz=FuzzParseJSON
```

## 四、Mock 与接口测试

### 4.1 接口 Mock

**面试要点**：依赖接口而非实现，测试时注入 mock。

```go
// internal/core/engine_test.go

// Mock 任务管理器
type MockTaskManager struct {
    createTaskFn func(ctx context.Context, task Task) (*TaskState, error)
    executeTaskFn func(ctx context.Context, id string) (*Result, error)
    getTaskFn     func(ctx context.Context, id string) (*TaskState, error)
}

func (m *MockTaskManager) CreateTask(ctx context.Context, task Task) (*TaskState, error) {
    if m.createTaskFn != nil {
        return m.createTaskFn(ctx, task)
    }
    return &TaskState{Task: task, Status: TaskStatusPending}, nil
}

func (m *MockTaskManager) ExecuteTask(ctx context.Context, id string) (*Result, error) {
    if m.executeTaskFn != nil {
        return m.executeTaskFn(ctx, id)
    }
    return &Result{TaskID: id, Status: TaskStatusCompleted}, nil
}

func (m *MockTaskManager) GetTask(ctx context.Context, id string) (*TaskState, error) {
    if m.getTaskFn != nil {
        return m.getTaskFn(ctx, id)
    }
    return &TaskState{Task: Task{ID: id}, Status: TaskStatusPending}, nil
}

// 测试
func TestEngine_ExecuteTask(t *testing.T) {
    mockTM := &MockTaskManager{
        executeTaskFn: func(ctx context.Context, id string) (*Result, error) {
            return &Result{
                TaskID: id,
                Status: TaskStatusCompleted,
                Output: "success",
            }, nil
        },
    }
    
    engine := NewEngine(mockTM, nil, nil, nil, nil)
    
    result, err := engine.ExecuteTask(context.Background(), "claude-code", Task{
        ID:          "test-1",
        Type:        "implement",
        Description: "Test task",
    })
    
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }
    
    if result.Status != TaskStatusCompleted {
        t.Errorf("Expected status completed, got %s", result.Status)
    }
}
```

### 4.2 使用 gomock

```go
// 使用 gomock 生成 mock
//go:generate mockgen -destination=mock_task_manager.go -package=core . TaskManager

func TestEngine_ExecuteTask(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    mockTM := NewMockTaskManager(ctrl)
    mockTM.EXPECT().ExecuteTask(gomock.Any(), "test-1").Return(&Result{
        TaskID: "test-1",
        Status: TaskStatusCompleted,
    }, nil)
    
    engine := NewEngine(mockTM, nil, nil, nil, nil)
    
    result, err := engine.ExecuteTask(context.Background(), "claude-code", Task{
        ID: "test-1",
    })
    
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }
    
    if result.Status != TaskStatusCompleted {
        t.Errorf("Expected status completed, got %s", result.Status)
    }
}
```

## 五、测试辅助函数

### 5.1 测试工具函数

```go
// internal/testutil/testutil.go

// 创建测试任务
func CreateTestTask(id string) Task {
    return Task{
        ID:          id,
        Type:        "implement",
        Description: "Test task " + id,
        Context:     make(map[string]any),
    }
}

// 创建测试任务状态
func CreateTestTaskState(id string, status TaskStatus) *TaskState {
    return &TaskState{
        Task:      CreateTestTask(id),
        Status:    status,
        CreatedAt: time.Now(),
    }
}

// 断言错误类型
func AssertError(t *testing.T, err error, code errors.ErrorCode) {
    t.Helper()
    
    if err == nil {
        t.Error("Expected error, got nil")
        return
    }
    
    var appErr *errors.Error
    if !errors.As(err, &appErr) {
        t.Errorf("Expected *errors.Error, got %T", err)
        return
    }
    
    if appErr.Code != code {
        t.Errorf("Expected error code %s, got %s", code, appErr.Code)
    }
}

// 断言无错误
func AssertNoError(t *testing.T, err error) {
    t.Helper()
    
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
}
```

### 5.2 测试夹具

```go
// internal/testutil/fixtures.go

// 测试夹具
type TestFixtures struct {
    Tasks    map[string]*TaskState
    Sessions map[string]*Session
    Knowledge map[string]*KnowledgeEntry
}

func NewTestFixtures() *TestFixtures {
    return &TestFixtures{
        Tasks:    make(map[string]*TaskState),
        Sessions: make(map[string]*Session),
        Knowledge: make(map[string]*KnowledgeEntry),
    }
}

func (f *TestFixtures) AddTask(id string, status TaskStatus) *TaskState {
    state := CreateTestTaskState(id, status)
    f.Tasks[id] = state
    return state
}

func (f *TestFixtures) GetTask(id string) *TaskState {
    return f.Tasks[id]
}
```

## 六、集成测试

### 6.1 数据库集成测试

```go
// internal/storage/sqlite_test.go

func TestSQLiteStorage_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // 创建临时数据库
    tmpFile, err := os.CreateTemp("", "harness-test-*.db")
    if err != nil {
        t.Fatalf("Failed to create temp file: %v", err)
    }
    defer os.Remove(tmpFile.Name())
    tmpFile.Close()
    
    // 创建存储
    store, err := NewSQLiteStorage(tmpFile.Name())
    if err != nil {
        t.Fatalf("Failed to create storage: %v", err)
    }
    defer store.Close()
    
    ctx := context.Background()
    
    // 测试任务操作
    t.Run("task operations", func(t *testing.T) {
        // 创建任务
        task := CreateTestTaskState("test-1", TaskStatusPending)
        if err := store.SaveTask(ctx, task); err != nil {
            t.Fatalf("Failed to save task: %v", err)
        }
        
        // 获取任务
        retrieved, err := store.GetTask(ctx, "test-1")
        if err != nil {
            t.Fatalf("Failed to get task: %v", err)
        }
        
        if retrieved.Task.ID != "test-1" {
            t.Errorf("Expected task ID test-1, got %s", retrieved.Task.ID)
        }
        
        // 列出任务
        tasks, err := store.ListTasks(ctx, TaskFilter{})
        if err != nil {
            t.Fatalf("Failed to list tasks: %v", err)
        }
        
        if len(tasks) != 1 {
            t.Errorf("Expected 1 task, got %d", len(tasks))
        }
        
        // 删除任务
        if err := store.DeleteTask(ctx, "test-1"); err != nil {
            t.Fatalf("Failed to delete task: %v", err)
        }
        
        // 验证删除
        _, err = store.GetTask(ctx, "test-1")
        if err == nil {
            t.Error("Expected error after delete, got nil")
        }
    })
}
```

### 6.2 HTTP 集成测试

```go
// internal/api/server_test.go

func TestServer_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // 创建测试服务器
    server := setupTestServer(t)
    
    // 测试健康检查
    t.Run("health check", func(t *testing.T) {
        req, _ := http.NewRequest("GET", "/api/monitor/health", nil)
        rr := httptest.NewRecorder()
        
        server.router.ServeHTTP(rr, req)
        
        if rr.Code != http.StatusOK {
            t.Errorf("Expected status 200, got %d", rr.Code)
        }
        
        var health map[string]any
        if err := json.Unmarshal(rr.Body.Bytes(), &health); err != nil {
            t.Fatalf("Failed to unmarshal response: %v", err)
        }
        
        if health["status"] != "ok" {
            t.Errorf("Expected status ok, got %v", health["status"])
        }
    })
    
    // 测试创建任务
    t.Run("create task", func(t *testing.T) {
        task := CreateTestTask("test-1")
        body, _ := json.Marshal(task)
        
        req, _ := http.NewRequest("POST", "/api/tasks", bytes.NewBuffer(body))
        req.Header.Set("Content-Type", "application/json")
        rr := httptest.NewRecorder()
        
        server.router.ServeHTTP(rr, req)
        
        if rr.Code != http.StatusOK {
            t.Errorf("Expected status 200, got %d", rr.Code)
        }
        
        var state TaskState
        if err := json.Unmarshal(rr.Body.Bytes(), &state); err != nil {
            t.Fatalf("Failed to unmarshal response: %v", err)
        }
        
        if state.Task.ID != "test-1" {
            t.Errorf("Expected task ID test-1, got %s", state.Task.ID)
        }
    })
}
```

## 七、测试覆盖率

### 7.1 生成覆盖率报告

```bash
# 运行测试并生成覆盖率
go test ./... -coverprofile=coverage.out

# 查看覆盖率报告
go tool cover -html=coverage.out

# 查看覆盖率统计
go test ./... -cover
```

### 7.2 覆盖率目标

```go
// Makefile
.PHONY: test-coverage
test-coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | tail -1
```

## 八、最佳实践

### 8.1 测试命名规范

```go
// 好的测试命名
func TestTaskManager_CreateTask_Success(t *testing.T) {}
func TestTaskManager_CreateTask_MissingID(t *testing.T) {}
func TestTaskManager_CreateTask_DuplicateID(t *testing.T) {}
func TestTaskManager_ExecuteTask_Success(t *testing.T) {}
func TestTaskManager_ExecuteTask_TaskNotFound(t *testing.T) {}
```

### 8.2 测试组织

```go
// 按功能组织测试
func TestTaskManager(t *testing.T) {
    t.Run("CreateTask", func(t *testing.T) {
        t.Run("success", func(t *testing.T) {})
        t.Run("missing ID", func(t *testing.T) {})
        t.Run("duplicate ID", func(t *testing.T) {})
    })
    
    t.Run("ExecuteTask", func(t *testing.T) {
        t.Run("success", func(t *testing.T) {})
        t.Run("task not found", func(t *testing.T) {})
    })
}
```

### 8.3 测试辅助函数

```go
// 使用 t.Helper()
func assertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
}

func assertError(t *testing.T, err error, code errors.ErrorCode) {
    t.Helper()
    // ...
}
```

## 九、测试工具

### 9.1 testify

```go
// 使用 testify
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestTaskManager_CreateTask(t *testing.T) {
    tm := NewTaskManager(nil, nil)
    
    state, err := tm.CreateTask(context.Background(), Task{
        ID:   "test-1",
        Type: "implement",
    })
    
    require.NoError(t, err)
    assert.Equal(t, "test-1", state.Task.ID)
    assert.Equal(t, TaskStatusPending, state.Status)
}
```

### 9.2 gomock

```go
//go:generate mockgen -destination=mock_task_manager.go -package=core . TaskManager

func TestEngine_ExecuteTask(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    mockTM := NewMockTaskManager(ctrl)
    mockTM.EXPECT().ExecuteTask(gomock.Any(), "test-1").Return(&Result{
        TaskID: "test-1",
        Status: TaskStatusCompleted,
    }, nil)
    
    engine := NewEngine(mockTM, nil, nil, nil, nil)
    
    result, err := engine.ExecuteTask(context.Background(), "claude-code", Task{
        ID: "test-1",
    })
    
    require.NoError(t, err)
    assert.Equal(t, TaskStatusCompleted, result.Status)
}
```

## 十、总结

Go 测试最佳实践在 Harness Engineering 中的应用：

| 实践 | 应用 |
|------|------|
| 表驱动测试 | 多场景测试 |
| 基准测试 | 性能优化 |
| 模糊测试 | 边界测试 |
| Mock 测试 | 依赖隔离 |
| 集成测试 | 端到端验证 |
| 测试覆盖率 | 质量保证 |

通过遵循这些最佳实践，可以构建出高质量、可维护的测试套件。
