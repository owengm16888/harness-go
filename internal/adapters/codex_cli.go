package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/models"
)

// CodexCLIAdapter Codex CLI 环境适配器
type CodexCLIAdapter struct {
	config     config.AdapterConfig
	agentsPath string
	codexDir   string
}

// NewCodexCLIAdapter 创建 Codex CLI 适配器
func NewCodexCLIAdapter() *CodexCLIAdapter {
	return &CodexCLIAdapter{}
}

// Name 返回适配器名称
func (a *CodexCLIAdapter) Name() string {
	return "codex-cli"
}

// Initialize 初始化适配器
func (a *CodexCLIAdapter) Initialize(ctx context.Context, cfg config.AdapterConfig) error {
	a.config = cfg
	a.agentsPath = filepath.Join(cfg.RootDir, cfg.AgentsPath)
	a.codexDir = filepath.Join(cfg.RootDir, ".codex")

	// 检查 Codex CLI 是否安装
	if err := a.checkCodexCLI(); err != nil {
		return fmt.Errorf("codex CLI not found: %w", err)
	}

	// 加载配置
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	return nil
}

// ExecuteTask 执行任务
func (a *CodexCLIAdapter) ExecuteTask(ctx context.Context, task models.Task) (models.Result, error) {
	startTime := time.Now()

	// 准备 Codex 命令
	cmd := a.prepareCommand(ctx, task)

	// 执行命令
	output, err := cmd.CombinedOutput()
	if err != nil {
		return models.Result{}, fmt.Errorf("codex execution failed: %w, output: %s", err, string(output))
	}

	// 解析输出
	result, err := a.parseOutput(output)
	if err != nil {
		return models.Result{}, fmt.Errorf("failed to parse output: %w", err)
	}

	// 计算指标
	result.Metrics.Duration = time.Since(startTime)

	return *result, nil
}

// GetState 获取状态
func (a *CodexCLIAdapter) GetState(ctx context.Context) (models.State, error) {
	// 读取 AGENTS.md
	content, err := os.ReadFile(a.agentsPath)
	if err != nil {
		return models.State{}, fmt.Errorf("failed to read AGENTS.md: %w", err)
	}

	// 解析状态
	state := a.parseAgents(content)

	return state, nil
}

// Cleanup 清理
func (a *CodexCLIAdapter) Cleanup(ctx context.Context) error {
	// 清理临时文件
	// 保存状态
	return nil
}

// checkCodexCLI 检查 Codex CLI 是否安装
func (a *CodexCLIAdapter) checkCodexCLI() error {
	cmd := exec.Command("codex", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codex CLI not found in PATH")
	}
	return nil
}

// loadConfig 加载配置
func (a *CodexCLIAdapter) loadConfig() error {
	configPath := filepath.Join(a.codexDir, "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // 配置文件不存在，使用默认配置
	}

	// 读取配置
	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// 解析 TOML
	// 这里可以使用 TOML 解析库
	_ = content

	return nil
}

// prepareCommand 准备 Codex 命令
func (a *CodexCLIAdapter) prepareCommand(ctx context.Context, task models.Task) *exec.Cmd {
	args := []string{"exec"}

	// 添加任务描述
	args = append(args, "--task", task.Description)

	// 添加上下文
	if task.Context != nil {
		contextJSON, _ := json.Marshal(task.Context)
		args = append(args, "--context", string(contextJSON))
	}

	// 添加约束
	for _, constraint := range task.Constraints {
		args = append(args, "--constraint", constraint.Rule)
	}

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = a.config.RootDir
	cmd.Env = a.prepareEnv()

	return cmd
}

// prepareEnv 准备环境变量
func (a *CodexCLIAdapter) prepareEnv() []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("CODEX_ROOT=%s", a.config.RootDir))
	return env
}

// parseOutput 解析 Codex 输出
func (a *CodexCLIAdapter) parseOutput(output []byte) (*models.Result, error) {
	var result models.Result
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// parseAgents 解析 AGENTS.md
func (a *CodexCLIAdapter) parseAgents(content []byte) models.State {
	// 解析 markdown
	// 提取任务状态
	// 构建 State
	return models.State{
		Environment: "codex-cli",
		Tasks:       []models.Task{},
		Context:     make(map[string]any),
		Timestamp:   time.Now(),
	}
}
