package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// ValidationError 配置校验错误
type ValidationError struct {
	Field   string
	Value   any
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("config[%s]: %s (got: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult 校验结果
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

func (vr *ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

func (vr *ValidationResult) Error() string {
	var msgs []string
	for _, e := range vr.Errors {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, "; ")
}

// Validate 校验配置
func (c *Config) Validate() *ValidationResult {
	result := &ValidationResult{}

	// Server 校验
	c.validateServer(result)

	// Engine 校验
	c.validateEngine(result)

	// Adapters 校验
	c.validateAdapters(result)

	// Storage 校验
	c.validateStorage(result)

	// Knowledge 校验
	c.validateKnowledge(result)

	// Patterns 校验
	c.validatePatterns(result)

	// Feedback 校验
	c.validateFeedback(result)

	// Monitor 校验
	c.validateMonitor(result)

	return result
}

func (c *Config) validateServer(r *ValidationResult) {
	if c.Server.Addr == "" {
		r.Errors = append(r.Errors, ValidationError{
			Field: "server.addr", Message: "address is required",
		})
	} else {
		// 验证地址格式
		host, port, err := net.SplitHostPort(c.Server.Addr)
		if err != nil {
			r.Errors = append(r.Errors, ValidationError{
				Field: "server.addr", Value: c.Server.Addr,
				Message: fmt.Sprintf("invalid address format: %v", err),
			})
		} else {
			if host != "" && net.ParseIP(host) == nil {
				r.Warnings = append(r.Warnings, ValidationError{
					Field: "server.addr", Value: host,
					Message: "hostname may not resolve in all environments",
				})
			}
			if port == "" {
				r.Errors = append(r.Errors, ValidationError{
					Field: "server.addr", Value: c.Server.Addr,
					Message: "port is required",
				})
			}
		}
	}

	if c.Server.Timeout <= 0 {
		r.Warnings = append(r.Warnings, ValidationError{
			Field: "server.timeout", Value: c.Server.Timeout,
			Message: "using default timeout (30s)",
		})
	}
}

func (c *Config) validateEngine(r *ValidationResult) {
	if c.Engine.MaxConcurrentTasks <= 0 {
		r.Errors = append(r.Errors, ValidationError{
			Field: "engine.max_concurrent_tasks", Value: c.Engine.MaxConcurrentTasks,
			Message: "must be > 0",
		})
	}
	if c.Engine.MaxConcurrentTasks > 100 {
		r.Warnings = append(r.Warnings, ValidationError{
			Field: "engine.max_concurrent_tasks", Value: c.Engine.MaxConcurrentTasks,
			Message: "high concurrency may cause resource exhaustion",
		})
	}
	if c.Engine.TaskTimeout <= 0 {
		r.Warnings = append(r.Warnings, ValidationError{
			Field: "engine.task_timeout", Value: c.Engine.TaskTimeout,
			Message: "using default timeout (5m)",
		})
	}
	if c.Engine.RetryCount < 0 {
		r.Errors = append(r.Errors, ValidationError{
			Field: "engine.retry_count", Value: c.Engine.RetryCount,
			Message: "must be >= 0",
		})
	}
}

func (c *Config) validateAdapters(r *ValidationResult) {
	enabledCount := 0

	if c.Adapters.ClaudeCode.Enabled {
		enabledCount++
		if c.Adapters.ClaudeCode.RootDir != "" {
			if _, err := os.Stat(c.Adapters.ClaudeCode.RootDir); os.IsNotExist(err) {
				r.Errors = append(r.Errors, ValidationError{
					Field: "adapters.claude_code.root_dir",
					Value: c.Adapters.ClaudeCode.RootDir,
					Message: "directory does not exist",
				})
			}
		}
	}

	if c.Adapters.Hermes.Enabled {
		enabledCount++
		if c.Adapters.Hermes.URL == "" {
			r.Errors = append(r.Errors, ValidationError{
				Field: "adapters.hermes.url", Message: "URL is required when Hermes is enabled",
			})
		} else if !strings.HasPrefix(c.Adapters.Hermes.URL, "http") {
			r.Errors = append(r.Errors, ValidationError{
				Field: "adapters.hermes.url", Value: c.Adapters.Hermes.URL,
				Message: "URL must start with http:// or https://",
			})
		}
	}

	if c.Adapters.CodexCLI.Enabled {
		enabledCount++
	}

	if enabledCount == 0 {
		r.Warnings = append(r.Warnings, ValidationError{
			Field: "adapters", Message: "no adapters enabled, engine will have no executors",
		})
	}
}

func (c *Config) validateStorage(r *ValidationResult) {
	validTypes := map[string]bool{"sqlite": true, "memory": true}
	if !validTypes[c.Storage.Type] {
		r.Errors = append(r.Errors, ValidationError{
			Field: "storage.type", Value: c.Storage.Type,
			Message: "must be 'sqlite' or 'memory'",
		})
	}

	if c.Storage.Type == "sqlite" && c.Storage.Path == "" {
		r.Errors = append(r.Errors, ValidationError{
			Field: "storage.path", Message: "path is required for sqlite storage",
		})
	}

	if c.Storage.Path != "" {
		dir := filepath.Dir(c.Storage.Path)
		if dir != "." && dir != "" {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				r.Warnings = append(r.Warnings, ValidationError{
					Field: "storage.path", Value: dir,
					Message: "directory does not exist, will be created on startup",
				})
			}
		}
	}
}

func (c *Config) validateKnowledge(r *ValidationResult) {
	if c.Knowledge.MaxEntries <= 0 {
		r.Errors = append(r.Errors, ValidationError{
			Field: "knowledge.max_entries", Value: c.Knowledge.MaxEntries,
			Message: "must be > 0",
		})
	}
	if c.Knowledge.MaxEntries > 1000000 {
		r.Warnings = append(r.Warnings, ValidationError{
			Field: "knowledge.max_entries", Value: c.Knowledge.MaxEntries,
			Message: "very high limit may cause memory issues",
		})
	}
}

func (c *Config) validatePatterns(r *ValidationResult) {
	if c.Patterns.MinSamples <= 0 {
		r.Errors = append(r.Errors, ValidationError{
			Field: "patterns.min_samples", Value: c.Patterns.MinSamples,
			Message: "must be > 0",
		})
	}
	if c.Patterns.Threshold < 0 || c.Patterns.Threshold > 1 {
		r.Errors = append(r.Errors, ValidationError{
			Field: "patterns.threshold", Value: c.Patterns.Threshold,
			Message: "must be between 0.0 and 1.0",
		})
	}
}

func (c *Config) validateFeedback(r *ValidationResult) {
	if c.Feedback.MaxRetries < 0 {
		r.Errors = append(r.Errors, ValidationError{
			Field: "feedback.max_retries", Value: c.Feedback.MaxRetries,
			Message: "must be >= 0",
		})
	}
	if c.Feedback.RetryDelay < 0 {
		r.Errors = append(r.Errors, ValidationError{
			Field: "feedback.retry_delay", Value: c.Feedback.RetryDelay,
			Message: "must be >= 0",
		})
	}
}

func (c *Config) validateMonitor(r *ValidationResult) {
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if c.Monitor.LogLevel != "" && !validLevels[c.Monitor.LogLevel] {
		r.Errors = append(r.Errors, ValidationError{
			Field: "monitor.log_level", Value: c.Monitor.LogLevel,
			Message: "must be one of: debug, info, warn, error",
		})
	}
	if c.Monitor.MetricsPort < 0 || c.Monitor.MetricsPort > 65535 {
		r.Errors = append(r.Errors, ValidationError{
			Field: "monitor.metrics_port", Value: c.Monitor.MetricsPort,
			Message: "must be between 0 and 65535",
		})
	}
}
