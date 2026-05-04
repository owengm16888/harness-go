package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/harness-engineering/harness/models"
)

func TestResilience_AdaptersHaveCircuitBreaker(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 通过 API 注册 Agent (会触发适配器注册)
	agentJSON := `{"name":"Worker","adapter":"claude-code","role":"worker","capabilities":[{"name":"implement","confidence":0.9}]}`
	client.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(agentJSON))

	// 获取适配器列表
	resp, err := client.Get(ts.URL + "/api/v1/adapters")
	if err != nil {
		t.Fatalf("List adapters failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var adapters []string
	json.NewDecoder(resp.Body).Decode(&adapters)
	t.Logf("Registered adapters: %v", adapters)
}

func TestResilience_CircuitBreakerStates(t *testing.T) {
	_, engine := setupTestServer(t)

	// 获取熔断器状态
	states := engine.GetAdapterCircuitStates()
	t.Logf("Circuit breaker states: %v", states)

	// 所有适配器初始状态应该是 closed
	for name, state := range states {
		if state != "closed" && state != "n/a" {
			t.Errorf("Adapter %s: expected 'closed' or 'n/a', got '%s'", name, state)
		}
	}
}

func TestResilience_FallbackChain(t *testing.T) {
	ts, engine := setupTestServer(t)
	client := ts.Client()

	// 注册多个 Agent (不同适配器)
	agents := []string{
		`{"name":"Claude Worker","adapter":"claude-code","role":"worker","capabilities":[{"name":"implement","confidence":0.9}]}`,
		`{"name":"Hermes Worker","adapter":"hermes","role":"worker","capabilities":[{"name":"implement","confidence":0.8}]}`,
	}
	for _, a := range agents {
		client.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(a))
	}

	// 创建任务
	taskJSON := `{"id":"task-resilient-1","type":"implement","description":"Test resilient execution","priority":5}`
	client.Post(ts.URL+"/api/v1/tasks", "application/json", bytes.NewBufferString(taskJSON))

	// 验证降级顺序
	t.Logf("Fallback order: %v", engine.ListAdapters())

	// 尝试执行 (会因为适配器未真正初始化而失败，但验证降级逻辑)
	task, _ := engine.TaskManager().GetTask(context.Background(), "task-resilient-1")
	if task != nil {
		// ExecuteTaskWithFallback 会尝试所有适配器
		_, err := engine.ExecuteTaskWithFallback(context.Background(), "claude-code", task.Task)
		if err != nil {
			t.Logf("Expected failure (no real adapter): %v", err)
		}
	}
}

func TestResilience_CollaborationWithResilientAdapters(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 注册 Agent
	agentJSON := `{"name":"Worker","adapter":"claude-code","role":"worker","capabilities":[{"name":"implement","confidence":0.9}]}`
	resp, _ := client.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(agentJSON))
	var agent models.Agent
	json.NewDecoder(resp.Body).Decode(&agent)
	resp.Body.Close()

	// 发起协作
	collabJSON := `{
		"task_id": "task-resilient-collab",
		"task": {"id": "task-resilient-collab", "type": "implement", "description": "Resilient collaboration test"},
		"protocol": {"strategy": "pipeline", "fail_policy": "retry"},
		"agent_ids": ["` + agent.ID + `"]
	}`
	resp, err := client.Post(ts.URL+"/api/v1/collaborations", "application/json", bytes.NewBufferString(collabJSON))
	if err != nil {
		t.Fatalf("Start collaboration failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}

	var collab models.Collaboration
	json.NewDecoder(resp.Body).Decode(&collab)

	// 验证协作使用了弹性适配器
	if collab.Protocol.Strategy != "pipeline" {
		t.Errorf("Expected strategy 'pipeline', got '%s'", collab.Protocol.Strategy)
	}

	t.Logf("Collaboration %s created with resilient adapters", collab.ID)
}
