package core

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
)

// ============================================================
// TaskManager 测试
// ============================================================

func TestTaskManager_CreateTask(t *testing.T) {
	store := &mockTaskStore{}
	executor := &Executor{engine: nil}
	tm := NewTaskManager(store, executor)

	task := models.Task{
		ID:          "test-task-1",
		Type:        "implement",
		Description: "Test task",
	}

	ctx := context.Background()
	state, err := tm.CreateTask(ctx, task)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if state == nil {
		t.Error("Expected state, got nil")
	}
	if state.Status != models.TaskStatusPending {
		t.Errorf("Expected status pending, got %s", state.Status)
	}
}

func TestTaskManager_CreateTask_Validation(t *testing.T) {
	store := &mockTaskStore{}
	executor := &Executor{engine: nil}
	tm := NewTaskManager(store, executor)

	// 缺少 ID
	task := models.Task{
		Type:        "implement",
		Description: "No ID",
	}

	ctx := context.Background()
	_, err := tm.CreateTask(ctx, task)
	if err == nil {
		t.Error("Expected error for missing task ID")
	}

	// 缺少 Type
	task = models.Task{
		ID:          "test-2",
		Description: "No type",
	}
	_, err = tm.CreateTask(ctx, task)
	if err == nil {
		t.Error("Expected error for missing task type")
	}

	// 缺少 Description
	task = models.Task{
		ID:   "test-3",
		Type: "implement",
	}
	_, err = tm.CreateTask(ctx, task)
	if err == nil {
		t.Error("Expected error for missing task description")
	}
}

