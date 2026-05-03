package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Plugin 插件接口
type Plugin interface {
	// Name 返回插件名称
	Name() string

	// Version 返回插件版本
	Version() string

	// Description 返回插件描述
	Description() string

	// Initialize 初始化插件
	Initialize(ctx context.Context, config map[string]any) error

	// Start 启动插件
	Start(ctx context.Context) error

	// Stop 停止插件
	Stop(ctx context.Context) error

	// Health 健康检查
	Health(ctx context.Context) error
}

// PluginInfo 插件信息
type PluginInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status"`
	Error       error  `json:"error,omitempty"`
}

// PluginManager 插件管理器
type PluginManager struct {
	mu       sync.RWMutex
	plugins  map[string]Plugin
	info     map[string]*PluginInfo
	loader   PluginLoader
}

// PluginLoader 插件加载器
type PluginLoader interface {
	Load(path string) (Plugin, error)
}

// PluginManagerConfig 插件管理器配置
type PluginManagerConfig struct {
	PluginDir string `yaml:"plugin_dir"`
	AutoLoad  bool   `yaml:"auto_load"`
}

// NewPluginManager 创建插件管理器
func NewPluginManager(cfg PluginManagerConfig) *PluginManager {
	return &PluginManager{
		plugins: make(map[string]Plugin),
		info:    make(map[string]*PluginInfo),
	}
}

// LoadPlugin 加载插件
func (m *PluginManager) LoadPlugin(path string, plugin Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := plugin.Name()

	// 检查是否已加载
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin already loaded: %s", name)
	}

	// 保存插件
	m.plugins[name] = plugin
	m.info[name] = &PluginInfo{
		Name:        name,
		Version:     plugin.Version(),
		Description: plugin.Description(),
		Enabled:     true,
		Status:      "loaded",
	}

	return nil
}

// UnloadPlugin 卸载插件
func (m *PluginManager) UnloadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// 停止插件
	ctx := context.Background()
	if err := plugin.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop plugin: %w", err)
	}

	// 删除插件
	delete(m.plugins, name)
	delete(m.info, name)

	return nil
}

// InitializePlugin 初始化插件
func (m *PluginManager) InitializePlugin(name string, config map[string]any) error {
	m.mu.RLock()
	plugin, exists := m.plugins[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	ctx := context.Background()
	if err := plugin.Initialize(ctx, config); err != nil {
		m.mu.Lock()
		m.info[name].Status = "error"
		m.info[name].Error = err
		m.mu.Unlock()
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}

	m.mu.Lock()
	m.info[name].Status = "initialized"
	m.mu.Unlock()

	return nil
}

// StartPlugin 启动插件
func (m *PluginManager) StartPlugin(name string) error {
	m.mu.RLock()
	plugin, exists := m.plugins[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	ctx := context.Background()
	if err := plugin.Start(ctx); err != nil {
		m.mu.Lock()
		m.info[name].Status = "error"
		m.info[name].Error = err
		m.mu.Unlock()
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	m.mu.Lock()
	m.info[name].Status = "running"
	m.mu.Unlock()

	return nil
}

// StopPlugin 停止插件
func (m *PluginManager) StopPlugin(name string) error {
	m.mu.RLock()
	plugin, exists := m.plugins[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	ctx := context.Background()
	if err := plugin.Stop(ctx); err != nil {
		m.mu.Lock()
		m.info[name].Status = "error"
		m.info[name].Error = err
		m.mu.Unlock()
		return fmt.Errorf("failed to stop plugin: %w", err)
	}

	m.mu.Lock()
	m.info[name].Status = "stopped"
	m.mu.Unlock()

	return nil
}

// GetPlugin 获取插件
func (m *PluginManager) GetPlugin(name string) (Plugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}

	return plugin, nil
}

// ListPlugins 列出插件
func (m *PluginManager) ListPlugins() []*PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*PluginInfo, 0, len(m.info))
	for _, info := range m.info {
		plugins = append(plugins, info)
	}

	return plugins
}

// GetPluginInfo 获取插件信息
func (m *PluginManager) GetPluginInfo(name string) (*PluginInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.info[name]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}

	return info, nil
}

// EnablePlugin 启用插件
func (m *PluginManager) EnablePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.info[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	info.Enabled = true
	return nil
}

// DisablePlugin 禁用插件
func (m *PluginManager) DisablePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.info[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	info.Enabled = false
	return nil
}

// HealthCheck 健康检查
func (m *PluginManager) HealthCheck(ctx context.Context) map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]error)
	for name, plugin := range m.plugins {
		results[name] = plugin.Health(ctx)
	}

	return results
}

// StartAll 启动所有插件
func (m *PluginManager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, plugin := range m.plugins {
		info := m.info[name]
		if !info.Enabled {
			continue
		}

		if err := plugin.Start(ctx); err != nil {
			info.Status = "error"
			info.Error = err
			return fmt.Errorf("failed to start plugin %s: %w", name, err)
		}

		info.Status = "running"
	}

	return nil
}

// StopAll 停止所有插件
func (m *PluginManager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastError error
	for name, plugin := range m.plugins {
		if err := plugin.Stop(ctx); err != nil {
			m.info[name].Status = "error"
			m.info[name].Error = err
			lastError = err
		} else {
			m.info[name].Status = "stopped"
		}
	}

	return lastError
}

// GetStats 获取统计信息
func (m *PluginManager) GetStats() *PluginStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &PluginStats{
		TotalPlugins: len(m.plugins),
	}

	for _, info := range m.info {
		if info.Enabled {
			stats.EnabledPlugins++
		}
		if info.Status == "running" {
			stats.RunningPlugins++
		}
		if info.Error != nil {
			stats.ErrorPlugins++
		}
	}

	return stats
}

// PluginStats 插件统计
type PluginStats struct {
	TotalPlugins   int `json:"total_plugins"`
	EnabledPlugins int `json:"enabled_plugins"`
	RunningPlugins int `json:"running_plugins"`
	ErrorPlugins   int `json:"error_plugins"`
}
