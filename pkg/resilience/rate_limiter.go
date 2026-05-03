package resilience

import (
	"sync"
	"time"
)

// RateLimiter 速率限制器接口
type RateLimiter interface {
	// Allow 检查是否允许请求
	Allow() bool
	// AllowN 检查是否允许 N 个请求
	AllowN(n int) bool
	// Wait 等待直到允许
	Wait()
	// WaitN 等待 N 个请求
	WaitN(n int)
	// Reserve 预留一个令牌
	Reserve() Reservation
	// ReserveN 预留 N 个令牌
	ReserveN(n int) Reservation
}

// Reservation 预留
type Reservation interface {
	// OK 是否成功预留
	OK() bool
	// Delay 延迟时间
	Delay() time.Duration
	// Cancel 取消预留
	Cancel()
}

// TokenBucket 令牌桶限流器
type TokenBucket struct {
	mu          sync.Mutex
	rate        float64   // 每秒生成的令牌数
	capacity    float64   // 桶容量
	tokens      float64   // 当前令牌数
	lastTime    time.Time // 上次更新时间
}

// TokenBucketConfig 令牌桶配置
type TokenBucketConfig struct {
	Rate     float64 // 每秒生成的令牌数
	Capacity float64 // 桶容量
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(cfg TokenBucketConfig) *TokenBucket {
	if cfg.Rate <= 0 {
		cfg.Rate = 10
	}
	if cfg.Capacity <= 0 {
		cfg.Capacity = 10
	}

	return &TokenBucket{
		rate:     cfg.Rate,
		capacity: cfg.Capacity,
		tokens:   cfg.Capacity,
		lastTime: time.Now(),
	}
}

// Allow 检查是否允许请求
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN 检查是否允许 N 个请求
func (tb *TokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refreshTokens()

	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}

	return false
}

// refreshTokens 刷新令牌
func (tb *TokenBucket) refreshTokens() {
	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.lastTime = now

	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
}

// Wait 等待直到允许
func (tb *TokenBucket) Wait() {
	tb.WaitN(1)
}

// WaitN 等待 N 个令牌
func (tb *TokenBucket) WaitN(n int) {
	for {
		tb.mu.Lock()
		tb.refreshTokens()

		if tb.tokens >= float64(n) {
			tb.tokens -= float64(n)
			tb.mu.Unlock()
			return
		}

		// 计算需要等待的时间
		deficit := float64(n) - tb.tokens
		waitTime := time.Duration(deficit/tb.rate*1000) * time.Millisecond
		tb.mu.Unlock()

		time.Sleep(waitTime)
	}
}

// Reserve 预留一个令牌
func (tb *TokenBucket) Reserve() Reservation {
	return tb.ReserveN(1)
}

// ReserveN 预留 N 个令牌
func (tb *TokenBucket) ReserveN(n int) Reservation {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refreshTokens()

	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return &tokenReservation{
			ok:     true,
			delay:  0,
			bucket: tb,
			n:      n,
		}
	}

	// 计算延迟
	deficit := float64(n) - tb.tokens
	delay := time.Duration(deficit/tb.rate*1000) * time.Millisecond

	return &tokenReservation{
		ok:     true,
		delay:  delay,
		bucket: tb,
		n:      n,
	}
}

// tokenReservation 令牌预留
type tokenReservation struct {
	ok     bool
	delay  time.Duration
	bucket *TokenBucket
	n      int
}

func (r *tokenReservation) OK() bool {
	return r.ok
}

func (r *tokenReservation) Delay() time.Duration {
	return r.delay
}

func (r *tokenReservation) Cancel() {
	if r.ok && r.delay == 0 {
		r.bucket.mu.Lock()
		r.bucket.tokens += float64(r.n)
		r.bucket.mu.Unlock()
	}
}

// SlidingWindow 滑动窗口限流器
type SlidingWindow struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	counters []time.Time
}

// SlidingWindowConfig 滑动窗口配置
type SlidingWindowConfig struct {
	Limit  int           // 窗口内最大请求数
	Window time.Duration // 窗口大小
}

// NewSlidingWindow 创建滑动窗口限流器
func NewSlidingWindow(cfg SlidingWindowConfig) *SlidingWindow {
	if cfg.Limit <= 0 {
		cfg.Limit = 100
	}
	if cfg.Window <= 0 {
		cfg.Window = 1 * time.Minute
	}

	return &SlidingWindow{
		limit:    cfg.Limit,
		window:   cfg.Window,
		counters: make([]time.Time, 0),
	}
}

// Allow 检查是否允许请求
func (sw *SlidingWindow) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-sw.window)

	// 清理过期的计数器
	validCounters := make([]time.Time, 0)
	for _, t := range sw.counters {
		if t.After(windowStart) {
			validCounters = append(validCounters, t)
		}
	}
	sw.counters = validCounters

	// 检查是否超过限制
	if len(sw.counters) >= sw.limit {
		return false
	}

	// 添加新的计数器
	sw.counters = append(sw.counters, now)
	return true
}

// AllowN 检查是否允许 N 个请求
func (sw *SlidingWindow) AllowN(n int) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-sw.window)

	// 清理过期的计数器
	validCounters := make([]time.Time, 0)
	for _, t := range sw.counters {
		if t.After(windowStart) {
			validCounters = append(validCounters, t)
		}
	}
	sw.counters = validCounters

	// 检查是否超过限制
	if len(sw.counters)+n > sw.limit {
		return false
	}

	// 添加新的计数器
	for i := 0; i < n; i++ {
		sw.counters = append(sw.counters, now)
	}
	return true
}

// Wait 等待直到允许
func (sw *SlidingWindow) Wait() {
	for {
		if sw.Allow() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// WaitN 等待 N 个请求
func (sw *SlidingWindow) WaitN(n int) {
	for {
		if sw.AllowN(n) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// GetCount 获取当前窗口内的请求数
func (sw *SlidingWindow) GetCount() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-sw.window)

	count := 0
	for _, t := range sw.counters {
		if t.After(windowStart) {
			count++
		}
	}
	return count
}

// GetStats 获取统计信息
func (sw *SlidingWindow) GetStats() SlidingWindowStats {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-sw.window)

	validCount := 0
	for _, t := range sw.counters {
		if t.After(windowStart) {
			validCount++
		}
	}

	return SlidingWindowStats{
		Limit:      sw.limit,
		Window:     sw.window,
		Current:    validCount,
		Remaining:  sw.limit - validCount,
	}
}

// SlidingWindowStats 滑动窗口统计
type SlidingWindowStats struct {
	Limit     int           `json:"limit"`
	Window    time.Duration `json:"window"`
	Current   int           `json:"current"`
	Remaining int           `json:"remaining"`
}
