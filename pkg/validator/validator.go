package validator

import (
	"fmt"
	"strings"

	"github.com/harness-engineering/harness/config"
)

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error 实现 error 接口
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid  bool               `json:"valid"`
	Errors []ValidationError  `json:"errors,omitempty"`
}

// Validator 验证器
type Validator struct {
	errors []ValidationError
}

// New 创建验证器
func New() *Validator {
	return &Validator{
		errors: []ValidationError{},
	}
}

// Validate 验证配置
func (v *Validator) Validate(cfg *config.Config) *ValidationResult {
	v.errors = []ValidationError{}

	// 验证服务器配置
	v.validateServer(cfg.Server)

	// 验证引擎配置
	v.validateEngine(cfg.Engine)

	// 验证适配器配置
	v.validateAdapters(cfg.Adapters)

	// 验证存储配置
	v.validateStorage(cfg.Storage)

	// 验证知识库配置
	v.validateKnowledge(cfg.Knowledge)

	// 验证模式配置
	v.validatePatterns(cfg.Patterns)

	// 验证反馈配置
	v.validateFeedback(cfg.Feedback)

	// 验证监控配置
	v.validateMonitor(cfg.Monitor)

	return &ValidationResult{
		Valid:  len(v.errors) == 0,
		Errors: v.errors,
	}
}

// validateServer 验证服务器配置
func (v *Validator) validateServer(cfg config.ServerConfig) {
	if cfg.Addr == "" {
		v.addError("server.addr", "服务器地址不能为空")
	}

	if !strings.HasPrefix(cfg.Addr, ":") && !strings.Contains(cfg.Addr, ":") {
		v.addError("server.addr", "服务器地址格式无效，应为 :port 或 host:port")
	}
}

// validateEngine 验证引擎配置
func (v *Validator) validateEngine(cfg config.EngineConfig) {
	if cfg.MaxConcurrentTasks <= 0 {
		v.addError("engine.max_concurrent_tasks", "最大并发任务数必须大于 0")
	}

	if cfg.RetryCount < 0 {
		v.addError("engine.retry_count", "重试次数不能为负数")
	}
}

// validateAdapters 验证适配器配置
func (v *Validator) validateAdapters(cfg config.AdaptersConfig) {
	// 验证 Claude Code 适配器
	if cfg.ClaudeCode.Enabled {
		v.validateClaudeCodeAdapter(cfg.ClaudeCode)
	}

	// 验证 Hermes 适配器
	if cfg.Hermes.Enabled {
		v.validateHermesAdapter(cfg.Hermes)
	}

	// 验证 Codex CLI 适配器
	if cfg.CodexCLI.Enabled {
		v.validateCodexCLIAdapter(cfg.CodexCLI)
	}
}

// validateClaudeCodeAdapter 验证 Claude Code 适配器
func (v *Validator) validateClaudeCodeAdapter(cfg config.AdapterConfig) {
	if cfg.RootDir == "" {
		v.addError("adapters.claude_code.root_dir", "根目录不能为空")
	}

	if cfg.HooksPath == "" {
		v.addError("adapters.claude_code.hooks_path", "Hooks 路径不能为空")
	}

	if cfg.PlansPath == "" {
		v.addError("adapters.claude_code.plans_path", "Plans 路径不能为空")
	}
}

// validateHermesAdapter 验证 Hermes 适配器
func (v *Validator) validateHermesAdapter(cfg config.HermesConfig) {
	if cfg.URL == "" {
		v.addError("adapters.hermes.url", "URL 不能为空")
	}
}

// validateCodexCLIAdapter 验证 Codex CLI 适配器
func (v *Validator) validateCodexCLIAdapter(cfg config.AdapterConfig) {
	if cfg.RootDir == "" {
		v.addError("adapters.codex_cli.root_dir", "根目录不能为空")
	}

	if cfg.AgentsPath == "" {
		v.addError("adapters.codex_cli.agents_path", "Agents 路径不能为空")
	}
}

// validateStorage 验证存储配置
func (v *Validator) validateStorage(cfg config.StorageConfig) {
	if cfg.Type == "" {
		v.addError("storage.type", "存储类型不能为空")
	}

	validTypes := []string{"sqlite", "postgres", "mysql", "memory"}
	valid := false
	for _, t := range validTypes {
		if cfg.Type == t {
			valid = true
			break
		}
	}
	if !valid {
		v.addError("storage.type", fmt.Sprintf("存储类型无效，应为: %s", strings.Join(validTypes, ", ")))
	}

	if cfg.Type != "memory" && cfg.Path == "" {
		v.addError("storage.path", "存储路径不能为空")
	}
}

// validateKnowledge 验证知识库配置
func (v *Validator) validateKnowledge(cfg config.KnowledgeConfig) {
	if cfg.MaxEntries <= 0 {
		v.addError("knowledge.max_entries", "最大条目数必须大于 0")
	}

	validIndexTypes := []string{"memory", "bleve", "sqlite"}
	valid := false
	for _, t := range validIndexTypes {
		if cfg.IndexType == t {
			valid = true
			break
		}
	}
	if !valid {
		v.addError("knowledge.index_type", fmt.Sprintf("索引类型无效，应为: %s", strings.Join(validIndexTypes, ", ")))
	}
}

// validatePatterns 验证模式配置
func (v *Validator) validatePatterns(cfg config.PatternsConfig) {
	if cfg.MinSamples <= 0 {
		v.addError("patterns.min_samples", "最小样本数必须大于 0")
	}

	if cfg.Threshold < 0 || cfg.Threshold > 1 {
		v.addError("patterns.threshold", "阈值必须在 0 到 1 之间")
	}
}

// validateFeedback 验证反馈配置
func (v *Validator) validateFeedback(cfg config.FeedbackConfig) {
	if cfg.MaxRetries < 0 {
		v.addError("feedback.max_retries", "最大重试次数不能为负数")
	}
}

// validateMonitor 验证监控配置
func (v *Validator) validateMonitor(cfg config.MonitorConfig) {
	if cfg.MetricsPort <= 0 || cfg.MetricsPort > 65535 {
		v.addError("monitor.metrics_port", "指标端口必须在 1 到 65535 之间")
	}

	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	valid := false
	for _, l := range validLogLevels {
		if cfg.LogLevel == l {
			valid = true
			break
		}
	}
	if !valid {
		v.addError("monitor.log_level", fmt.Sprintf("日志级别无效，应为: %s", strings.Join(validLogLevels, ", ")))
	}
}

// addError 添加错误
func (v *Validator) addError(field, message string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: message,
	})
}
