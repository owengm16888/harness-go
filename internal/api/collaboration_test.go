package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/harness-engineering/harness/models"
)

// ========== Agent API Tests ==========

func TestAgent_RegisterAndList(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 注册 Agent
	agentJSON := `{
		"name": "Claude Worker",
		"adapter": "claude-code",
		"role": "worker",
		"capabilities": [
			{"name": "implement", "confidence": 0.9, "languages": ["go"], "domains": ["backend"]},
			{"name": "review", "confidence": 0.8}
		]
	}`
	resp, err := client.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(agentJSON))
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
	if agent.Adapter != "claude-code" {
		t.Errorf("Expected adapter 'claude-code', got '%s'", agent.Adapter)
	}
	if agent.Status != "idle" {
		t.Errorf("Expected status 'idle', got '%s'", agent.Status)
	}
	if agent.ID == "" {
		t.Error("Expected auto-generated ID")
	}

	// 列出 Agent
	resp, err = client.Get(ts.URL + "/api/v1/agents")
	if err != nil {
		t.Fatalf("List agents failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var agents []*models.Agent
	json.NewDecoder(resp.Body).Decode(&agents)
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}
}

func TestAgent_GetByID(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 先注册
	agentJSON := `{"name":"Test Agent","adapter":"hermes","role":"reviewer","capabilities":[]}`
	resp, _ := client.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(agentJSON))
	var created models.Agent
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// 获取
	resp, err := client.Get(ts.URL + "/api/v1/agents/" + created.ID)
	if err != nil {
		t.Fatalf("Get agent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var agent models.Agent
	json.NewDecoder(resp.Body).Decode(&agent)
	if agent.Name != "Test Agent" {
		t.Errorf("Expected name 'Test Agent', got '%s'", agent.Name)
	}
}

func TestAgent_GetNotFound(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	resp, err := client.Get(ts.URL + "/api/v1/agents/nonexistent")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}
}

// ========== Collaboration API Tests ==========

func TestCollaboration_StartAndList(t *testing.T) {
	ts, engine := setupTestServer(t)
	client := ts.Client()

	// 先注册 Agent
	agentJSON := `{"name":"Worker","adapter":"claude-code","role":"worker","capabilities":[{"name":"implement","confidence":0.9}]}`
	resp, _ := client.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(agentJSON))
	var agent models.Agent
	json.NewDecoder(resp.Body).Decode(&agent)
	resp.Body.Close()

	// 发起协作
	collabJSON := `{
		"task_id": "task-collab-1",
		"task": {
			"id": "task-collab-1",
			"type": "implement",
			"description": "Implement OAuth2 authentication",
			"priority": 8
		},
		"protocol": {
			"strategy": "pipeline",
			"fail_policy": "abort"
		},
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

	if collab.ID == "" {
		t.Error("Expected auto-generated collaboration ID")
	}
	if collab.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", collab.Status)
	}
	if collab.Protocol.Strategy != "pipeline" {
		t.Errorf("Expected strategy 'pipeline', got '%s'", collab.Protocol.Strategy)
	}

	// 列出协作
	resp, err = client.Get(ts.URL + "/api/v1/collaborations")
	if err != nil {
		t.Fatalf("List collaborations failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var collabs []*models.Collaboration
	json.NewDecoder(resp.Body).Decode(&collabs)
	if len(collabs) != 1 {
		t.Errorf("Expected 1 collaboration, got %d", len(collabs))
	}

	_ = engine
}

func TestCollaboration_GetByID(t *testing.T) {
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
		"task_id": "task-get-1",
		"task": {"id": "task-get-1", "type": "implement", "description": "Build feature"},
		"protocol": {"strategy": "fan_out"},
		"agent_ids": ["` + agent.ID + `"]
	}`
	resp, _ = client.Post(ts.URL+"/api/v1/collaborations", "application/json", bytes.NewBufferString(collabJSON))
	var created models.Collaboration
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// 获取
	resp, err := client.Get(ts.URL + "/api/v1/collaborations/" + created.ID)
	if err != nil {
		t.Fatalf("Get collaboration failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var collab models.Collaboration
	json.NewDecoder(resp.Body).Decode(&collab)
	if collab.ID != created.ID {
		t.Errorf("Expected ID '%s', got '%s'", created.ID, collab.ID)
	}
}

func TestCollaboration_GetNotFound(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	resp, err := client.Get(ts.URL + "/api/v1/collaborations/nonexistent")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}
}

func TestCollaboration_Strategies(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 注册 Agent
	agentJSON := `{"name":"Worker","adapter":"claude-code","role":"worker","capabilities":[{"name":"implement","confidence":0.9}]}`
	resp, _ := client.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(agentJSON))
	var agent models.Agent
	json.NewDecoder(resp.Body).Decode(&agent)
	resp.Body.Close()

	strategies := []string{"pipeline", "fan_out", "discussion", "review", "debate"}

	for _, strategy := range strategies {
		collabJSON := `{
			"task_id": "task-` + strategy + `",
			"task": {"id": "task-` + strategy + `", "type": "implement", "description": "Test ` + strategy + ` strategy"},
			"protocol": {"strategy": "` + strategy + `"},
			"agent_ids": ["` + agent.ID + `"]
		}`
		resp, err := client.Post(ts.URL+"/api/v1/collaborations", "application/json", bytes.NewBufferString(collabJSON))
		if err != nil {
			t.Fatalf("Start %s collaboration failed: %v", strategy, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Strategy %s: Expected 201, got %d", strategy, resp.StatusCode)
		}

		var collab models.Collaboration
		json.NewDecoder(resp.Body).Decode(&collab)
		if string(collab.Protocol.Strategy) != strategy {
			t.Errorf("Expected strategy '%s', got '%s'", strategy, collab.Protocol.Strategy)
		}
	}
}

func TestCollaboration_EmptyList(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	resp, err := client.Get(ts.URL + "/api/v1/collaborations")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var collabs []*models.Collaboration
	json.NewDecoder(resp.Body).Decode(&collabs)
	if collabs == nil {
		t.Error("Expected empty array, got nil")
	}
	if len(collabs) != 0 {
		t.Errorf("Expected 0 collaborations, got %d", len(collabs))
	}
}

func TestCollaboration_BadRequest(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	resp, err := client.Post(ts.URL+"/api/v1/collaborations", "application/json",
		bytes.NewBufferString(`not json`))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", resp.StatusCode)
	}
}
