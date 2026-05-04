package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/internal/collaboration"
	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
	"github.com/harness-engineering/harness/pkg/cache"
	pkgevent "github.com/harness-engineering/harness/pkg/event"
	"github.com/harness-engineering/harness/pkg/resilience"
	pkgwebhook "github.com/harness-engineering/harness/pkg/webhook"
)

// Adapter 是环境适配器接口
type Adapter interface {
	Name() string
	Initialize(ctx context.Context, config config.AdapterConfig) error
	ExecuteTask(ctx context.Context, task models.Task) (models.Result, error)
	GetState(ctx context.Context) (models.State, error)
	Cleanup(ctx context.Context) error
}

// Engine 是 Harness 的核心引擎
type Engine struct {
	mu            sync.RWMutex
	config        *config.Config
	adapters      map[string]Adapter
	taskManager   *TaskManager
	stateManager  *StateManager
	feedbackLoop  *FeedbackLoop
	knowledge     *KnowledgeBase
	pattern       *PatternEngine
	monitor       *Monitor
	storage       storage.Storage
	semaphore     chan struct{} // 并发信号量
	registry      *collaboration.Registry
	bus           *collaboration.MessageBus
	orchestrator  *collaboration.Orchestrator
	eventBus      *pkgevent.EventBus
	cache         *cache.MemoryCache
	webhooks      *pkgwebhook.WebhookManager
	fallbackOrder []string // 适配器降级顺序
	shutdownOnce  sync.Once
	shutdownCh    chan struct{}
}

// NewEngine 创建引擎
func NewEngine(cfg *config.Config, store storage.Storage) (*Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if store == nil {
		return nil, fmt.Errorf("storage is required")
	}

	engine := &Engine{
		config:     cfg,
		adapters:   make(map[string]Adapter),
		storage:    store,
		monitor:    NewMonitor(),
		shutdownCh: make(chan struct{}),
	}

	// 初始化任务管理器
	engine.taskManager = NewTaskManager(store, NewExecutor(engine))

	// 初始化状态管理器
	engine.stateManager = NewStateManager(store, NewEventNotifier())

	// 初始化反馈循环
	engine.feedbackLoop = NewFeedbackLoop(models.FeedbackConfig{
		MaxRetries: cfg.Feedback.MaxRetries,
		RetryDelay: cfg.Feedback.RetryDelay,
		AutoFix:    cfg.Feedback.AutoFix,
	}, engine.monitor)

	// 初始化知识库
	engine.knowledge = NewKnowledgeBase(store, NewIndexer())

	// 初始化模式引擎
	engine.pattern = NewPatternEngine(
		store,
		cfg.Patterns.MinSamples,
		cfg.Patterns.Threshold,
	)

	// 初始化全局事件总线
	engine.eventBus = pkgevent.NewEventBus(pkgevent.EventBusConfig{
		Async:      true,
		BufferSize: 1000,
	})
	engine.eventBus.SubscribeAll(func(ctx context.Context, event pkgevent.Event) error {
		slog.Debug("event", "type", event.Type, "source", event.Source)
		return nil
	})

	// 初始化 LRU 缓存
	engine.cache = cache.NewMemoryCache(1000, 5*time.Minute)

	// 初始化 Webhook 管理器
	engine.webhooks = pkgwebhook.NewWebhookManager(pkgwebhook.WebhookManagerConfig{
		Workers:   5,
		QueueSize: 1000,
		Timeout:   30 * time.Second,
	})

	// 初始化协作组件
	engine.registry = collaboration.NewRegistry()
	engine.bus = collaboration.NewMessageBus(1000)
	engine.orchestrator = collaboration.NewOrchestrator(
		engine.registry,
		engine.bus,
		func(ctx context.Context, adapterName string, task models.Task) (*models.Result, error) {
			return engine.ExecuteTask(ctx, adapterName, task)
		},
	)

	// 初始化并发信号量
	maxConcurrent := cfg.Engine.MaxConcurrentTasks
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	engine.semaphore = make(chan struct{}, maxConcurrent)

	return engine, nil
}

