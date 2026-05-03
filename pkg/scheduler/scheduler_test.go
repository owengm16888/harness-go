package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestScheduler_AddTask(t *testing.T) {
	scheduler := NewScheduler(SchedulerConfig{})

	task := &Task{
		ID:       "test-1",
		Name:     "Test Task",
		Type:     ScheduleOnce,
		Schedule: "",
		Handler: func(ctx context.Context, task *Task) error {
			return nil
		},
		Enabled: true,
	}

	err := scheduler.AddTask(task)
	if err != nil {
		t.Fatalf("Failed to add task: %v", err)
	}

	tasks := scheduler.ListTasks()
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
}

func TestScheduler_RemoveTask(t *testing.T) {
	scheduler := NewScheduler(SchedulerConfig{})

	task := &Task{
		ID:       "test-1",
		Name:     "Test Task",
		Type:     ScheduleOnce,
		Enabled:  true,
	}

	scheduler.AddTask(task)

	err := scheduler.RemoveTask("test-1")
	if err != nil {
		t.Fatalf("Failed to remove task: %v", err)
	}

	tasks := scheduler.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks, got %d", len(tasks))
	}
}

func TestScheduler_GetTask(t *testing.T) {
	scheduler := NewScheduler(SchedulerConfig{})

	task := &Task{
		ID:       "test-1",
		Name:     "Test Task",
		Type:     ScheduleOnce,
		Enabled:  true,
	}

	scheduler.AddTask(task)

	retrieved, err := scheduler.GetTask("test-1")
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.ID != "test-1" {
		t.Errorf("Expected task ID test-1, got %s", retrieved.ID)
	}
}

func TestScheduler_EnableDisableTask(t *testing.T) {
	scheduler := NewScheduler(SchedulerConfig{})

	task := &Task{
		ID:       "test-1",
		Name:     "Test Task",
		Type:     ScheduleOnce,
		Enabled:  true,
	}

	scheduler.AddTask(task)

	// 禁用任务
	err := scheduler.DisableTask("test-1")
	if err != nil {
		t.Fatalf("Failed to disable task: %v", err)
	}

	retrieved, _ := scheduler.GetTask("test-1")
	if retrieved.Enabled {
		t.Error("Expected task to be disabled")
	}

	// 启用任务
	err = scheduler.EnableTask("test-1")
	if err != nil {
		t.Fatalf("Failed to enable task: %v", err)
	}

	retrieved, _ = scheduler.GetTask("test-1")
	if !retrieved.Enabled {
		t.Error("Expected task to be enabled")
	}
}

func TestScheduler_StartStop(t *testing.T) {
	scheduler := NewScheduler(SchedulerConfig{})

	ctx := context.Background()

	// 启动
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("Expected scheduler to be running")
	}

	// 停止
	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	if scheduler.IsRunning() {
		t.Error("Expected scheduler to be stopped")
	}
}

func TestScheduler_RunTaskNow(t *testing.T) {
	scheduler := NewScheduler(SchedulerConfig{})

	executed := false
	task := &Task{
		ID:       "test-1",
		Name:     "Test Task",
		Type:     ScheduleOnce,
		Enabled:  true,
		Handler: func(ctx context.Context, task *Task) error {
			executed = true
			return nil
		},
	}

	scheduler.AddTask(task)

	ctx := context.Background()
	err := scheduler.RunTaskNow(ctx, "test-1")
	if err != nil {
		t.Fatalf("Failed to run task: %v", err)
	}

	// 等待任务执行
	time.Sleep(100 * time.Millisecond)

	if !executed {
		t.Error("Expected task to be executed")
	}
}

func TestScheduler_GetStats(t *testing.T) {
	scheduler := NewScheduler(SchedulerConfig{})

	task1 := &Task{
		ID:       "test-1",
		Name:     "Test Task 1",
		Type:     ScheduleOnce,
		Enabled:  true,
	}

	task2 := &Task{
		ID:       "test-2",
		Name:     "Test Task 2",
		Type:     ScheduleOnce,
		Enabled:  false,
	}

	scheduler.AddTask(task1)
	scheduler.AddTask(task2)

	stats := scheduler.GetStats()

	if stats.TotalTasks != 2 {
		t.Errorf("Expected 2 total tasks, got %d", stats.TotalTasks)
	}

	if stats.EnabledTasks != 1 {
		t.Errorf("Expected 1 enabled task, got %d", stats.EnabledTasks)
	}
}