func TestTaskManager_CancelTask(t *testing.T) {
	store := &mockTaskStore{}
	executor := &Executor{engine: nil}
	tm := NewTaskManager(store, executor)

	task := models.Task{
		ID:          "cancel-test",
		Type:        "implement",
		Description: "Will be cancelled",
	}

	ctx := context.Background()
	tm.CreateTask(ctx, task)

	err := tm.CancelTask(ctx, "cancel-test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	state, _ := tm.GetTask(ctx, "cancel-test")
	if state.Status != models.TaskStatusCancelled {
		t.Errorf("Expected cancelled status, got %s", state.Status)
	}
}

func TestTaskManager_CancelNonexistentTask(t *testing.T) {
	store := &mockTaskStore{}
	executor := &Executor{engine: nil}
	tm := NewTaskManager(store, executor)

	ctx := context.Background()
	err := tm.CancelTask(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent task")
	}
}

// ============================================================
// StateManager 测试
// ============================================================

func TestStateManager_CreateSession(t *testing.T) {
	store := &mockStateStore{}
	notifier := NewEventNotifier()
	sm := NewStateManager(store, notifier)

	ctx := context.Background()
	session, err := sm.CreateSession(ctx, "test")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if session == nil {
		t.Error("Expected session, got nil")
	}
	if session.Environment != "test" {
		t.Errorf("Expected environment test, got %s", session.Environment)
	}
}

func TestStateManager_UpdateState(t *testing.T) {
	store := &mockStateStore{}
	notifier := NewEventNotifier()
	sm := NewStateManager(store, notifier)

	ctx := context.Background()
	session, _ := sm.CreateSession(ctx, "test")

	// 添加任务
	taskData := []byte(`{"id":"task-1","type":"implement","description":"test"}`)
	err := sm.UpdateState(ctx, session.ID, models.StateUpdate{
		Type:   "add_task",
		Data:   taskData,
		Reason: "test add",
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	state, _ := sm.GetState(ctx, session.ID)
	if len(state.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(state.Tasks))
	}
}

func TestStateManager_EventNotification(t *testing.T) {
	store := &mockStateStore{}
	notifier := NewEventNotifier()
	sm := NewStateManager(store, notifier)

	// 订阅事件
	ch := notifier.Subscribe()

	ctx := context.Background()
	sm.CreateSession(ctx, "test")

	// 应该收到事件
	select {
	case event := <-ch:
		if event.Type != "session_created" {
			t.Errorf("Expected session_created, got %s", event.Type)
		}
	case <-time.After(time.Second):
		t.Error("Expected event notification, got timeout")
	}
}

// ============================================================
// Monitor 测试
// ============================================================

func TestMonitor_RecordTask(t *testing.T) {
	monitor := NewMonitor()

	result := models.Result{
		TaskID: "test-task",
		Status: models.TaskStatusCompleted,
		Metrics: models.Metrics{
			Duration:   time.Second,
			TokenCount: 100,
			ToolUses:   5,
		},
	}

	monitor.RecordTask(result)

	metrics := monitor.GetMetrics()
	if metrics.TotalTasks != 1 {
		t.Errorf("Expected total tasks 1, got %d", metrics.TotalTasks)
	}
	if metrics.SuccessTasks != 1 {
		t.Errorf("Expected success tasks 1, got %d", metrics.SuccessTasks)
	}
	if metrics.TotalTokens != 100 {
		t.Errorf("Expected total tokens 100, got %d", metrics.TotalTokens)
	}
}

func TestMonitor_RecordFailedTask(t *testing.T) {
	monitor := NewMonitor()

	monitor.RecordTask(models.Result{
		TaskID: "ok",
		Status: models.TaskStatusCompleted,
		Metrics: models.Metrics{Duration: time.Second},
	})
	monitor.RecordTask(models.Result{
		TaskID: "fail",
		Status: models.TaskStatusFailed,
		Metrics: models.Metrics{Duration: 2 * time.Second},
	})

	metrics := monitor.GetMetrics()
	if metrics.TotalTasks != 2 {
		t.Errorf("Expected 2 tasks, got %d", metrics.TotalTasks)
	}
	if metrics.SuccessTasks != 1 {
		t.Errorf("Expected 1 success, got %d", metrics.SuccessTasks)
	}
	if metrics.FailedTasks != 1 {
		t.Errorf("Expected 1 failed, got %d", metrics.FailedTasks)
	}
	if metrics.AverageDuration != 1500*time.Millisecond {
		t.Errorf("Expected avg duration 1.5s, got %v", metrics.AverageDuration)
	}
}

func TestMonitor_RecordFeedback(t *testing.T) {
	monitor := NewMonitor()

	monitor.RecordFeedback(&models.FeedbackResult{
		TaskID: "t1",
		Status: "passed",
	})
	monitor.RecordFeedback(&models.FeedbackResult{
		TaskID: "t2",
		Status: "violations_found",
	})

	metrics := monitor.GetMetrics()
	if metrics.TotalFeedback != 2 {
		t.Errorf("Expected 2 feedback, got %d", metrics.TotalFeedback)
	}
	if metrics.PassedFeedback != 1 {
		t.Errorf("Expected 1 passed, got %d", metrics.PassedFeedback)
	}
	if metrics.ViolatedFeedback != 1 {
		t.Errorf("Expected 1 violated, got %d", metrics.ViolatedFeedback)
	}
}

func TestMonitor_PrometheusExport(t *testing.T) {
	monitor := NewMonitor()
	monitor.RecordTask(models.Result{
		TaskID: "t1",
		Status: models.TaskStatusCompleted,
		Metrics: models.Metrics{Duration: time.Second, TokenCount: 50},
	})

	output := monitor.ExportPrometheus()

	// 检查关键指标存在
	if !containsStr(output, "harness_tasks_total 1") {
		t.Error("Expected harness_tasks_total 1 in output")
	}
	if !containsStr(output, "harness_tasks_success 1") {
		t.Error("Expected harness_tasks_success 1 in output")
	}
	if !containsStr(output, "harness_tokens_total 50") {
		t.Error("Expected harness_tokens_total 50 in output")
	}
	if !containsStr(output, "harness_uptime_seconds") {
		t.Error("Expected harness_uptime_seconds in output")
	}
}

func TestMonitor_GetFeedback(t *testing.T) {
	monitor := NewMonitor()

	monitor.RecordFeedback(&models.FeedbackResult{TaskID: "t1", Status: "passed"})
	monitor.RecordFeedback(&models.FeedbackResult{TaskID: "t2", Status: "fixed"})
	monitor.RecordFeedback(&models.FeedbackResult{TaskID: "t1", Status: "passed"})

	results := monitor.GetFeedback("t1")
	if len(results) != 2 {
		t.Errorf("Expected 2 feedback for t1, got %d", len(results))
	}
}

// ============================================================
// FeedbackLoop 测试
// ============================================================

func TestFeedbackLoop_NoViolations(t *testing.T) {
	monitor := NewMonitor()
	config := models.FeedbackConfig{
		MaxRetries: 3,
		RetryDelay: time.Second,
		AutoFix:    false,
	}
	loop := NewFeedbackLoop(config, monitor)

	result := models.Result{
		TaskID: "test",
		Status: models.TaskStatusCompleted,
	}

	ctx := context.Background()
	feedback, err := loop.Process(ctx, result)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if feedback.Status != "passed" {
		t.Errorf("Expected status passed, got %s", feedback.Status)
	}
}

func TestFeedbackLoop_WithValidator(t *testing.T) {
	monitor := NewMonitor()
	config := models.FeedbackConfig{
		MaxRetries: 3,
		RetryDelay: time.Second,
		AutoFix:    false,
	}
	loop := NewFeedbackLoop(config, monitor)

	// 添加一个总是返回违规的验证器
	loop.AddValidator(&mockValidator{
		name: "test-validator",
		violations: []models.Violation{
			{Rule: "test-rule", Severity: models.SeverityError, Message: "test violation"},
		},
	})

	result := models.Result{
		TaskID: "test",
		Status: models.TaskStatusCompleted,
	}

	ctx := context.Background()
	feedback, err := loop.Process(ctx, result)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if feedback.Status != "violations_found" {
		t.Errorf("Expected violations_found, got %s", feedback.Status)
	}
	if len(feedback.Violations) != 1 {
		t.Errorf("Expected 1 violation, got %d", len(feedback.Violations))
	}
}

func TestFeedbackLoop_AutoFix(t *testing.T) {
	monitor := NewMonitor()
	config := models.FeedbackConfig{
		MaxRetries: 3,
		RetryDelay: time.Second,
		AutoFix:    true,
	}
	loop := NewFeedbackLoop(config, monitor)

	// 添加可修复的验证器
	loop.AddValidator(&mockValidator{
		name: "fixable-validator",
		violations: []models.Violation{
			{Rule: "fix-me", Severity: models.SeverityWarning, Message: "fixable", Fixable: true},
		},
	})

	// 添加修复器
	loop.AddFixer(&mockFixer{
		name: "test-fixer",
		canFix: true,
		result: &models.FixResult{Success: true, Message: "fixed"},
	})

	result := models.Result{
		TaskID: "test",
		Status: models.TaskStatusCompleted,
	}

	ctx := context.Background()
	feedback, err := loop.Process(ctx, result)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if feedback.Status != "fixed" {
		t.Errorf("Expected fixed, got %s", feedback.Status)
	}
}

// ============================================================
// KnowledgeBase 测试
// ============================================================

func TestKnowledgeBase_AddAndSearch(t *testing.T) {
	store := &mockKnowledgeStore{}
	indexer := NewIndexer()
	kb := NewKnowledgeBase(store, indexer)

	ctx := context.Background()

	// 添加条目
	entry := models.KnowledgeEntry{
		ID:      "kb-1",
		Type:    "solution",
		Title:   "OAuth2 Authentication",
		Content: "How to implement OAuth2 in Go",
		Tags:    []string{"auth", "oauth", "go"},
	}

	err := kb.AddEntry(ctx, entry)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 搜索
	results, err := kb.Search(ctx, "OAuth2", 10)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].Title != "OAuth2 Authentication" {
		t.Errorf("Expected title 'OAuth2 Authentication', got '%s'", results[0].Title)
	}

	// 访问计数应增加
	if results[0].AccessCount != 1 {
		t.Errorf("Expected access count 1, got %d", results[0].AccessCount)
	}
}

func TestKnowledgeBase_UpdateEntry(t *testing.T) {
	store := &mockKnowledgeStore{}
	indexer := NewIndexer()
	kb := NewKnowledgeBase(store, indexer)

	ctx := context.Background()

	kb.AddEntry(ctx, models.KnowledgeEntry{
		ID:      "kb-2",
		Type:    "solution",
		Title:   "Old Title",
		Content: "Old content",
		Tags:    []string{"old"},
	})

	err := kb.UpdateEntry(ctx, "kb-2", models.KnowledgeUpdate{
		Title:   "New Title",
		Content: "New content",
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	entry, _ := kb.GetEntry(ctx, "kb-2")
	if entry.Title != "New Title" {
		t.Errorf("Expected 'New Title', got '%s'", entry.Title)
	}
}

func TestKnowledgeBase_DeleteEntry(t *testing.T) {
	store := &mockKnowledgeStore{}
	indexer := NewIndexer()
	kb := NewKnowledgeBase(store, indexer)

	ctx := context.Background()
	kb.AddEntry(ctx, models.KnowledgeEntry{
		ID:      "kb-3",
		Title:   "To Delete",
		Content: "Will be deleted",
	})

	err := kb.DeleteEntry(ctx, "kb-3")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	_, err = kb.GetEntry(ctx, "kb-3")
	if err == nil {
		t.Error("Expected error for deleted entry")
	}
}

func TestKnowledgeBase_ListEntries(t *testing.T) {
	store := &mockKnowledgeStore{}
	indexer := NewIndexer()
	kb := NewKnowledgeBase(store, indexer)

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		kb.AddEntry(ctx, models.KnowledgeEntry{
			ID:      "kb-list-" + string(rune('a'+i)),
			Title:   "Entry " + string(rune('a'+i)),
			Content: "Content",
		})
	}

	entries, err := kb.ListEntries(ctx, 0, 3)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
}

// ============================================================
// Mock 实现
// ============================================================

type mockTaskStore struct {
	tasks map[string]*models.TaskState
}

func (m *mockTaskStore) SaveTask(ctx context.Context, state *models.TaskState) error {
	if m.tasks == nil {
		m.tasks = make(map[string]*models.TaskState)
	}
	m.tasks[state.Task.ID] = state
	return nil
}
func (m *mockTaskStore) GetTask(ctx context.Context, id string) (*models.TaskState, error) {
	if m.tasks != nil {
		if t, ok := m.tasks[id]; ok {
			return t, nil
		}
	}
	return nil, nil
}
func (m *mockTaskStore) ListTasks(ctx context.Context, filter models.TaskFilter) ([]*models.TaskState, error) {
	var tasks []*models.TaskState
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	// 按优先级降序排序
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Task.Priority > tasks[j].Task.Priority
	})
	return tasks, nil
}
func (m *mockTaskStore) DeleteTask(ctx context.Context, id string) error {
	return nil
}
func (m *mockTaskStore) BatchSaveTasks(ctx context.Context, states []*models.TaskState) error {
	if m.tasks == nil {
		m.tasks = make(map[string]*models.TaskState)
	}
	for _, s := range states {
		m.tasks[s.Task.ID] = s
	}
	return nil
}

