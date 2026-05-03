package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/internal/core"
	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
)

func main() {
	// 示例 1: 基本使用
	basicUsage()

	// 示例 2: 任务管理
	taskManagement()

	// 示例 3: 知识管理
	knowledgeManagement()

	// 示例 4: 模式匹配
	patternMatching()

	// 示例 5: 反馈循环
	feedbackLoop()
}

// basicUsage 基本使用示例
func basicUsage() {
	fmt.Println("=== 基本使用示例 ===")

	// 加载配置
	cfg, err := config.Load("harness.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 创建存储
	store, err := storage.NewSQLiteStorage(storage.DefaultSQLiteConfig(cfg.Storage.Path))
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	// 创建引擎
	engine, err := core.NewEngine(cfg, store)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	// 初始化引擎
	ctx := context.Background()
	if err := engine.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize engine: %v", err)
	}

	fmt.Println("引擎初始化成功")
}

// taskManagement 任务管理示例
func taskManagement() {
	fmt.Println("\n=== 任务管理示例 ===")

	// 创建引擎
	cfg, _ := config.Load("harness.yaml")
	store, _ := storage.NewSQLiteStorage(storage.DefaultSQLiteConfig(cfg.Storage.Path))
	engine, _ := core.NewEngine(cfg, store)

	ctx := context.Background()

	// 创建任务
	task := models.Task{
		ID:          "task-1",
		Type:        "implement",
		Description: "实现用户认证功能",
		Context: map[string]any{
			"environment": "production",
			"language":    "go",
		},
		Constraints: []models.Constraint{
			{
				Type:     "security",
				Rule:     "no-hardcoded-secrets",
				Severity: models.SeverityError,
				Message:  "不能使用硬编码的密钥",
			},
			{
				Type:     "testing",
				Rule:     "min-coverage-80",
				Severity: models.SeverityWarning,
				Message:  "测试覆盖率不能低于 80%",
			},
		},
		Priority: 1,
	}

	// 创建任务
	state, err := engine.TaskManager().CreateTask(ctx, task)
	if err != nil {
		log.Printf("Failed to create task: %v", err)
		return
	}
	fmt.Printf("任务创建成功: %s (状态: %s)\n", state.Task.ID, state.Status)

	// 执行任务
	result, err := engine.TaskManager().ExecuteTask(ctx, "task-1")
	if err != nil {
		log.Printf("Failed to execute task: %v", err)
		return
	}
	fmt.Printf("任务执行完成: %s (状态: %s)\n", result.TaskID, result.Status)

	// 获取任务
	taskState, err := engine.TaskManager().GetTask(ctx, "task-1")
	if err != nil {
		log.Printf("Failed to get task: %v", err)
		return
	}
	fmt.Printf("任务详情: %+v\n", taskState)

	// 列出任务
	filter := models.TaskFilter{
		Status: "completed",
	}
	tasks, err := engine.TaskManager().ListTasks(ctx, filter)
	if err != nil {
		log.Printf("Failed to list tasks: %v", err)
		return
	}
	fmt.Printf("已完成任务数: %d\n", len(tasks))
}

// knowledgeManagement 知识管理示例
func knowledgeManagement() {
	fmt.Println("\n=== 知识管理示例 ===")

	// 创建引擎
	cfg, _ := config.Load("harness.yaml")
	store, _ := storage.NewSQLiteStorage(storage.DefaultSQLiteConfig(cfg.Storage.Path))
	engine, _ := core.NewEngine(cfg, store)

	ctx := context.Background()

	// 添加知识
	entry := models.KnowledgeEntry{
		ID:      "knowledge-1",
		Type:    "pattern",
		Title:   "Go 错误处理最佳实践",
		Content: "在 Go 中，应该使用 errors.Wrap 来包装错误，添加上下文信息...",
		Tags:    []string{"go", "error-handling", "best-practices"},
		Metadata: map[string]any{
			"language": "go",
			"category": "error-handling",
		},
	}

	if err := engine.Knowledge().AddEntry(ctx, entry); err != nil {
		log.Printf("Failed to add knowledge: %v", err)
		return
	}
	fmt.Println("知识添加成功")

	// 搜索知识
	results, err := engine.Knowledge().Search(ctx, "错误处理", 10)
	if err != nil {
		log.Printf("Failed to search knowledge: %v", err)
		return
	}
	fmt.Printf("搜索结果数: %d\n", len(results))
	for _, r := range results {
		fmt.Printf("  - %s: %s\n", r.ID, r.Title)
	}

	// 获取知识
	knowledge, err := engine.Knowledge().GetEntry(ctx, "knowledge-1")
	if err != nil {
		log.Printf("Failed to get knowledge: %v", err)
		return
	}
	fmt.Printf("知识详情: %s - %s\n", knowledge.ID, knowledge.Title)

	// 更新知识
	update := models.KnowledgeUpdate{
		Title:   "Go 错误处理最佳实践 (更新)",
		Content: "更新后的内容...",
	}
	if err := engine.Knowledge().UpdateEntry(ctx, "knowledge-1", update); err != nil {
		log.Printf("Failed to update knowledge: %v", err)
		return
	}
	fmt.Println("知识更新成功")
}

