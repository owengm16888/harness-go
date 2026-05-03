package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_Closed(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	})

	ctx := context.Background()

	// 正常执行
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cb.GetState() != CircuitStateClosed {
		t.Errorf("Expected closed state, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_Open(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	})

	ctx := context.Background()

	// 触发失败
	for i := 0; i < 3; i++ {
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("test error")
		})
	}

	if cb.GetState() != CircuitStateOpen {
		t.Errorf("Expected open state, got %v", cb.GetState())
	}

	// 熔断状态下应该拒绝请求
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})

	if err == nil {
		t.Error("Expected error in open state")
	}
}

func TestCircuitBreaker_HalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	ctx := context.Background()

	// 触发失败
	for i := 0; i < 3; i++ {
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("test error")
		})
	}

	// 等待超时
	time.Sleep(150 * time.Millisecond)

	// 应该允许请求（半开状态）
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cb.GetState() != CircuitStateHalfOpen {
		t.Errorf("Expected half-open state, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	ctx := context.Background()

	// 触发失败
	for i := 0; i < 3; i++ {
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("test error")
		})
	}

	// 等待超时
	time.Sleep(150 * time.Millisecond)

	// 成功执行两次
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
	}

	if cb.GetState() != CircuitStateClosed {
		t.Errorf("Expected closed state, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	})

	ctx := context.Background()

	// 触发失败
	for i := 0; i < 3; i++ {
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("test error")
		})
	}

	// 重置
	cb.Reset()

	if cb.GetState() != CircuitStateClosed {
		t.Errorf("Expected closed state, got %v", cb.GetState())
	}
}

func TestRetry_Success(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, 3, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return NewRetryableError(errors.New("not yet"))
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_MaxAttempts(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, 3, func(ctx context.Context) error {
		attempts++
		return NewRetryableError(errors.New("always fail"))
	})

	if err == nil {
		t.Error("Expected error")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, 3, func(ctx context.Context) error {
		attempts++
		return errors.New("non-retryable")
	})

	if err == nil {
		t.Error("Expected error")
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetryWithBackoff(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attempts := 0
	start := time.Now()

	result := RetryWithBackoff(ctx, cfg, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return NewRetryableError(errors.New("not yet"))
		}
		return nil
	})

	elapsed := time.Since(start)

	if !result.Success {
		t.Error("Expected success")
	}

	if result.Attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", result.Attempts)
	}

	// 应该有延迟 (10ms + 20ms = 30ms)
	if elapsed < 25*time.Millisecond {
		t.Errorf("Expected delay, got %v", elapsed)
	}
}

func TestRetryWithResult(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultRetryConfig()

	attempts := 0
	val, result := RetryWithResult(ctx, cfg, func(ctx context.Context) (int, error) {
		attempts++
		if attempts < 2 {
			return 0, NewRetryableError(errors.New("not yet"))
		}
		return 42, nil
	})

	if !result.Success {
		t.Error("Expected success")
	}

	if val != 42 {
		t.Errorf("Expected 42, got %d", val)
	}
}

