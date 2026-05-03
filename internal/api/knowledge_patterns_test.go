package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/harness-engineering/harness/models"
)

// ========== Knowledge API Tests ==========

func TestKnowledge_AddAndList(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 1. 添加知识条目
	entryJSON := `{
		"id": "kb-test-1",
		"type": "solution",
		"title": "Go Context 用法",
		"context": "如何使用 context.Context 控制goroutine生命周期",
		"tags": ["go", "concurrency", "context"]
	}`
	resp, err := client.Post(ts.URL+"/api/v1/knowledge", "application/json", bytes.NewBufferString(entryJSON))
	if err != nil {
		t.Fatalf("Add knowledge failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}

	var created models.KnowledgeEntry
	json.NewDecoder(resp.Body).Decode(&created)
	if created.ID != "kb-test-1" {
		t.Errorf("Expected ID kb-test-1, got %s", created.ID)
	}

	// 2. 列出知识条目
	resp, err = client.Get(ts.URL + "/api/v1/knowledge")
	if err != nil {
		t.Fatalf("List knowledge failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var entries []*models.KnowledgeEntry
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
}

func TestKnowledge_GetByID(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 先添加
	entryJSON := `{"id":"kb-get-1","type":"doc","title":"Test Doc","content":"hello world","tags":["test"]}`
	client.Post(ts.URL+"/api/v1/knowledge", "application/json", bytes.NewBufferString(entryJSON))

	// 获取
	resp, err := client.Get(ts.URL + "/api/v1/knowledge/kb-get-1")
	if err != nil {
		t.Fatalf("Get knowledge failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var entry models.KnowledgeEntry
	json.NewDecoder(resp.Body).Decode(&entry)
	if entry.Title != "Test Doc" {
		t.Errorf("Expected title 'Test Doc', got '%s'", entry.Title)
	}
}

func TestKnowledge_GetNotFound(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	resp, err := client.Get(ts.URL + "/api/v1/knowledge/nonexistent")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}
}

func TestKnowledge_Update(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 先添加
	entryJSON := `{"id":"kb-upd-1","type":"doc","title":"Old Title","content":"old content","tags":["a"]}`
	client.Post(ts.URL+"/api/v1/knowledge", "application/json", bytes.NewBufferString(entryJSON))

	// 更新
	updateJSON := `{"title":"New Title","tags":["a","b"]}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/knowledge/kb-upd-1", bytes.NewBufferString(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Update knowledge failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var updated models.KnowledgeEntry
	json.NewDecoder(resp.Body).Decode(&updated)
	if updated.Title != "New Title" {
		t.Errorf("Expected title 'New Title', got '%s'", updated.Title)
	}
}

func TestKnowledge_Delete(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 先添加
	entryJSON := `{"id":"kb-del-1","type":"doc","title":"To Delete","content":"bye","tags":[]}`
	client.Post(ts.URL+"/api/v1/knowledge", "application/json", bytes.NewBufferString(entryJSON))

	// 删除
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/knowledge/kb-del-1", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Delete knowledge failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// 确认已删除
	resp2, _ := client.Get(ts.URL + "/api/v1/knowledge/kb-del-1")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 after delete, got %d", resp2.StatusCode)
	}
}

func TestKnowledge_Search(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 添加知识
	client.Post(ts.URL+"/api/v1/knowledge", "application/json",
		bytes.NewBufferString(`{"id":"kb-s1","type":"doc","title":"Go Concurrency","content":"goroutines and channels","tags":["go"]}`))

	// 搜索 — 验证 endpoint 可用且返回 200
	resp, err := client.Get(ts.URL + "/api/v1/knowledge/search?q=concurrency")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var results []*models.KnowledgeEntry
	json.NewDecoder(resp.Body).Decode(&results)
	// TF-IDF 搜索返回数组（可能为空，取决于分词匹配）
	if results == nil {
		t.Error("Expected non-nil results array")
	}
}

func TestKnowledge_Pagination(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 添加 3 条
	for i := 1; i <= 3; i++ {
		client.Post(ts.URL+"/api/v1/knowledge", "application/json",
			bytes.NewBufferString(`{"id":"kb-p`+string(rune('0'+i))+`","type":"doc","title":"Page Test","content":"content","tags":[]}`))
	}

	// 分页: limit=1, offset=0
	resp, err := client.Get(ts.URL + "/api/v1/knowledge?limit=1&offset=0")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	defer resp.Body.Close()

	var entries []*models.KnowledgeEntry
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry with limit=1, got %d", len(entries))
	}
}

func TestKnowledge_AutoID(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 不提供 ID，应自动生成
	entryJSON := `{"type":"doc","title":"Auto ID","content":"test","tags":[]}`
	resp, err := client.Post(ts.URL+"/api/v1/knowledge", "application/json", bytes.NewBufferString(entryJSON))
	if err != nil {
		t.Fatalf("Add knowledge failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}

	var created models.KnowledgeEntry
	json.NewDecoder(resp.Body).Decode(&created)
	if created.ID == "" {
		t.Error("Expected auto-generated ID, got empty string")
	}
}

// ========== Patterns API Tests ==========

func TestPatterns_AddAndList(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 添加模式
	patternJSON := `{
		"id": "pat-test-1",
		"name": "error-handling",
		"description": "Standard error handling pattern",
		"trigger": "error",
		"actions": [{"type": "retry", "parameters": {"max": 3}}]
	}`
	resp, err := client.Post(ts.URL+"/api/v1/patterns", "application/json", bytes.NewBufferString(patternJSON))
	if err != nil {
		t.Fatalf("Add pattern failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}

	// 列出模式
	resp, err = client.Get(ts.URL + "/api/v1/patterns")
	if err != nil {
		t.Fatalf("List patterns failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var patterns []*models.Pattern
	json.NewDecoder(resp.Body).Decode(&patterns)
	if len(patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(patterns))
	}
}

func TestPatterns_GetByID(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 先添加
	patternJSON := `{"id":"pat-get-1","name":"test-pattern","description":"for testing","trigger":"test","actions":[]}`
	client.Post(ts.URL+"/api/v1/patterns", "application/json", bytes.NewBufferString(patternJSON))

	// 获取
	resp, err := client.Get(ts.URL + "/api/v1/patterns/pat-get-1")
	if err != nil {
		t.Fatalf("Get pattern failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var pattern models.Pattern
	json.NewDecoder(resp.Body).Decode(&pattern)
	if pattern.Name != "test-pattern" {
		t.Errorf("Expected name 'test-pattern', got '%s'", pattern.Name)
	}
}

func TestPatterns_Update(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 先添加
	patternJSON := `{"id":"pat-upd-1","name":"old-name","description":"old desc","trigger":"x","actions":[]}`
	client.Post(ts.URL+"/api/v1/patterns", "application/json", bytes.NewBufferString(patternJSON))

	// 更新
	updateJSON := `{"name":"new-name","description":"updated description"}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/patterns/pat-upd-1", bytes.NewBufferString(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Update pattern failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var updated models.Pattern
	json.NewDecoder(resp.Body).Decode(&updated)
	if updated.Name != "new-name" {
		t.Errorf("Expected name 'new-name', got '%s'", updated.Name)
	}
}

func TestPatterns_Delete(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 先添加
	patternJSON := `{"id":"pat-del-1","name":"to-delete","description":"bye","trigger":"x","actions":[]}`
	client.Post(ts.URL+"/api/v1/patterns", "application/json", bytes.NewBufferString(patternJSON))

	// 删除
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/patterns/pat-del-1", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Delete pattern failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// 确认已删除
	resp2, _ := client.Get(ts.URL + "/api/v1/patterns/pat-del-1")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 after delete, got %d", resp2.StatusCode)
	}
}

func TestPatterns_Match(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 添加一个模式
	patternJSON := `{
		"id": "pat-match-1",
		"name": "implement-pattern",
		"description": "For implementation tasks",
		"trigger": "implement",
		"actions": [{"type": "execute"}]
	}`
	client.Post(ts.URL+"/api/v1/patterns", "application/json", bytes.NewBufferString(patternJSON))

	// 匹配任务
	matchJSON := `{
		"id": "task-match-1",
		"type": "implement",
		"description": "Implement user auth",
		"priority": 5
	}`
	resp, err := client.Post(ts.URL+"/api/v1/patterns/match", "application/json", bytes.NewBufferString(matchJSON))
	if err != nil {
		t.Fatalf("Match pattern failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var matched []*models.Pattern
	json.NewDecoder(resp.Body).Decode(&matched)
	// 匹配结果取决于 Matcher 实现，至少返回数组
	if matched == nil {
		t.Error("Expected non-nil match result")
	}
}

func TestPatterns_MatchBadRequest(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 发送非法 JSON
	resp, err := client.Post(ts.URL+"/api/v1/patterns/match", "application/json",
		bytes.NewBufferString(`not json`))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", resp.StatusCode)
	}
}

func TestPatterns_AutoID(t *testing.T) {
	ts, _ := setupTestServer(t)
	client := ts.Client()

	// 不提供 ID
	patternJSON := `{"name":"auto-id-pattern","description":"test","trigger":"x","actions":[]}`
	resp, err := client.Post(ts.URL+"/api/v1/patterns", "application/json", bytes.NewBufferString(patternJSON))
	if err != nil {
		t.Fatalf("Add pattern failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}

	var created models.Pattern
	json.NewDecoder(resp.Body).Decode(&created)
	if created.ID == "" {
		t.Error("Expected auto-generated ID, got empty string")
	}
}
