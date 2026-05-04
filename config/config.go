package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Engine    EngineConfig    `yaml:"engine"`
	Adapters  AdaptersConfig  `yaml:"adapters"`
	Storage   StorageConfig   `yaml:"storage"`
	Knowledge KnowledgeConfig `yaml:"knowledge"`
	Patterns  PatternsConfig  `yaml:"patterns"`
	Feedback  FeedbackConfig  `yaml:"feedback"`
	Monitor   MonitorConfig   `yaml:"monitor"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Addr            string        `yaml:"addr"`
	Timeout         time.Duration `yaml:"timeout"`
	APIKey          string        `yaml:"api_key,omitempty"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout,omitempty"` // 优雅关闭超时
	CORSOrigins     []string      `yaml:"cors_origins,omitempty"`    // 允许的 CORS 来源
}

// EngineConfig 引擎配置
type EngineConfig struct {
	MaxConcurrentTasks int           `yaml:"max_concurrent_tasks"`
	TaskTimeout        time.Duration `yaml:"task_timeout"`
	RetryCount         int           `yaml:"retry_count"`
	ContextInjection   ContextInjectionConfig `yaml:"context_injection"`
}

// ContextInjectionConfig 上下文注入配置
type ContextInjectionConfig struct {
	Enabled           bool `yaml:"enabled"`            // 是否启用智能上下文注入
	KnowledgeLimit    int  `yaml:"knowledge_limit"`    // 注入知识条目上限
	PatternLimit      int  `yaml:"pattern_limit"`      // 注入匹配模式上限
	CacheResults      bool `yaml:"cache_results"`      // 是否缓存注入结果
	InjectConstraints bool `yaml:"inject_constraints"` // 是否注入约束摘要
	InjectMetadata    bool `yaml:"inject_metadata"`    // 是否注入任务元数据
}

// AdaptersConfig 适配器配置
type AdaptersConfig struct {
	ClaudeCode AdapterConfig `yaml:"claude_code"`
	Hermes     HermesConfig  `yaml:"hermes"`
	CodexCLI   AdapterConfig `yaml:"codex_cli"`
}

// AdapterConfig 适配器配置
type AdapterConfig struct {
	Enabled   bool   `yaml:"enabled"`
	RootDir   string `yaml:"root_dir"`
	HooksPath string `yaml:"hooks_path,omitempty"`
	PlansPath string `yaml:"plans_path,omitempty"`
	AgentsPath string `yaml:"agents_path,omitempty"`
	DBPath    string `yaml:"db_path,omitempty"` // 知识库 SQLite 路径
}

// HermesConfig Hermes 配置
type HermesConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type string `yaml:"type"`
	Path string `yaml:"path"`
}

// KnowledgeConfig 知识库配置
type KnowledgeConfig struct {
	MaxEntries int    `yaml:"max_entries"`
	IndexType  string `yaml:"index_type"`
}

// PatternsConfig 模式配置
type PatternsConfig struct {
	MinSamples int     `yaml:"min_samples"`
	Threshold  float64 `yaml:"threshold"`
}

// FeedbackConfig 反馈配置
type FeedbackConfig struct {
	MaxRetries int           `yaml:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay"`
	AutoFix    bool          `yaml:"auto_fix"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	Enabled     bool   `yaml:"enabled"`
	MetricsPort int    `yaml:"metrics_port"`
	LogLevel    string `yaml:"log_level"`
}

// Load 加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 替换环境变量
	data = []byte(os.ExpandEnv(string(data)))

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	// 设置默认值
	config.setDefaults()

	return config, nil
}

// setDefaults 设置默认值
func (c *Config) setDefaults() {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Server.Timeout == 0 {
		c.Server.Timeout = 30 * time.Second
	}
	if c.Server.ShutdownTimeout == 0 {
		c.Server.ShutdownTimeout = 15 * time.Second
	}
	if len(c.Server.CORSOrigins) == 0 {
		c.Server.CORSOrigins = []string{"*"}
	}
	if c.Engine.MaxConcurrentTasks == 0 {
		c.Engine.MaxConcurrentTasks = 10
	}
	if c.Engine.TaskTimeout == 0 {
		c.Engine.TaskTimeout = 5 * time.Minute
	}
	if c.Engine.RetryCount == 0 {
		c.Engine.RetryCount = 3
	}
	if c.Engine.ContextInjection.KnowledgeLimit == 0 {
		c.Engine.ContextInjection.KnowledgeLimit = 5
	}
	if c.Engine.ContextInjection.PatternLimit == 0 {
		c.Engine.ContextInjection.PatternLimit = 3
	}
	// 默认启用上下文注入
	if !c.Engine.ContextInjection.Enabled {
		// Enabled 默认 false，需显式开启
	}
	if c.Storage.Type == "" {
		c.Storage.Type = "sqlite"
	}
	if c.Storage.Path == "" {
		c.Storage.Path = "./data/harness.db"
	}
	if c.Knowledge.MaxEntries == 0 {
		c.Knowledge.MaxEntries = 10000
	}
	if c.Knowledge.IndexType == "" {
		c.Knowledge.IndexType = "bleve"
	}
	if c.Patterns.MinSamples == 0 {
		c.Patterns.MinSamples = 5
	}
	if c.Patterns.Threshold == 0 {
		c.Patterns.Threshold = 0.7
	}
	if c.Feedback.MaxRetries == 0 {
		c.Feedback.MaxRetries = 3
	}
	if c.Feedback.RetryDelay == 0 {
		c.Feedback.RetryDelay = 1 * time.Second
	}
	if c.Monitor.MetricsPort == 0 {
		c.Monitor.MetricsPort = 9090
	}
	if c.Monitor.LogLevel == "" {
		c.Monitor.LogLevel = "info"
	}
}
