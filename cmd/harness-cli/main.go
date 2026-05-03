package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/internal/adapters"
	"github.com/harness-engineering/harness/internal/api"
	"github.com/harness-engineering/harness/internal/core"
	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
	"github.com/harness-engineering/harness/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	engine  *core.Engine
	store   storage.Storage
	log     *logger.Logger
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "harness",
		Short: "Harness Engineering CLI",
		Long:  "Harness Engineering - AI Agent 框架，支持 Claude Code、Hermes 和 Codex CLI",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initEngine()
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			cleanup()
		},
	}

	// 全局标志
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "harness.yaml", "配置文件路径")

	// 添加子命令
	rootCmd.AddCommand(
		newTaskCmd(),
		newSessionCmd(),
		newKnowledgeCmd(),
		newPatternCmd(),
		newMonitorCmd(),
		newServeCmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// initEngine 初始化引擎
func initEngine() error {
	// 加载配置
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 创建日志器
	log, err = logger.New(logger.Config{
		Level:  cfg.Monitor.LogLevel,
		Output: "stdout",
	})
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	// 创建存储
	store, err = storage.NewSQLiteStorage(storage.DefaultSQLiteConfig(cfg.Storage.Path))
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	// 创建引擎
	engine, err = core.NewEngine(cfg, store)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	// 注册适配器
	if cfg.Adapters.ClaudeCode.Enabled {
		engine.RegisterAdapter("claude-code", adapters.NewClaudeCodeAdapter())
	}
	if cfg.Adapters.Hermes.Enabled {
		engine.RegisterAdapter("hermes", adapters.NewHermesAdapter())
	}
	if cfg.Adapters.CodexCLI.Enabled {
		engine.RegisterAdapter("codex-cli", adapters.NewCodexCLIAdapter())
	}

	// 初始化引擎
	ctx := context.Background()
	if err := engine.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize engine: %w", err)
	}

	return nil
}

// cleanup 清理资源
func cleanup() {
	if store != nil {
		store.Close()
	}
	if log != nil {
		log.Close()
	}
}

// newTaskCmd 创建任务命令
func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "任务管理",
		Long:  "创建、执行、查询任务",
	}

	// 创建任务
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "创建任务",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("id")
			taskType, _ := cmd.Flags().GetString("type")
			description, _ := cmd.Flags().GetString("description")

			task := models.Task{
				ID:          id,
				Type:        taskType,
				Description: description,
				Context:     make(map[string]any),
			}

			ctx := context.Background()
			state, err := engine.TaskManager().CreateTask(ctx, task)
			if err != nil {
				return fmt.Errorf("failed to create task: %w", err)
			}

			printJSON(state)
			return nil
		},
	}
	createCmd.Flags().String("id", "", "任务 ID")
	createCmd.Flags().String("type", "", "任务类型")
	createCmd.Flags().String("description", "", "任务描述")
	createCmd.MarkFlagRequired("id")
	createCmd.MarkFlagRequired("type")
	createCmd.MarkFlagRequired("description")

	// 列出任务
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "列出任务",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, _ := cmd.Flags().GetString("status")
			taskType, _ := cmd.Flags().GetString("type")

			filter := models.TaskFilter{
				Status: status,
				Type:   taskType,
			}

			ctx := context.Background()
			tasks, err := engine.TaskManager().ListTasks(ctx, filter)
			if err != nil {
				return fmt.Errorf("failed to list tasks: %w", err)
			}

			printTaskTable(tasks)
			return nil
		},
	}
	listCmd.Flags().String("status", "", "任务状态")
	listCmd.Flags().String("type", "", "任务类型")

	// 获取任务
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "获取任务",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			state, err := engine.TaskManager().GetTask(ctx, args[0])
			if err != nil {
				return fmt.Errorf("failed to get task: %w", err)
			}

			printJSON(state)
			return nil
		},
	}

	// 执行任务
	executeCmd := &cobra.Command{
		Use:   "execute [id]",
		Short: "执行任务",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			result, err := engine.TaskManager().ExecuteTask(ctx, args[0])
			if err != nil {
				return fmt.Errorf("failed to execute task: %w", err)
			}

			printJSON(result)
			return nil
		},
	}

	// 取消任务
	cancelCmd := &cobra.Command{
		Use:   "cancel [id]",
		Short: "取消任务",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := engine.TaskManager().CancelTask(ctx, args[0]); err != nil {
				return fmt.Errorf("failed to cancel task: %w", err)
			}

			fmt.Println("Task cancelled successfully")
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, getCmd, executeCmd, cancelCmd)
	return cmd
}

