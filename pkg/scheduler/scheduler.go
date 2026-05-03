package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ScheduleType 调度类型
type ScheduleType string

const (
	ScheduleOnce     ScheduleType = "once"
	ScheduleInterval ScheduleType = "interval"
	ScheduleCron     ScheduleType = "cron"
	ScheduleDaily    ScheduleType = "daily"
	ScheduleWeekly   ScheduleType = "weekly"
	ScheduleMonthly  ScheduleType = "monthly"
)

// Task 任务
type Task struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        ScheduleType      `json:"type"`
	Schedule    string            `json:"schedule"`
	Handler     TaskHandler       `json:"-"`
	Data        map[string]any    `json:"data,omitempty"`
	Enabled     bool              `json:"enabled"`
	LastRun     time.Time         `json:"last_run,omitempty"`
	NextRun     time.Time         `json:"next_run,omitempty"`
	RunCount    int               `json:"run_count"`
	LastError   error             `json:"last_error,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TaskHandler 任务处理器
type TaskHandler func(ctx context.Context, task *Task) error

// Scheduler 调度器
type Scheduler struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	running  bool
	stopChan chan struct{}
	handlers map[string]TaskHandler
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	MaxConcurrent int           `yaml:"max_concurrent"`
	CheckInterval time.Duration `yaml:"check_interval"`
}

// NewScheduler 创建调度器
func NewScheduler(cfg SchedulerConfig) *Scheduler {
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 10
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 1 * time.Second
	}

	return &Scheduler{
		tasks:    make(map[string]*Task),
		stopChan: make(chan struct{}),
		handlers: make(map[string]TaskHandler),
	}
}

// AddTask 添加任务
func (s *Scheduler) AddTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task already exists: %s", task.ID)
	}

	// 计算下次运行时间
	if err := s.calculateNextRun(task); err != nil {
		return fmt.Errorf("failed to calculate next run: %w", err)
	}

	s.tasks[task.ID] = task
	return nil
}

// RemoveTask 移除任务
func (s *Scheduler) RemoveTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	delete(s.tasks, id)
	return nil
}

// UpdateTask 更新任务
func (s *Scheduler) UpdateTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; !exists {
		return fmt.Errorf("task not found: %s", task.ID)
	}

	// 重新计算下次运行时间
	if err := s.calculateNextRun(task); err != nil {
		return fmt.Errorf("failed to calculate next run: %w", err)
	}

	s.tasks[task.ID] = task
	return nil
}

// GetTask 获取任务
func (s *Scheduler) GetTask(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	return task, nil
}

// ListTasks 列出任务
func (s *Scheduler) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// Start 启动调度器
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler is already running")
	}
	s.running = true
	s.mu.Unlock()

	go s.run(ctx)

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("scheduler is not running")
	}

	close(s.stopChan)
	s.running = false

	return nil
}

// IsRunning 检查是否运行中
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.running
}

// run 运行调度器
func (s *Scheduler) run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.checkAndRun(ctx)
		}
	}
}

// checkAndRun 检查并运行任务
func (s *Scheduler) checkAndRun(ctx context.Context) {
	s.mu.Lock()
	now := time.Now()
	tasksToRun := []*Task{}

	for _, task := range s.tasks {
		if !task.Enabled {
			continue
		}

		if now.After(task.NextRun) || now.Equal(task.NextRun) {
			tasksToRun = append(tasksToRun, task)
		}
	}
	s.mu.Unlock()

	// 运行任务
	for _, task := range tasksToRun {
		go s.executeTask(ctx, task)
	}
}

// executeTask 执行任务
func (s *Scheduler) executeTask(ctx context.Context, task *Task) {
	s.mu.Lock()
	task.LastRun = time.Now()
	task.RunCount++
	s.mu.Unlock()

	// 执行任务处理器
	var err error
	if task.Handler != nil {
		err = task.Handler(ctx, task)
	}

	s.mu.Lock()
	if err != nil {
		task.LastError = err
	} else {
		task.LastError = nil
	}

	// 计算下次运行时间
	if err := s.calculateNextRun(task); err != nil {
		task.LastError = err
	}
	s.mu.Unlock()
}

// calculateNextRun 计算下次运行时间
func (s *Scheduler) calculateNextRun(task *Task) error {
	now := time.Now()

	switch task.Type {
	case ScheduleOnce:
		if task.LastRun.IsZero() {
			task.NextRun = now
		} else {
			task.NextRun = time.Time{} // 不再运行
		}

	case ScheduleInterval:
		duration, err := time.ParseDuration(task.Schedule)
		if err != nil {
			return fmt.Errorf("invalid interval: %w", err)
		}
		if task.LastRun.IsZero() {
			task.NextRun = now.Add(duration)
		} else {
			task.NextRun = task.LastRun.Add(duration)
		}

	case ScheduleDaily:
		// 解析时间 (HH:MM)
		t, err := time.Parse("15:04", task.Schedule)
		if err != nil {
			return fmt.Errorf("invalid daily schedule: %w", err)
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		task.NextRun = next

	case ScheduleWeekly:
		// 解析 (weekday HH:MM)
		// 简化实现
		task.NextRun = now.Add(7 * 24 * time.Hour)

	case ScheduleMonthly:
		// 解析 (day HH:MM)
		// 简化实现
		task.NextRun = now.Add(30 * 24 * time.Hour)

	default:
		return fmt.Errorf("unsupported schedule type: %s", task.Type)
	}

	return nil
}

// GetTaskStatus 获取任务状态
func (s *Scheduler) GetTaskStatus(id string) (*TaskStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	return &TaskStatus{
		ID:        task.ID,
		Name:      task.Name,
		Enabled:   task.Enabled,
		LastRun:   task.LastRun,
		NextRun:   task.NextRun,
		RunCount:  task.RunCount,
		LastError: task.LastError,
	}, nil
}

// TaskStatus 任务状态
type TaskStatus struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	LastRun   time.Time `json:"last_run,omitempty"`
	NextRun   time.Time `json:"next_run,omitempty"`
	RunCount  int       `json:"run_count"`
	LastError error     `json:"last_error,omitempty"`
}

// EnableTask 启用任务
func (s *Scheduler) EnableTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.Enabled = true
	return nil
}

// DisableTask 禁用任务
func (s *Scheduler) DisableTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.Enabled = false
	return nil
}

// RunTaskNow 立即运行任务
func (s *Scheduler) RunTaskNow(ctx context.Context, id string) error {
	s.mu.RLock()
	task, exists := s.tasks[id]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	go s.executeTask(ctx, task)
	return nil
}

// GetStats 获取统计信息
func (s *Scheduler) GetStats() *SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &SchedulerStats{
		TotalTasks: len(s.tasks),
	}

	for _, task := range s.tasks {
		if task.Enabled {
			stats.EnabledTasks++
		}
		if task.LastError != nil {
			stats.FailedTasks++
		}
		stats.TotalRuns += task.RunCount
	}

	return stats
}

// SchedulerStats 调度器统计
type SchedulerStats struct {
	TotalTasks   int `json:"total_tasks"`
	EnabledTasks int `json:"enabled_tasks"`
	FailedTasks  int `json:"failed_tasks"`
	TotalRuns    int `json:"total_runs"`
}
