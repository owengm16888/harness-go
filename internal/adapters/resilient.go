
package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/models"
	"github.com/harness-engineering/harness/pkg/resilience"
)

// Adapter 是环境适配器接口（与 core.Adapter 一致）
type Adapter interface {
	Name() string
	Initialize(ctx context.Context, cfg config.AdapterConfig) error
	ExecuteTask(ctx context.Context, task models.Task) (models.Result, error)
	GetState(ctx context.Context) (models.State, error)
	Cleanup(ctx context.Context) error
}

// ResilientAdapter 弹性适配器 — 包装任意 Adapter，添加熔断 + 重试
type ResilientAdapter struct {
	inner          Adapter
	name           string
	circuitBreaker *resilience.CircuitBreaker
	retryConfig    resilience.RetryConfig
}

// NewResilientAdapter 创建弹性适配器
func NewResilientAdapter(inner Adapter, name string) *ResilientAdapter {
	return &ResilientAdapter{
		inner: inner,
		name:  name,
		circuitBreaker: resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
			Name:             fmt.Sprintf("adapter-%s", name),
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          30 * time.Second,
		}),
		retryConfig: resilience.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Second,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       true,
			RetryIf:      isRetryable,
		},
	}
}

func (r *ResilientAdapter) Name() string { return r.name }

func (r *ResilientAdapter) Initialize(ctx context.Context, cfg config.AdapterConfig) error {
	return r.inner.Initialize(ctx, cfg)
}

func (r *ResilientAdapter) ExecuteTask(ctx context.Context, task models.Task) (models.Result, error) {
	var result models.Result

	// 通过熔断器 + 重试执行
	retryResult := resilience.RetryWithBackoff(ctx, r.retryConfig, func(ctx context.Context) error {
		// 检查熔断器状态
		if !r.circuitBreaker.AllowRequest() {
			return fmt.Errorf("circuit breaker open for adapter %s", r.name)
		}

		var err error
		result, err = r.inner.ExecuteTask(ctx, task)
		if err != nil {
			r.circuitBreaker.RecordResult(err)
			return err
		}

		r.circuitBreaker.RecordResult(nil)
		return nil
	})

	if !retryResult.Success {
		slog.Error("adapter execution failed after retries",
			"adapter", r.name,
			"attempts", retryResult.Attempts,
			"error", retryResult.LastError,
		)
		return result, retryResult.LastError
	}

	return result, nil
}

func (r *ResilientAdapter) GetState(ctx context.Context) (models.State, error) {
	return r.inner.GetState(ctx)
}

func (r *ResilientAdapter) Cleanup(ctx context.Context) error {
	return r.inner.Cleanup(ctx)
}

// GetCircuitState 获取熔断器状态
func (r *ResilientAdapter) GetCircuitState() string {
	return r.circuitBreaker.GetState().String()
}

// isRetryable 判断错误是否可重试
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	retryablePatterns := []string{
		"connection refused", "timeout", "temporary",
		"EOF", "503", "429", "circuit breaker",
	}
	for _, p := range retryablePatterns {
		if len(msg) >= len(p) {
			for i := 0; i <= len(msg)-len(p); i++ {
				if msg[i:i+len(p)] == p {
					return true
				}
			}
		}
	}
	return false
}