// patternMatching 模式匹配示例
func patternMatching() {
	fmt.Println("\n=== 模式匹配示例 ===")

	// 创建引擎
	cfg, _ := config.Load("harness.yaml")
	store, _ := storage.NewSQLiteStorage(storage.DefaultSQLiteConfig(cfg.Storage.Path))
	engine, _ := core.NewEngine(cfg, store)

	ctx := context.Background()

	// 添加模式
	pattern := models.Pattern{
		ID:          "pattern-1",
		Name:        "用户认证模式",
		Description: "实现用户认证功能的通用模式",
		Trigger:     "认证|authentication|auth",
		Actions: []models.Action{
			{
				Type: "implement",
				Parameters: map[string]any{
					"framework": "jwt",
					"storage":   "database",
				},
				Timeout:   5 * time.Minute,
				Retryable: true,
			},
		},
		Metadata: map[string]any{
			"task_type": "implement",
			"context": map[string]any{
				"language": "go",
			},
		},
	}

	if err := engine.Pattern().AddPattern(ctx, pattern); err != nil {
		log.Printf("Failed to add pattern: %v", err)
		return
	}
	fmt.Println("模式添加成功")

	// 匹配模式
	task := models.Task{
		Type:        "implement",
		Description: "实现用户认证功能",
		Context: map[string]any{
			"language": "go",
		},
	}

	matched, err := engine.Pattern().Match(ctx, task)
	if err != nil {
		log.Printf("Failed to match patterns: %v", err)
		return
	}
	fmt.Printf("匹配到的模式数: %d\n", len(matched))
	for _, p := range matched {
		fmt.Printf("  - %s: %s (成功率: %.2f)\n", p.ID, p.Name, p.SuccessRate)
	}

	// 列出模式
	patterns, err := engine.Pattern().ListPatterns(ctx)
	if err != nil {
		log.Printf("Failed to list patterns: %v", err)
		return
	}
	fmt.Printf("总模式数: %d\n", len(patterns))
}

// feedbackLoop 反馈循环示例
func feedbackLoop() {
	fmt.Println("\n=== 反馈循环示例 ===")

	// 创建引擎
	cfg, _ := config.Load("harness.yaml")
	store, _ := storage.NewSQLiteStorage(storage.DefaultSQLiteConfig(cfg.Storage.Path))
	engine, _ := core.NewEngine(cfg, store)

	ctx := context.Background()

	// 创建任务结果
	result := models.Result{
		TaskID: "task-1",
		Status: models.TaskStatusCompleted,
		Output: "任务完成",
		Evidence: []models.Evidence{
			{
				Type:      "test",
				Content:   "所有测试通过",
				Source:    "test-runner",
				Timestamp: time.Now(),
				Verified:  true,
			},
		},
		Metrics: models.Metrics{
			Duration:   5 * time.Second,
			TokenCount: 1000,
			ToolUses:   10,
		},
	}

	// 处理反馈
	feedback, err := engine.ProcessFeedback(ctx, result)
	if err != nil {
		log.Printf("Failed to process feedback: %v", err)
		return
	}
	fmt.Printf("反馈状态: %s\n", feedback.Status)
	fmt.Printf("违规数: %d\n", len(feedback.Violations))
	fmt.Printf("修复数: %d\n", len(feedback.Fixes))

	// 获取指标
	metrics := engine.Monitor().GetMetrics()
	fmt.Printf("总任务数: %d\n", metrics.TotalTasks)
	fmt.Printf("成功任务数: %d\n", metrics.SuccessTasks)
	fmt.Printf("失败任务数: %d\n", metrics.FailedTasks)
	fmt.Printf("平均执行时间: %s\n", metrics.AverageDuration)
}
