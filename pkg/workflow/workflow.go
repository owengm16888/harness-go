package workflow

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusReady      TaskStatus = "ready"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusSkipped    TaskStatus = "skipped"
)

// DependencyType 依赖类型
type DependencyType string

const (
	DepFinishToStart DependencyType = "finish_to_start" // 完成后才能开始 (默认)
	DepStartToStart  DependencyType = "start_to_start"  // 同时开始
	DepFinishToFinish DependencyType = "finish_to_finish" // 同时完成
)

// WFTask 工作流任务
type WFTask struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Description  string            `json:"description"`
	Status       TaskStatus        `json:"status"`
	Priority     int               `json:"priority"`
	Dependencies []Dependency      `json:"dependencies"`
	Timeout      time.Duration     `json:"timeout"`
	RetryCount   int               `json:"retry_count"`
	MaxRetries   int               `json:"max_retries"`
	Properties   map[string]string `json:"properties"`
	Result       interface{}       `json:"result,omitempty"`
	Error        string            `json:"error,omitempty"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// Dependency 依赖关系
type Dependency struct {
	TaskID string         `json:"task_id"`
	Type   DependencyType `json:"type"`
}

// Workflow 工作流
type Workflow struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      TaskStatus        `json:"status"`
	Tasks       map[string]*WFTask `json:"tasks"`
	Properties  map[string]string  `json:"properties"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// WorkflowEngine 工作流引擎
type WorkflowEngine struct {
	mu        sync.RWMutex
	workflows map[string]*Workflow
	executors map[string]TaskExecutor
	eventCh   chan WorkflowEvent
}

// TaskExecutor 任务执行器
type TaskExecutor func(ctx context.Context, task *WFTask) (interface{}, error)