func TestTokenBucket_Allow(t *testing.T) {
	tb := NewTokenBucket(TokenBucketConfig{
		Rate:     10,
		Capacity: 10,
	})

	// 应该允许 10 个请求
	for i := 0; i < 10; i++ {
		if !tb.Allow() {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// 第 11 个应该被拒绝
	if tb.Allow() {
		t.Error("Expected request 11 to be denied")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	tb := NewTokenBucket(TokenBucketConfig{
		Rate:     100,
		Capacity: 10,
	})

	// 用完所有令牌
	for i := 0; i < 10; i++ {
		tb.Allow()
	}

	// 等待刷新
	time.Sleep(110 * time.Millisecond)

	// 应该有新的令牌
	if !tb.Allow() {
		t.Error("Expected request to be allowed after refill")
	}
}

func TestTokenBucket_Wait(t *testing.T) {
	tb := NewTokenBucket(TokenBucketConfig{
		Rate:     100,
		Capacity: 1,
	})

	// 用完令牌
	tb.Allow()

	start := time.Now()
	tb.Wait()
	elapsed := time.Since(start)

	// 应该等待大约 10ms
	if elapsed < 5*time.Millisecond {
		t.Errorf("Expected wait, got %v", elapsed)
	}
}

func TestSlidingWindow_Allow(t *testing.T) {
	sw := NewSlidingWindow(SlidingWindowConfig{
		Limit:  5,
		Window: 1 * time.Second,
	})

	// 应该允许 5 个请求
	for i := 0; i < 5; i++ {
		if !sw.Allow() {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// 第 6 个应该被拒绝
	if sw.Allow() {
		t.Error("Expected request 6 to be denied")
	}
}

func TestSlidingWindow_Slide(t *testing.T) {
	sw := NewSlidingWindow(SlidingWindowConfig{
		Limit:  5,
		Window: 100 * time.Millisecond,
	})

	// 用完配额
	for i := 0; i < 5; i++ {
		sw.Allow()
	}

	// 等待窗口滑动
	time.Sleep(150 * time.Millisecond)

	// 应该允许新请求
	if !sw.Allow() {
		t.Error("Expected request to be allowed after window slide")
	}
}

func TestSlidingWindow_GetStats(t *testing.T) {
	sw := NewSlidingWindow(SlidingWindowConfig{
		Limit:  10,
		Window: 1 * time.Second,
	})

	// 发送 3 个请求
	for i := 0; i < 3; i++ {
		sw.Allow()
	}

	stats := sw.GetStats()

	if stats.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", stats.Limit)
	}

	if stats.Current != 3 {
		t.Errorf("Expected current 3, got %d", stats.Current)
	}

	if stats.Remaining != 7 {
		t.Errorf("Expected remaining 7, got %d", stats.Remaining)
	}
}

func TestManager_CircuitBreaker(t *testing.T) {
	mgr := NewManager(ManagerConfig{})

	cb := mgr.GetOrCreateCircuitBreaker("test", CircuitBreakerConfig{
		FailureThreshold: 3,
	})

	ctx := context.Background()

	// 正常执行
	err := mgr.ExecuteWithCircuitBreaker(ctx, "test", func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cb.GetState() != CircuitStateClosed {
		t.Errorf("Expected closed state, got %v", cb.GetState())
	}
}

func TestManager_RateLimiter(t *testing.T) {
	mgr := NewManager(ManagerConfig{})

	_ = mgr.GetOrCreateRateLimiter("test", TokenBucketConfig{
		Rate:     10,
		Capacity: 10,
	})

	ctx := context.Background()

	// 应该允许请求
	err := mgr.ExecuteWithRateLimit(ctx, "test", func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// 用完配额
	for i := 0; i < 9; i++ {
		mgr.ExecuteWithRateLimit(ctx, "test", func(ctx context.Context) error {
			return nil
		})
	}

	// 应该被限流
	err = mgr.ExecuteWithRateLimit(ctx, "test", func(ctx context.Context) error {
		return nil
	})

	if err == nil {
		t.Error("Expected rate limit error")
	}
}

func TestManager_Resilience(t *testing.T) {
	mgr := NewManager(ManagerConfig{})

	cb := mgr.GetOrCreateCircuitBreaker("test-cb", CircuitBreakerConfig{
		FailureThreshold: 3,
	})

	rl := mgr.GetOrCreateRateLimiter("test-rl", TokenBucketConfig{
		Rate:     100,
		Capacity: 100,
	})

	ctx := context.Background()
	attempts := 0

	err := mgr.ExecuteWithResilience(ctx, "test", ResilienceConfig{
		CircuitBreaker: cb,
		RateLimiter:    rl,
		MaxRetries:     3,
		RetryDelay:     10 * time.Millisecond,
	}, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return NewRetryableError(errors.New("not yet"))
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetryableError(t *testing.T) {
	err := NewRetryableError(errors.New("test"))

	if !IsRetryableError(err) {
		t.Error("Expected retryable error")
	}

	if !IsRetryable(err) {
		t.Error("Expected retryable")
	}

	if err.Error() != "test" {
		t.Errorf("Expected 'test', got '%s'", err.Error())
	}
}

func TestRateLimitExceededError(t *testing.T) {
	err := NewRateLimitExceededError("test")

	if !IsRateLimitExceededError(err) {
		t.Error("Expected rate limit exceeded error")
	}

	if err.Error() != "rate limit exceeded for test" {
		t.Errorf("Expected 'rate limit exceeded for test', got '%s'", err.Error())
	}
}
