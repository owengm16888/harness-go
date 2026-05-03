package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
)

// Sentinel errors — 面试知识点: errors.Is/As 错误处理链
var (
	ErrTaskNotFound    = fmt.Errorf("task not found")
	ErrTaskInvalid     = fmt.Errorf("invalid task")
	ErrTaskNotCancellable = fmt.Errorf("task cannot be cancelled")
)

// TaskManager 管理任务的生命周期
type TaskManager struct {
	mu       sync.RWMutex
	tasks    map[string]*models.TaskState
	executor *Executor
	store    storage.TaskStore
}

// Executor 任务执行器
type Executor struct {
	engine *Engine
}

// NewExecutor 创建执行器
func NewExecutor(engine *Engine) *Executor {
	return &Executor{engine: engine}
}

// Execute 执行任务 — 调用适配器而非返回模拟结果
func (e *Executor) Execute(ctx context.Context, task models.Task) (models.Result, error) {
	// 通过引擎执行任务（使用第一个可用的适配器）
	for name := range e.engine.adapters {
		result, err := e.engine.ExecuteTask(ctx, name, task)
		if err != nil {
			return models.Result{}, err
		}
		return *result, nil
	}
	return models.Result{}, fmt.Errorf("no adapter available")
}

// NewTaskManager 创建任务管理器
func NewTaskManager(store storage.TaskStore, executor *Executor) *TaskManager {
	return &TaskManager{
		tasks:    make(map[string]*models.TaskState),
		executor: executor,
		store:    store,
	}
}

// LoadFromStorage 从持久化存储加载所有任务到内存
func (tm *TaskManager) LoadFromStorage(ctx context.Context) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tasks, err := tm.store.ListTasks(ctx, models.TaskFilter{})
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	for _, state := range tasks {
		tm.tasks[state.Task.ID] = state
	}

	return nil
}

// CreateTask 创建新任务
func (tm *TaskManager) CreateTask(ctx context.Context, task models.Task) (*models.TaskState, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 验证任务
	if err := tm.validateTask(task); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTaskInvalid, err)
	}

	// 创建任务状态
	state := &models.TaskState{
		Task:      task,
		Status:    models.TaskStatusPending,
		History:   []models.StatusChange{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 保存到存储
	if err := tm.store.SaveTask(ctx, state); err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	// 添加到内存
	tm.tasks[task.ID] = state

	return state, nil
}

// ExecuteTask 执行任务 — 修复竞态条件: 状态机守卫代替 Lock/Unlock 缝隙
func (tm *TaskManager) ExecuteTask(ctx context.Context, taskID string) (*models.Result, error) {
	// 获取任务并原子性地更新状态为 in_progress
	tm.mu.Lock()
	state, exists := tm.tasks[taskID]
	if !exists {
		tm.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}

	// 状态机守卫: 只有 pending 状态才能执行
	if state.Status != models.TaskStatusPending {
		tm.mu.Unlock()
		return nil, fmt.Errorf("task %s is in status %s, cannot execute", taskID, state.Status)
	}

	// 原子性更新状态
	oldStatus := state.Status
	state.Status = models.TaskStatusInProgress
	state.UpdatedAt = time.Now()
	state.History = append(state.History, models.StatusChange{
		From:      oldStatus,
		To:        models.TaskStatusInProgress,
		Reason:    "execution started",
		Timestamp: time.Now(),
	})
	tm.mu.Unlock()

	// 执行任务 (不持锁)
	result, err := tm.executor.Execute(ctx, state.Task)

	// 获取结果并更新状态
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if err != nil {
		state.Status = models.TaskStatusFailed
		state.UpdatedAt = time.Now()
		state.History = append(state.History, models.StatusChange{
			From:      models.TaskStatusInProgress,
			To:        models.TaskStatusFailed,
			Reason:    err.Error(),
			Timestamp: time.Now(),
		})
		return nil, err
	}

	// 检查是否在执行期间被取消
	if state.Status == models.TaskStatusCancelled {
		return nil, fmt.Errorf("task %s was cancelled during execution", taskID)
	}

	state.Status = models.TaskStatusCompleted
	state.Result = &result
	state.UpdatedAt = time.Now()
	state.History = append(state.History, models.StatusChange{
		From:      models.TaskStatusInProgress,
		To:        models.TaskStatusCompleted,
		Reason:    "execution completed",
		Timestamp: time.Now(),
	})

	// 保存到存储
	if err := tm.store.SaveTask(ctx, state); err != nil {
		return nil, fmt.Errorf("failed to save task result: %w", err)
	}

	return &result, nil
}

// GetTask 获取任务状态
func (tm *TaskManager) GetTask(ctx context.Context, taskID string) (*models.TaskState, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	state, exists := tm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}

	return state, nil
}

// ListTasks 列出所有任务
func (tm *TaskManager) ListTasks(ctx context.Context, filter models.TaskFilter) ([]*models.TaskState, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var tasks []*models.TaskState
	for _, state := range tm.tasks {
		if filter.Match(state) {
			tasks = append(tasks, state)
		}
	}

	return tasks, nil
}

// CancelTask 取消任务 — 修复: 先保存旧状态再修改
func (tm *TaskManager) CancelTask(ctx context.Context, taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	state, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}

	if state.Status == models.TaskStatusCompleted || state.Status == models.TaskStatusFailed {
		return fmt.Errorf("%w: task %s is in terminal status %s", ErrTaskNotCancellable, taskID, state.Status)
	}

	// 先保存旧状态, 再修改 — 修复原来的 From 字段 bug
	oldStatus := state.Status
	state.Status = models.TaskStatusCancelled
	state.UpdatedAt = time.Now()
	state.History = append(state.History, models.StatusChange{
		From:      oldStatus, // 修复: 之前这里是 state.Status (已经是 Cancelled)
		To:        models.TaskStatusCancelled,
		Reason:    "cancelled by user",
		Timestamp: time.Now(),
	})

	return nil
}

// validateTask 验证任务
func (tm *TaskManager) validateTask(task models.Task) error {
	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}
	if task.Type == "" {
		return fmt.Errorf("task type is required")
	}
	if task.Description == "" {
		return fmt.Errorf("task description is required")
	}
	return nil
}
