package resilience

import (
	"context"
	"sync"
	"time"
)

// Manager 弹性管理器
type Manager struct {
	mu             sync.RWMutex
	circuitBreakers map[string]*CircuitBreaker
	rateLimiters    map[string]RateLimiter
	config         ManagerConfig
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	DefaultCircuitBreaker CircuitBreakerConfig
	DefaultRateLimiter    TokenBucketConfig
}

// NewManager 创建管理器
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		circuitBreakers: make(map[string]*CircuitBreaker),
		rateLimiters:    make(map[string]RateLimiter),
		config:         cfg,
	}
}

// GetOrCreateCircuitBreaker 获取或创建熔断器
func (m *Manager) GetOrCreateCircuitBreaker(name string, cfg ...CircuitBreakerConfig) *CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cb, exists := m.circuitBreakers[name]; exists {
		return cb
	}

	config := m.config.DefaultCircuitBreaker
	if len(cfg) > 0 {
		config = cfg[0]
	}
	config.Name = name

	cb := NewCircuitBreaker(config)
	m.circuitBreakers[name] = cb
	return cb
}

// GetOrCreateRateLimiter 获取或创建限流器
func (m *Manager) GetOrCreateRateLimiter(name string, cfg ...TokenBucketConfig) RateLimiter {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rl, exists := m.rateLimiters[name]; exists {
		return rl
	}

	config := m.config.DefaultRateLimiter
	if len(cfg) > 0 {
		config = cfg[0]
	}

	rl := NewTokenBucket(config)
	m.rateLimiters[name] = rl
	return rl
}

// GetCircuitBreaker 获取熔断器
func (m *Manager) GetCircuitBreaker(name string) (*CircuitBreaker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cb, exists := m.circuitBreakers[name]
	return cb, exists
}

// GetRateLimiter 获取限流器
func (m *Manager) GetRateLimiter(name string) (RateLimiter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rl, exists := m.rateLimiters[name]
	return rl, exists
}

// RemoveCircuitBreaker 移除熔断器
func (m *Manager) RemoveCircuitBreaker(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.circuitBreakers, name)
}

// RemoveRateLimiter 移除限流器
func (m *Manager) RemoveRateLimiter(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.rateLimiters, name)
}

// ListCircuitBreakers 列出所有熔断器
func (m *Manager) ListCircuitBreakers() map[string]CircuitBreakerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats)
	for name, cb := range m.circuitBreakers {
		stats[name] = cb.GetStats()
	}
	return stats
}

// ResetAllCircuitBreakers 重置所有熔断器
func (m *Manager) ResetAllCircuitBreakers() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cb := range m.circuitBreakers {
		cb.Reset()
	}
}

// ExecuteWithCircuitBreaker 使用熔断器执行
func (m *Manager) ExecuteWithCircuitBreaker(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	cb := m.GetOrCreateCircuitBreaker(name)
	return cb.Execute(ctx, fn)
}

// ExecuteWithRateLimit 使用限流器执行
func (m *Manager) ExecuteWithRateLimit(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	rl := m.GetOrCreateRateLimiter(name)

	if !rl.Allow() {
		return NewRateLimitExceededError(name)
	}

	return fn(ctx)
}

// ExecuteWithResilience 使用全部弹性机制执行
func (m *Manager) ExecuteWithResilience(ctx context.Context, name string, cfg ResilienceConfig, fn func(ctx context.Context) error) error {
	// 1. 检查速率限制
	if cfg.RateLimiter != nil {
		if !cfg.RateLimiter.Allow() {
			return NewRateLimitExceededError(name)
		}
	}

	// 2. 使用熔断器和重试
	retryCfg := DefaultRetryConfig()
	retryCfg.RetryIf = DefaultRetryIf
	if cfg.MaxRetries > 0 {
		retryCfg.MaxAttempts = cfg.MaxRetries
	}
	if cfg.RetryDelay > 0 {
		retryCfg.InitialDelay = cfg.RetryDelay
	}

	if cfg.CircuitBreaker != nil {
		result := RetryWithBackoff(ctx, retryCfg, func(ctx context.Context) error {
			return cfg.CircuitBreaker.Execute(ctx, fn)
		})
		return result.LastError
	}

	result := RetryWithBackoff(ctx, retryCfg, fn)
	return result.LastError
}

// ResilienceConfig 弹性配置
type ResilienceConfig struct {
	CircuitBreaker *CircuitBreaker
	RateLimiter    RateLimiter
	MaxRetries     int
	RetryDelay     time.Duration
}

// RateLimitExceededError 速率限制超出错误
type RateLimitExceededError struct {
	LimiterName string
}

func (e *RateLimitExceededError) Error() string {
	return "rate limit exceeded for " + e.LimiterName
}

// NewRateLimitExceededError 创建速率限制超出错误
func NewRateLimitExceededError(name string) *RateLimitExceededError {
	return &RateLimitExceededError{LimiterName: name}
}

// IsRateLimitExceededError 检查是否为速率限制超出错误
func IsRateLimitExceededError(err error) bool {
	_, ok := err.(*RateLimitExceededError)
	return ok
}
