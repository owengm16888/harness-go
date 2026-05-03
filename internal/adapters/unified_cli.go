package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/models"
)

// ============================================================
// 统一 CLI 适配器 - 同时支持 Claude Code 和 Codex CLI
// ============================================================

// CLIAdapterType CLI 适配器类型
type CLIAdapterType string

const (
	ClaudeCode CLIAdapterType = "claude-code"
	CodexCLI   CLIAdapterType = "codex-cli"
)

// UnifiedCLIAdapter 统一 CLI 适配器
type UnifiedCLIAdapter struct {
	config      config.AdapterConfig
	adapterType CLIAdapterType
}

// NewUnifiedCLIAdapter 创建统一 CLI 适配器
func NewUnifiedCLIAdapter(adapterType CLIAdapterType) *UnifiedCLIAdapter {
	return &UnifiedCLIAdapter{
		adapterType: adapterType,
	}
}

// Name 返回适配器名称
func (a *UnifiedCLIAdapter) Name() string {
	return string(a.adapterType)
}

// Initialize 初始化适配器
func (a *UnifiedCLIAdapter) Initialize(ctx context.Context, cfg config.AdapterConfig) error {
	a.config = cfg

	// 检查 CLI 是否可用
	if err := a.checkCLI(); err != nil {
		return fmt.Errorf("%s CLI not available: %w", a.adapterType, err)
	}

	return nil
}

// checkCLI 检查 CLI 是否安装
func (a *UnifiedCLIAdapter) checkCLI() error {
	var cmd *exec.Cmd

	switch a.adapterType {
	case ClaudeCode:
		cmd = exec.Command("claude", "--version")
	case CodexCLI:
		cmd = exec.Command("codex", "--version")
	default:
		return fmt.Errorf("unknown adapter type: %s", a.adapterType)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not found in PATH")
	}
	return nil
}

// ExecuteTask 执行任务
func (a *UnifiedCLIAdapter) ExecuteTask(ctx context.Context, task models.Task) (models.Result, error) {
	startTime := time.Now()

	// 构建 prompt (包含 context 和 constraints)
	prompt := a.buildFullPrompt(task)

	// 调用 CLI
	output, err := a.invokeCLI(ctx, prompt, task)
	if err != nil {
		return models.Result{
			TaskID: task.ID,
			Status: models.TaskStatusFailed,
			Errors: []models.Error{{
				Code:        "CLI_EXEC_FAILED",
				Message:     err.Error(),
				Recoverable: true,
				Timestamp:   time.Now(),
			}},
		}, err
	}

	return models.Result{
		TaskID: task.ID,
		Status: models.TaskStatusCompleted,
		Output: output,
		Evidence: []models.Evidence{{
			Type:      task.Type,
			Content:   output,
			Source:    string(a.adapterType),
			Timestamp: time.Now(),
			Verified:  true,
		}},
		Metrics: models.Metrics{
			Duration: time.Since(startTime),
		},
	}, nil
}

// buildFullPrompt 构建完整 prompt (核心逻辑)
func (a *UnifiedCLIAdapter) buildFullPrompt(task models.Task) string {
	var parts []string

	// 1. 任务描述
	parts = append(parts, task.Description)

	// 2. 上下文信息
	if len(task.Context) > 0 {
		contextStr := "\n## Context\n"
		for k, v := range task.Context {
			contextStr += fmt.Sprintf("- %s: %v\n", k, v)
		}
		parts = append(parts, contextStr)
	}

	// 3. 约束条件
	if len(task.Constraints) > 0 {
		constraintsStr := "\n## Constraints (MUST follow)\n"
		for _, c := range task.Constraints {
			constraintsStr += fmt.Sprintf("- [%s] %s: %s\n", c.Severity, c.Rule, c.Message)
		}
		parts = append(parts, constraintsStr)
	}

	// 4. 项目路径信息
	parts = append(parts, fmt.Sprintf("\n## Project\nWorking directory: %s", a.config.RootDir))

	return strings.Join(parts, "\n")
}

// invokeCLI 调用 CLI 命令
func (a *UnifiedCLIAdapter) invokeCLI(ctx context.Context, prompt string, task models.Task) (string, error) {
	switch a.adapterType {
	case ClaudeCode:
		return a.invokeClaudeCode(ctx, prompt, task)
	case CodexCLI:
		return a.invokeCodexCLI(ctx, prompt, task)
	default:
		return "", fmt.Errorf("unknown adapter type: %s", a.adapterType)
	}
}

