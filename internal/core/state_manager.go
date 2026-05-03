package core

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
)

// StateManager 管理系统状态
type StateManager struct {
	mu       sync.RWMutex
	sessions map[string]*storage.Session
	store    storage.StateStore
	notifier *EventNotifier
}

// EventNotifier 事件通知器
type EventNotifier struct {
	subscribers []chan models.Event
}

// NewEventNotifier 创建事件通知器
func NewEventNotifier() *EventNotifier {
	return &EventNotifier{
		subscribers: []chan models.Event{},
	}
}

// Subscribe 订阅事件
func (n *EventNotifier) Subscribe() chan models.Event {
	ch := make(chan models.Event, 100)
	n.subscribers = append(n.subscribers, ch)
	return ch
}

// Notify 通知事件
func (n *EventNotifier) Notify(event models.Event) {
	for _, ch := range n.subscribers {
		select {
		case ch <- event:
		default:
			// 队列满，丢弃事件
		}
	}
}

// NewStateManager 创建状态管理器
func NewStateManager(store storage.StateStore, notifier *EventNotifier) *StateManager {
	return &StateManager{
		sessions: make(map[string]*storage.Session),
		store:    store,
		notifier: notifier,
	}
}

// LoadFromStorage 从持久化存储加载所有会话到内存
func (sm *StateManager) LoadFromStorage(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessions, err := sm.store.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to load sessions from storage: %w", err)
	}

	for _, session := range sessions {
		sm.sessions[session.ID] = session
	}

	return nil
}

// CreateSession 创建新会话
func (sm *StateManager) CreateSession(ctx context.Context, env string) (*storage.Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := &storage.Session{
		ID:          generateID(),
		Environment: env,
		State: models.State{
			SessionID:   generateID(),
			Environment: env,
			Tasks:       []models.Task{},
			Context:     make(map[string]any),
			Timestamp:   time.Now(),
		},
		History:   []storage.StateSnapshot{},
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	// 保存到存储
	if err := sm.store.SaveSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// 添加到内存
	sm.sessions[session.ID] = session

	// 通知事件
	sm.notifier.Notify(models.Event{
		Type:      "session_created",
		SessionID: session.ID,
		Data:      session,
		Timestamp: time.Now(),
	})

	return session, nil
}

// UpdateState 更新状态
func (sm *StateManager) UpdateState(ctx context.Context, sessionID string, update models.StateUpdate) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// 保存当前状态到历史
	session.History = append(session.History, storage.StateSnapshot{
		State:     session.State,
		Timestamp: time.Now().Format(time.RFC3339),
		Reason:    update.Reason,
	})

	// 应用更新
	if err := sm.applyUpdate(session, update); err != nil {
		return fmt.Errorf("failed to apply update: %w", err)
	}

	session.UpdatedAt = time.Now().Format(time.RFC3339)

	// 保存到存储
	if err := sm.store.SaveSession(ctx, session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// 通知事件
	sm.notifier.Notify(models.Event{
		Type:      "state_updated",
		SessionID: sessionID,
		Data:      update,
		Timestamp: time.Now(),
	})

	return nil
}

// GetState 获取状态
func (sm *StateManager) GetState(ctx context.Context, sessionID string) (*models.State, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return &session.State, nil
}

// GetHistory 获取状态历史
func (sm *StateManager) GetHistory(ctx context.Context, sessionID string) ([]storage.StateSnapshot, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session.History, nil
}

// ListSessions 列出会话
func (sm *StateManager) ListSessions(ctx context.Context) ([]*storage.Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var sessions []*storage.Session
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// DeleteSession 删除会话
func (sm *StateManager) DeleteSession(ctx context.Context, sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// 从存储删除
	if err := sm.store.DeleteSession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// 从内存删除
	delete(sm.sessions, sessionID)

	// 通知事件
	sm.notifier.Notify(models.Event{
		Type:      "session_deleted",
		SessionID: sessionID,
		Data:      session,
		Timestamp: time.Now(),
	})

	return nil
}

// applyUpdate 应用状态更新
func (sm *StateManager) applyUpdate(session *storage.Session, update models.StateUpdate) error {
	switch update.Type {
	case "add_task":
		var task models.Task
		if err := json.Unmarshal(update.Data, &task); err != nil {
			return err
		}
		session.State.Tasks = append(session.State.Tasks, task)

	case "update_task":
		var taskUpdate models.TaskUpdate
		if err := json.Unmarshal(update.Data, &taskUpdate); err != nil {
			return err
		}
		for i, task := range session.State.Tasks {
			if task.ID == taskUpdate.ID {
				session.State.Tasks[i] = taskUpdate.Task
				break
			}
		}

	case "set_context":
		var contextUpdate models.ContextUpdate
		if err := json.Unmarshal(update.Data, &contextUpdate); err != nil {
			return err
		}
		for k, v := range contextUpdate {
			session.State.Context[k] = v
		}

	default:
		return fmt.Errorf("unknown update type: %s", update.Type)
	}

	return nil
}

// generateID 生成不重复的 ID（crypto/rand + 时间戳）
func generateID() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%s-%06d", time.Now().Format("20060102150405.000000000"), n.Int64())
}
