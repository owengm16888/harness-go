package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/internal/core"
	"github.com/harness-engineering/harness/models"
)

// Server HTTP API 服务器
type Server struct {
	config *config.Config
	engine *core.Engine
	server *http.Server
	router *mux.Router
}

// NewServer 创建 API 服务器
func NewServer(cfg *config.Config, engine *core.Engine) *Server {
	s := &Server{
		config: cfg,
		engine: engine,
		router: mux.NewRouter(),
	}

	s.setupRoutes()
	s.setupMiddleware()

	return s
}

// setupMiddleware 设置中间件
func (s *Server) setupMiddleware() {
	// 中间件链: Recovery -> RequestID -> Logging -> CORS -> RateLimit -> Handler
	s.router.Use(s.recoveryMiddleware)
	s.router.Use(s.requestIDMiddleware)
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.corsMiddleware)
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// 健康检查
	s.router.HandleFunc("/health", s.healthHandler).Methods("GET")
	s.router.HandleFunc("/ready", s.readyHandler).Methods("GET")

	// API v1
	v1 := s.router.PathPrefix("/api/v1").Subrouter()

	// 任务 API
	v1.HandleFunc("/tasks", s.createTask).Methods("POST")
	v1.HandleFunc("/tasks", s.listTasks).Methods("GET")
	v1.HandleFunc("/tasks/{id}", s.getTask).Methods("GET")
	v1.HandleFunc("/tasks/{id}/execute", s.executeTask).Methods("POST")
	v1.HandleFunc("/tasks/{id}/cancel", s.cancelTask).Methods("POST")

	// 知识 API
	v1.HandleFunc("/knowledge", s.addKnowledge).Methods("POST")
	v1.HandleFunc("/knowledge", s.listKnowledge).Methods("GET")
	v1.HandleFunc("/knowledge/search", s.searchKnowledge).Methods("GET")
	v1.HandleFunc("/knowledge/{id}", s.getKnowledge).Methods("GET")
	v1.HandleFunc("/knowledge/{id}", s.updateKnowledge).Methods("PUT")
	v1.HandleFunc("/knowledge/{id}", s.deleteKnowledge).Methods("DELETE")

	// 模式 API
	v1.HandleFunc("/patterns", s.addPattern).Methods("POST")
	v1.HandleFunc("/patterns", s.listPatterns).Methods("GET")
	v1.HandleFunc("/patterns/match", s.matchPattern).Methods("POST")
	v1.HandleFunc("/patterns/{id}", s.getPattern).Methods("GET")
	v1.HandleFunc("/patterns/{id}", s.updatePattern).Methods("PUT")
	v1.HandleFunc("/patterns/{id}", s.deletePattern).Methods("DELETE")

	// 监控 API
	v1.HandleFunc("/metrics", s.getMetrics).Methods("GET")

	// 适配器 API
	v1.HandleFunc("/adapters", s.listAdapters).Methods("GET")
	v1.HandleFunc("/adapters/{name}/state", s.getAdapterState).Methods("GET")
}

// Start 启动服务器
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:         s.config.Server.Addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s.server.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// 中间件

// recoveryMiddleware 恢复中间件
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "error", err, "path", r.URL.Path)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// requestIDMiddleware 请求 ID 中间件
func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware 日志中间件
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 包装 ResponseWriter 以捕获状态码
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration", time.Since(start),
			"remote", r.RemoteAddr,
		)
	})
}

// corsMiddleware CORS 中间件
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// responseWriter 响应写入器
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// 处理器

// healthHandler 健康检查
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := s.engine.HealthCheck(ctx); err != nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// readyHandler 就绪检查
func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	if s.engine.IsShutdown() {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "shutting_down",
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}

// createTask 创建任务
func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 设置默认值
	if task.ID == "" {
		task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	if task.Priority == 0 {
		task.Priority = 2 // medium
	}

	ctx := r.Context()

	// 创建任务
	if _, err := s.engine.TaskManager().CreateTask(ctx, task); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, task)
}

// listTasks 列出任务
func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 解析过滤器
	filter := models.TaskFilter{}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = status
	}
	if taskType := r.URL.Query().Get("type"); taskType != "" {
		filter.Type = taskType
	}

	tasks, err := s.engine.TaskManager().ListTasks(ctx, filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, tasks)
}

// getTask 获取任务
func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := r.Context()

	task, err := s.engine.TaskManager().GetTask(ctx, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, task)
}

