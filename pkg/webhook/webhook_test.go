package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebhookManager_AddWebhook(t *testing.T) {
	mgr := NewWebhookManager(WebhookManagerConfig{})
	defer mgr.Stop()

	webhook := &Webhook{
		ID:      "test-1",
		Name:    "Test Webhook",
		URL:     "http://example.com/webhook",
		Events:  []string{"task.created"},
		Enabled: true,
	}

	err := mgr.AddWebhook(webhook)
	if err != nil {
		t.Fatalf("Failed to add webhook: %v", err)
	}

	webhooks := mgr.ListWebhooks()
	if len(webhooks) != 1 {
		t.Errorf("Expected 1 webhook, got %d", len(webhooks))
	}
}

func TestWebhookManager_RemoveWebhook(t *testing.T) {
	mgr := NewWebhookManager(WebhookManagerConfig{})
	defer mgr.Stop()

	webhook := &Webhook{
		ID:      "test-1",
		Name:    "Test Webhook",
		URL:     "http://example.com/webhook",
		Events:  []string{"task.created"},
		Enabled: true,
	}

	mgr.AddWebhook(webhook)

	err := mgr.RemoveWebhook("test-1")
	if err != nil {
		t.Fatalf("Failed to remove webhook: %v", err)
	}

	webhooks := mgr.ListWebhooks()
	if len(webhooks) != 0 {
		t.Errorf("Expected 0 webhooks, got %d", len(webhooks))
	}
}

func TestWebhookManager_GetWebhook(t *testing.T) {
	mgr := NewWebhookManager(WebhookManagerConfig{})
	defer mgr.Stop()

	webhook := &Webhook{
		ID:      "test-1",
		Name:    "Test Webhook",
		URL:     "http://example.com/webhook",
		Events:  []string{"task.created"},
		Enabled: true,
	}

	mgr.AddWebhook(webhook)

	retrieved, err := mgr.GetWebhook("test-1")
	if err != nil {
		t.Fatalf("Failed to get webhook: %v", err)
	}

	if retrieved.ID != "test-1" {
		t.Errorf("Expected webhook ID test-1, got %s", retrieved.ID)
	}
}

func TestWebhookManager_Fire(t *testing.T) {
	// 创建测试服务器
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mgr := NewWebhookManager(WebhookManagerConfig{})
	defer mgr.Stop()

	webhook := &Webhook{
		ID:      "test-1",
		Name:    "Test Webhook",
		URL:     server.URL,
		Events:  []string{"task.created"},
		Enabled: true,
	}

	mgr.AddWebhook(webhook)

	ctx := context.Background()
	data := map[string]any{"task_id": "test-1"}

	err := mgr.Fire(ctx, "task.created", data)
	if err != nil {
		t.Fatalf("Failed to fire webhook: %v", err)
	}

	// 等待异步处理
	time.Sleep(100 * time.Millisecond)

	if !received {
		t.Error("Expected webhook to be fired")
	}
}

func TestWebhookManager_FireDisabled(t *testing.T) {
	// 创建测试服务器
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mgr := NewWebhookManager(WebhookManagerConfig{})
	defer mgr.Stop()

	webhook := &Webhook{
		ID:      "test-1",
		Name:    "Test Webhook",
		URL:     server.URL,
		Events:  []string{"task.created"},
		Enabled: false,
	}

	mgr.AddWebhook(webhook)

	ctx := context.Background()
	data := map[string]any{"task_id": "test-1"}

	mgr.Fire(ctx, "task.created", data)
	time.Sleep(100 * time.Millisecond)

	if received {
		t.Error("Expected webhook not to be fired when disabled")
	}
}

func TestWebhookManager_GetStats(t *testing.T) {
	mgr := NewWebhookManager(WebhookManagerConfig{})
	defer mgr.Stop()

	webhook1 := &Webhook{
		ID:      "test-1",
		Name:    "Test Webhook 1",
		URL:     "http://example.com/webhook1",
		Events:  []string{"task.created"},
		Enabled: true,
	}

	webhook2 := &Webhook{
		ID:      "test-2",
		Name:    "Test Webhook 2",
		URL:     "http://example.com/webhook2",
		Events:  []string{"task.created"},
		Enabled: false,
	}

	mgr.AddWebhook(webhook1)
	mgr.AddWebhook(webhook2)

	stats := mgr.GetStats()

	if stats.TotalWebhooks != 2 {
		t.Errorf("Expected 2 total webhooks, got %d", stats.TotalWebhooks)
	}

	if stats.EnabledWebhooks != 1 {
		t.Errorf("Expected 1 enabled webhook, got %d", stats.EnabledWebhooks)
	}
}

func TestWebhookPayload_Serialization(t *testing.T) {
	payload := &WebhookPayload{
		Event:     "task.created",
		Timestamp: time.Now(),
		Data:      map[string]any{"task_id": "test-1"},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	var decoded WebhookPayload
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if decoded.Event != "task.created" {
		t.Errorf("Expected event task.created, got %s", decoded.Event)
	}
}
