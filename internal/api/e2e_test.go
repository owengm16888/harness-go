package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/internal/core"
	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
)

// setupTestServer 创建测试服务器
func setupTestServer(t *testing.T) (*httptest.Server, *core.Engine) {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{Addr: ":0"},
		Engine: config.EngineConfig{
			MaxConcurrentTasks: 5,
			TaskTimeout:        60,
			RetryCount:         1,
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

	server := NewServer(cfg, engine)
	ts := httptest.NewServer(server.router)

	t.Cleanup(func() {
		ts.Close()
		store.Close()
	})

	return ts, engine
}

func TestE2E_HealthCheck(t *testing.T) {
	ts, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var health map[string]any
	json.NewDecoder(resp.Body).Decode(&health)

	if health["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", health["status"])
	}
}

func TestE2E_TaskLifecycle(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 1. 创建任务
	taskJSON := `{
		"id": "e2e-task-1",
		"type": "implement",
		"description": "Implement user authentication",
		"priority": 8
	}`
	resp, err := client.Post(ts.URL+"/api/v1/tasks", "application/json", bytes.NewBufferString(taskJSON))
	if err != nil {
		t.Fatalf("Create task failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// 2. 获取任务
	resp, err = client.Get(ts.URL + "/api/v1/tasks/e2e-task-1")
	if err != nil {
		t.Fatalf("Get task failed: %v", err)
	}
	defer resp.Body.Close()

	var taskState models.TaskState
	json.NewDecoder(resp.Body).Decode(&taskState)

	if taskState.Task.ID != "e2e-task-1" {
		t.Errorf("Expected task ID e2e-task-1, got %s", taskState.Task.ID)
	}

	// 3. 列出任务
	resp, err = client.Get(ts.URL + "/api/v1/tasks")
	if err != nil {
		t.Fatalf("List tasks failed: %v", err)
	}
	defer resp.Body.Close()

	var tasks []models.TaskState
	json.NewDecoder(resp.Body).Decode(&tasks)

	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
}

func TestE2E_KnowledgeWorkflow(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 添加知识
	entryJSON := `{
		"id": "kb-e2e-1",
		"type": "solution",
		"title": "OAuth2 Implementation",
		"content": "How to implement OAuth2 with PKCE",
		"tags": ["auth", "oauth"]
	}`
	resp, err := client.Post(ts.URL+"/api/v1/knowledge", "application/json", bytes.NewBufferString(entryJSON))
	if err != nil {
		t.Fatalf("Add knowledge failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}

	// 搜索知识
	resp, err = client.Get(ts.URL + "/api/v1/knowledge/search?q=OAuth2")
	if err != nil {
		t.Fatalf("Search knowledge failed: %v", err)
	}
	defer resp.Body.Close()

	var entries []models.KnowledgeEntry
	json.NewDecoder(resp.Body).Decode(&entries)

	if len(entries) != 1 {
		t.Errorf("Expected 1 result, got %d", len(entries))
	}
}

func TestE2E_AgentRegistration(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 注册 Agent
	agentJSON := `{
		"name": "Claude Worker",
		"adapter": "claude-code",
		"role": "worker",
		"capabilities": [
			{"name": "implement", "confidence": 0.9},
			{"name": "review", "confidence": 0.8}
		]
	}`
	resp, err := client.Post(ts.URL+"/api/agents", "application/json", bytes.NewBufferString(agentJSON))
	if err != nil {
		t.Fatalf("Register agent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}

	var agent models.Agent
	json.NewDecoder(resp.Body).Decode(&agent)

	if agent.Name != "Claude Worker" {
		t.Errorf("Expected name 'Claude Worker', got '%s'", agent.Name)
	}

	// 列出 Agent
	resp, err = client.Get(ts.URL + "/api/agents")
	if err != nil {
		t.Fatalf("List agents failed: %v", err)
	}
	defer resp.Body.Close()

	var agents []models.Agent
	json.NewDecoder(resp.Body).Decode(&agents)

	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}
}

func TestE2E_MetricsEndpoint(t *testing.T) {
	ts, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("Metrics endpoint failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// 检查 Prometheus 格式
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	output := buf.String()

	if len(output) == 0 {
		t.Error("Expected non-empty metrics output")
	}
}

func TestE2E_Unauthorized(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Addr: ":0", APIKey: "secret-key"},
		Engine: config.EngineConfig{MaxConcurrentTasks: 5},
		Patterns: config.PatternsConfig{MinSamples: 3, Threshold: 0.7},
		Feedback: config.FeedbackConfig{MaxRetries: 2},
	}

	store, _ := storage.NewSQLiteStorage(storage.DefaultSQLiteConfig(":memory:"))
	engine, _ := core.NewEngine(cfg, store)
	server := NewServer(cfg, engine)
	ts := httptest.NewServer(server.router)
	defer ts.Close()
	defer store.Close()

	client := ts.Client()

	// 无 API key → 401
	resp, err := client.Get(ts.URL + "/api/tasks")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}

	// 正确 API key → 200
	req, _ := http.NewRequest("GET", ts.URL+"/api/tasks", nil)
	req.Header.Set("X-API-Key", "secret-key")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}
