package collaboration

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/harness-engineering/harness/models"
)

// MessageBus Agent 间消息总线
type MessageBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan models.Message // agentID -> channels
	broadcast   []chan models.Message            // 广播频道
	history     []models.Message                // 消息历史
	maxHistory  int
}

// NewMessageBus 创建消息总线
func NewMessageBus(maxHistory int) *MessageBus {
	return &MessageBus{
		subscribers: make(map[string][]chan models.Message),
		maxHistory:  maxHistory,
	}
}

// Subscribe 订阅消息（Agent 注册自己的消息频道）
func (mb *MessageBus) Subscribe(agentID string) chan models.Message {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	ch := make(chan models.Message, 100)
	mb.subscribers[agentID] = append(mb.subscribers[agentID], ch)
	return ch
}

// Unsubscribe 取消订阅
func (mb *MessageBus) Unsubscribe(agentID string, ch chan models.Message) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	subs := mb.subscribers[agentID]
	for i, sub := range subs {
		if sub == ch {
			mb.subscribers[agentID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
}

// SubscribeBroadcast 订阅广播消息
func (mb *MessageBus) SubscribeBroadcast() chan models.Message {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	ch := make(chan models.Message, 100)
	mb.broadcast = append(mb.broadcast, ch)
	return ch
}

// Publish 发布消息（带背压处理）
func (mb *MessageBus) Publish(msg models.Message) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if msg.ID == "" {
		msg.ID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// 记录历史（环形缓冲）
	if len(mb.history) >= mb.maxHistory {
		// 移除最旧的 10% 消息
		trimCount := mb.maxHistory / 10
		if trimCount < 1 {
			trimCount = 1
		}
		mb.history = mb.history[trimCount:]
	}
	mb.history = append(mb.history, msg)

	if msg.To == "" {
		// 广播消息
		dropped := 0
		for _, ch := range mb.broadcast {
			select {
			case ch <- msg:
			default:
				dropped++
			}
		}
		if dropped > 0 {
			slog.Warn("broadcast message dropped for some subscribers",
				"msg_id", msg.ID, "dropped", dropped, "total", len(mb.broadcast))
		}
	} else {
		// 点对点消息
		subs, exists := mb.subscribers[msg.To]
		if !exists {
			return fmt.Errorf("subscriber not found: %s", msg.To)
		}
		dropped := 0
		for _, ch := range subs {
			select {
			case ch <- msg:
			default:
				dropped++
			}
		}
		if dropped > 0 {
			slog.Warn("message dropped for subscriber",
				"msg_id", msg.ID, "to", msg.To, "dropped", dropped)
		}
	}

	return nil
}

// GetHistory 获取消息历史
func (mb *MessageBus) GetHistory(collabID string) []models.Message {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	var result []models.Message
	for _, msg := range mb.history {
		if collabID == "" || msg.CollabID == collabID {
			result = append(result, msg)
		}
	}
	return result
}

// GetMessagesForAgent 获取 Agent 的消息历史
func (mb *MessageBus) GetMessagesForAgent(agentID string) []models.Message {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	var result []models.Message
	for _, msg := range mb.history {
		if msg.From == agentID || msg.To == agentID {
			result = append(result, msg)
		}
	}
	return result
}

// WaitForReply 等待回复消息（带超时）
func (mb *MessageBus) WaitForReply(agentID string, replyTo string, timeout time.Duration) (*models.Message, error) {
	ch := mb.Subscribe(agentID)
	defer mb.Unsubscribe(agentID, ch)

	deadline := time.After(timeout)
	for {
		select {
		case msg := <-ch:
			if msg.ReplyTo == replyTo {
				return &msg, nil
			}
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for reply to %s", replyTo)
		}
	}
}

// WaitForType 等待特定类型的消息
func (mb *MessageBus) WaitForType(agentID string, msgType models.MessageType, timeout time.Duration) (*models.Message, error) {
	ch := mb.Subscribe(agentID)
	defer mb.Unsubscribe(agentID, ch)

	deadline := time.After(timeout)
	for {
		select {
		case msg := <-ch:
			if msg.Type == msgType {
				return &msg, nil
			}
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for message type %s", msgType)
		}
	}
}
