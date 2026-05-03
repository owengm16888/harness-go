package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ConfigWatcher 配置文件监视器
type ConfigWatcher struct {
	mu       sync.RWMutex
	path     string
	config   *Config
	onChange func(*Config)
	lastMod  time.Time
	stopCh   chan struct{}
}

// NewConfigWatcher 创建配置监视器
func NewConfigWatcher(path string, onChange func(*Config)) *ConfigWatcher {
	return &ConfigWatcher{
		path:     path,
		onChange: onChange,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动监视（后台 goroutine）
func (w *ConfigWatcher) Start(ctx context.Context, interval time.Duration) {
	// 记录初始修改时间
	if info, err := os.Stat(w.path); err == nil {
		w.lastMod = info.ModTime()
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			case <-ticker.C:
				w.check()
			}
		}
	}()

	slog.Info("config watcher started", "path", w.path, "interval", interval)
}

// Stop 停止监视
func (w *ConfigWatcher) Stop() {
	close(w.stopCh)
}

// Get 获取当前配置
func (w *ConfigWatcher) Get() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

// check 检查文件变更
func (w *ConfigWatcher) check() {
	info, err := os.Stat(w.path)
	if err != nil {
		return
	}

	if !info.ModTime().After(w.lastMod) {
		return
	}

	w.lastMod = info.ModTime()

	// 重新加载配置
	newCfg, err := Load(w.path)
	if err != nil {
		slog.Error("config reload failed", "error", err, "path", w.path)
		return
	}

	// 校验新配置
	if validation := newCfg.Validate(); validation.HasErrors() {
		for _, e := range validation.Errors {
			slog.Error("config validation failed on reload", "field", e.Field, "error", e.Message)
		}
		slog.Warn("config reload aborted due to validation errors, keeping previous config")
		return
	}

	w.mu.Lock()
	w.config = newCfg
	w.mu.Unlock()

	slog.Info("config reloaded", "path", w.path)

	// 触发回调
	if w.onChange != nil {
		w.onChange(newCfg)
	}
}

// WatchConfigFile 监视配置文件变更并自动重载
func WatchConfigFile(ctx context.Context, path string, onChange func(*Config)) *ConfigWatcher {
	absPath, _ := filepath.Abs(path)
	watcher := NewConfigWatcher(absPath, onChange)

	// 加载初始配置
	cfg, err := Load(absPath)
	if err != nil {
		slog.Error("initial config load failed", "error", err)
	} else {
		watcher.mu.Lock()
		watcher.config = cfg
		watcher.mu.Unlock()
	}

	watcher.Start(ctx, 5*time.Second)
	return watcher
}