// WorkflowEvent 工作流事件
type WorkflowEvent struct {
	Type      string    `json:"type"`
	WorkflowID string   `json:"workflow_id"`
	TaskID    string    `json:"task_id,omitempty"`
	Status    TaskStatus `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// NewWorkflowEngine 创建工作流引擎
func NewWorkflowEngine() *WorkflowEngine {
	engine := &WorkflowEngine{
		workflows: make(map[string]*Workflow),
		executors: make(map[string]TaskExecutor),
		eventCh:   make(chan WorkflowEvent, 1000),
	}
	go engine.eventLoop()
	return engine
}

// RegisterExecutor 注册任务执行器
func (e *WorkflowEngine) RegisterExecutor(taskType string, executor TaskExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executors[taskType] = executor
}

// CreateWorkflow 创建工作流
func (e *WorkflowEngine) CreateWorkflow(ctx context.Context, workflow *Workflow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if workflow.ID == "" {
		return fmt.Errorf("workflow ID is required")
	}

	if _, exists := e.workflows[workflow.ID]; exists {
		return fmt.Errorf("workflow already exists: %s", workflow.ID)
	}

	now := time.Now()
	workflow.Status = TaskStatusPending
	workflow.CreatedAt = now
	workflow.UpdatedAt = now

	if workflow.Tasks == nil {
		workflow.Tasks = make(map[string]*WFTask)
	}
	if workflow.Properties == nil {
		workflow.Properties = make(map[string]string)
	}

	// 初始化所有任务状态
	for _, task := range workflow.Tasks {
		task.Status = TaskStatusPending
		task.CreatedAt = now
		if task.Properties == nil {
			task.Properties = make(map[string]string)
		}
	}

	// 验证依赖关系
	if err := e.validateDependencies(workflow); err != nil {
		return err
	}

	e.workflows[workflow.ID] = workflow
	return nil
}

// validateDependencies 验证依赖关系
func (e *WorkflowEngine) validateDependencies(wf *Workflow) error {
	// 检查循环依赖
	visited := make(map[string]bool)
	inStack := make(map[string]bool)

	var hasCycle func(taskID string) bool
	hasCycle = func(taskID string) bool {
		if inStack[taskID] {
			return true
		}
		if visited[taskID] {
			return false
		}

		visited[taskID] = true
		inStack[taskID] = true

		task, exists := wf.Tasks[taskID]
		if exists {
			for _, dep := range task.Dependencies {
				if hasCycle(dep.TaskID) {
					return true
				}
			}
		}

		inStack[taskID] = false
		return false
	}

	for taskID := range wf.Tasks {
		if hasCycle(taskID) {
			return fmt.Errorf("circular dependency detected involving task: %s", taskID)
		}
	}

	// 检查依赖是否存在
	for taskID, task := range wf.Tasks {
		for _, dep := range task.Dependencies {
			if _, exists := wf.Tasks[dep.TaskID]; !exists {
				return fmt.Errorf("task %s depends on non-existent task: %s", taskID, dep.TaskID)
			}
		}
	}

	return nil
}

// StartWorkflow 启动工作流
func (e *WorkflowEngine) StartWorkflow(ctx context.Context, workflowID string) error {
	e.mu.Lock()
	wf, exists := e.workflows[workflowID]
	if !exists {
		e.mu.Unlock()
		return fmt.Errorf("workflow not found: %s", workflowID)
	}
	if wf.Status != TaskStatusPending {
		e.mu.Unlock()
		return fmt.Errorf("workflow is not in pending status: %s", wf.Status)
	}

	now := time.Now()
	wf.Status = TaskStatusRunning
	wf.StartedAt = &now
	wf.UpdatedAt = now
	e.mu.Unlock()

	e.emitEvent(WorkflowEvent{
		Type:       "workflow_started",
		WorkflowID: workflowID,
		Status:     TaskStatusRunning,
		Timestamp:  time.Now(),
	})

	// 启动执行循环
	go e.executionLoop(ctx, workflowID)

	return nil
}

// executionLoop 执行循环
func (e *WorkflowEngine) executionLoop(ctx context.Context, workflowID string) {
	for {
		// 获取就绪任务
		readyTasks := e.getReadyTasks(workflowID)
		if len(readyTasks) == 0 {
			// 检查是否所有任务都完成
			if e.isWorkflowComplete(workflowID) {
				e.completeWorkflow(workflowID)
				return
			}
			// 检查是否有失败任务
			if e.hasFailedTasks(workflowID) {
				e.failWorkflow(workflowID)
				return
			}
			// 等待任务完成
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// 并发执行就绪任务
		var wg sync.WaitGroup
		for _, task := range readyTasks {
			wg.Add(1)
			go func(t *WFTask) {
				defer wg.Done()
				e.executeTask(ctx, workflowID, t)
			}(task)
		}
		wg.Wait()
	}
}

// getReadyTasks 获取就绪任务
func (e *WorkflowEngine) getReadyTasks(workflowID string) []*WFTask {
	e.mu.RLock()
	defer e.mu.RUnlock()

	wf, exists := e.workflows[workflowID]
	if !exists {
		return nil
	}

	var ready []*WFTask
	for _, task := range wf.Tasks {
		if task.Status != TaskStatusPending {
			continue
		}

		// 检查所有依赖是否满足
		allDepsMet := true
		for _, dep := range task.Dependencies {
			depTask, exists := wf.Tasks[dep.TaskID]
			if !exists {
				allDepsMet = false
				break
			}

			switch dep.Type {
			case DepFinishToStart:
				if depTask.Status != TaskStatusCompleted {
					allDepsMet = false
				}
			case DepStartToStart:
				if depTask.Status != TaskStatusRunning && depTask.Status != TaskStatusCompleted {
					allDepsMet = false
				}
			case DepFinishToFinish:
				if depTask.Status == TaskStatusPending {
					allDepsMet = false
				}
			}
		}

		if allDepsMet {
			ready = append(ready, task)
		}
	}

	// 按优先级排序
	sort.Slice(ready, func(i, j int) bool {
		return ready[i].Priority > ready[j].Priority
	})

	return ready
}

// executeTask 执行任务
func (e *WorkflowEngine) executeTask(ctx context.Context, workflowID string, task *WFTask) {
	e.mu.Lock()
	task.Status = TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	e.mu.Unlock()

	e.emitEvent(WorkflowEvent{
		Type:       "task_started",
		WorkflowID: workflowID,
		TaskID:     task.ID,
		Status:     TaskStatusRunning,
		Timestamp:  time.Now(),
	})

	// 查找执行器
	e.mu.RLock()
	executor, exists := e.executors[task.Type]
	e.mu.RUnlock()

	if !exists {
		e.failTask(workflowID, task.ID, fmt.Sprintf("no executor for task type: %s", task.Type))
		return
	}

	// 创建超时上下文
	taskCtx := ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// 执行
	result, err := executor(taskCtx, task)

	e.mu.Lock()
	defer e.mu.Unlock()

	if err != nil {
		task.Error = err.Error()
		task.RetryCount++

		if task.RetryCount < task.MaxRetries {
			// 重试
			task.Status = TaskStatusPending
			task.StartedAt = nil
			e.emitEvent(WorkflowEvent{
				Type:       "task_retrying",
				WorkflowID: workflowID,
				TaskID:     task.ID,
				Status:     TaskStatusPending,
				Message:    fmt.Sprintf("retry %d/%d: %s", task.RetryCount, task.MaxRetries, err.Error()),
				Timestamp:  time.Now(),
			})
		} else {
			// 失败
			task.Status = TaskStatusFailed
			completedAt := time.Now()
			task.CompletedAt = &completedAt
			e.emitEvent(WorkflowEvent{
				Type:       "task_failed",
				WorkflowID: workflowID,
				TaskID:     task.ID,
				Status:     TaskStatusFailed,
				Message:    err.Error(),
				Timestamp:  time.Now(),
			})
		}
	} else {
		// 成功
		task.Status = TaskStatusCompleted
		task.Result = result
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		e.emitEvent(WorkflowEvent{
			Type:       "task_completed",
			WorkflowID: workflowID,
			TaskID:     task.ID,
			Status:     TaskStatusCompleted,
			Timestamp:  time.Now(),
		})
	}
}

// failTask 标记任务失败
func (e *WorkflowEngine) failTask(workflowID, taskID, message string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, exists := e.workflows[workflowID]
	if !exists {
		return
	}

	task, exists := wf.Tasks[taskID]
	if !exists {
		return
	}

	task.Status = TaskStatusFailed
	task.Error = message
	completedAt := time.Now()
	task.CompletedAt = &completedAt
}

// isWorkflowComplete 检查工作流是否完成
func (e *WorkflowEngine) isWorkflowComplete(workflowID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	wf, exists := e.workflows[workflowID]
	if !exists {
		return false
	}

	for _, task := range wf.Tasks {
		if task.Status != TaskStatusCompleted && task.Status != TaskStatusSkipped {
			return false
		}
	}
	return true
}

// hasFailedTasks 检查是否有失败任务
func (e *WorkflowEngine) hasFailedTasks(workflowID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	wf, exists := e.workflows[workflowID]
	if !exists {
		return false
	}

	for _, task := range wf.Tasks {
		if task.Status == TaskStatusFailed {
			return true
		}
	}
	return false
}

// completeWorkflow 完成工作流
func (e *WorkflowEngine) completeWorkflow(workflowID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, exists := e.workflows[workflowID]
	if !exists {
		return
	}

	wf.Status = TaskStatusCompleted
	now := time.Now()
	wf.CompletedAt = &now
	wf.UpdatedAt = now

	e.emitEvent(WorkflowEvent{
		Type:       "workflow_completed",
		WorkflowID: workflowID,
		Status:     TaskStatusCompleted,
		Timestamp:  time.Now(),
	})
}

// failWorkflow 标记工作流失败
func (e *WorkflowEngine) failWorkflow(workflowID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, exists := e.workflows[workflowID]
	if !exists {
		return
	}

	wf.Status = TaskStatusFailed
	now := time.Now()
	wf.CompletedAt = &now
	wf.UpdatedAt = now

	e.emitEvent(WorkflowEvent{
		Type:       "workflow_failed",
		WorkflowID: workflowID,
		Status:     TaskStatusFailed,
		Timestamp:  time.Now(),
	})
}

// GetWorkflow 获取工作流
func (e *WorkflowEngine) GetWorkflow(ctx context.Context, workflowID string) (*Workflow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	wf, exists := e.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	return wf, nil
}

// ListWorkflows 列出工作流
func (e *WorkflowEngine) ListWorkflows(ctx context.Context) []*Workflow {
	e.mu.RLock()
	defer e.mu.RUnlock()

	workflows := make([]*Workflow, 0, len(e.workflows))
	for _, wf := range e.workflows {
		workflows = append(workflows, wf)
	}
	return workflows
}

// CancelWorkflow 取消工作流
func (e *WorkflowEngine) CancelWorkflow(ctx context.Context, workflowID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, exists := e.workflows[workflowID]
	if !exists {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	if wf.Status != TaskStatusRunning {
		return fmt.Errorf("workflow is not running: %s", wf.Status)
	}

	// 取消所有待处理任务
	for _, task := range wf.Tasks {
		if task.Status == TaskStatusPending || task.Status == TaskStatusReady {
			task.Status = TaskStatusCancelled
		}
	}

	wf.Status = TaskStatusCancelled
	now := time.Now()
	wf.CompletedAt = &now
	wf.UpdatedAt = now

	e.emitEvent(WorkflowEvent{
		Type:       "workflow_cancelled",
		WorkflowID: workflowID,
		Status:     TaskStatusCancelled,
		Timestamp:  time.Now(),
	})

	return nil
}

// GetEventChannel 获取事件通道
func (e *WorkflowEngine) GetEventChannel() <-chan WorkflowEvent {
	return e.eventCh
}

// emitEvent 发送事件
func (e *WorkflowEngine) emitEvent(event WorkflowEvent) {
	select {
	case e.eventCh <- event:
	default:
		// 通道满了，丢弃事件
	}
}

// eventLoop 事件循环 (用于日志等)
func (e *WorkflowEngine) eventLoop() {
	for event := range e.eventCh {
		// 可以在这里添加日志、监控等
		_ = event
	}
}

// GetStats 获取统计信息
func (e *WorkflowEngine) GetStats(ctx context.Context) map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := map[string]interface{}{
		"total_workflows": len(e.workflows),
		"by_status": map[string]int{
			"pending":   0,
			"running":   0,
			"completed": 0,
			"failed":    0,
			"cancelled": 0,
		},
		"total_tasks": 0,
		"tasks_by_status": map[string]int{
			"pending":   0,
			"running":   0,
			"completed": 0,
			"failed":    0,
			"cancelled": 0,
			"skipped":   0,
		},
	}

	byStatus := stats["by_status"].(map[string]int)
	tasksByStatus := stats["tasks_by_status"].(map[string]int)

	for _, wf := range e.workflows {
		byStatus[string(wf.Status)]++
		for _, task := range wf.Tasks {
			stats["total_tasks"] = stats["total_tasks"].(int) + 1
			tasksByStatus[string(task.Status)]++
		}
	}

	return stats
}
