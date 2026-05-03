package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitState 熔断器状态
type CircuitState int

const (
	CircuitStateClosed   CircuitState = iota // 关闭状态（正常）
	CircuitStateOpen                         // 打开状态（熔断）
	CircuitStateHalfOpen                     // 半开状态（试探）
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu               sync.RWMutex
	name             string
	state            CircuitState
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	lastFailureTime  time.Time
	onStateChange    func(from, to CircuitState)
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Name             string
	FailureThreshold int           // 失败阈值
	SuccessThreshold int           // 成功阈值（半开状态）
	Timeout          time.Duration // 熔断超时时间
	OnStateChange    func(from, to CircuitState)
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &CircuitBreaker{
		name:             cfg.Name,
		state:            CircuitStateClosed,
		failureThreshold: cfg.FailureThreshold,
		successThreshold: cfg.SuccessThreshold,
		timeout:          cfg.Timeout,
		onStateChange:    cfg.OnStateChange,
	}
}

// Execute 执行操作
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	if !cb.AllowRequest() {
		return errors.New("circuit breaker is open")
	}

	err := fn(ctx)

	cb.RecordResult(err)
	return err
}

// AllowRequest 是否允许请求
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		// 检查是否超时，可以转为半开状态
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.setState(CircuitStateHalfOpen)
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case CircuitStateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordResult 记录结果
func (cb *CircuitBreaker) RecordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.successCount = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitStateClosed:
		if cb.failureCount >= cb.failureThreshold {
			cb.setState(CircuitStateOpen)
		}
	case CircuitStateHalfOpen:
		cb.setState(CircuitStateOpen)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.successCount++
	cb.failureCount = 0

	switch cb.state {
	case CircuitStateHalfOpen:
		if cb.successCount >= cb.successThreshold {
			cb.setState(CircuitStateClosed)
		}
	}
}

func (cb *CircuitBreaker) setState(newState CircuitState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState

	if cb.onStateChange != nil {
		go cb.onStateChange(oldState, newState)
	}
}

// GetState 获取状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitStateClosed
	cb.failureCount = 0
	cb.successCount = 0
}

// GetStats 获取统计信息
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		Name:             cb.name,
		State:            cb.state,
		FailureCount:     cb.failureCount,
		SuccessCount:     cb.successCount,
		FailureThreshold: cb.failureThreshold,
		SuccessThreshold: cb.successThreshold,
		Timeout:          cb.timeout,
		LastFailureTime:  cb.lastFailureTime,
	}
}

// CircuitBreakerStats 熔断器统计
type CircuitBreakerStats struct {
	Name             string        `json:"name"`
	State            CircuitState  `json:"state"`
	FailureCount     int           `json:"failure_count"`
	SuccessCount     int           `json:"success_count"`
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
	LastFailureTime  time.Time     `json:"last_failure_time"`
}

// StateString 获取状态字符串
func (s CircuitState) String() string {
	switch s {
	case CircuitStateClosed:
		return "closed"
	case CircuitStateOpen:
		return "open"
	case CircuitStateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
