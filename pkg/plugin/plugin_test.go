package plugin

import (
	"context"
	"testing"
)

// MockPlugin 模拟插件
type MockPlugin struct {
	name        string
	version     string
	description string
	initialized bool
	started     bool
	stopped     bool
	healthy     bool
}

func NewMockPlugin(name, version, description string) *MockPlugin {
	return &MockPlugin{
		name:        name,
		version:     version,
		description: description,
		healthy:     true,
	}
}

func (p *MockPlugin) Name() string {
	return p.name
}

func (p *MockPlugin) Version() string {
	return p.version
}

func (p *MockPlugin) Description() string {
	return p.description
}

func (p *MockPlugin) Initialize(ctx context.Context, config map[string]any) error {
	p.initialized = true
	return nil
}

func (p *MockPlugin) Start(ctx context.Context) error {
	p.started = true
	return nil
}

func (p *MockPlugin) Stop(ctx context.Context) error {
	p.stopped = true
	return nil
}

func (p *MockPlugin) Health(ctx context.Context) error {
	if !p.healthy {
		return &PluginError{Message: "unhealthy"}
	}
	return nil
}

type PluginError struct {
	Message string
}

func (e *PluginError) Error() string {
	return e.Message
}

func TestPluginManager_LoadPlugin(t *testing.T) {
	mgr := NewPluginManager(PluginManagerConfig{})
	mockPlugin := NewMockPlugin("test-plugin", "1.0.0", "Test plugin")

	err := mgr.LoadPlugin("/path/to/plugin", mockPlugin)
	if err != nil {
		t.Fatalf("Failed to load plugin: %v", err)
	}

	plugins := mgr.ListPlugins()
	if len(plugins) != 1 {
		t.Errorf("Expected 1 plugin, got %d", len(plugins))
	}
}

func TestPluginManager_UnloadPlugin(t *testing.T) {
	mgr := NewPluginManager(PluginManagerConfig{})
	mockPlugin := NewMockPlugin("test-plugin", "1.0.0", "Test plugin")

	mgr.LoadPlugin("/path/to/plugin", mockPlugin)

	err := mgr.UnloadPlugin("test-plugin")
	if err != nil {
		t.Fatalf("Failed to unload plugin: %v", err)
	}

	plugins := mgr.ListPlugins()
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins, got %d", len(plugins))
	}
}

func TestPluginManager_GetPlugin(t *testing.T) {
	mgr := NewPluginManager(PluginManagerConfig{})
	mockPlugin := NewMockPlugin("test-plugin", "1.0.0", "Test plugin")

	mgr.LoadPlugin("/path/to/plugin", mockPlugin)

	retrieved, err := mgr.GetPlugin("test-plugin")
	if err != nil {
		t.Fatalf("Failed to get plugin: %v", err)
	}

	if retrieved.Name() != "test-plugin" {
		t.Errorf("Expected plugin name test-plugin, got %s", retrieved.Name())
	}
}

func TestPluginManager_EnableDisablePlugin(t *testing.T) {
	mgr := NewPluginManager(PluginManagerConfig{})
	mockPlugin := NewMockPlugin("test-plugin", "1.0.0", "Test plugin")

	mgr.LoadPlugin("/path/to/plugin", mockPlugin)

	// 禁用插件
	err := mgr.DisablePlugin("test-plugin")
	if err != nil {
		t.Fatalf("Failed to disable plugin: %v", err)
	}

	info, _ := mgr.GetPluginInfo("test-plugin")
	if info.Enabled {
		t.Error("Expected plugin to be disabled")
	}

	// 启用插件
	err = mgr.EnablePlugin("test-plugin")
	if err != nil {
		t.Fatalf("Failed to enable plugin: %v", err)
	}

	info, _ = mgr.GetPluginInfo("test-plugin")
	if !info.Enabled {
		t.Error("Expected plugin to be enabled")
	}
}

func TestPluginManager_HealthCheck(t *testing.T) {
	mgr := NewPluginManager(PluginManagerConfig{})
	mockPlugin := NewMockPlugin("test-plugin", "1.0.0", "Test plugin")

	mgr.LoadPlugin("/path/to/plugin", mockPlugin)

	ctx := context.Background()
	results := mgr.HealthCheck(ctx)

	if len(results) != 1 {
		t.Errorf("Expected 1 health check result, got %d", len(results))
	}

	if results["test-plugin"] != nil {
		t.Errorf("Expected healthy, got %v", results["test-plugin"])
	}
}

func TestPluginManager_GetStats(t *testing.T) {
	mgr := NewPluginManager(PluginManagerConfig{})
	mockPlugin1 := NewMockPlugin("plugin-1", "1.0.0", "Plugin 1")
	mockPlugin2 := NewMockPlugin("plugin-2", "1.0.0", "Plugin 2")

	mgr.LoadPlugin("/path/to/plugin1", mockPlugin1)
	mgr.LoadPlugin("/path/to/plugin2", mockPlugin2)

	// 禁用一个插件
	mgr.DisablePlugin("plugin-2")

	stats := mgr.GetStats()

	if stats.TotalPlugins != 2 {
		t.Errorf("Expected 2 total plugins, got %d", stats.TotalPlugins)
	}

	if stats.EnabledPlugins != 1 {
		t.Errorf("Expected 1 enabled plugin, got %d", stats.EnabledPlugins)
	}
}

func TestPluginManager_StartAll(t *testing.T) {
	mgr := NewPluginManager(PluginManagerConfig{})
	mockPlugin1 := NewMockPlugin("plugin-1", "1.0.0", "Plugin 1")
	mockPlugin2 := NewMockPlugin("plugin-2", "1.0.0", "Plugin 2")

	mgr.LoadPlugin("/path/to/plugin1", mockPlugin1)
	mgr.LoadPlugin("/path/to/plugin2", mockPlugin2)

	ctx := context.Background()
	err := mgr.StartAll(ctx)
	if err != nil {
		t.Fatalf("Failed to start all plugins: %v", err)
	}

	if !mockPlugin1.started {
		t.Error("Expected plugin-1 to be started")
	}

	if !mockPlugin2.started {
		t.Error("Expected plugin-2 to be started")
	}
}

func TestPluginManager_StopAll(t *testing.T) {
	mgr := NewPluginManager(PluginManagerConfig{})
	mockPlugin1 := NewMockPlugin("plugin-1", "1.0.0", "Plugin 1")
	mockPlugin2 := NewMockPlugin("plugin-2", "1.0.0", "Plugin 2")

	mgr.LoadPlugin("/path/to/plugin1", mockPlugin1)
	mgr.LoadPlugin("/path/to/plugin2", mockPlugin2)

	ctx := context.Background()
	err := mgr.StopAll(ctx)
	if err != nil {
		t.Fatalf("Failed to stop all plugins: %v", err)
	}

	if !mockPlugin1.stopped {
		t.Error("Expected plugin-1 to be stopped")
	}

	if !mockPlugin2.stopped {
		t.Error("Expected plugin-2 to be stopped")
	}
}