// RegisterAdapter 注册适配器（自动用 ResilientAdapter 包装）
func (e *Engine) RegisterAdapter(name string, adapter Adapter) error {
	if name == "" {
		return fmt.Errorf("adapter name is required")
	}
	if adapter == nil {
		return fmt.Errorf("adapter is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.adapters[name]; exists {
		return fmt.Errorf("adapter already registered: %s", name)
	}

	// 用 ResilientAdapter 包装 (熔断 + 重试)
	wrapped := NewResilientAdapterWrapper(adapter, name)
	e.adapters[name] = wrapped

	// 添加到降级顺序
	e.fallbackOrder = append(e.fallbackOrder, name)

	slog.Info("adapter registered", "name", name, "resilient", true)
	return nil
}

// ExecuteTaskWithFallback 带降级的任务执行
// 如果指定适配器失败 (熔断/不可用)，自动尝试下一个适配器
func (e *Engine) ExecuteTaskWithFallback(ctx context.Context, adapterName string, task models.Task) (*models.Result, error) {
	// 先尝试指定的适配器
	result, err := e.ExecuteTask(ctx, adapterName, task)
	if err == nil {
		return result, nil
	}

	slog.Warn("adapter failed, trying fallback",
		"adapter", adapterName, "error", err)

	// 遍历降级顺序，找下一个可用的适配器
	e.mu.RLock()
	fallbackOrder := make([]string, len(e.fallbackOrder))
	copy(fallbackOrder, e.fallbackOrder)
	e.mu.RUnlock()

	for _, fallbackName := range fallbackOrder {
		if fallbackName == adapterName {
			continue // 跳过已经失败的
		}

		// 检查熔断器状态
		if adapter, ok := e.adapters[fallbackName]; ok {
			if w, ok := adapter.(*resilientAdapterWrapper); ok {
				if w.GetCircuitState() == "open" {
					slog.Debug("fallback adapter circuit open, skipping",
						"adapter", fallbackName)
					continue
				}
			}
		}

		slog.Info("trying fallback adapter", "adapter", fallbackName)
		result, err := e.ExecuteTask(ctx, fallbackName, task)
		if err == nil {
			return result, nil
		}

		slog.Warn("fallback adapter also failed",
			"adapter", fallbackName, "error", err)
	}

	return nil, fmt.Errorf("all adapters failed for task %s", task.ID)
}

// GetAdapterCircuitStates 获取所有适配器的熔断器状态
func (e *Engine) GetAdapterCircuitStates() map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	states := make(map[string]string)
	for name, adapter := range e.adapters {
		if w, ok := adapter.(*resilientAdapterWrapper); ok {
			states[name] = w.GetCircuitState()
		} else {
			states[name] = "n/a"
		}
	}
	return states
}

// Initialize 初始化引擎
func (e *Engine) Initialize(ctx context.Context) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 从持久化存储加载数据
	if err := e.taskManager.LoadFromStorage(ctx); err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}
	if err := e.stateManager.LoadFromStorage(ctx); err != nil {
		return fmt.Errorf("failed to load sessions: %w", err)
	}
	if err := e.knowledge.LoadFromStorage(ctx); err != nil {
		return fmt.Errorf("failed to load knowledge: %w", err)
	}
	if err := e.pattern.LoadFromStorage(ctx); err != nil {
		return fmt.Errorf("failed to load patterns: %w", err)
	}

	// 初始化所有适配器
	for name, adapter := range e.adapters {
		var cfg config.AdapterConfig
		switch name {
		case "claude-code":
			cfg = e.config.Adapters.ClaudeCode
			// 注入数据库路径供面试适配器查询知识库
			if cfg.DBPath == "" {
				cfg.DBPath = e.config.Storage.Path
			}
		case "hermes":
			cfg = config.AdapterConfig{
				Enabled: e.config.Adapters.Hermes.Enabled,
			}
			if adapterHermes, ok := adapter.(interface{ SetConfig(config.HermesConfig) }); ok {
				adapterHermes.SetConfig(e.config.Adapters.Hermes)
			}
		case "codex-cli":
			cfg = e.config.Adapters.CodexCLI
		default:
			slog.Warn("unknown adapter, skipping", "name", name)
			continue
		}

		if cfg.Enabled {
			if err := adapter.Initialize(ctx, cfg); err != nil {
				return fmt.Errorf("failed to initialize adapter %s: %w", name, err)
			}
			slog.Info("adapter initialized", "name", name)
		}
	}

	slog.Info("engine initialized", "adapters", len(e.adapters))
	return nil
}

