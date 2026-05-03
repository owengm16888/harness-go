package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/harness-engineering/harness/models"
)

// ============================================================
// TaskManager 基准测试
// ============================================================

func BenchmarkTaskManager_CreateTask(b *testing.B) {
	store := &mockTaskStore{}
	executor := &Executor{engine: nil}
	tm := NewTaskManager(store, executor)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.CreateTask(ctx, models.Task{
			ID:          fmt.Sprintf("bench-%d", i),
			Type:        "implement",
			Description: "Benchmark task",
			Priority:    5,
		})
	}
}

func BenchmarkTaskManager_ListTasks(b *testing.B) {
	store := &mockTaskStore{}
	executor := &Executor{engine: nil}
	tm := NewTaskManager(store, executor)
	ctx := context.Background()

	// 预填充 1000 个任务
	for i := 0; i < 1000; i++ {
		tm.CreateTask(ctx, models.Task{
			ID:          fmt.Sprintf("bench-%d", i),
			Type:        "implement",
			Description: "Benchmark task",
			Priority:    i % 10,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.ListTasks(ctx, models.TaskFilter{})
	}
}

// ============================================================
// KnowledgeBase 基准测试
// ============================================================

func BenchmarkKnowledgeBase_Search(b *testing.B) {
	store := &mockKnowledgeStore{}
	indexer := NewIndexer()
	kb := NewKnowledgeBase(store, indexer)
	ctx := context.Background()

	// 预填充知识库
	topics := []string{
		"OAuth2 authentication implementation",
		"JWT token validation best practices",
		"Database connection pooling PostgreSQL",
		"Redis caching strategy for API",
		"Docker container deployment guide",
		"Kubernetes pod scaling configuration",
		"Go goroutine concurrency patterns",
		"REST API design principles",
		"GraphQL resolver implementation",
		"Microservice communication gRPC",
	}
	for i, topic := range topics {
		kb.AddEntry(ctx, models.KnowledgeEntry{
			ID:      fmt.Sprintf("kb-%d", i),
			Type:    "solution",
			Title:   topic,
			Content: "Detailed guide about " + topic,
			Tags:    []string{"go", "backend", "architecture"},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kb.Search(ctx, topics[i%len(topics)], 5)
	}
}

// ============================================================
// Monitor 基准测试
// ============================================================

func BenchmarkMonitor_RecordTask(b *testing.B) {
	monitor := NewMonitor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.RecordTask(models.Result{
			TaskID: fmt.Sprintf("bench-%d", i),
			Status: models.TaskStatusCompleted,
			Metrics: models.Metrics{
				Duration:   time.Millisecond,
				TokenCount: 100,
			},
		})
	}
}

func BenchmarkMonitor_ExportPrometheus(b *testing.B) {
	monitor := NewMonitor()
	for i := 0; i < 100; i++ {
		monitor.RecordTask(models.Result{
			TaskID: fmt.Sprintf("t-%d", i),
			Status: models.TaskStatusCompleted,
			Metrics: models.Metrics{Duration: time.Millisecond},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.ExportPrometheus()
	}
}

// ============================================================
// FeedbackLoop 基准测试
// ============================================================

func BenchmarkFeedbackLoop_Process(b *testing.B) {
	monitor := NewMonitor()
	loop := NewFeedbackLoop(models.FeedbackConfig{
		MaxRetries: 3,
		AutoFix:    false,
	}, monitor)

	loop.AddValidator(&mockValidator{
		name:       "bench-validator",
		violations: nil, // 无违规，最快路径
	})

	result := models.Result{
		TaskID: "bench",
		Status: models.TaskStatusCompleted,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loop.Process(ctx, result)
	}
}

// ============================================================
// Learner 基准测试
// ============================================================

func BenchmarkLearner_Observe(b *testing.B) {
	learner := NewLearner(5, 0.7)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		learner.Observe(ctx, models.Observation{
			Task: models.Task{
				ID:      fmt.Sprintf("t-%d", i),
				Type:    "implement",
				Context: map[string]any{"environment": "dev"},
			},
			Result:    models.Result{Metrics: models.Metrics{Duration: time.Second}},
			Success:   true,
			Timestamp: time.Now(),
		})
	}
}

func BenchmarkLearner_Predict(b *testing.B) {
	learner := NewLearner(3, 0.5)
	ctx := context.Background()

	// 预填充模式
	for i := 0; i < 50; i++ {
		learner.Observe(ctx, models.Observation{
			Task: models.Task{
				ID: fmt.Sprintf("t-%d", i), Type: "implement",
				Context: map[string]any{"environment": "dev"},
			},
			Result: models.Result{Metrics: models.Metrics{Duration: time.Second}},
			Success: true, Timestamp: time.Now(),
		})
	}

	task := models.Task{
		Type: "implement", Context: map[string]any{"environment": "dev"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		learner.Predict(ctx, task)
	}
}

// ============================================================
// Indexer 基准测试
// ============================================================

func BenchmarkIndexer_Search(b *testing.B) {
	indexer := NewIndexer()
	ctx := context.Background()

	// 预填充索引
	entries := []string{
		"OAuth2 authentication implementation guide",
		"JWT token validation with RS256",
		"Database connection pooling PostgreSQL",
		"Redis caching strategy for REST API",
		"Docker container deployment best practices",
	}
	for i, e := range entries {
		indexer.Index(ctx, &models.KnowledgeEntry{
			ID: fmt.Sprintf("k-%d", i), Title: e,
			Content: "Detailed guide about " + e,
			Tags:    []string{"go", "backend"},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indexer.Search(ctx, entries[i%len(entries)], 5)
	}
}

func BenchmarkIndexer_Tokenize(b *testing.B) {
	text := "This is a benchmark test for the tokenizer with various words including Go programming and database management"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenize(text)
	}
}

// ============================================================
// StateManager 基准测试
// ============================================================

func BenchmarkStateManager_CreateSession(b *testing.B) {
	store := &mockStateStore{}
	notifier := NewEventNotifier()
	sm := NewStateManager(store, notifier)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.CreateSession(ctx, "benchmark")
	}
}
