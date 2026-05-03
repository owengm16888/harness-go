package core

import (
	"context"
	"testing"
	"time"

	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
)

// ============================================================
// 端到端集成测试
// ============================================================

func TestIntegration_TaskLifecycle(t *testing.T) {
	// 创建完整的引擎（使用 mock 存储）
	store := &integrationStore{
		taskStore:    &mockTaskStore{},
		stateStore:   &mockStateStore{},
		knowledgeStore: &mockKnowledgeStore{},
		patternStore: &mockPatternStore{},
	}

	engine := &Engine{
		adapters: make(map[string]Adapter),
		storage:  store,
		monitor:  NewMonitor(),
	}

	// 初始化组件
	engine.taskManager = NewTaskManager(store.taskStore, NewExecutor(engine))
	engine.stateManager = NewStateManager(store.stateStore, NewEventNotifier())
	engine.feedbackLoop = NewFeedbackLoop(models.FeedbackConfig{
		MaxRetries: 3,
		RetryDelay: time.Second,
		AutoFix:    false,
	}, engine.monitor)
	engine.knowledge = NewKnowledgeBase(store.knowledgeStore, NewIndexer())
	engine.pattern = NewPatternEngine(store.patternStore, 5, 0.7)
	engine.semaphore = make(chan struct{}, 10)

	ctx := context.Background()

	// 1. 创建任务
	task := models.Task{
		ID:          "integration-task-1",
		Type:        "implement",
		Description: "Implement user authentication",
		Priority:    8,
		Constraints: []models.Constraint{
			{Type: "security", Rule: "no-hardcoded-secrets", Severity: models.SeverityError, Message: "No secrets"},
		},
		Context: map[string]any{
			"environment": "production",
		},
	}

	state, err := engine.TaskManager().CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if state.Status != models.TaskStatusPending {
		t.Errorf("Expected pending, got %s", state.Status)
	}

	// 2. 列出任务
	tasks, err := engine.TaskManager().ListTasks(ctx, models.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	// 3. 获取任务
	got, err := engine.TaskManager().GetTask(ctx, "integration-task-1")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Task.Description != task.Description {
		t.Errorf("Description mismatch")
	}
}

func TestIntegration_KnowledgeWorkflow(t *testing.T) {
	store := &mockKnowledgeStore{}
	indexer := NewIndexer()
	kb := NewKnowledgeBase(store, indexer)
	ctx := context.Background()

	// 添加知识条目
	entries := []models.KnowledgeEntry{
		{ID: "k1", Type: "solution", Title: "OAuth2 Implementation", Content: "How to implement OAuth2 in Go with PKCE flow", Tags: []string{"auth", "oauth", "go"}},
		{ID: "k2", Type: "solution", Title: "JWT Token Validation", Content: "Validate JWT tokens using RS256 algorithm", Tags: []string{"auth", "jwt", "security"}},
		{ID: "k3", Type: "pattern", Title: "Database Connection Pool", Content: "Configure connection pooling for PostgreSQL", Tags: []string{"database", "postgres", "performance"}},
	}

	for _, e := range entries {
		if err := kb.AddEntry(ctx, e); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// 搜索 "auth"
	results, err := kb.Search(ctx, "auth", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) < 2 {
		t.Errorf("Expected at least 2 results for 'auth', got %d", len(results))
	}

	// 搜索 "database"
	results, err = kb.Search(ctx, "database", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'database', got %d", len(results))
	}

	// 更新知识
	err = kb.UpdateEntry(ctx, "k1", models.KnowledgeUpdate{
		Content: "Updated OAuth2 implementation with PKCE and refresh tokens",
	})
	if err != nil {
		t.Fatalf("UpdateEntry failed: %v", err)
	}

	entry, _ := kb.GetEntry(ctx, "k1")
	if entry.Content != "Updated OAuth2 implementation with PKCE and refresh tokens" {
		t.Errorf("Update not applied")
	}

	// 删除知识
	err = kb.DeleteEntry(ctx, "k3")
	if err != nil {
		t.Fatalf("DeleteEntry failed: %v", err)
	}

	_, err = kb.GetEntry(ctx, "k3")
	if err == nil {
		t.Error("Expected error for deleted entry")
	}
}

func TestIntegration_FeedbackWithAutoFix(t *testing.T) {
	monitor := NewMonitor()
	loop := NewFeedbackLoop(models.FeedbackConfig{
		MaxRetries: 3,
		RetryDelay: time.Millisecond,
		AutoFix:    true,
	}, monitor)

	// 注册安全验证器
	loop.AddValidator(&mockValidator{
		name: "security-scanner",
		violations: []models.Violation{
			{Rule: "no-hardcoded-secrets", Severity: models.SeverityError, Message: "Found API key in code", Fixable: true},
		},
	})

	// 注册修复器
	loop.AddFixer(&mockFixer{
		name:   "secret-replacer",
		canFix: true,
		result: &models.FixResult{
			Success: true,
			Message: "Replaced hardcoded key with env var",
			Changes: []models.Change{
				{File: "config.go", Line: 42, Before: "apiKey = \"sk-123\"", After: "apiKey = os.Getenv(\"API_KEY\")"},
			},
		},
	})

	result := models.Result{
		TaskID: "security-task",
		Status: models.TaskStatusCompleted,
		Output: "Generated code with hardcoded secret",
	}

	ctx := context.Background()
	feedback, err := loop.Process(ctx, result)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if feedback.Status != "fixed" {
		t.Errorf("Expected fixed, got %s", feedback.Status)
	}
	if len(feedback.Fixes) != 1 {
		t.Errorf("Expected 1 fix, got %d", len(feedback.Fixes))
	}
	if !feedback.Fixes[0].Success {
		t.Error("Expected fix to succeed")
	}

	// 检查指标
	metrics := monitor.GetMetrics()
	if metrics.TotalFeedback != 1 {
		t.Errorf("Expected 1 feedback recorded, got %d", metrics.TotalFeedback)
	}
	if metrics.FixedFeedback != 1 {
		t.Errorf("Expected 1 fixed, got %d", metrics.FixedFeedback)
	}
}

func TestIntegration_MonitorMetrics(t *testing.T) {
	monitor := NewMonitor()

	// 模拟一系列任务
	for i := 0; i < 10; i++ {
		status := models.TaskStatusCompleted
		if i%3 == 0 {
			status = models.TaskStatusFailed
		}
		monitor.RecordTask(models.Result{
			TaskID: "task-" + string(rune('a'+i)),
			Status: status,
			Metrics: models.Metrics{
				Duration:   time.Duration(i+1) * 100 * time.Millisecond,
				TokenCount: (i + 1) * 100,
				ToolUses:   i + 1,
			},
		})
	}

	metrics := monitor.GetMetrics()

	if metrics.TotalTasks != 10 {
		t.Errorf("Expected 10 tasks, got %d", metrics.TotalTasks)
	}
	if metrics.SuccessTasks != 7 {
		t.Errorf("Expected 7 success, got %d", metrics.SuccessTasks)
	}
	if metrics.FailedTasks != 3 {
		t.Errorf("Expected 3 failed, got %d", metrics.FailedTasks)
	}
	if metrics.TotalTokens != 5500 { // sum(100..1000)
		t.Errorf("Expected 5500 tokens, got %d", metrics.TotalTokens)
	}

	// 检查 Prometheus 输出
	prom := monitor.ExportPrometheus()
	if !containsStr(prom, "harness_tasks_total 10") {
		t.Error("Expected harness_tasks_total 10")
	}
	if !containsStr(prom, "harness_tokens_total 5500") {
		t.Error("Expected harness_tokens_total 5500")
	}
}

func TestIntegration_PriorityOrdering(t *testing.T) {
	store := &mockTaskStore{}
	executor := &Executor{engine: nil, defaultAdapter: "test"}
	tm := NewTaskManager(store, executor)
	ctx := context.Background()

	// 创建不同优先级的任务
	priorities := []int{3, 10, 1, 7, 5}
	for i, p := range priorities {
		tm.CreateTask(ctx, models.Task{
			ID:          "priority-" + string(rune('a'+i)),
			Type:        "implement",
			Description: "Task",
			Priority:    p,
		})
	}

	// 列出任务 — 应该按优先级降序
	tasks, _ := tm.ListTasks(ctx, models.TaskFilter{})
	if len(tasks) != 5 {
		t.Fatalf("Expected 5 tasks, got %d", len(tasks))
	}

	expectedOrder := []int{10, 7, 5, 3, 1}
	for i, task := range tasks {
		if task.Task.Priority != expectedOrder[i] {
			t.Errorf("Position %d: expected priority %d, got %d", i, expectedOrder[i], task.Task.Priority)
		}
	}
}

// ============================================================
// 集成测试用的组合存储
// ============================================================

type integrationStore struct {
	taskStore      *mockTaskStore
	stateStore     *mockStateStore
	knowledgeStore *mockKnowledgeStore
	patternStore   *mockPatternStore
}

func (s *integrationStore) SaveTask(ctx context.Context, state *models.TaskState) error {
	return s.taskStore.SaveTask(ctx, state)
}
func (s *integrationStore) GetTask(ctx context.Context, id string) (*models.TaskState, error) {
	return s.taskStore.GetTask(ctx, id)
}
func (s *integrationStore) ListTasks(ctx context.Context, filter models.TaskFilter) ([]*models.TaskState, error) {
	return s.taskStore.ListTasks(ctx, filter)
}
func (s *integrationStore) DeleteTask(ctx context.Context, id string) error {
	return s.taskStore.DeleteTask(ctx, id)
}
func (s *integrationStore) SaveSession(ctx context.Context, session *Session) error {
	return s.stateStore.SaveSession(ctx, session)
}
func (s *integrationStore) GetSession(ctx context.Context, id string) (*Session, error) {
	return s.stateStore.GetSession(ctx, id)
}
func (s *integrationStore) ListSessions(ctx context.Context) ([]*Session, error) {
	return s.stateStore.ListSessions(ctx)
}
func (s *integrationStore) DeleteSession(ctx context.Context, id string) error {
	return s.stateStore.DeleteSession(ctx, id)
}
func (s *integrationStore) SaveKnowledge(ctx context.Context, entry *models.KnowledgeEntry) error {
	return s.knowledgeStore.SaveKnowledge(ctx, entry)
}
func (s *integrationStore) GetKnowledge(ctx context.Context, id string) (*models.KnowledgeEntry, error) {
	return s.knowledgeStore.GetKnowledge(ctx, id)
}
func (s *integrationStore) ListKnowledge(ctx context.Context, offset, limit int) ([]*models.KnowledgeEntry, error) {
	return s.knowledgeStore.ListKnowledge(ctx, offset, limit)
}
func (s *integrationStore) DeleteKnowledge(ctx context.Context, id string) error {
	return s.knowledgeStore.DeleteKnowledge(ctx, id)
}
func (s *integrationStore) SearchKnowledge(ctx context.Context, query string, limit int) ([]*models.KnowledgeEntry, error) {
	return s.knowledgeStore.SearchKnowledge(ctx, query, limit)
}
func (s *integrationStore) SavePattern(ctx context.Context, pattern *models.Pattern) error {
	return s.patternStore.SavePattern(ctx, pattern)
}
func (s *integrationStore) GetPattern(ctx context.Context, id string) (*models.Pattern, error) {
	return s.patternStore.GetPattern(ctx, id)
}
func (s *integrationStore) ListPatterns(ctx context.Context) ([]*models.Pattern, error) {
	return s.patternStore.ListPatterns(ctx)
}
func (s *integrationStore) DeletePattern(ctx context.Context, id string) error {
	return s.patternStore.DeletePattern(ctx, id)
}
func (s *integrationStore) Close() error { return nil }