// executeTask 执行任务
func (s *Server) executeTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// 解析请求
	var req struct {
		Adapter string `json:"adapter"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Adapter == "" {
		req.Adapter = "claude-code"
	}

	ctx := r.Context()

	// 获取任务
	task, err := s.engine.TaskManager().GetTask(ctx, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// 执行任务
	result, err := s.engine.ExecuteTask(ctx, req.Adapter, task.Task)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

// cancelTask 取消任务
func (s *Server) cancelTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := r.Context()

	if err := s.engine.TaskManager().CancelTask(ctx, id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "cancelled",
	})
}

// addKnowledge 添加知识
func (s *Server) addKnowledge(w http.ResponseWriter, r *http.Request) {
	var entry models.KnowledgeEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 设置默认 ID
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("kb-%d", time.Now().UnixNano())
	}

	ctx := r.Context()

	if err := s.engine.Knowledge().AddEntry(ctx, entry); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, entry)
}

// listKnowledge 列出知识
func (s *Server) listKnowledge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 解析分页参数
	offset := 0
	limit := 50
	if v := r.URL.Query().Get("offset"); v != "" {
		fmt.Sscanf(v, "%d", &offset)
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
		if limit > 200 {
			limit = 200
		}
	}

	entries, err := s.engine.Knowledge().ListEntries(ctx, offset, limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回空数组而非 null
	if entries == nil {
		entries = []*models.KnowledgeEntry{}
	}

	s.writeJSON(w, http.StatusOK, entries)
}

// getKnowledge 获取单条知识
func (s *Server) getKnowledge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ctx := r.Context()

	entry, err := s.engine.Knowledge().GetEntry(ctx, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, entry)
}

// updateKnowledge 更新知识
func (s *Server) updateKnowledge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ctx := r.Context()

	var update models.KnowledgeUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.engine.Knowledge().UpdateEntry(ctx, id, update); err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// 返回更新后的条目
	entry, _ := s.engine.Knowledge().GetEntry(ctx, id)
	s.writeJSON(w, http.StatusOK, entry)
}

// deleteKnowledge 删除知识
func (s *Server) deleteKnowledge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ctx := r.Context()

	if err := s.engine.Knowledge().DeleteEntry(ctx, id); err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

// searchKnowledge 搜索知识
func (s *Server) searchKnowledge(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		s.writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	ctx := r.Context()

	results, err := s.engine.Knowledge().Search(ctx, query, 10)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回空数组而非 null
	if results == nil {
		results = []*models.KnowledgeEntry{}
	}

	s.writeJSON(w, http.StatusOK, results)
}

// addPattern 添加模式
func (s *Server) addPattern(w http.ResponseWriter, r *http.Request) {
	var pattern models.Pattern
	if err := json.NewDecoder(r.Body).Decode(&pattern); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 设置默认 ID
	if pattern.ID == "" {
		pattern.ID = fmt.Sprintf("pat-%d", time.Now().UnixNano())
	}

	ctx := r.Context()

	if err := s.engine.Pattern().AddPattern(ctx, pattern); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, pattern)
}

// listPatterns 列出模式
func (s *Server) listPatterns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	patterns, err := s.engine.Pattern().ListPatterns(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回空数组而非 null
	if patterns == nil {
		patterns = []*models.Pattern{}
	}

	s.writeJSON(w, http.StatusOK, patterns)
}

// getPattern 获取单条模式
func (s *Server) getPattern(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ctx := r.Context()

	pattern, err := s.engine.Pattern().GetPattern(ctx, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, pattern)
}

// updatePattern 更新模式
func (s *Server) updatePattern(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ctx := r.Context()

	var update models.PatternUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.engine.Pattern().UpdatePattern(ctx, id, update); err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// 返回更新后的模式
	pattern, _ := s.engine.Pattern().GetPattern(ctx, id)
	s.writeJSON(w, http.StatusOK, pattern)
}

// deletePattern 删除模式
func (s *Server) deletePattern(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ctx := r.Context()

	if err := s.engine.Pattern().DeletePattern(ctx, id); err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

// matchPattern 匹配模式
func (s *Server) matchPattern(w http.ResponseWriter, r *http.Request) {
	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body: expected Task JSON")
		return
	}

	ctx := r.Context()

	matched, err := s.engine.Pattern().Match(ctx, task)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回空数组而非 null
	if matched == nil {
		matched = []*models.Pattern{}
	}

	s.writeJSON(w, http.StatusOK, matched)
}

// getMetrics 获取指标
func (s *Server) getMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.engine.Monitor().GetMetrics()
	s.writeJSON(w, http.StatusOK, metrics)
}

// listAdapters 列出适配器
func (s *Server) listAdapters(w http.ResponseWriter, r *http.Request) {
	adapters := s.engine.ListAdapters()
	s.writeJSON(w, http.StatusOK, adapters)
}

// getAdapterState 获取适配器状态
func (s *Server) getAdapterState(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	ctx := r.Context()

	state, err := s.engine.GetState(ctx, name)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, state)
}

// 辅助方法

// writeJSON 写入 JSON 响应
func (s *Server) writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func (s *Server) writeError(w http.ResponseWriter, code int, message string) {
	s.writeJSON(w, code, map[string]string{
		"error": message,
	})
}
