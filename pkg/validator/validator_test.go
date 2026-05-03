package validator

import (
	"testing"

	"github.com/harness-engineering/harness/config"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Addr: ":8080",
		},
		Engine: config.EngineConfig{
			MaxConcurrentTasks: 10,
			RetryCount:         3,
		},
		Storage: config.StorageConfig{
			Type: "sqlite",
			Path: "./data/harness.db",
		},
		Knowledge: config.KnowledgeConfig{
			MaxEntries: 10000,
			IndexType:  "memory",
		},
		Patterns: config.PatternsConfig{
			MinSamples: 5,
			Threshold:  0.7,
		},
		Feedback: config.FeedbackConfig{
			MaxRetries: 3,
		},
		Monitor: config.MonitorConfig{
			MetricsPort: 9090,
			LogLevel:    "info",
		},
	}

	v := New()
	result := v.Validate(cfg)

	if !result.Valid {
		t.Errorf("Expected valid config, got errors: %v", result.Errors)
	}
}

func TestValidate_InvalidServerAddr(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Addr: "",
		},
	}

	v := New()
	result := v.Validate(cfg)

	if result.Valid {
		t.Error("Expected invalid config for empty server address")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "server.addr" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for server.addr")
	}
}

func TestValidate_InvalidEngineConfig(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Addr: ":8080",
		},
		Engine: config.EngineConfig{
			MaxConcurrentTasks: 0,
			RetryCount:         3,
		},
	}

	v := New()
	result := v.Validate(cfg)

	if result.Valid {
		t.Error("Expected invalid config for zero max concurrent tasks")
	}
}

func TestValidate_InvalidStorageType(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Addr: ":8080",
		},
		Engine: config.EngineConfig{
			MaxConcurrentTasks: 10,
			RetryCount:         3,
		},
		Storage: config.StorageConfig{
			Type: "invalid",
			Path: "./data/harness.db",
		},
	}

	v := New()
	result := v.Validate(cfg)

	if result.Valid {
		t.Error("Expected invalid config for invalid storage type")
	}
}

func TestValidate_InvalidMonitorPort(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Addr: ":8080",
		},
		Engine: config.EngineConfig{
			MaxConcurrentTasks: 10,
			RetryCount:         3,
		},
		Storage: config.StorageConfig{
			Type: "sqlite",
			Path: "./data/harness.db",
		},
		Monitor: config.MonitorConfig{
			MetricsPort: 0,
			LogLevel:    "info",
		},
	}

	v := New()
	result := v.Validate(cfg)

	if result.Valid {
		t.Error("Expected invalid config for invalid monitor port")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Addr: ":8080",
		},
		Engine: config.EngineConfig{
			MaxConcurrentTasks: 10,
			RetryCount:         3,
		},
		Storage: config.StorageConfig{
			Type: "sqlite",
			Path: "./data/harness.db",
		},
		Monitor: config.MonitorConfig{
			MetricsPort: 9090,
			LogLevel:    "invalid",
		},
	}

	v := New()
	result := v.Validate(cfg)

	if result.Valid {
		t.Error("Expected invalid config for invalid log level")
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Addr: "",
		},
		Engine: config.EngineConfig{
			MaxConcurrentTasks: 0,
			RetryCount:         -1,
		},
	}

	v := New()
	result := v.Validate(cfg)

	if result.Valid {
		t.Error("Expected invalid config with multiple errors")
	}

	if len(result.Errors) < 2 {
		t.Errorf("Expected at least 2 errors, got %d", len(result.Errors))
	}
}