// ExecuteTask 执行任务（带并发控制）
func (e *Engine) ExecuteTask(ctx context.Context, adapterName string, task models.Task) (*models.Result, error) {
	// 获取并发信号量
	select {
	case e.semaphore <- struct{}{}:
		defer func() { <-e.semaphore }()
	case <-ctx.Done():
		return nil, fmt.Errorf("task cancelled waiting for execution slot: %w", ctx.Err())
	case <-e.shutdownCh:
		return nil, fmt.Errorf("engine is shutting down")
	}

	e.mu.RLock()
	adapter, exists := e.adapters[adapterName]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("adapter not found: %s", adapterName)
	}

	// 执行前验证
	if err := e.validateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("task validation failed: %w", err)
	}

	// 智能上下文注入: 拉取相关知识 + 匹配模式 + 元数据
	task = e.enrichTaskContext(ctx, task)

	// 记录开始时间
	startTime := time.Now()
	slog.Info("task execution started", "task_id", task.ID, "adapter", adapterName)

	// 执行任务
	result, err := adapter.ExecuteTask(ctx, task)
	if err != nil {
		e.monitor.RecordTask(models.Result{
			TaskID: task.ID,
			Status: models.TaskStatusFailed,
			Metrics: models.Metrics{Duration: time.Since(startTime)},
		})
		return nil, fmt.Errorf("task execution failed: %w", err)
	}

	// 处理反馈
	feedback, err := e.feedbackLoop.Process(ctx, result)
	if err != nil {
		slog.Warn("feedback processing failed", "task_id", task.ID, "error", err)
	}

	// 如果有违规且未修复，返回错误
	if feedback != nil && feedback.Status == "violations_found" {
		return &result, fmt.Errorf("task has violations: %v", feedback.Violations)
	}

	// 学习模式
	if err := e.pattern.Learn(ctx, models.Observation{
		Task:    task,
		Result:  result,
		Success: result.Status == models.TaskStatusCompleted,
	}); err != nil {
		slog.Warn("pattern learning failed", "task_id", task.ID, "error", err)
	}

	// 记录指标
	e.monitor.RecordTask(result)

	// 触发 Webhook
	e.fireWebhook(ctx, task, result)

	// 发布事件
	e.publishEvent(ctx, task, result)

	slog.Info("task execution completed",
		"task_id", task.ID,
		"adapter", adapterName,
		"status", result.Status,
		"duration", time.Since(startTime),
	)

	return &result, nil
}

// fireWebhook 触发 Webhook
func (e *Engine) fireWebhook(ctx context.Context, task models.Task, result models.Result) {
	if e.webhooks == nil {
		return
	}

	status := "completed"
	if result.Status == models.TaskStatusFailed {
		status = "failed"
	}

	if err := e.webhooks.Fire(ctx, "task."+status, map[string]any{
		"task_id":  task.ID,
		"type":     task.Type,
		"status":   status,
		"duration": result.Metrics.Duration.String(),
	}); err != nil {
		slog.Warn("webhook fire failed", "task_id", task.ID, "error", err)
	}
}