// invokeClaudeCode 调用 Claude Code CLI
//
// 命令格式:
//   claude -p "prompt" --output-format json --model claude-sonnet-4-...
//
// 参数说明:
//   -p                    非交互模式，打印输出后退出
//   --output-format json  JSON 格式输出
//   --model               指定模型
//   --system-prompt       系统提示
//   --append-system-prompt 追加系统提示
func (a *UnifiedCLIAdapter) invokeClaudeCode(ctx context.Context, prompt string, task models.Task) (string, error) {
	args := []string{
		"-p", prompt,
		"--output-format", "text", // 或 "json"
	}

	// 可选: 指定模型
	if model, ok := task.Context["model"]; ok {
		args = append(args, "--model", fmt.Sprintf("%v", model))
	}

	// 可选: 系统提示
	if systemPrompt, ok := task.Context["system_prompt"]; ok {
		args = append(args, "--system-prompt", fmt.Sprintf("%v", systemPrompt))
	}

	// 可选: 追加系统提示
	if appendPrompt, ok := task.Context["append_system_prompt"]; ok {
		args = append(args, "--append-system-prompt", fmt.Sprintf("%v", appendPrompt))
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = a.config.RootDir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude CLI failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// invokeCodexCLI 调用 Codex CLI
//
// 命令格式:
//   codex --quiet "prompt"
//   codex -m o4-mini --approval-mode full-auto "prompt"
//
// 参数说明:
//   --quiet               非交互模式
//   -m model              指定模型
//   --approval-mode       审批模式 (suggest/auto-edit/full-auto)
func (a *UnifiedCLIAdapter) invokeCodexCLI(ctx context.Context, prompt string, task models.Task) (string, error) {
	args := []string{
		"--quiet", // 非交互模式
	}

	// 可选: 指定模型
	if model, ok := task.Context["model"]; ok {
		args = append(args, "-m", fmt.Sprintf("%v", model))
	}

	// 可选: 审批模式
	if approvalMode, ok := task.Context["approval_mode"]; ok {
		args = append(args, "--approval-mode", fmt.Sprintf("%v", approvalMode))
	} else {
		// 默认全自动化模式
		args = append(args, "--approval-mode", "full-auto")
	}

	// 添加 prompt
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = a.config.RootDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CODEX_ROOT=%s", a.config.RootDir),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("codex CLI failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// GetState 获取状态
func (a *UnifiedCLIAdapter) GetState(ctx context.Context) (models.State, error) {
	return models.State{
		Environment: string(a.adapterType),
		Tasks:       []models.Task{},
		Context:     make(map[string]any),
		Timestamp:   time.Now(),
	}, nil
}

// Cleanup 清理
func (a *UnifiedCLIAdapter) Cleanup(ctx context.Context) error {
	// 清理临时文件和连接
	return nil
}

// ============================================================
// 便捷构造函数
// ============================================================

// NewClaudeCodeAdapter2 创建 Claude Code 适配器
func NewClaudeCodeAdapter2() *UnifiedCLIAdapter {
	return NewUnifiedCLIAdapter(ClaudeCode)
}

// NewCodexCLIAdapter2 创建 Codex CLI 适配器
func NewCodexCLIAdapter2() *UnifiedCLIAdapter {
	return NewUnifiedCLIAdapter(CodexCLI)
}

// ============================================================
// 使用示例
// ============================================================

// ExampleUsage 示例用法
func ExampleUsage() {
	ctx := context.Background()

	// 创建适配器
	claudeAdapter := NewClaudeCodeAdapter2()
	codexAdapter := NewCodexCLIAdapter2()

	// 初始化
	cfg := config.AdapterConfig{
		RootDir: "/path/to/project",
	}
	claudeAdapter.Initialize(ctx, cfg)
	codexAdapter.Initialize(ctx, cfg)

	// 构建任务 (带完整 context 和 constraints)
	task := models.Task{
		ID:          "task-001",
		Type:        "implement",
		Description: "实现一个 HTTP 中间件，支持请求日志和限流",
		Context: map[string]any{
			"language":             "Go",
			"framework":            "gin",
			"model":                "claude-sonnet-4-20250514", // Claude Code 专用
			"system_prompt":        "你是一个资深 Go 开发者",
			"append_system_prompt": "请遵循项目现有的代码风格",
			"approval_mode":        "full-auto", // Codex 专用
			"existing_code":        `func main() { r := gin.Default(); r.Run() }`,
		},
		Constraints: []models.Constraint{
			{
				Type:     "quality",
				Rule:     "require-tests",
				Severity: models.SeverityError,
				Message:  "必须编写单元测试",
			},
			{
				Type:     "security",
				Rule:     "no-hardcoded-secrets",
				Severity: models.SeverityCritical,
				Message:  "禁止硬编码密钥",
			},
		},
	}

	// 调用 Claude Code
	claudeResult, err := claudeAdapter.ExecuteTask(ctx, task)
	if err != nil {
		fmt.Printf("Claude Code failed: %v\n", err)
	} else {
		fmt.Printf("Claude Code result: %v\n", claudeResult.Output)
	}

	// 调用 Codex CLI
	codexResult, err := codexAdapter.ExecuteTask(ctx, task)
	if err != nil {
		fmt.Printf("Codex CLI failed: %v\n", err)
	} else {
		fmt.Printf("Codex CLI result: %v\n", codexResult.Output)
	}

	// JSON 序列化结果
	claudeJSON, _ := json.MarshalIndent(claudeResult, "", "  ")
	fmt.Printf("Claude Result JSON:\n%s\n", claudeJSON)
}
