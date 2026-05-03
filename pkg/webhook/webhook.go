package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Webhook Webhook 配置
type Webhook struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Events      []string          `json:"events"`
	Headers     map[string]string `json:"headers,omitempty"`
	Secret      string            `json:"secret,omitempty"`
	Enabled     bool              `json:"enabled"`
	RetryCount  int               `json:"retry_count"`
	RetryDelay  time.Duration     `json:"retry_delay"`
	Timeout     time.Duration     `json:"timeout"`
	LastFired   time.Time         `json:"last_fired,omitempty"`
	LastError   error             `json:"last_error,omitempty"`
	FireCount   int               `json:"fire_count"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// WebhookPayload Webhook 负载
type WebhookPayload struct {
	Event     string            `json:"event"`
	Timestamp time.Time         `json:"timestamp"`
	Data      map[string]any    `json:"data"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// WebhookManager Webhook 管理器
type WebhookManager struct {
	mu        sync.RWMutex
	webhooks  map[string]*Webhook
	client    *http.Client
	workers   int
	queue     chan *webhookJob
	stopChan  chan struct{}
}

// webhookJob Webhook 任务
type webhookJob struct {
	webhook *Webhook
	payload *WebhookPayload
	ctx     context.Context
}

// WebhookManagerConfig Webhook 管理器配置
type WebhookManagerConfig struct {
	Workers    int           `yaml:"workers"`
	QueueSize  int           `yaml:"queue_size"`
	Timeout    time.Duration `yaml:"timeout"`
	MaxRetries int           `yaml:"max_retries"`
}

// NewWebhookManager 创建 Webhook 管理器
func NewWebhookManager(cfg WebhookManagerConfig) *WebhookManager {
	if cfg.Workers == 0 {
		cfg.Workers = 5
	}
	if cfg.QueueSize == 0 {
		cfg.QueueSize = 1000
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	mgr := &WebhookManager{
		webhooks: make(map[string]*Webhook),
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		workers:  cfg.Workers,
		queue:    make(chan *webhookJob, cfg.QueueSize),
		stopChan: make(chan struct{}),
	}

	// 启动工作协程
	for i := 0; i < cfg.Workers; i++ {
		go mgr.worker()
	}

	return mgr
}

// AddWebhook 添加 Webhook
func (m *WebhookManager) AddWebhook(webhook *Webhook) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if webhook.ID == "" {
		return fmt.Errorf("webhook ID is required")
	}

	if _, exists := m.webhooks[webhook.ID]; exists {
		return fmt.Errorf("webhook already exists: %s", webhook.ID)
	}

	// 设置默认值
	if webhook.RetryCount == 0 {
		webhook.RetryCount = 3
	}
	if webhook.RetryDelay == 0 {
		webhook.RetryDelay = 1 * time.Second
	}
	if webhook.Timeout == 0 {
		webhook.Timeout = 30 * time.Second
	}

	m.webhooks[webhook.ID] = webhook
	return nil
}

// RemoveWebhook 移除 Webhook
func (m *WebhookManager) RemoveWebhook(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.webhooks[id]; !exists {
		return fmt.Errorf("webhook not found: %s", id)
	}

	delete(m.webhooks, id)
	return nil
}

// UpdateWebhook 更新 Webhook
func (m *WebhookManager) UpdateWebhook(webhook *Webhook) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.webhooks[webhook.ID]; !exists {
		return fmt.Errorf("webhook not found: %s", webhook.ID)
	}

	m.webhooks[webhook.ID] = webhook
	return nil
}

// GetWebhook 获取 Webhook
func (m *WebhookManager) GetWebhook(id string) (*Webhook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	webhook, exists := m.webhooks[id]
	if !exists {
		return nil, fmt.Errorf("webhook not found: %s", id)
	}

	return webhook, nil
}