// publishEvent 发布事件
func (e *Engine) publishEvent(ctx context.Context, task models.Task, result models.Result) {
	if e.eventBus == nil {
		return
	}

	eventType := pkgevent.EventTaskCompleted
	if result.Status == models.TaskStatusFailed {
		eventType = pkgevent.EventTaskFailed
	}

	if err := e.eventBus.Publish(ctx, pkgevent.Event{
		Type:   eventType,
		Source: "engine",
		Data: map[string]any{
			"task_id": task.ID,
			"type":    task.Type,
			"status":  result.Status,
		},
		Timestamp: time.Now(),
	}); err != nil {
		slog.Warn("event publish failed", "task_id", task.ID, "error", err)
	}
}

// ProcessFeedback 处理反馈
func (e *Engine) ProcessFeedback(ctx context.Context, result models.Result) (*models.FeedbackResult, error) {
	return e.feedbackLoop.Process(ctx, result)
}

// GetState 获取状态
func (e *Engine) GetState(ctx context.Context, adapterName string) (*models.State, error) {
	e.mu.RLock()
	adapter, exists := e.adapters[adapterName]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("adapter not found: %s", adapterName)
	}

	state, err := adapter.GetState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	return &state, nil
}

// TaskManager 获取任务管理器
func (e *Engine) TaskManager() *TaskManager {
	return e.taskManager
}

// StateManager 获取状态管理器
func (e *Engine) StateManager() *StateManager {
	return e.stateManager
}

// Knowledge 获取知识库
func (e *Engine) Knowledge() *KnowledgeBase {
	return e.knowledge
}

// Pattern 获取模式引擎
func (e *Engine) Pattern() *PatternEngine {
	return e.pattern
}

// Monitor 获取监控器
func (e *Engine) Monitor() *Monitor {
	return e.monitor
}

// EventBus 获取全局事件总线
func (e *Engine) EventBus() *pkgevent.EventBus {
	return e.eventBus
}

// Cache 获取 LRU 缓存
func (e *Engine) Cache() *cache.MemoryCache {
	return e.cache
}

// Webhooks 获取 Webhook 管理器
func (e *Engine) Webhooks() *pkgwebhook.WebhookManager {
	return e.webhooks
}

// Registry 获取 Agent 注册表
func (e *Engine) Registry() *collaboration.Registry {
	return e.registry
}

// MessageBus 获取消息总线
func (e *Engine) MessageBus() *collaboration.MessageBus {
	return e.bus
}

// Orchestrator 获取协作编排器
func (e *Engine) Orchestrator() *collaboration.Orchestrator {
	return e.orchestrator
}

// KnowledgeHint 知识条目精简摘要 (注入到 task.Context)
type KnowledgeHint struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// PatternHint 模式匹配精简摘要 (注入到 task.Context)
type PatternHint struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	SuccessRate float64 `json:"success_rate"`
	UsageCount  int     `json:"usage_count"`
}

