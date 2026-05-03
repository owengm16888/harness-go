package event

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType 事件类型
type EventType string

const (
	// 任务事件
	EventTaskCreated   EventType = "task.created"
	EventTaskStarted   EventType = "task.started"
	EventTaskCompleted EventType = "task.completed"
	EventTaskFailed    EventType = "task.failed"
	EventTaskCancelled EventType = "task.cancelled"

	// 会话事件
	EventSessionCreated EventType = "session.created"
	EventSessionUpdated EventType = "session.updated"
	EventSessionDeleted EventType = "session.deleted"

	// 知识事件
	EventKnowledgeCreated EventType = "knowledge.created"
	EventKnowledgeUpdated EventType = "knowledge.updated"
	EventKnowledgeDeleted EventType = "knowledge.deleted"

	// 模式事件
	EventPatternCreated EventType = "pattern.created"
	EventPatternMatched EventType = "pattern.matched"
	EventPatternLearned EventType = "pattern.learned"

	// 反馈事件
	EventFeedbackProcessed EventType = "feedback.processed"
	EventFeedbackViolation EventType = "feedback.violation"
	EventFeedbackFixed     EventType = "feedback.fixed"

	// 系统事件
	EventSystemStarted  EventType = "system.started"
	EventSystemStopped  EventType = "system.stopped"
	EventSystemError    EventType = "system.error"
	EventSystemWarning  EventType = "system.warning"
)

// Event 事件
type Event struct {
	ID        string            `json:"id"`
	Type      EventType         `json:"type"`
	Source    string            `json:"source"`
	Data      map[string]any    `json:"data"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// EventHandler 事件处理器
type EventHandler func(ctx context.Context, event Event) error

// EventBus 事件总线
type EventBus struct {
	mu          sync.RWMutex
	handlers    map[EventType][]handlerEntry
	subscribers map[string][]handlerEntry
	global      []handlerEntry
	async       bool
	bufferSize  int
}

// handlerEntry 处理器条目
type handlerEntry struct {
	handler EventHandler
	filter  EventFilter
	async   bool
}

// EventFilter 事件过滤器
type EventFilter func(event Event) bool

// EventBusConfig 事件总线配置
type EventBusConfig struct {
	Async      bool `yaml:"async"`
	BufferSize int  `yaml:"buffer_size"`
}

// NewEventBus 创建事件总线
func NewEventBus(cfg EventBusConfig) *EventBus {
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 1000
	}

	return &EventBus{
		handlers:    make(map[EventType][]handlerEntry),
		subscribers: make(map[string][]handlerEntry),
		global:      []handlerEntry{},
		async:       cfg.Async,
		bufferSize:  cfg.BufferSize,
	}
}

// Subscribe 订阅事件
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) string {
	return eb.SubscribeWithFilter(eventType, handler, nil)
}

// SubscribeWithFilter 带过滤器订阅事件
func (eb *EventBus) SubscribeWithFilter(eventType EventType, handler EventHandler, filter EventFilter) string {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := generateHandlerID()
	entry := handlerEntry{
		handler: handler,
		filter:  filter,
		async:   eb.async,
	}

	eb.handlers[eventType] = append(eb.handlers[eventType], entry)

	return id
}

// SubscribeAll 订阅所有事件
func (eb *EventBus) SubscribeAll(handler EventHandler) string {
	return eb.SubscribeAllWithFilter(handler, nil)
}

// SubscribeAllWithFilter 带过滤器订阅所有事件
func (eb *EventBus) SubscribeAllWithFilter(handler EventHandler, filter EventFilter) string {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := generateHandlerID()
	entry := handlerEntry{
		handler: handler,
		filter:  filter,
		async:   eb.async,
	}

	eb.global = append(eb.global, entry)

	return id
}

// Unsubscribe 取消订阅
func (eb *EventBus) Unsubscribe(eventType EventType, handlerID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if handlers, exists := eb.handlers[eventType]; exists {
		for i := range handlers {
			if generateHandlerID() == handlerID {
				eb.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
	}
}

// Publish 发布事件
func (eb *EventBus) Publish(ctx context.Context, event Event) error {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// 设置事件元数据
	if event.ID == "" {
		event.ID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Metadata == nil {
		event.Metadata = make(map[string]string)
	}

	// 处理特定类型的处理器
	if handlers, exists := eb.handlers[event.Type]; exists {
		for _, entry := range handlers {
			if entry.filter != nil && !entry.filter(event) {
				continue
			}

			if entry.async {
				go eb.executeHandler(ctx, entry.handler, event)
			} else {
				if err := eb.executeHandler(ctx, entry.handler, event); err != nil {
					return fmt.Errorf("handler error: %w", err)
				}
			}
		}
	}

	// 处理全局处理器
	for _, entry := range eb.global {
		if entry.filter != nil && !entry.filter(event) {
			continue
		}

		if entry.async {
			go eb.executeHandler(ctx, entry.handler, event)
		} else {
			if err := eb.executeHandler(ctx, entry.handler, event); err != nil {
				return fmt.Errorf("global handler error: %w", err)
			}
		}
	}

	return nil
}

// executeHandler 执行处理器
func (eb *EventBus) executeHandler(ctx context.Context, handler EventHandler, event Event) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Event handler panic: %v\n", r)
		}
	}()

	return handler(ctx, event)
}

// GetHandlerCount 获取处理器数量
func (eb *EventBus) GetHandlerCount(eventType EventType) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return len(eb.handlers[eventType])
}

// GetGlobalHandlerCount 获取全局处理器数量
func (eb *EventBus) GetGlobalHandlerCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return len(eb.global)
}

// Clear 清空所有处理器
func (eb *EventBus) Clear() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers = make(map[EventType][]handlerEntry)
	eb.subscribers = make(map[string][]handlerEntry)
	eb.global = []handlerEntry{}
}

// generateEventID 生成事件 ID
func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

// generateHandlerID 生成处理器 ID
func generateHandlerID() string {
	return fmt.Sprintf("hdl_%d", time.Now().UnixNano())
}

// EventStore 事件存储
type EventStore interface {
	Save(ctx context.Context, event Event) error
	Get(ctx context.Context, id string) (*Event, error)
	List(ctx context.Context, filter EventFilter) ([]Event, error)
	Delete(ctx context.Context, id string) error
}

// MemoryEventStore 内存事件存储
type MemoryEventStore struct {
	mu     sync.RWMutex
	events map[string]Event
}

// NewMemoryEventStore 创建内存事件存储
func NewMemoryEventStore() *MemoryEventStore {
	return &MemoryEventStore{
		events: make(map[string]Event),
	}
}

// Save 保存事件
func (s *MemoryEventStore) Save(ctx context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events[event.ID] = event
	return nil
}

// Get 获取事件
func (s *MemoryEventStore) Get(ctx context.Context, id string) (*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, exists := s.events[id]
	if !exists {
		return nil, fmt.Errorf("event not found: %s", id)
	}

	return &event, nil
}

// List 列出事件
func (s *MemoryEventStore) List(ctx context.Context, filter EventFilter) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var events []Event
	for _, event := range s.events {
		if filter == nil || filter(event) {
			events = append(events, event)
		}
	}

	return events, nil
}

// Delete 删除事件
func (s *MemoryEventStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.events, id)
	return nil
}
