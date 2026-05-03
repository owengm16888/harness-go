package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"log/slog"
)

// RequestID 请求 ID 中间件
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		ctx := context.WithValue(r.Context(), "request_id", requestID)
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logging 日志中间件
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 包装 ResponseWriter 以捕获状态码
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		slog.Info("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration", duration.String(),
			"remote_addr", r.RemoteAddr,
			"request_id", r.Context().Value("request_id"),
		)
	})
}

// Recovery 恢复中间件
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()

				slog.Error("panic recovered",
					"error", err,
					"stack", string(stack),
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", r.Context().Value("request_id"),
				)

				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// CORS CORS 中间件
func CORS(allowOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			allowed := false
			for _, allowedOrigin := range allowOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit 限流中间件
func RateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Auth 认证中间件
func Auth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// 检查 Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token == apiKey {
					next.ServeHTTP(w, r)
					return
				}
			}

			// 检查 query parameter
			queryKey := r.URL.Query().Get("api_key")
			if queryKey == apiKey {
				next.ServeHTTP(w, r)
				return
			}

			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}

// Timeout 超时中间件
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Chain 链式组合中间件
func Chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// responseWriter 包装 ResponseWriter 以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader 写入状态码
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// RateLimiter 限流器
type RateLimiter struct {
	requests []time.Time
	limit    int
}

// NewRateLimiter 创建限流器
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		requests: []time.Time{},
		limit:    requestsPerMinute,
	}
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow() bool {
	now := time.Now()
	windowStart := now.Add(-time.Minute)

	// 清理过期请求
	validRequests := []time.Time{}
	for _, t := range rl.requests {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}
	rl.requests = validRequests

	// 检查是否超过限制
	if len(rl.requests) >= rl.limit {
		return false
	}

	// 添加当前请求
	rl.requests = append(rl.requests, now)
	return true
}

// generateRequestID 生成请求 ID
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
