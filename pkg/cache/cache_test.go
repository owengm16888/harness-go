package cache

import (
	"testing"
	"time"
)

func TestMemoryCache_SetAndGet(t *testing.T) {
	cache := NewMemoryCache(100, 1*time.Minute)

	// 测试设置和获取
	cache.Set("key1", "value1", 1*time.Minute)
	value, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected key1 to exist")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	// 测试不存在的键
	_, exists = cache.Get("key2")
	if exists {
		t.Error("Expected key2 to not exist")
	}
}

func TestMemoryCache_Expiration(t *testing.T) {
	cache := NewMemoryCache(100, 100*time.Millisecond)

	// 设置短 TTL
	cache.Set("key1", "value1", 100*time.Millisecond)

	// 立即获取
	value, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected key1 to exist")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 过期后获取
	_, exists = cache.Get("key1")
	if exists {
		t.Error("Expected key1 to be expired")
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	cache := NewMemoryCache(100, 1*time.Minute)

	// 设置值
	cache.Set("key1", "value1", 1*time.Minute)

	// 删除
	cache.Delete("key1")

	// 验证删除
	_, exists := cache.Get("key1")
	if exists {
		t.Error("Expected key1 to be deleted")
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(100, 1*time.Minute)

	// 设置多个值
	cache.Set("key1", "value1", 1*time.Minute)
	cache.Set("key2", "value2", 1*time.Minute)
	cache.Set("key3", "value3", 1*time.Minute)

	// 清空
	cache.Clear()

	// 验证清空
	if cache.Size() != 0 {
		t.Errorf("Expected size 0, got %d", cache.Size())
	}
}

func TestMemoryCache_Size(t *testing.T) {
	cache := NewMemoryCache(100, 1*time.Minute)

	// 初始大小
	if cache.Size() != 0 {
		t.Errorf("Expected size 0, got %d", cache.Size())
	}

	// 添加元素
	cache.Set("key1", "value1", 1*time.Minute)
	if cache.Size() != 1 {
		t.Errorf("Expected size 1, got %d", cache.Size())
	}

	// 添加更多元素
	cache.Set("key2", "value2", 1*time.Minute)
	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}
}

func TestMemoryCache_MaxSize(t *testing.T) {
	cache := NewMemoryCache(2, 1*time.Minute)

	// 添加超过最大大小的元素
	cache.Set("key1", "value1", 1*time.Minute)
	cache.Set("key2", "value2", 1*time.Minute)
	cache.Set("key3", "value3", 1*time.Minute)

	// 验证大小不超过最大值
	if cache.Size() > 2 {
		t.Errorf("Expected size <= 2, got %d", cache.Size())
	}
}

func TestNewCacheFromConfig(t *testing.T) {
	cfg := CacheConfig{
		MaxSize:         500,
		CleanupInterval: 5 * time.Minute,
		DefaultTTL:      10 * time.Minute,
	}

	cache := NewCacheFromConfig(cfg)

	if cache == nil {
		t.Error("Expected cache to be created")
	}

	// 测试使用
	cache.Set("key1", "value1", cfg.DefaultTTL)
	value, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected key1 to exist")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}
}