// ListWebhooks 列出 Webhooks
func (m *WebhookManager) ListWebhooks() []*Webhook {
	m.mu.RLock()
	defer m.mu.RUnlock()

	webhooks := make([]*Webhook, 0, len(m.webhooks))
	for _, webhook := range m.webhooks {
		webhooks = append(webhooks, webhook)
	}

	return webhooks
}

// Fire 触发 Webhook
func (m *WebhookManager) Fire(ctx context.Context, event string, data map[string]any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	payload := &WebhookPayload{
		Event:     event,
		Timestamp: time.Now(),
		Data:      data,
	}

	for _, webhook := range m.webhooks {
		if !webhook.Enabled {
			continue
		}

		// 检查事件是否匹配
		if !m.matchesEvent(webhook, event) {
			continue
		}

		// 发送到队列
		job := &webhookJob{
			webhook: webhook,
			payload: payload,
			ctx:     ctx,
		}

		select {
		case m.queue <- job:
			// 成功入队
		default:
			// 队列已满，丢弃
			fmt.Printf("Webhook queue full, dropping event for %s\n", webhook.ID)
		}
	}

	return nil
}

// matchesEvent 检查事件是否匹配
func (m *WebhookManager) matchesEvent(webhook *Webhook, event string) bool {
	if len(webhook.Events) == 0 {
		return true
	}

	for _, e := range webhook.Events {
		if e == "*" || e == event {
			return true
		}
	}

	return false
}

// worker 工作协程
func (m *WebhookManager) worker() {
	for {
		select {
		case job := <-m.queue:
			m.executeWebhook(job)
		case <-m.stopChan:
			return
		}
	}
}

// executeWebhook 执行 Webhook
func (m *WebhookManager) executeWebhook(job *webhookJob) {
	webhook := job.webhook
	payload := job.payload

	// 序列化负载
	body, err := json.Marshal(payload)
	if err != nil {
		m.recordError(webhook, err)
		return
	}

	// 创建请求
	req, err := http.NewRequestWithContext(job.ctx, "POST", webhook.URL, bytes.NewReader(body))
	if err != nil {
		m.recordError(webhook, err)
		return
	}

	// 设置头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Harness-Webhook/1.0")
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	// 发送请求（带重试）
	var resp *http.Response
	for i := 0; i <= webhook.RetryCount; i++ {
		resp, err = m.client.Do(req)
		if err == nil && resp.StatusCode < 400 {
			break
		}

		if i < webhook.RetryCount {
			time.Sleep(webhook.RetryDelay * time.Duration(i+1))
		}
	}

	if err != nil {
		m.recordError(webhook, err)
		return
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		m.recordError(webhook, fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body)))
		return
	}

	// 记录成功
	m.recordSuccess(webhook)
}

// recordError 记录错误
func (m *WebhookManager) recordError(webhook *Webhook, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	webhook.LastError = err
	webhook.LastFired = time.Now()
	webhook.FireCount++
}

// recordSuccess 记录成功
func (m *WebhookManager) recordSuccess(webhook *Webhook) {
	m.mu.Lock()
	defer m.mu.Unlock()

	webhook.LastError = nil
	webhook.LastFired = time.Now()
	webhook.FireCount++
}

// Stop 停止管理器
func (m *WebhookManager) Stop() {
	close(m.stopChan)
}

// GetStats 获取统计信息
func (m *WebhookManager) GetStats() *WebhookStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &WebhookStats{
		TotalWebhooks: len(m.webhooks),
	}

	for _, webhook := range m.webhooks {
		if webhook.Enabled {
			stats.EnabledWebhooks++
		}
		if webhook.LastError != nil {
			stats.FailedWebhooks++
		}
		stats.TotalFires += webhook.FireCount
	}

	return stats
}

// WebhookStats Webhook 统计
type WebhookStats struct {
	TotalWebhooks   int `json:"total_webhooks"`
	EnabledWebhooks int `json:"enabled_webhooks"`
	FailedWebhooks  int `json:"failed_webhooks"`
	TotalFires      int `json:"total_fires"`
}
