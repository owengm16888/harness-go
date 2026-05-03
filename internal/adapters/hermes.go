package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/models"
)

// HermesAdapter Hermes 环境适配器
type HermesAdapter struct {
	config    config.HermesConfig
	client    *http.Client
	sessionID string
}

// NewHermesAdapter 创建 Hermes 适配器
func NewHermesAdapter() *HermesAdapter {
	return &HermesAdapter{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name 返回适配器名称
func (a *HermesAdapter) Name() string {
	return "hermes"
}

// SetConfig 设置 Hermes 特有配置 — 修复: 从 Engine 传递完整配置
func (a *HermesAdapter) SetConfig(cfg config.HermesConfig) {
	a.config = cfg
}

// Initialize 初始化适配器
func (a *HermesAdapter) Initialize(ctx context.Context, cfg config.AdapterConfig) error {
	// 配置已通过 SetConfig 从 Engine 传入, 不再硬编码
	// 如果 URL 为空则使用默认值
	if a.config.URL == "" {
		// URL 已通过 SetConfig 设置
	}

	// 创建会话
	sessionID, err := a.createSession(ctx)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	a.sessionID = sessionID

	return nil
}

// ExecuteTask 执行任务
func (a *HermesAdapter) ExecuteTask(ctx context.Context, task models.Task) (models.Result, error) {
	startTime := time.Now()

	// 准备请求
	req := taskRequest{
		SessionID:   a.sessionID,
		Task:        task,
		Environment: "hermes",
	}

	// 发送请求（带重试）
	resp, err := a.sendRequestWithRetry(ctx, "POST", "/api/tasks", req, 3)
	if err != nil {
		return models.Result{}, fmt.Errorf("failed to send task: %w", err)
	}

	// 解析响应
	var result models.Result
	if err := json.Unmarshal(resp, &result); err != nil {
		return models.Result{}, fmt.Errorf("failed to parse response: %w", err)
	}

	// 计算指标
	result.Metrics.Duration = time.Since(startTime)

	return result, nil
}

// GetState 获取状态
func (a *HermesAdapter) GetState(ctx context.Context) (models.State, error) {
	// 发送请求
	resp, err := a.sendRequest(ctx, "GET", fmt.Sprintf("/api/sessions/%s/state", a.sessionID), nil)
	if err != nil {
		return models.State{}, fmt.Errorf("failed to get state: %w", err)
	}

	// 解析响应
	var state models.State
	if err := json.Unmarshal(resp, &state); err != nil {
		return models.State{}, fmt.Errorf("failed to parse state: %w", err)
	}

	return state, nil
}

// Cleanup 清理
func (a *HermesAdapter) Cleanup(ctx context.Context) error {
	if a.sessionID == "" {
		return nil
	}
	_, err := a.sendRequest(ctx, "DELETE", fmt.Sprintf("/api/sessions/%s", a.sessionID), nil)
	if err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}
	return nil
}

// HealthCheck 检查 Hermes 服务健康状态
func (a *HermesAdapter) HealthCheck(ctx context.Context) error {
	resp, err := a.sendRequest(ctx, "GET", "/api/monitor/health", nil)
	if err != nil {
		return fmt.Errorf("hermes health check failed: %w", err)
	}

	var health map[string]any
	if err := json.Unmarshal(resp, &health); err != nil {
		return fmt.Errorf("invalid health response: %w", err)
	}

	if status, ok := health["status"].(string); !ok || status != "ok" {
		return fmt.Errorf("hermes unhealthy: %v", health["status"])
	}

	return nil
}

// sendRequestWithRetry 带重试的 HTTP 请求
func (a *HermesAdapter) sendRequestWithRetry(ctx context.Context, method, path string, body any, maxRetries int) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Second * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := a.sendRequest(ctx, method, path, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// createSession 创建会话
func (a *HermesAdapter) createSession(ctx context.Context) (string, error) {
	req := createSessionRequest{
		Environment: "hermes",
		Config:      make(map[string]any),
	}

	resp, err := a.sendRequest(ctx, "POST", "/api/sessions", req)
	if err != nil {
		return "", err
	}

	var session createSessionResponse
	if err := json.Unmarshal(resp, &session); err != nil {
		return "", err
	}

	return session.ID, nil
}

// sendRequest 发送 HTTP 请求
func (a *HermesAdapter) sendRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	// 构建 URL
	url := a.config.URL + path

	// 准备请求体
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置头
	req.Header.Set("Content-Type", "application/json")
	if a.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.config.APIKey)
	}
	if a.sessionID != "" {
		req.Header.Set("X-Session-ID", a.sessionID)
	}

	// 发送请求
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 检查状态码
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// taskRequest 任务请求
type taskRequest struct {
	SessionID   string      `json:"session_id"`
	Task        models.Task `json:"task"`
	Environment string      `json:"environment"`
}

// createSessionRequest 创建会话请求
type createSessionRequest struct {
	Environment string         `json:"environment"`
	Config      map[string]any `json:"config,omitempty"`
}

// createSessionResponse 创建会话响应
type createSessionResponse struct {
	ID string `json:"id"`
}
