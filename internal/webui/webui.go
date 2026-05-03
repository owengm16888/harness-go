package webui

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"text/template"

	"github.com/gorilla/mux"
)

//go:embed static/*
var staticFiles embed.FS

// WebUI Web UI 服务器
type WebUI struct {
	router *mux.Router
}

// New 创建 Web UI
func New() *WebUI {
	ui := &WebUI{
		router: mux.NewRouter(),
	}
	ui.setupRoutes()
	return ui
}

// setupRoutes 设置路由
func (ui *WebUI) setupRoutes() {
	// 静态文件
	staticFS, _ := fs.Sub(staticFiles, "static")
	ui.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// 页面路由
	ui.router.HandleFunc("/", ui.indexHandler).Methods("GET")
	ui.router.HandleFunc("/tasks", ui.tasksHandler).Methods("GET")
	ui.router.HandleFunc("/tasks/{id}", ui.taskDetailHandler).Methods("GET")
	ui.router.HandleFunc("/adapters", ui.adaptersHandler).Methods("GET")
	ui.router.HandleFunc("/settings", ui.settingsHandler).Methods("GET")

	// API 路由
	api := ui.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/tasks", ui.apiGetTasks).Methods("GET")
	api.HandleFunc("/tasks", ui.apiCreateTask).Methods("POST")
	api.HandleFunc("/tasks/{id}", ui.apiGetTask).Methods("GET")
	api.HandleFunc("/tasks/{id}", ui.apiUpdateTask).Methods("PUT")
	api.HandleFunc("/tasks/{id}", ui.apiDeleteTask).Methods("DELETE")
	api.HandleFunc("/tasks/{id}/execute", ui.apiExecuteTask).Methods("POST")
	api.HandleFunc("/adapters", ui.apiGetAdapters).Methods("GET")
	api.HandleFunc("/stats", ui.apiGetStats).Methods("GET")
}

// Router 获取路由器
func (ui *WebUI) Router() *mux.Router {
	return ui.router
}

// indexHandler 首页处理器
func (ui *WebUI) indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(staticFiles, "static/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title": "Harness Engineering",
	}

	tmpl.Execute(w, data)
}

// tasksHandler 任务列表处理器
func (ui *WebUI) tasksHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(staticFiles, "static/tasks.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, nil)
}

// taskDetailHandler 任务详情处理器
func (ui *WebUI) taskDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	tmpl, err := template.ParseFS(staticFiles, "static/task-detail.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"TaskID": id,
	}

	tmpl.Execute(w, data)
}

// adaptersHandler 适配器处理器
func (ui *WebUI) adaptersHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(staticFiles, "static/adapters.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, nil)
}

// settingsHandler 设置处理器
func (ui *WebUI) settingsHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(staticFiles, "static/settings.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, nil)
}

// apiGetTasks 获取任务列表
func (ui *WebUI) apiGetTasks(w http.ResponseWriter, r *http.Request) {
	// TODO: 从引擎获取任务
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `[]`)
}

// apiCreateTask 创建任务
func (ui *WebUI) apiCreateTask(w http.ResponseWriter, r *http.Request) {
	// TODO: 创建任务
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"status": "created"}`)
}

// apiGetTask 获取任务
func (ui *WebUI) apiGetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"id": "%s"}`, id)
}

// apiUpdateTask 更新任务
func (ui *WebUI) apiUpdateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "updated"}`)
}

// apiDeleteTask 删除任务
func (ui *WebUI) apiDeleteTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "deleted"}`)
}

// apiExecuteTask 执行任务
func (ui *WebUI) apiExecuteTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "executing"}`)
}

// apiGetAdapters 获取适配器列表
func (ui *WebUI) apiGetAdapters(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `[{"name": "claude-code", "status": "online"}, {"name": "hermes", "status": "online"}, {"name": "codex-cli", "status": "offline"}]`)
}

// apiGetStats 获取统计信息
func (ui *WebUI) apiGetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"total_tasks": 12, "running_tasks": 3, "success_rate": 85, "adapters": 3}`)
}
