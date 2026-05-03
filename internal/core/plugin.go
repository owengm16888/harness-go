package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/harness-engineering/harness/models"
)

// ============================================================
// 插件系统 — 动态扩展 Validator / Fixer / Strategy
// ============================================================

// PluginType 插件类型
type PluginType string

const (
	PluginTypeValidator  PluginType = "validator"
	PluginTypeFixer      PluginType = "fixer"
	PluginTypeStrategy   PluginType = "strategy"
	PluginTypeAdapter    PluginType = "adapter"
)

// Plugin 插件接口
type Plugin interface {
	// Name 插件名称
	Name() string
	// Type 插件类型
	Type() PluginType
	// Version 版本号
	Version() string
	// Initialize 初始化插件
	Initialize(ctx context.Context, config map[string]any) error
	// Shutdown 关闭插件
	Shutdown(ctx context.Context) error
}

// ValidatorPlugin 验证器插件接口
type ValidatorPlugin interface {
	Plugin
	Validator
}

// FixerPlugin 修复器插件接口
type FixerPlugin interface {
	Plugin
	Fixer
}

// StrategyPlugin 协作策略插件接口
type StrategyPlugin interface {
	Plugin
	// Execute 执行自定义协作策略
	Execute(ctx context.Context, task models.Task, agents []string) (*models.Result, error)
}

// PluginInfo 插件元信息
type PluginInfo struct {
	Name        string            `json:"name"`
	Type        PluginType        `json:"type"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Config      map[string]any    `json:"config,omitempty"`
	Enabled     bool              `json:"enabled"`
}

// PluginManager 插件管理器
type PluginManager struct {
	mu            sync.RWMutex
	plugins       map[string]Plugin
	pluginInfo    map[string]*PluginInfo
	validators    []ValidatorPlugin
	fixers        []FixerPlugin
	strategies    map[string]StrategyPlugin
}

// NewPluginManager 创建插件管理器
func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins:    make(map[string]Plugin),
		pluginInfo: make(map[string]*PluginInfo),
		strategies: make(map[string]StrategyPlugin),
	}
}

// Register 注册插件
func (pm *PluginManager) Register(plugin Plugin) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	name := plugin.Name()
	if _, exists := pm.plugins[name]; exists {
		return fmt.Errorf("plugin already registered: %s", name)
	}

	pm.plugins[name] = plugin
	pm.pluginInfo[name] = &PluginInfo{
		Name:    name,
		Type:    plugin.Type(),
		Version: plugin.Version(),
		Enabled: true,
	}

	// 按类型索引
	switch p := plugin.(type) {
	case ValidatorPlugin:
		pm.validators = append(pm.validators, p)
	case FixerPlugin:
		pm.fixers = append(pm.fixers, p)
	case StrategyPlugin:
		pm.strategies[name] = p
	}

	slog.Info("plugin registered", "name", name, "type", plugin.Type(), "version", plugin.Version())
	return nil
}

// Unregister 注销插件
func (pm *PluginManager) Unregister(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// 关闭插件
	if err := plugin.Shutdown(context.Background()); err != nil {
		slog.Error("plugin shutdown failed", "name", name, "error", err)
	}

	delete(pm.plugins, name)
	delete(pm.pluginInfo, name)
	delete(pm.strategies, name)

	// 从 validator/fixer 列表中移除
	pm.removeFromValidatorList(name)
	pm.removeFromFixerList(name)

	slog.Info("plugin unregistered", "name", name)
	return nil
}

// InitializeAll 初始化所有插件
func (pm *PluginManager) InitializeAll(ctx context.Context, configs map[string]map[string]any) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for name, plugin := range pm.plugins {
		cfg := configs[name]
		if err := plugin.Initialize(ctx, cfg); err != nil {
			return fmt.Errorf("failed to initialize plugin %s: %w", name, err)
		}
	}

	return nil
}

// GetValidators 获取所有验证器插件
func (pm *PluginManager) GetValidators() []ValidatorPlugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]ValidatorPlugin, len(pm.validators))
	copy(result, pm.validators)
	return result
}

// GetFixers 获取所有修复器插件
func (pm *PluginManager) GetFixers() []FixerPlugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]FixerPlugin, len(pm.fixers))
	copy(result, pm.fixers)
	return result
}

// GetStrategy 获取策略插件
func (pm *PluginManager) GetStrategy(name string) (StrategyPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	s, ok := pm.strategies[name]
	return s, ok
}

// List 列出所有插件
func (pm *PluginManager) List() []*PluginInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*PluginInfo
	for _, info := range pm.pluginInfo {
		result = append(result, info)
	}
	return result
}

// Enable 启用插件
func (pm *PluginManager) Enable(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	info, exists := pm.pluginInfo[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}
	info.Enabled = true
	return nil
}

// Disable 禁用插件
func (pm *PluginManager) Disable(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	info, exists := pm.pluginInfo[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}
	info.Enabled = false
	return nil
}

// ShutdownAll 关闭所有插件
func (pm *PluginManager) ShutdownAll(ctx context.Context) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for name, plugin := range pm.plugins {
		if err := plugin.Shutdown(ctx); err != nil {
			slog.Error("plugin shutdown failed", "name", name, "error", err)
		}
	}
}

// removeFromValidatorList 从验证器列表移除
func (pm *PluginManager) removeFromValidatorList(name string) {
	for i, v := range pm.validators {
		if v.Name() == name {
			pm.validators = append(pm.validators[:i], pm.validators[i+1:]...)
			break
		}
	}
}

// removeFromFixerList 从修复器列表移除
func (pm *PluginManager) removeFromFixerList(name string) {
	for i, f := range pm.fixers {
		if f.Name() == name {
			pm.fixers = append(pm.fixers[:i], pm.fixers[i+1:]...)
			break
		}
	}
}

// ============================================================
// 内置插件示例：安全扫描验证器
// ============================================================

// SecurityScanPlugin 安全扫描验证器插件
type SecurityScanPlugin struct {
	config map[string]any
}

func NewSecurityScanPlugin() *SecurityScanPlugin {
	return &SecurityScanPlugin{}
}

func (p *SecurityScanPlugin) Name() string     { return "security-scanner" }
func (p *SecurityScanPlugin) Type() PluginType  { return PluginTypeValidator }
func (p *SecurityScanPlugin) Version() string   { return "1.0.0" }

func (p *SecurityScanPlugin) Initialize(ctx context.Context, config map[string]any) error {
	p.config = config
	return nil
}

func (p *SecurityScanPlugin) Shutdown(ctx context.Context) error {
	p.config = nil
	return nil
}

func (p *SecurityScanPlugin) Validate(ctx context.Context, result models.Result) ([]models.Violation, error) {
	var violations []models.Violation

	// 检查输出中是否包含敏感信息
	if output, ok := result.Output.(string); ok {
		sensitivePatterns := []string{
			"password", "secret", "api_key", "token",
		}
		for _, pattern := range sensitivePatterns {
			if containsIgnoreCase(output, pattern) {
				violations = append(violations, models.Violation{
					Rule:     "no-sensitive-data-in-output",
					Severity: models.SeverityWarning,
					Message:  fmt.Sprintf("Output may contain sensitive data: %s", pattern),
					Fixable:  false,
				})
			}
		}
	}

	return violations, nil
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
