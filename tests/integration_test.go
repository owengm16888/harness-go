package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/internal/core"
	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
)

func setupTestEngine(t *testing.T) (*core.Engine, func()) {
	t.Helper()

	// 创建配置
	cfg := &config.Config{
		Engine: config.EngineConfig{
			MaxConcurrentTasks: 10,
			RetryCount:         3,
			// RetryDelay removed - not in EngineConfig
			TaskTimeout:        5 * time.Second,
		},
		Storage: config.StorageConfig{
			Type: "sqlite",
			Path: ":memory:",
		},
	}

	// 创建存储
	store, err := storage.NewSQLiteStorage(cfg.Storage)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// 创建引擎
	engine, err := core.NewEngine(cfg, store)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// 清理函数
	cleanup := func() {
		store.Close()
	}

	return engine, cleanup
}

func TestEngine_CreateTask(t *testing.T) {
	engine, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	// 创建任务
	task := &models.Task{
		ID:          "test-task-1",
		Type:        models.TaskTypeCodeGeneration,
		Description: "Generate Go code",
		Context:     map[string]any{"language": "go"},
		Constraints: []string{"no-external-deps"},
		Priority:    models.PriorityHigh,
	}

	err := engine.CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 获取任务
	retrieved, err := engine.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, retrieved.ID)
	}

	if retrieved.Status != models.TaskStatusPending {
		t.Errorf("Expected status pending, got %v", retrieved.Status)
	}
}

func TestEngine_ExecuteTask(t *testing.T) {
	engine, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	// 注册处理器
	engine.RegisterTaskHandler(models.TaskTypeCodeGeneration, func(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
		return &models.TaskResult{
			TaskID:  task.ID,
			Success: true,
			Output:  "Generated code",
		}, nil
	})

	// 创建任务
	task := &models.Task{
		ID:          "test-task-1",
		Type:        models.TaskTypeCodeGeneration,
		Description: "Generate Go code",
		Priority:    models.PriorityHigh,
	}

	engine.CreateTask(ctx, task)

	// 执行任务
	result, err := engine.ExecuteTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to execute task: %v", err)
	}

	if !result.Success {
		t.Error("Expected task to succeed")
	}

	// 检查任务状态
	retrieved, _ := engine.GetTask(ctx, task.ID)
	if retrieved.Status != models.TaskStatusCompleted {
		t.Errorf("Expected status completed, got %v", retrieved.Status)
	}
}

func TestEngine_TaskTimeout(t *testing.T) {
	engine, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	// 注册慢处理器
	engine.RegisterTaskHandler(models.TaskTypeCodeGeneration, func(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return &models.TaskResult{
				TaskID:  task.ID,
				Success: true,
			}, nil
		}
	})

	// 创建任务
	task := &models.Task{
		ID:          "test-task-1",
		Type:        models.TaskTypeCodeGeneration,
		Description: "Slow task",
		Priority:    models.PriorityHigh,
	}

	engine.CreateTask(ctx, task)

	// 执行任务（应该超时）
	_, err := engine.ExecuteTask(ctx, task.ID)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestEngine_TaskRetry(t *testing.T) {
	engine, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()
	attempts := 0

	// 注册会失败的处理器
	engine.RegisterTaskHandler(models.TaskTypeCodeGeneration, func(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
		attempts++
		if attempts < 3 {
			return &models.TaskResult{
				TaskID:  task.ID,
				Success: false,
				Error:   "Temporary error",
			}, nil
		}
		return &models.TaskResult{
			TaskID:  task.ID,
			Success: true,
		}, nil
	})

	// 创建任务
	task := &models.Task{
		ID:          "test-task-1",
		Type:        models.TaskTypeCodeGeneration,
		Description: "Retry task",
		Priority:    models.PriorityHigh,
	}

	engine.CreateTask(ctx, task)

	// 执行任务
	result, err := engine.ExecuteTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to execute task: %v", err)
	}

	if !result.Success {
		t.Error("Expected task to succeed after retries")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestEngine_ConcurrentTasks(t *testing.T) {
	engine, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	// 注册处理器
	engine.RegisterTaskHandler(models.TaskTypeCodeGeneration, func(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
		time.Sleep(100 * time.Millisecond)
		return &models.TaskResult{
			TaskID:  task.ID,
			Success: true,
		}, nil
	})

	// 创建多个任务
	taskCount := 5
	for i := 0; i < taskCount; i++ {
		task := &models.Task{
			ID:          fmt.Sprintf("task-%d", i),
			Type:        models.TaskTypeCodeGeneration,
			Description: fmt.Sprintf("Task %d", i),
			Priority:    models.PriorityHigh,
		}
		engine.CreateTask(ctx, task)
	}

	// 并发执行
	start := time.Now()
	var wg sync.WaitGroup
	errors := make(chan error, taskCount)

	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := engine.ExecuteTask(ctx, fmt.Sprintf("task-%d", idx))
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	elapsed := time.Since(start)

	// 检查错误
	for err := range errors {
		t.Errorf("Task failed: %v", err)
	}

	// 并发执行应该比串行快
	if elapsed > 1*time.Second {
		t.Errorf("Expected concurrent execution, took %v", elapsed)
	}
}

func TestEngine_ListTasks(t *testing.T) {
	engine, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	// 创建多个任务
	for i := 0; i < 5; i++ {
		task := &models.Task{
			ID:          fmt.Sprintf("task-%d", i),
			Type:        models.TaskTypeCodeGeneration,
			Description: fmt.Sprintf("Task %d", i),
			Priority:    models.PriorityHigh,
		}
		engine.CreateTask(ctx, task)
	}

	// 列出任务
	tasks, err := engine.ListTasks(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 5 {
		t.Errorf("Expected 5 tasks, got %d", len(tasks))
	}
}

func TestEngine_DeleteTask(t *testing.T) {
	engine, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	// 创建任务
	task := &models.Task{
		ID:          "test-task-1",
		Type:        models.TaskTypeCodeGeneration,
		Description: "Test task",
		Priority:    models.PriorityHigh,
	}

	engine.CreateTask(ctx, task)

	// 删除任务
	err := engine.DeleteTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	// 确认删除
	_, err = engine.GetTask(ctx, task.ID)
	if err == nil {
		t.Error("Expected error after delete")
	}
}
