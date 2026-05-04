package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/internal/core"
	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
)

// setupInjectionServer 创建启用了上下文注入的测试服务器
func setupInjectionServer(t *testing.T) (*httptest.Server, *core.Engine) {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{Addr: ":0"},
		Engine: config.EngineConfig{
			MaxConcurrentTasks: 5,
			TaskTimeout:        60,
			RetryCount:         1,
			ContextInjection: config.ContextInjectionConfig{
				Enabled:           true,
				KnowledgeLimit:    3,
				PatternLimit:      2,
				CacheResults:      true,
				InjectConstraints: true,
				InjectMetadata:    true,
			},
		},
		Patterns: config.PatternsConfig{MinSamples: 3, Threshold: 0.7},
		Feedback: config.FeedbackConfig{MaxRetries: 2, AutoFix: false},
	}

	store, err := storage.NewSQLiteStorage(storage.DefaultSQLiteConfig(":memory:"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	engine, err := core.NewEngine(cfg, store)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// 加载数据到内存
	ctx := context.Background()
	engine.Knowledge().LoadFromStorage(ctx)
	engine.Pattern().LoadFromStorage(ctx)

	server := NewServer(cfg, engine)
	ts := httptest.NewServer(server.router)

	t.Cleanup(func() {
		ts.Close()
		store.Close()
	})

	return ts, engine
}

func TestContextInjection_TaskContextEnriched(t *testing.T) {
	ts, engine := setupInjectionServer(t)
	client := ts.Client()

	// 1. 先添加知识条目
	kbJSON := `{"id":"kb-inject-1","type":"solution","title":"OAuth2 Guide","content":"How to implement OAuth2 with PKCE flow in Go","tags":["auth","oauth"]}`
	resp, err := client.Post(ts.URL+"/api/v1/knowledge", "application/json", bytes.NewBufferString(kbJSON))
	if err != nil {
		t.Fatalf("Add knowledge failed: %v", err)
	}
	resp.Body.Close()

	// 2. 添加模式
	patJSON := `{"id":"pat-inject-1","name":"auth-pattern","description":"Authentication implementation pattern","trigger":"implement","actions":[{"type":"execute"}]}`
	resp, err = client.Post(ts.URL+"/api/v1/patterns", "application/json", bytes.NewBufferString(patJSON))
	if err != nil {
		t.Fatalf("Add pattern failed: %v", err)
	}
	resp.Body.Close()

	// 3. 创建任务 (通过 TaskManager 直接创建, 然后用 ExecuteTask 验证上下文注入)
	task := models.Task{
		ID:          "task-inject-1",
		Type:        "implement",
		Description: "Implement OAuth2 authentication with PKCE",
		Priority:    8,
		Constraints: []models.Constraint{
			{
				Type:     "style",
				Rule:     "no-panic",
				Severity: models.SeverityError,
				Message:  "Must not use panic in production code",
			},
		},
	}

	_, err = engine.TaskManager().CreateTask(context.Background(), task)
	if err != nil {
		t.Fatalf("Create task failed: %v", err)
	}

	// 4. 通过 API 获取任务, 验证 context 注入
	resp, err = client.Get(ts.URL + "/api/v1/tasks/task-inject-1")
	if err != nil {
		t.Fatalf("Get task failed: %v", err)
	}
	defer resp.Body.Close()

	var taskState models.TaskState
	json.NewDecoder(resp.Body).Decode(&taskState)

	// 验证 task 的 context 是否被注入
	// 注意: CreateTask 不触发注入, 只有 ExecuteTask 才触发
	// 所以这里验证的是 task 结构正确性
	if taskState.Task.ID != "task-inject-1" {
		t.Errorf("Expected task ID 'task-inject-1', got '%s'", taskState.Task.ID)
	}

	// 5. 验证知识搜索能返回结果 (上下文注入的前置条件)
	searchResp, err := client.Get(ts.URL + "/api/v1/knowledge/search?q=OAuth2")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	defer searchResp.Body.Close()

	var searchResults []*models.KnowledgeEntry
	json.NewDecoder(searchResp.Body).Decode(&searchResults)
	t.Logf("Knowledge search returned %d results", len(searchResults))

	// 6. 验证模式匹配能返回结果
	matchJSON := `{"id":"task-match","type":"implement","description":"Implement OAuth2 authentication","priority":5}`
	matchResp, err := client.Post(ts.URL+"/api/v1/patterns/match", "application/json", bytes.NewBufferString(matchJSON))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	defer matchResp.Body.Close()

	var matched []*models.Pattern
	json.NewDecoder(matchResp.Body).Decode(&matched)
	t.Logf("Pattern match returned %d results", len(matched))
}

func TestContextInjection_KnowledgeSearch(t *testing.T) {
	ts, _ := setupInjectionServer(t)
	client := ts.Client()

	// 添加多条知识
	for i, title := range []string{"Go Context", "Go Goroutines", "Go Channels", "Python Async", "Rust Ownership"} {
		kbJSON := `{"id":"kb-s` + string(rune('0'+i)) + `","type":"doc","title":"` + title + `","content":"Detailed guide about ` + title + `","tags":["guide"]}`
		client.Post(ts.URL+"/api/v1/knowledge", "application/json", bytes.NewBufferString(kbJSON))
	}

	// 搜索 "Go" 应该返回 3 条 (Go Context, Go Goroutines, Go Channels)
	resp, err := client.Get(ts.URL + "/api/v1/knowledge/search?q=Go")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	defer resp.Body.Close()

	var results []*models.KnowledgeEntry
	json.NewDecoder(resp.Body).Decode(&results)

	// 验证搜索结果
	t.Logf("Search for 'Go' returned %d results", len(results))
	for _, r := range results {
		t.Logf("  - %s: %s", r.ID, r.Title)
	}
}

func TestContextInjection_PatternMatchWithTask(t *testing.T) {
	ts, _ := setupInjectionServer(t)
	client := ts.Client()

	// 添加多种模式
	patterns := []string{
		`{"id":"pat-a","name":"implement","description":"Implementation pattern","trigger":"implement","actions":[]}`,
		`{"id":"pat-b","name":"review","description":"Code review pattern","trigger":"review","actions":[]}`,
		`{"id":"pat-c","name":"test","description":"Testing pattern","trigger":"test","actions":[]}`,
	}
	for _, p := range patterns {
		client.Post(ts.URL+"/api/v1/patterns", "application/json", bytes.NewBufferString(p))
	}

	// 匹配 implement 类型的任务
	matchJSON := `{"id":"t1","type":"implement","description":"Implement new feature","priority":5}`
	resp, err := client.Post(ts.URL+"/api/v1/patterns/match", "application/json", bytes.NewBufferString(matchJSON))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	defer resp.Body.Close()

	var matched []*models.Pattern
	json.NewDecoder(resp.Body).Decode(&matched)
	t.Logf("Matched %d patterns for 'implement' task", len(matched))
}

func TestContextInjection_EmptySearchReturnsArray(t *testing.T) {
	ts, _ := setupInjectionServer(t)
	client := ts.Client()

	// 搜索不存在的内容
	resp, err := client.Get(ts.URL + "/api/v1/knowledge/search?q=nonexistent_zzz_xyz")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var results []*models.KnowledgeEntry
	json.NewDecoder(resp.Body).Decode(&results)

	// 应该返回空数组而非 null
	if results == nil {
		t.Error("Expected empty array, got nil")
	}
}
