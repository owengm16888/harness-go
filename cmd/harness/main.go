package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/internal/adapters"
	"github.com/harness-engineering/harness/internal/api"
	"github.com/harness-engineering/harness/internal/core"
	"github.com/harness-engineering/harness/internal/storage"
)

func main() {
	// 初始化日志
	initLogger()

	// 加载配置
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// 创建存储
	store, err := storage.NewSQLiteStorage(storage.DefaultSQLiteConfig(cfg.Storage.Path))
	if err != nil {
		slog.Error("failed to create storage", "error", err)
		os.Exit(1)
	}

	// 创建引擎
	engine, err := core.NewEngine(cfg, store)
	if err != nil {
		slog.Error("failed to create engine", "error", err)
		os.Exit(1)
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
		slog.Error("failed to initialize engine", "error", err)
		os.Exit(1)
	}

	// 创建 API 服务器
	server := api.NewServer(cfg, engine)

	// 启动服务器
	go func() {
		slog.Info("server starting", "addr", cfg.Server.Addr)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// 等待信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")

	// 创建超时上下文
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 关闭服务器
	if err := server.Stop(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	// 关闭引擎
	if err := engine.GracefulShutdown(shutdownCtx); err != nil {
		slog.Error("engine shutdown error", "error", err)
	}

	slog.Info("server stopped")
}

func initLogger() {
	level := slog.LevelInfo
	if os.Getenv("HARNESS_LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

func loadConfig() (*config.Config, error) {
	configPath := os.Getenv("HARNESS_CONFIG")
	if configPath == "" {
		configPath = "harness.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	// 应用环境变量覆盖
	if addr := os.Getenv("HARNESS_SERVER_ADDR"); addr != "" {
		cfg.Server.Addr = addr
	}
	if logLevel := os.Getenv("HARNESS_LOG_LEVEL"); logLevel != "" {
		cfg.Monitor.LogLevel = logLevel
	}

	return cfg, nil
}
