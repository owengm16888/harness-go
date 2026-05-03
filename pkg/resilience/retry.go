package resilience

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts  int           // 最大重试次数
	InitialDelay time.Duration // 初始延迟
	MaxDelay     time.Duration // 最大延迟
	Multiplier   float64       // 延迟倍数
	Jitter       bool          // 是否添加抖动
	RetryIf      func(err error) bool // 自定义重试条件
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// RetryResult 重试结果
type RetryResult struct {
	Attempts    int           `json:"attempts"`
	TotalDelay  time.Duration `json:"total_delay"`
	LastError   error         `json:"last_error"`
	Success     bool          `json:"success"`
}

// RetryWithBackoff 带退避的重试
func RetryWithBackoff(ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) error) RetryResult {
	result := RetryResult{}

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result.Attempts = attempt

		// 执行操作
		err := fn(ctx)
		if err == nil {
			result.Success = true
			result.LastError = nil
			return result
		}

		result.LastError = err

		// 检查是否应该重试
		if cfg.RetryIf != nil && !cfg.RetryIf(err) {
			return result
		}

		// 如果是最后一次尝试，不再等待
		if attempt == cfg.MaxAttempts {
			return result
		}

		// 计算延迟
		delay := calculateDelay(cfg, attempt)

		// 等待或取消
		select {
		case <-ctx.Done():
			result.LastError = ctx.Err()
			return result
		case <-time.After(delay):
			result.TotalDelay += delay
		}
	}

	return result
}

// Retry 简单重试
func Retry(ctx context.Context, maxAttempts int, fn func(ctx context.Context) error) error {
	cfg := RetryConfig{
		MaxAttempts:  maxAttempts,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		RetryIf:      DefaultRetryIf,
	}

	result := RetryWithBackoff(ctx, cfg, fn)
	return result.LastError
}

// RetryWithResult 带结果的重试
func RetryWithResult[T any](ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) (T, error)) (T, RetryResult) {
	var zero T
	result := RetryResult{}

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result.Attempts = attempt

		// 执行操作
		val, err := fn(ctx)
		if err == nil {
			result.Success = true
			return val, result
		}

		result.LastError = err

		// 检查是否应该重试
		if cfg.RetryIf != nil && !cfg.RetryIf(err) {
			return zero, result
		}

		// 如果是最后一次尝试，不再等待
		if attempt == cfg.MaxAttempts {
			return zero, result
		}

		// 计算延迟
		delay := calculateDelay(cfg, attempt)

		// 等待或取消
		select {
		case <-ctx.Done():
			result.LastError = ctx.Err()
			return zero, result
		case <-time.After(delay):
			result.TotalDelay += delay
		}
	}

	return zero, result
}

// calculateDelay 计算延迟
func calculateDelay(cfg RetryConfig, attempt int) time.Duration {
	// 指数退避
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))

	// 添加抖动
	if cfg.Jitter {
		jitter := rand.Float64() * 0.1 * delay
		delay += jitter
	}

	// 限制最大延迟
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	return time.Duration(delay)
}

// RetryableError 可重试错误
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError 创建可重试错误
func NewRetryableError(err error) *RetryableError {
	return &RetryableError{Err: err}
}

// IsRetryableError 检查是否为可重试错误
func IsRetryableError(err error) bool {
	_, ok := err.(*RetryableError)
	return ok
}

// IsRetryable 检查错误是否可重试
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否为 RetryableError
	if IsRetryableError(err) {
		return true
	}

	// 可以添加更多可重试错误类型
	return false
}

// DefaultRetryIf 默认重试条件
func DefaultRetryIf(err error) bool {
	return IsRetryable(err)
}

// AlwaysRetry 始终重试
func AlwaysRetry(err error) bool {
	return true
}

// NeverRetry 从不重试
func NeverRetry(err error) bool {
	return false
}
