package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Context().Value("request_id")
		if requestID == nil {
			t.Error("Expected request_id in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestID(handler)

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("Expected X-Request-ID header")
	}
}

func TestRequestID_Existing(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Context().Value("request_id")
		if requestID != "existing-id" {
			t.Errorf("Expected existing-id, got %v", requestID)
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestID(handler)

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "existing-id")
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") != "existing-id" {
		t.Error("Expected X-Request-ID to be existing-id")
	}
}

func TestCORS(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CORS([]string{"*"})

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://example.com")
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Error("Expected Access-Control-Allow-Origin header")
	}
}

func TestCORS_Preflight(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for OPTIONS")
	})

	middleware := CORS([]string{"*"})

	req, _ := http.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "http://example.com")
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestRateLimit(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RateLimit(2)

	// 第一个请求
	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// 第二个请求
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// 第三个请求（应该被限制）
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rr.Code)
	}
}

func TestAuth(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := Auth("test-api-key")

	// 无认证头
	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}

	// 错误的 API Key
	req, _ = http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}

	// 正确的 API Key
	req, _ = http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer test-api-key")
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestAuth_Disabled(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 空 API Key 表示禁用认证
	middleware := Auth("")

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(2)

	// 前两个请求应该允许
	if !limiter.Allow() {
		t.Error("Expected first request to be allowed")
	}

	if !limiter.Allow() {
		t.Error("Expected second request to be allowed")
	}

	// 第三个请求应该被限制
	if limiter.Allow() {
		t.Error("Expected third request to be denied")
	}
}

func TestChain(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-1", "true")
			next.ServeHTTP(w, r)
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-2", "true")
			next.ServeHTTP(w, r)
		})
	}

	chained := Chain(handler, middleware1, middleware2)

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	chained.ServeHTTP(rr, req)

	if rr.Header().Get("X-Middleware-1") != "true" {
		t.Error("Expected X-Middleware-1 header")
	}

	if rr.Header().Get("X-Middleware-2") != "true" {
		t.Error("Expected X-Middleware-2 header")
	}
}
