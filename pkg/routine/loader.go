package routine

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigLoader 配置加载器
type ConfigLoader struct {
	configDir string
}

// NewConfigLoader 创建配置加载器
func NewConfigLoader(configDir string) *ConfigLoader {
	return &ConfigLoader{configDir: configDir}
}

// Load 加载配置
func (l *ConfigLoader) Load(filename string) (*RoutineConfig, error) {
	path := filepath.Join(l.configDir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config RoutineConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 设置默认值
	l.setDefaults(&config)

	return &config, nil
}

// LoadAll 加载所有配置
func (l *ConfigLoader) LoadAll() ([]*RoutineConfig, error) {
	entries, err := os.ReadDir(l.configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	var configs []*RoutineConfig
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		config, err := l.Load(entry.Name())
		if err != nil {
			continue
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// setDefaults 设置默认值
func (l *ConfigLoader) setDefaults(config *RoutineConfig) {
	if config.Settings.MaxRounds <= 0 {
		config.Settings.MaxRounds = 10
	}

	if config.Settings.Timeout <= 0 {
		config.Settings.Timeout = 60 // 60 分钟
	}

	if config.Settings.OutputFormat == "" {
		config.Settings.OutputFormat = "text"
	}

	// 设置默认 Agent
	if config.Agents == nil {
		config.Agents = make(map[string]Agent)
	}

	if _, exists := config.Agents["interviewer"]; !exists {
		config.Agents["interviewer"] = Agent{
			Role:   RoleInterviewer,
			Name:   "interviewer",
			Prompt: getDefaultInterviewerPrompt(),
		}
	}

	if _, exists := config.Agents["evaluator"]; !exists {
		config.Agents["evaluator"] = Agent{
			Role:   RoleEvaluator,
			Name:   "evaluator",
			Prompt: getDefaultEvaluatorPrompt(),
		}
	}

	if _, exists := config.Agents["followup_generator"]; !exists {
		config.Agents["followup_generator"] = Agent{
			Role:   RoleFollowup,
			Name:   "followup_generator",
			Prompt: getDefaultFollowupPrompt(),
		}
	}

	if _, exists := config.Agents["knowledge_gap_analyzer"]; !exists {
		config.Agents["knowledge_gap_analyzer"] = Agent{
			Role:   RoleAnalyzer,
			Name:   "knowledge_gap_analyzer",
			Prompt: getDefaultAnalyzerPrompt(),
		}
	}

	// 设置默认工作流
	if len(config.Workflow) == 0 {
		config.Workflow = getDefaultWorkflow()
	}
}

// getDefaultInterviewerPrompt 获取默认面试官 Prompt
func getDefaultInterviewerPrompt() string {
	return `你是一位资深 Go 语言面试官。

职责：
1. 提出技术问题
2. 基于候选人回答进行追问
3. 深挖项目经验

规则：
- 一次只问一个问题
- 不允许一次问多个问题
- 回答模糊时必须追问
- 回答正确时提高难度

面试范围：
- Go runtime
- scheduler (GMP)
- GC (三色标记、写屏障、STW)
- concurrency (channel、select、sync)
- memory management
- distributed systems`
}

// getDefaultEvaluatorPrompt 获取默认评估 Prompt
func getDefaultEvaluatorPrompt() string {
	return `你是技术评估官。

对候选人回答进行评估：

输出格式：
score:
  correctness: 1-10
  depth: 1-10
  clarity: 1-10
  practical: 1-10

strengths:
  - ...

weaknesses:
  - ...

missing_points:
  - ...`
}

// getDefaultFollowupPrompt 获取默认追问 Prompt
func getDefaultFollowupPrompt() string {
	return `你负责生成下一轮追问。

输入：
- 当前问题
- 候选人回答
- 评估结果

输出：
- 一个更深层问题

规则：
- 必须追问薄弱点
- 优先追问 runtime / GC / scheduler`
}

// getDefaultAnalyzerPrompt 获取默认分析 Prompt
func getDefaultAnalyzerPrompt() string {
	return `面试结束后分析候选人知识盲区。

输出：
final_report:
  level: junior/mid/senior
  pass: true/false

  strong_areas:
    - ...

  weak_areas:
    - ...

  study_plan:
    - topic
    - why
    - resource`
}

// getDefaultWorkflow 获取默认工作流
func getDefaultWorkflow() []WorkflowStep {
	return []WorkflowStep{
		{
			Name:   "start_interview",
			Agent:  "interviewer",
			Action: "ask_question",
		},
		{
			Name:   "evaluate_answer",
			Agent:  "evaluator",
			Action: "evaluate_answer",
		},
		{
			Name:   "generate_followup",
			Agent:  "followup_generator",
			Action: "generate_followup",
		},
		{
			Name:    "repeat_loop",
			Agent:   "interviewer",
			Action:  "ask_question",
			Until:   "max_rounds_reached",
			MaxIter: 10,
		},
		{
			Name:   "final_review",
			Agent:  "knowledge_gap_analyzer",
			Action: "final_review",
		},
	}
}

// SaveConfig 保存配置
func (l *ConfigLoader) SaveConfig(config *RoutineConfig, filename string) error {
	path := filepath.Join(l.configDir, filename)

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// ListConfigs 列出配置文件
func (l *ConfigLoader) ListConfigs() ([]string, error) {
	entries, err := os.ReadDir(l.configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext == ".yaml" || ext == ".yml" {
			names = append(names, entry.Name())
		}
	}

	return names, nil
}

// CreateFromTemplate 从模板创建配置
func (l *ConfigLoader) CreateFromTemplate(templateName string, params map[string]any) (*RoutineConfig, error) {
	// 预定义模板
	templates := map[string]RoutineConfig{
		"basic-interview": {
			Name:        "basic-go-interview",
			Description: "基础 Go 面试",
			Type:        TypeInterview,
			Settings: RoutineSettings{
				MaxRounds:    5,
				AutoEvaluate: true,
				EnableScoring: true,
			},
		},
		"advanced-interview": {
			Name:        "advanced-go-interview",
			Description: "高级 Go 面试（含压力追问）",
			Type:        TypeInterview,
			Settings: RoutineSettings{
				MaxRounds:      10,
				AutoEvaluate:   true,
				EnableScoring:  true,
				EnableFollowup: true,
			},
		},
		"code-review": {
			Name:        "code-review",
			Description: "代码审查",
			Type:        TypeCodeReview,
			Settings: RoutineSettings{
				MaxRounds:    3,
				AutoEvaluate: true,
			},
		},
	}

	template, exists := templates[templateName]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	// 应用参数
	if name, ok := params["name"].(string); ok {
		template.Name = name
	}

	l.setDefaults(&template)

	return &template, nil
}
