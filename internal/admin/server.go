package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/harness-engineering/harness/internal/core"
	"github.com/harness-engineering/harness/models"
	"github.com/harness-engineering/harness/pkg/metrics"
)

// AdminServer 管理服务器
type AdminServer struct {
	engine *core.Engine
	router *mux.Router
	addr   string
}

// NewAdminServer 创建管理服务器
func NewAdminServer(engine *core.Engine, addr string) *AdminServer {
	s := &AdminServer{
		engine: engine,
		router: mux.NewRouter(),
		addr:   addr,
	}

	s.setupRoutes()
	return s
}

// setupRoutes 设置路由
func (s *AdminServer) setupRoutes() {
	// 仪表板 API
	s.router.HandleFunc("/admin/dashboard", s.dashboard).Methods("GET")
	s.router.HandleFunc("/admin/stats", s.stats).Methods("GET")

	// 任务管理
	s.router.HandleFunc("/admin/tasks", s.listTasks).Methods("GET")
	s.router.HandleFunc("/admin/tasks/{id}", s.getTask).Methods("GET")
	s.router.HandleFunc("/admin/tasks/{id}/cancel", s.cancelTask).Methods("POST")

	// 会话管理
	s.router.HandleFunc("/admin/sessions", s.listSessions).Methods("GET")
	s.router.HandleFunc("/admin/sessions/{id}", s.getSession).Methods("GET")
	s.router.HandleFunc("/admin/sessions/{id}/delete", s.deleteSession).Methods("POST")

	// 知识管理
	s.router.HandleFunc("/admin/knowledge", s.listKnowledge).Methods("GET")
	s.router.HandleFunc("/admin/knowledge/{id}", s.getKnowledge).Methods("GET")
	s.router.HandleFunc("/admin/knowledge/{id}/delete", s.deleteKnowledge).Methods("POST")

	// 模式管理
	s.router.HandleFunc("/admin/patterns", s.listPatterns).Methods("GET")
	s.router.HandleFunc("/admin/patterns/{id}", s.getPattern).Methods("GET")
	s.router.HandleFunc("/admin/patterns/{id}/delete", s.deletePattern).Methods("POST")

	// 指标
	s.router.HandleFunc("/admin/metrics", s.getMetrics).Methods("GET")
	s.router.HandleFunc("/admin/metrics/prometheus", s.prometheusMetrics).Methods("GET")

	// 健康检查
	s.router.HandleFunc("/admin/health", s.health).Methods("GET")
	s.router.HandleFunc("/admin/ready", s.ready).Methods("GET")

	// 配置
	s.router.HandleFunc("/admin/config", s.getConfig).Methods("GET")

	// 日志
	s.router.HandleFunc("/admin/logs", s.getLogs).Methods("GET")
}

// Start 启动服务器
func (s *AdminServer) Start() error {
	srv := &http.Server{
		Handler:      s.router,
		Addr:         s.addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return srv.ListenAndServe()
}

// dashboard 仪表板
func (s *AdminServer) dashboard(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"timestamp": time.Now(),
		"stats":     s.getStats(),
		"metrics":   s.getMetricsData(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// stats 统计信息
func (s *AdminServer) stats(w http.ResponseWriter, r *http.Request) {
	stats := s.getStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// listTasks 列出任务
func (s *AdminServer) listTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tasks, err := s.engine.TaskManager().ListTasks(ctx, models.TaskFilter{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// getTask 获取任务
func (s *AdminServer) getTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := r.Context()
	task, err := s.engine.TaskManager().GetTask(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// cancelTask 取消任务
func (s *AdminServer) cancelTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := r.Context()
	if err := s.engine.TaskManager().CancelTask(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// listSessions 列出会话
func (s *AdminServer) listSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessions, err := s.engine.StateManager().ListSessions(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// getSession 获取会话
func (s *AdminServer) getSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := r.Context()
	state, err := s.engine.StateManager().GetState(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// deleteSession 删除会话
func (s *AdminServer) deleteSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := r.Context()
	if err := s.engine.StateManager().DeleteSession(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// listKnowledge 列出知识
func (s *AdminServer) listKnowledge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entries, err := s.engine.Knowledge().ListEntries(ctx, 0, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// getKnowledge 获取知识
func (s *AdminServer) getKnowledge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := r.Context()
	entry, err := s.engine.Knowledge().GetEntry(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// deleteKnowledge 删除知识
func (s *AdminServer) deleteKnowledge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := r.Context()
	if err := s.engine.Knowledge().DeleteEntry(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// listPatterns 列出模式
func (s *AdminServer) listPatterns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	patterns, err := s.engine.Pattern().ListPatterns(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(patterns)
}

// getPattern 获取模式
func (s *AdminServer) getPattern(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := r.Context()
	pattern, err := s.engine.Pattern().GetPattern(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pattern)
}

// deletePattern 删除模式
func (s *AdminServer) deletePattern(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := r.Context()
	if err := s.engine.Pattern().DeletePattern(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getMetrics 获取指标
func (s *AdminServer) getMetrics(w http.ResponseWriter, r *http.Request) {
	metricsData := s.getMetricsData()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metricsData)
}

// prometheusMetrics Prometheus 指标
func (s *AdminServer) prometheusMetrics(w http.ResponseWriter, r *http.Request) {
	registry := metrics.GetDefaultRegistry()
	allMetrics := registry.GetAll()

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(metrics.FormatMetrics(allMetrics)))
}

// health 健康检查
func (s *AdminServer) health(w http.ResponseWriter, r *http.Request) {
	health := map[string]any{
		"status":    "ok",
		"timestamp": time.Now(),
		"version":   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// ready 就绪检查
func (s *AdminServer) ready(w http.ResponseWriter, r *http.Request) {
	ready := map[string]any{
		"ready":     true,
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ready)
}

// getConfig 获取配置
func (s *AdminServer) getConfig(w http.ResponseWriter, r *http.Request) {
	// 返回配置（隐藏敏感信息）
	config := map[string]any{
		"server": map[string]any{
			"addr": ":8080",
		},
		"engine": map[string]any{
			"max_concurrent_tasks": 10,
			"task_timeout":        "5m",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// getLogs 获取日志
func (s *AdminServer) getLogs(w http.ResponseWriter, r *http.Request) {
	// 返回最近的日志
	logs := []map[string]any{
		{
			"timestamp": time.Now(),
			"level":     "info",
			"message":   "System started",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// getStats 获取统计信息
func (s *AdminServer) getStats() map[string]any {
	metrics := s.engine.Monitor().GetMetrics()

	return map[string]any{
		"tasks": map[string]any{
			"total":   metrics.TotalTasks,
			"success": metrics.SuccessTasks,
			"failed":  metrics.FailedTasks,
		},
		"feedback": map[string]any{
			"total":  metrics.TotalFeedback,
			"passed": metrics.PassedFeedback,
			"fixed":  metrics.FixedFeedback,
		},
		"performance": map[string]any{
			"average_duration": metrics.AverageDuration.String(),
		},
	}
}

// getMetricsData 获取指标数据
func (s *AdminServer) getMetricsData() map[string]any {
	registry := metrics.GetDefaultRegistry()
	allMetrics := registry.GetAll()

	result := map[string]any{
		"timestamp": time.Now(),
		"metrics":   allMetrics,
	}

	return result
}
