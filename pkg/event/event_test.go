package event

import (
	"context"
	"testing"
)

func TestEventBus_Subscribe(t *testing.T) {
	bus := NewEventBus(EventBusConfig{})

	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	id := bus.Subscribe(EventTaskCreated, handler)
	if id == "" {
		t.Error("Expected non-empty handler ID")
	}

	if bus.GetHandlerCount(EventTaskCreated) != 1 {
		t.Errorf("Expected 1 handler, got %d", bus.GetHandlerCount(EventTaskCreated))
	}
}

func TestEventBus_SubscribeAll(t *testing.T) {
	bus := NewEventBus(EventBusConfig{})

	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	id := bus.SubscribeAll(handler)
	if id == "" {
		t.Error("Expected non-empty handler ID")
	}

	if bus.GetGlobalHandlerCount() != 1 {
		t.Errorf("Expected 1 global handler, got %d", bus.GetGlobalHandlerCount())
	}
}

func TestEventBus_Publish(t *testing.T) {
	bus := NewEventBus(EventBusConfig{})

	received := false
	handler := func(ctx context.Context, event Event) error {
		received = true
		return nil
	}

	bus.Subscribe(EventTaskCreated, handler)

	event := Event{
		Type: EventTaskCreated,
		Data: map[string]any{"task_id": "test-1"},
	}

	ctx := context.Background()
	err := bus.Publish(ctx, event)
	if err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	if !received {
		t.Error("Expected handler to be called")
	}
}

func TestEventBus_Clear(t *testing.T) {
	bus := NewEventBus(EventBusConfig{})

	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	bus.Subscribe(EventTaskCreated, handler)
	bus.SubscribeAll(handler)

	bus.Clear()

	if bus.GetHandlerCount(EventTaskCreated) != 0 {
		t.Errorf("Expected 0 handlers, got %d", bus.GetHandlerCount(EventTaskCreated))
	}

	if bus.GetGlobalHandlerCount() != 0 {
		t.Errorf("Expected 0 global handlers, got %d", bus.GetGlobalHandlerCount())
	}
}

func TestMemoryEventStore(t *testing.T) {
	store := NewMemoryEventStore()
	ctx := context.Background()

	// 测试保存
	event := Event{
		ID:   "test-1",
		Type: EventTaskCreated,
		Data: map[string]any{"task_id": "test-1"},
	}

	err := store.Save(ctx, event)
	if err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// 测试获取
	retrieved, err := store.Get(ctx, "test-1")
	if err != nil {
		t.Fatalf("Failed to get event: %v", err)
	}

	if retrieved.ID != "test-1" {
		t.Errorf("Expected event ID test-1, got %s", retrieved.ID)
	}

	// 测试列表
	events, err := store.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	// 测试删除
	err = store.Delete(ctx, "test-1")
	if err != nil {
		t.Fatalf("Failed to delete event: %v", err)
	}

	_, err = store.Get(ctx, "test-1")
	if err == nil {
		t.Error("Expected error after delete, got nil")
	}
}