// newSessionCmd 创建会话命令
func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "会话管理",
		Long:  "创建、查询会话",
	}

	// 创建会话
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "创建会话",
		RunE: func(cmd *cobra.Command, args []string) error {
			env, _ := cmd.Flags().GetString("environment")

			ctx := context.Background()
			session, err := engine.StateManager().CreateSession(ctx, env)
			if err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}

			printJSON(session)
			return nil
		},
	}
	createCmd.Flags().String("environment", "default", "环境名称")

	// 列出会话
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "列出会话",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			sessions, err := engine.StateManager().ListSessions(ctx)
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}

			printJSON(sessions)
			return nil
		},
	}

	// 获取会话
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "获取会话",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			state, err := engine.StateManager().GetState(ctx, args[0])
			if err != nil {
				return fmt.Errorf("failed to get session: %w", err)
			}

			printJSON(state)
			return nil
		},
	}

	// 获取历史
	historyCmd := &cobra.Command{
		Use:   "history [id]",
		Short: "获取会话历史",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			history, err := engine.StateManager().GetHistory(ctx, args[0])
			if err != nil {
				return fmt.Errorf("failed to get history: %w", err)
			}

			printJSON(history)
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, getCmd, historyCmd)
	return cmd
}

// newKnowledgeCmd 创建知识命令
func newKnowledgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge",
		Short: "知识管理",
		Long:  "添加、搜索、查询知识",
	}

	// 添加知识
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "添加知识",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("id")
			title, _ := cmd.Flags().GetString("title")
			content, _ := cmd.Flags().GetString("content")
			knowledgeType, _ := cmd.Flags().GetString("type")
			tags, _ := cmd.Flags().GetStringSlice("tags")

			entry := models.KnowledgeEntry{
				ID:       id,
				Type:     knowledgeType,
				Title:    title,
				Content:  content,
				Tags:     tags,
				Metadata: make(map[string]any),
			}

			ctx := context.Background()
			if err := engine.Knowledge().AddEntry(ctx, entry); err != nil {
				return fmt.Errorf("failed to add knowledge: %w", err)
			}

			fmt.Println("Knowledge added successfully")
			return nil
		},
	}
	addCmd.Flags().String("id", "", "知识 ID")
	addCmd.Flags().String("title", "", "标题")
	addCmd.Flags().String("content", "", "内容")
	addCmd.Flags().String("type", "general", "类型")
	addCmd.Flags().StringSlice("tags", []string{}, "标签")
	addCmd.MarkFlagRequired("id")
	addCmd.MarkFlagRequired("title")
	addCmd.MarkFlagRequired("content")

	// 搜索知识
	searchCmd := &cobra.Command{
		Use:   "search [query]",
		Short: "搜索知识",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")

			ctx := context.Background()
			entries, err := engine.Knowledge().Search(ctx, args[0], limit)
			if err != nil {
				return fmt.Errorf("failed to search knowledge: %w", err)
			}

			printJSON(entries)
			return nil
		},
	}
	searchCmd.Flags().Int("limit", 10, "结果数量限制")

	// 列出知识
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "列出知识",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			entries, err := engine.Knowledge().ListEntries(ctx, 0, 100)
			if err != nil {
				return fmt.Errorf("failed to list knowledge: %w", err)
			}

			printJSON(entries)
			return nil
		},
	}

	// 获取知识
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "获取知识",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			entry, err := engine.Knowledge().GetEntry(ctx, args[0])
			if err != nil {
				return fmt.Errorf("failed to get knowledge: %w", err)
			}

			printJSON(entry)
			return nil
		},
	}

	// 删除知识
	deleteCmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "删除知识",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := engine.Knowledge().DeleteEntry(ctx, args[0]); err != nil {
				return fmt.Errorf("failed to delete knowledge: %w", err)
			}

			fmt.Println("Knowledge deleted successfully")
			return nil
		},
	}

	cmd.AddCommand(addCmd, searchCmd, listCmd, getCmd, deleteCmd)
	return cmd
}