type mockStateStore struct {
	sessions map[string]*storage.Session
}

func (m *mockStateStore) SaveSession(ctx context.Context, session *storage.Session) error {
	if m.sessions == nil {
		m.sessions = make(map[string]*storage.Session)
	}
	m.sessions[session.ID] = session
	return nil
}
func (m *mockStateStore) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	if m.sessions != nil {
		if s, ok := m.sessions[id]; ok {
			return s, nil
		}
	}
	return nil, nil
}
func (m *mockStateStore) ListSessions(ctx context.Context) ([]*storage.Session, error) {
	return nil, nil
}
func (m *mockStateStore) DeleteSession(ctx context.Context, id string) error {
	return nil
}

type mockKnowledgeStore struct {
	entries map[string]*models.KnowledgeEntry
}

func (m *mockKnowledgeStore) SaveKnowledge(ctx context.Context, entry *models.KnowledgeEntry) error {
	if m.entries == nil {
		m.entries = make(map[string]*models.KnowledgeEntry)
	}
	m.entries[entry.ID] = entry
	return nil
}
func (m *mockKnowledgeStore) GetKnowledge(ctx context.Context, id string) (*models.KnowledgeEntry, error) {
	if m.entries != nil {
		if e, ok := m.entries[id]; ok {
			return e, nil
		}
	}
	return nil, nil
}
func (m *mockKnowledgeStore) ListKnowledge(ctx context.Context, offset, limit int) ([]*models.KnowledgeEntry, error) {
	return nil, nil
}
func (m *mockKnowledgeStore) DeleteKnowledge(ctx context.Context, id string) error {
	if m.entries != nil {
		delete(m.entries, id)
	}
	return nil
}
func (m *mockKnowledgeStore) SearchKnowledge(ctx context.Context, query string, limit int) ([]*models.KnowledgeEntry, error) {
	return nil, nil
}

type mockPatternStore struct{}

func (m *mockPatternStore) SavePattern(ctx context.Context, pattern *models.Pattern) error   { return nil }
func (m *mockPatternStore) GetPattern(ctx context.Context, id string) (*models.Pattern, error) { return nil, nil }
func (m *mockPatternStore) ListPatterns(ctx context.Context) ([]*models.Pattern, error)        { return nil, nil }
func (m *mockPatternStore) DeletePattern(ctx context.Context, id string) error                 { return nil }

type mockValidator struct {
	name       string
	violations []models.Violation
}

func (m *mockValidator) Name() string { return m.name }
func (m *mockValidator) Validate(ctx context.Context, result models.Result) ([]models.Violation, error) {
	return m.violations, nil
}

type mockFixer struct {
	name   string
	canFix bool
	result *models.FixResult
}

func (m *mockFixer) Name() string                    { return m.name }
func (m *mockFixer) CanFix(v models.Violation) bool   { return m.canFix }
func (m *mockFixer) Fix(ctx context.Context, v models.Violation) (*models.FixResult, error) {
	return m.result, nil
}

// 辅助函数
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