// ConstraintHint 约束条件精简摘要 (注入到 task.Context)
type ConstraintHint struct {
	Type     string `json:"type"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// NewResilientAdapterWrapper 用熔断器+重试包装适配器
func NewResilientAdapterWrapper(inner Adapter, name string) *resilientAdapterWrapper {
	return &resilientAdapterWrapper{
		inner: inner,
		name:  name,
		cb: resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
			Name:             fmt.Sprintf("adapter-%s", name),
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          30 * time.Second,
		}),
		retryCfg: resilience.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Second,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       true,
		},
	}
}

// resilientAdapterWrapper 弹性适配器包装 (熔断 + 重试)
type resilientAdapterWrapper struct {
	inner     Adapter
	name      string
	cb        *resilience.CircuitBreaker
	retryCfg  resilience.RetryConfig
}

func (w *resilientAdapterWrapper) Name() string { return w.name }

func (w *resilientAdapterWrapper) Initialize(ctx context.Context, cfg config.AdapterConfig) error {
	return w.inner.Initialize(ctx, cfg)
}

func (w *resilientAdapterWrapper) ExecuteTask(ctx context.Context, task models.Task) (models.Result, error) {
	var result models.Result

	retryResult := resilience.RetryWithBackoff(ctx, w.retryCfg, func(ctx context.Context) error {
		if !w.cb.AllowRequest() {
			return fmt.Errorf("circuit breaker open for adapter %s", w.name)
		}
		var err error
		result, err = w.inner.ExecuteTask(ctx, task)
		w.cb.RecordResult(err)
		return err
	})

	if !retryResult.Success {
		return result, retryResult.LastError
	}
	return result, nil
}

func (w *resilientAdapterWrapper) GetState(ctx context.Context) (models.State, error) {
	return w.inner.GetState(ctx)
}

func (w *resilientAdapterWrapper) Cleanup(ctx context.Context) error {
	return w.inner.Cleanup(ctx)
}

func (w *resilientAdapterWrapper) GetCircuitState() string {
	return w.cb.GetState().String()
}

// enrichTaskContext 智能上下文注入
// 在任务执行前，自动从知识库、模式引擎拉取相关信息注入 task.Context
// 让 AI Agent 拿到更多上下文，提升执行质量
func (e *Engine) enrichTaskContext(ctx context.Context, task models.Task) models.Task {
	ci := e.config.Engine.ContextInjection
	if !ci.Enabled {
		return task
	}

	// 初始化 context map
	if task.Context == nil {
		task.Context = make(map[string]any)
	}

	// 1. 注入相关知识条目
	if e.knowledge != nil {
		cacheKey := fmt.Sprintf("ctx:kb:%s", task.Description)
		var knowledgeHits []*models.KnowledgeEntry

		if ci.CacheResults {
			if cached, ok := e.cache.Get(cacheKey); ok {
				knowledgeHits = cached.([]*models.KnowledgeEntry)
			}
		}

		if knowledgeHits == nil {
			hits, err := e.knowledge.Search(ctx, task.Description, ci.KnowledgeLimit)
			if err != nil {
				slog.Warn("context injection: knowledge search failed", "task_id", task.ID, "error", err)
			} else {
				knowledgeHits = hits
			}

			if ci.CacheResults && knowledgeHits != nil {
				e.cache.Set(cacheKey, knowledgeHits, 2*time.Minute)
			}
		}

		if len(knowledgeHits) > 0 {
			hints := make([]KnowledgeHint, 0, len(knowledgeHits))
			for _, entry := range knowledgeHits {
				summary := entry.Content
				if len(summary) > 200 {
					summary = summary[:200] + "..."
				}
				hints = append(hints, KnowledgeHint{
					ID:      entry.ID,
					Title:   entry.Title,
					Summary: summary,
				})
			}
			task.Context["related_knowledge"] = hints
			slog.Debug("context injection: knowledge injected",
				"task_id", task.ID, "count", len(hints))
		}
	}

	// 2. 注入匹配模式
	if e.pattern != nil {
		cacheKey := fmt.Sprintf("ctx:pat:%s:%s", task.Type, task.Description)
		var matchedPatterns []*models.Pattern

		if ci.CacheResults {
			if cached, ok := e.cache.Get(cacheKey); ok {
				matchedPatterns = cached.([]*models.Pattern)
			}
		}

		if matchedPatterns == nil {
			matches, err := e.pattern.Match(ctx, task)
			if err != nil {
				slog.Warn("context injection: pattern match failed", "task_id", task.ID, "error", err)
			} else {
				matchedPatterns = matches
			}

			if ci.CacheResults && matchedPatterns != nil {
				e.cache.Set(cacheKey, matchedPatterns, 2*time.Minute)
			}
		}

		// 限制数量
		limit := ci.PatternLimit
		if len(matchedPatterns) > limit {
			matchedPatterns = matchedPatterns[:limit]
		}

		if len(matchedPatterns) > 0 {
			hints := make([]PatternHint, 0, len(matchedPatterns))
			for _, p := range matchedPatterns {
				hints = append(hints, PatternHint{
					ID:          p.ID,
					Name:        p.Name,
					Description: p.Description,
					SuccessRate: p.SuccessRate,
					UsageCount:  p.UsageCount,
				})
			}
			task.Context["matched_patterns"] = hints
			slog.Debug("context injection: patterns injected",
				"task_id", task.ID, "count", len(hints))
		}
	}

	// 3. 注入约束摘要
	if ci.InjectConstraints && len(task.Constraints) > 0 {
		hints := make([]ConstraintHint, 0, len(task.Constraints))
		for _, c := range task.Constraints {
			hints = append(hints, ConstraintHint{
				Type:     c.Type,
				Rule:     c.Rule,
				Severity: string(c.Severity),
				Message:  c.Message,
			})
		}
		task.Context["constraints_summary"] = hints
	}

	// 4. 注入任务元数据
	if ci.InjectMetadata {
		task.Context["_meta"] = map[string]any{
			"task_id":      task.ID,
			"task_type":    task.Type,
			"priority":     task.Priority,
			"has_deadline": task.Deadline != nil,
			"enriched_at":  time.Now().Format(time.RFC3339),
		}
	}

	return task
}

// validateTask 验证任务
func (e *Engine) validateTask(ctx context.Context, task models.Task) error {
	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}
	if task.Type == "" {
		return fmt.Errorf("task type is required")
	}
	if task.Description == "" {
		return fmt.Errorf("task description is required")
	}

	// 验证约束
	for _, constraint := range task.Constraints {
		if constraint.Validator != nil {
			if !constraint.Validator(task.Context) {
				return fmt.Errorf("constraint violation: %s", constraint.Message)
			}
		}
	}

	return nil
}

// HealthCheck 探测引擎健康状态
func (e *Engine) HealthCheck(ctx context.Context) error {
	if e.storage == nil {
		return fmt.Errorf("storage not initialized")
	}

	_, err := e.storage.ListTasks(ctx, models.TaskFilter{})
	if err != nil {
		return fmt.Errorf("storage unhealthy: %w", err)
	}

	return nil
}

// GracefulShutdown 优雅关闭引擎
func (e *Engine) GracefulShutdown(ctx context.Context) error {
	var shutdownErr error

	e.shutdownOnce.Do(func() {
		slog.Info("engine shutting down...")
		close(e.shutdownCh)

		// 停止 Webhook 管理器
		if e.webhooks != nil {
			e.webhooks.Stop()
		}

		// 等待正在执行的任务完成（带超时）
		select {
		case <-ctx.Done():
			slog.Warn("shutdown timeout, forcing cleanup")
		case <-e.waitForTasks():
			slog.Info("all tasks completed")
		}

		// 清理适配器
		e.mu.Lock()
		for name, adapter := range e.adapters {
			if err := adapter.Cleanup(ctx); err != nil {
				slog.Error("adapter cleanup failed", "name", name, "error", err)
				shutdownErr = fmt.Errorf("failed to cleanup adapter %s: %w", name, err)
			}
		}
		e.mu.Unlock()

		// 关闭存储
		if err := e.storage.Close(); err != nil {
			shutdownErr = fmt.Errorf("failed to close storage: %w", err)
		}

		slog.Info("engine shutdown complete")
	})

	return shutdownErr
}

// waitForTasks 等待所有任务完成
func (e *Engine) waitForTasks() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			// 检查信号量是否为空
			if len(e.semaphore) == 0 {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	return done
}

// Cleanup 清理引擎（已废弃，使用 GracefulShutdown）
func (e *Engine) Cleanup(ctx context.Context) error {
	return e.GracefulShutdown(ctx)
}

// GetAdapter 获取适配器
func (e *Engine) GetAdapter(name string) (Adapter, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	adapter, exists := e.adapters[name]
	return adapter, exists
}

// ListAdapters 列出所有适配器
func (e *Engine) ListAdapters() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.adapters))
	for name := range e.adapters {
		names = append(names, name)
	}
	return names
}

// GetConfig 获取配置
func (e *Engine) GetConfig() *config.Config {
	return e.config
}

// IsShutdown 检查是否正在关闭
func (e *Engine) IsShutdown() bool {
	select {
	case <-e.shutdownCh:
		return true
	default:
		return false
	}
}