// newPatternCmd 创建模式命令
func newPatternCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pattern",
		Short: "模式管理",
		Long:  "添加、匹配、查询模式",
	}

	// 添加模式
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "添加模式",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("id")
			name, _ := cmd.Flags().GetString("name")
			description, _ := cmd.Flags().GetString("description")
			trigger, _ := cmd.Flags().GetString("trigger")

			pattern := models.Pattern{
				ID:          id,
				Name:        name,
				Description: description,
				Trigger:     trigger,
				Metadata:    make(map[string]any),
			}

			ctx := context.Background()
			if err := engine.Pattern().AddPattern(ctx, pattern); err != nil {
				return fmt.Errorf("failed to add pattern: %w", err)
			}

			fmt.Println("Pattern added successfully")
			return nil
		},
	}
	addCmd.Flags().String("id", "", "模式 ID")
	addCmd.Flags().String("name", "", "名称")
	addCmd.Flags().String("description", "", "描述")
	addCmd.Flags().String("trigger", "", "触发器")
	addCmd.MarkFlagRequired("id")
	addCmd.MarkFlagRequired("name")
	addCmd.MarkFlagRequired("trigger")

	// 匹配模式
	matchCmd := &cobra.Command{
		Use:   "match",
		Short: "匹配模式",
		RunE: func(cmd *cobra.Command, args []string) error {
			taskType, _ := cmd.Flags().GetString("type")
			description, _ := cmd.Flags().GetString("description")

			task := models.Task{
				Type:        taskType,
				Description: description,
				Context:     make(map[string]any),
			}

			ctx := context.Background()
			patterns, err := engine.Pattern().Match(ctx, task)
			if err != nil {
				return fmt.Errorf("failed to match patterns: %w", err)
			}

			printJSON(patterns)
			return nil
		},
	}
	matchCmd.Flags().String("type", "", "任务类型")
	matchCmd.Flags().String("description", "", "任务描述")

	// 列出模式
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "列出模式",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			patterns, err := engine.Pattern().ListPatterns(ctx)
			if err != nil {
				return fmt.Errorf("failed to list patterns: %w", err)
			}

			printJSON(patterns)
			return nil
		},
	}

	// 获取模式
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "获取模式",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			pattern, err := engine.Pattern().GetPattern(ctx, args[0])
			if err != nil {
				return fmt.Errorf("failed to get pattern: %w", err)
			}

			printJSON(pattern)
			return nil
		},
	}

	// 删除模式
	deleteCmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "删除模式",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := engine.Pattern().DeletePattern(ctx, args[0]); err != nil {
				return fmt.Errorf("failed to delete pattern: %w", err)
			}

			fmt.Println("Pattern deleted successfully")
			return nil
		},
	}

	cmd.AddCommand(addCmd, matchCmd, listCmd, getCmd, deleteCmd)
	return cmd
}

// newMonitorCmd 创建监控命令
func newMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "监控管理",
		Long:  "查看指标和健康状态",
	}

	// 获取指标
	metricsCmd := &cobra.Command{
		Use:   "metrics",
		Short: "获取指标",
		RunE: func(cmd *cobra.Command, args []string) error {
			metrics := engine.Monitor().GetMetrics()
			printJSON(metrics)
			return nil
		},
	}

	// 健康检查
	healthCmd := &cobra.Command{
		Use:   "health",
		Short: "健康检查",
		RunE: func(cmd *cobra.Command, args []string) error {
			health := map[string]any{
				"status":    "ok",
				"timestamp": fmt.Sprintf("%v", time.Now()),
				"version":   "1.0.0",
			}
			printJSON(health)
			return nil
		},
	}

	cmd.AddCommand(metricsCmd, healthCmd)
	return cmd
}

// newServeCmd 创建服务器命令
func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "启动 API 服务器",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, _ := cmd.Flags().GetString("addr")

			// 加载配置
			cfgFile, _ := cmd.Flags().GetString("config")
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			if addr != "" {
				cfg.Server.Addr = addr
			}

			// 创建存储
			store, err := storage.NewSQLiteStorage(storage.DefaultSQLiteConfig(cfg.Storage.Path))
			if err != nil {
				return fmt.Errorf("failed to create storage: %w", err)
			}

			// 创建引擎
			engine, err := core.NewEngine(cfg, store)
			if err != nil {
				return fmt.Errorf("failed to create engine: %w", err)
			}

			server := api.NewServer(cfg, engine)
			fmt.Printf("Starting Harness Engine on %s\n", addr)

			return server.Start()
		},
	}
}

// newVersionCmd 创建版本命令
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Harness Engineering v1.0.0")
		},
	}
}

// printJSON 打印 JSON
func printJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

// printTaskTable 打印任务表格
func printTaskTable(tasks []*models.TaskState) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tSTATUS\tCREATED\tUPDATED")
	fmt.Fprintln(w, strings.Repeat("-", 80))

	for _, task := range tasks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			task.Task.ID,
			task.Task.Type,
			task.Status,
			task.CreatedAt.Format("2006-01-02 15:04:05"),
			task.UpdatedAt.Format("2006-01-02 15:04:05"),
		)
	}

	w.Flush()
}


