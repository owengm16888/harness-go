package cache

import (
	"sync"
	"time"
)

// Cache 缓存接口
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
	Size() int
}

// MemoryCache 内存缓存
type MemoryCache struct {
	mu       sync.RWMutex
	items    map[string]*cacheItem
	maxSize  int
	stopChan chan struct{}
}

// cacheItem 缓存项
type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(maxSize int, cleanupInterval time.Duration) *MemoryCache {
	cache := &MemoryCache{
		items:    make(map[string]*cacheItem),
		maxSize:  maxSize,
		stopChan: make(chan struct{}),
	}

	// 启动清理协程
	go cache.cleanup(cleanupInterval)

	return cache
}

// Get 获取缓存
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(item.expiration) {
		return nil, false
	}

	return item.value, true
}

// Set 设置缓存
func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否超过最大大小
	if len(c.items) >= c.maxSize {
		c.evict()
	}

	c.items[key] = &cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

// Delete 删除缓存
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear 清空缓存
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem)
}

// Size 获取缓存大小
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// Stop 停止缓存
func (c *MemoryCache) Stop() {
	close(c.stopChan)
}

// cleanup 清理过期缓存
func (c *MemoryCache) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for key, item := range c.items {
				if now.After(item.expiration) {
					delete(c.items, key)
				}
			}
			c.mu.Unlock()
		case <-c.stopChan:
			return
		}
	}
}

// evict 驱逐缓存
func (c *MemoryCache) evict() {
	// 简单的 LRU 驱逐策略
	// 删除最旧的项
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestKey == "" || item.expiration.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.expiration
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

// CacheConfig 缓存配置
type CacheConfig struct {
	MaxSize         int           `yaml:"max_size"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	DefaultTTL      time.Duration `yaml:"default_ttl"`
}

// NewCacheFromConfig 从配置创建缓存
func NewCacheFromConfig(cfg CacheConfig) *MemoryCache {
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 1000
	}
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}
	if cfg.DefaultTTL == 0 {
		cfg.DefaultTTL = 10 * time.Minute
	}

	return NewMemoryCache(cfg.MaxSize, cfg.CleanupInterval)
}
