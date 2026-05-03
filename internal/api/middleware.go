package api

import (
	"encoding/json"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// ============================================================
// WebSocket 实时推送 — 任务状态变更通知
// ============================================================

// WSClient WebSocket 客户端
type WSClient struct {
	ID       string
	Filter   string // 订阅过滤: "all", collabID, taskID
	Send     chan []byte
	Server   *WSServer
	mu       sync.Mutex
	closed   bool
}

// WSServer WebSocket 服务器
type WSServer struct {
	mu         sync.RWMutex
	clients    map[string]*WSClient
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
}

// WSMessage WebSocket 消息
type WSMessage struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Payload any    `json:"payload"`
	Time    string `json:"time"`
}

// NewWSServer 创建 WebSocket 服务器
func NewWSServer() *WSServer {
	s := &WSServer{
		clients:    make(map[string]*WSClient),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
	go s.run()
	return s
}

// run 主循环
func (s *WSServer) run() {
	for {
		select {
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client.ID] = client
			s.mu.Unlock()
			slog.Info("websocket client connected", "client_id", client.ID, "total", len(s.clients))

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client.ID]; ok {
				delete(s.clients, client.ID)
				close(client.Send)
			}
			s.mu.Unlock()
			slog.Info("websocket client disconnected", "client_id", client.ID)

		case message := <-s.broadcast:
			s.mu.RLock()
			for _, client := range s.clients {
				select {
				case client.Send <- message:
				default:
					// 客户端缓冲满，断开
					go func(c *WSClient) {
						s.unregister <- c
					}(client)
				}
			}
			s.mu.RUnlock()
		}
	}
}

// Broadcast 广播消息
func (s *WSServer) Broadcast(msgType string, id string, payload any) {
	msg := WSMessage{
		Type:    msgType,
		ID:      id,
		Payload: payload,
		Time:    time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal ws message", "error", err)
		return
	}

	select {
	case s.broadcast <- data:
	default:
		slog.Warn("websocket broadcast channel full, dropping message")
	}
}

// NotifyTaskUpdate 通知任务状态更新
func (s *WSServer) NotifyTaskUpdate(taskID string, status string, result any) {
	s.Broadcast("task_update", taskID, map[string]any{
		"task_id": taskID,
		"status":  status,
		"result":  result,
	})
}

// NotifyCollabUpdate 通知协作状态更新
func (s *WSServer) NotifyCollabUpdate(collabID string, status string) {
	s.Broadcast("collab_update", collabID, map[string]any{
		"collab_id": collabID,
		"status":    status,
	})
}

// ClientCount 获取连接数
func (s *WSServer) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// HandleWS WebSocket HTTP handler（使用长轮询模拟，避免外部依赖）
func (s *WSServer) HandleWS(w http.ResponseWriter, r *http.Request) {
	// 使用 Server-Sent Events (SSE) 作为 WebSocket 替代方案
	// 无需外部依赖，浏览器原生支持
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientID := fmt.Sprintf("sse-%d", time.Now().UnixNano())
	client := &WSClient{
		ID:     clientID,
		Filter: r.URL.Query().Get("filter"),
		Send:   make(chan []byte, 64),
		Server: s,
	}

	s.register <- client
	defer func() { s.unregister <- client }()

	// 发送连接确认
	fmt.Fprintf(w, "event: connected\ndata: {\"client_id\":\"%s\"}\n\n", clientID)
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-client.Send:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// ============================================================
// 令牌桶限流中间件
// ============================================================

// RateLimiter 令牌桶限流器
type RateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	maxTokens float64
	refillRate float64 // 每秒补充的令牌数
	lastRefill time.Time
}

// NewRateLimiter 创建限流器
func NewRateLimiter(maxTokens, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"rate_limit_exceeded","message":"too many requests, please retry later"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ============================================================
// 输入校验
// ============================================================

// CompressionMiddleware 响应压缩中间件（gzip）
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查客户端是否支持 gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// 包装 ResponseWriter
		gz := gzip.NewWriter(w)
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")

		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gz}, r)
	})
}

// gzipResponseWriter gzip 响应写入器
type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (grw *gzipResponseWriter) Write(b []byte) (int, error) {
	return grw.Writer.Write(b)
}

// ValidateTaskRequest 校验任务请求
func ValidateTaskRequest(task map[string]any) error {
	if _, ok := task["id"]; !ok {
		return fmt.Errorf("task.id is required")
	}
	if id, ok := task["id"].(string); !ok || len(id) == 0 || len(id) > 256 {
		return fmt.Errorf("task.id must be a non-empty string (max 256 chars)")
	}
	if _, ok := task["type"]; !ok {
		return fmt.Errorf("task.type is required")
	}
	if desc, ok := task["description"].(string); ok && len(desc) > 10000 {
		return fmt.Errorf("task.description must be <= 10000 chars (got %d)", len(desc))
	}
	return nil
}

// RequestIDMiddleware 请求 ID 追踪中间件
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", requestID)
		r.Header.Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware 跨域资源共享中间件
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// 检查来源是否允许
			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Request-ID")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// 预检请求直接返回
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SanitizeString 清理字符串输入
func SanitizeString(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	// 移除控制字符
	var result []rune
	for _, r := range s {
		if r >= 32 || r == '\n' || r == '\t' {
			result = append(result, r)
		}
	}
	return string(result)
}
