package pool

import (
	"context"
	"testing"
	"time"
)

func TestGenericPool_GetPut(t *testing.T) {
	factory := func(ctx context.Context) (interface{}, error) {
		return "connection", nil
	}

	close := func(conn interface{}) error {
		return nil
	}

	validate := func(conn interface{}) bool {
		return true
	}

	cfg := PoolConfig{
		MaxSize:     10,
		MinSize:     2,
		MaxIdle:     5,
		MaxLifetime: 1 * time.Hour,
		Timeout:     30 * time.Second,
	}

	pool, err := NewGenericPool(factory, close, validate, cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 获取连接
	ctx := context.Background()
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	if conn != "connection" {
		t.Errorf("Expected connection, got %v", conn)
	}

	// 归还连接
	err = pool.Put(ctx, conn)
	if err != nil {
		t.Fatalf("Failed to put connection: %v", err)
	}
}

func TestGenericPool_Close(t *testing.T) {
	_ = false // closed flag removed
	factory := func(ctx context.Context) (interface{}, error) {
		return "connection", nil
	}

	close := func(conn interface{}) error {
		// closed = true // removed
		return nil
	}

	validate := func(conn interface{}) bool {
		return true
	}

	cfg := PoolConfig{
		MaxSize:     10,
		MinSize:     2,
		MaxIdle:     5,
		MaxLifetime: 1 * time.Hour,
		Timeout:     30 * time.Second,
	}

	pool, err := NewGenericPool(factory, close, validate, cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	// 关闭池
	err = pool.Close()
	if err != nil {
		t.Fatalf("Failed to close pool: %v", err)
	}

	// 尝试从已关闭的池获取连接
	ctx := context.Background()
	_, err = pool.Get(ctx)
	if err == nil {
		t.Error("Expected error when getting from closed pool")
	}
}

func TestGenericPool_Size(t *testing.T) {
	factory := func(ctx context.Context) (interface{}, error) {
		return "connection", nil
	}

	close := func(conn interface{}) error {
		return nil
	}

	validate := func(conn interface{}) bool {
		return true
	}

	cfg := PoolConfig{
		MaxSize:     10,
		MinSize:     2,
		MaxIdle:     5,
		MaxLifetime: 1 * time.Hour,
		Timeout:     30 * time.Second,
	}

	pool, err := NewGenericPool(factory, close, validate, cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 初始大小应该等于 MinSize
	if pool.Size() != 2 {
		t.Errorf("Expected size 2, got %d", pool.Size())
	}
}

func TestGenericPool_Available(t *testing.T) {
	factory := func(ctx context.Context) (interface{}, error) {
		return "connection", nil
	}

	close := func(conn interface{}) error {
		return nil
	}

	validate := func(conn interface{}) bool {
		return true
	}

	cfg := PoolConfig{
		MaxSize:     10,
		MinSize:     2,
		MaxIdle:     5,
		MaxLifetime: 1 * time.Hour,
		Timeout:     30 * time.Second,
	}

	pool, err := NewGenericPool(factory, close, validate, cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 初始可用数应该等于 MinSize
	if pool.Available() != 2 {
		t.Errorf("Expected available 2, got %d", pool.Available())
	}

	// 获取一个连接
	ctx := context.Background()
	conn, _ := pool.Get(ctx)

	// 可用数应该减少
	if pool.Available() != 1 {
		t.Errorf("Expected available 1, got %d", pool.Available())
	}

	// 归还连接
	pool.Put(ctx, conn)

	// 可用数应该恢复
	if pool.Available() != 2 {
		t.Errorf("Expected available 2, got %d", pool.Available())
	}
}

func TestPoolError(t *testing.T) {
	err := &PoolError{
		Code:    "TEST_ERROR",
		Message: "Test error message",
	}

	if err.Error() != "TEST_ERROR: Test error message" {
		t.Errorf("Expected 'TEST_ERROR: Test error message', got '%s'", err.Error())
	}
}
