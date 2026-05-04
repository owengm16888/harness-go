package routine

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// ============================================================
// LLM 适配器 — 让 Agent 调用大模型
// ============================================================

// LLMProvider LLM 提供者接口
type LLMProvider interface {
	// Chat 发送消息并获取回复
	Chat(ctx context.Context, messages []LLMMessage) (string, error)

	// ChatWithSystem 带系统提示的对话
	ChatWithSystem(ctx context.Context, system string, messages []LLMMessage) (string, error)

	// Name 返回提供者名称
	Name() string
}

// LLMMessage LLM 消息
type LLMMessage struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"`
}

// ============================================================
// Claude 适配器
// ============================================================

// ClaudeProvider Claude LLM 提供者
type ClaudeProvider struct {
	apiKey string
	model  string
}

// NewClaudeProvider 创建 Claude 提供者
func NewClaudeProvider(apiKey, model string) *ClaudeProvider {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &ClaudeProvider{
		apiKey: apiKey,
		model:  model,
	}
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) Chat(ctx context.Context, messages []LLMMessage) (string, error) {
	return p.ChatWithSystem(ctx, "", messages)
}

func (p *ClaudeProvider) ChatWithSystem(ctx context.Context, system string, messages []LLMMessage) (string, error) {
	// TODO: 实现 Claude API 调用
	// 暂时返回模拟响应
	slog.Debug("claude chat", "model", p.model, "messages", len(messages))

	if system != "" {
		return fmt.Sprintf("[Claude %s] 系统提示已收到，基于 %d 条消息生成回复", p.model, len(messages)), nil
	}

	return fmt.Sprintf("[Claude %s] 基于 %d 条消息生成回复", p.model, len(messages)), nil
}

// ============================================================
// OpenAI 适配器
// ============================================================

// OpenAIProvider OpenAI LLM 提供者
type OpenAIProvider struct {
	apiKey string
	model  string
}

// NewOpenAIProvider 创建 OpenAI 提供者
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	if model == "" {
		model = "gpt-4"
	}
	return &OpenAIProvider{
		apiKey: apiKey,
		model:  model,
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Chat(ctx context.Context, messages []LLMMessage) (string, error) {
	return p.ChatWithSystem(ctx, "", messages)
}

func (p *OpenAIProvider) ChatWithSystem(ctx context.Context, system string, messages []LLMMessage) (string, error) {
	// TODO: 实现 OpenAI API 调用
	slog.Debug("openai chat", "model", p.model, "messages", len(messages))

	if system != "" {
		return fmt.Sprintf("[OpenAI %s] 系统提示已收到，基于 %d 条消息生成回复", p.model, len(messages)), nil
	}

	return fmt.Sprintf("[OpenAI %s] 基于 %d 条消息生成回复", p.model, len(messages)), nil
}

// ============================================================
// Claude CLI 适配器 — 通过 claude 命令行调用
// ============================================================

// ClaudeCLIProvider 通过 claude CLI 调用 LLM
type ClaudeCLIProvider struct {
	binPath string
}

// NewClaudeCLIProvider 创建 Claude CLI 提供者
func NewClaudeCLIProvider() *ClaudeCLIProvider {
	return &ClaudeCLIProvider{binPath: "claude"}
}

func (p *ClaudeCLIProvider) Name() string {
	return "claude-cli"
}

func (p *ClaudeCLIProvider) Chat(ctx context.Context, messages []LLMMessage) (string, error) {
	return p.ChatWithSystem(ctx, "", messages)
}

func (p *ClaudeCLIProvider) ChatWithSystem(ctx context.Context, system string, messages []LLMMessage) (string, error) {
	// 合并所有消息为单个 prompt
	var sb strings.Builder
	if system != "" {
		sb.WriteString(system)
		sb.WriteString("\n\n")
	}
	for _, msg := range messages {
		if msg.Role == "assistant" {
			sb.WriteString("Assistant: ")
		} else {
			sb.WriteString("Human: ")
		}
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
	}

	cmd := exec.CommandContext(ctx, p.binPath, "-p", sb.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude CLI failed: %w, output: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// ============================================================
// LLM Agent 执行器 — 包装 LLM 调用
// ============================================================

// LLMAgentExecutor LLM Agent 执行器
type LLMAgentExecutor struct {
	name     string
	role     AgentRole
	provider LLMProvider
	prompt   string
}

// NewLLMAgentExecutor 创建 LLM Agent 执行器
func NewLLMAgentExecutor(name string, role AgentRole, provider LLMProvider, prompt string) *LLMAgentExecutor {
	return &LLMAgentExecutor{
		name:     name,
		role:     role,
		provider: provider,
		prompt:   prompt,
	}
}

func (e *LLMAgentExecutor) Name() string {
	return e.name
}

func (e *LLMAgentExecutor) Role() AgentRole {
	return e.role
}

func (e *LLMAgentExecutor) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	// 构建消息
	messages := e.buildMessages(input)

	// 调用 LLM
	response, err := e.provider.ChatWithSystem(ctx, e.prompt, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// 解析响应
	output := e.parseResponse(response, input)

	return output, nil
}

// buildMessages 构建消息
func (e *LLMAgentExecutor) buildMessages(input AgentInput) []LLMMessage {
	var messages []LLMMessage

	// 添加历史消息
	for _, msg := range input.History {
		role := "user"
		if msg.Role == "interviewer" || msg.Role == "evaluator" || msg.Role == "system" {
			role = "assistant"
		}
		messages = append(messages, LLMMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// 添加当前输入
	if input.Question != "" {
		messages = append(messages, LLMMessage{
			Role:    "user",
			Content: fmt.Sprintf("问题: %s\n\n回答: %s", input.Question, input.Answer),
		})
	}

	return messages
}

// parseResponse 解析响应
func (e *LLMAgentExecutor) parseResponse(response string, input AgentInput) *AgentOutput {
	output := &AgentOutput{
		Content:    response,
		NextAction: "next_step",
	}

	// 根据角色解析不同格式
	switch e.role {
	case RoleInterviewer:
		output.NextAction = "wait_answer"
	case RoleEvaluator:
		// 解析评分
		output.Score = e.parseScore(response)
	case RoleFollowup:
		output.NextAction = "next_step"
	case RoleAnalyzer:
		output.Done = true
		output.NextAction = "end"
		output.Analysis = e.parseAnalysis(response)
	}

	return output
}

// parseScore 解析评分
func (e *LLMAgentExecutor) parseScore(response string) *Score {
	// 简化解析，实际应使用正则或 JSON
	score := &Score{
		Correctness: 7,
		Depth:       6,
		Clarity:     7,
		Practical:   6,
		Total:       65,
		Strengths:   []string{"回答了问题"},
		Weaknesses:  []string{},
		Missing:     []string{},
	}

	return score
}

// parseAnalysis 解析分析
func (e *LLMAgentExecutor) parseAnalysis(response string) *Analysis {
	analysis := &Analysis{
		Level:       "mid",
		Pass:        true,
		StrongAreas: []string{"基础知识扎实"},
		WeakAreas:   []string{},
		StudyPlan:   []StudyItem{},
		Summary:     response,
	}

	return analysis
}

// ============================================================
// LLM Agent 工厂
// ============================================================

// LLMAgentFactory LLM Agent 工厂
type LLMAgentFactory struct {
	provider LLMProvider
}

// NewLLMAgentFactory 创建 LLM Agent 工厂
func NewLLMAgentFactory(provider LLMProvider) *LLMAgentFactory {
	return &LLMAgentFactory{provider: provider}
}

// CreateInterviewer 创建面试官
func (f *LLMAgentFactory) CreateInterviewer(prompt string) AgentExecutor {
	return NewLLMAgentExecutor("interviewer", RoleInterviewer, f.provider, prompt)
}

// CreateEvaluator 创建评估官
func (f *LLMAgentFactory) CreateEvaluator(prompt string) AgentExecutor {
	return NewLLMAgentExecutor("evaluator", RoleEvaluator, f.provider, prompt)
}

// CreateFollowupGenerator 创建追问生成器
func (f *LLMAgentFactory) CreateFollowupGenerator(prompt string) AgentExecutor {
	return NewLLMAgentExecutor("followup_generator", RoleFollowup, f.provider, prompt)
}

// CreateAnalyzer 创建分析器
func (f *LLMAgentFactory) CreateAnalyzer(prompt string) AgentExecutor {
	return NewLLMAgentExecutor("knowledge_gap_analyzer", RoleAnalyzer, f.provider, prompt)
}

// CreateAll 创建所有 Agent
func (f *LLMAgentFactory) CreateAll(config RoutineConfig) map[string]AgentExecutor {
	agents := make(map[string]AgentExecutor)

	for name, agentConfig := range config.Agents {
		var executor AgentExecutor

		switch agentConfig.Role {
		case RoleInterviewer:
			executor = f.CreateInterviewer(agentConfig.Prompt)
		case RoleEvaluator:
			executor = f.CreateEvaluator(agentConfig.Prompt)
		case RoleFollowup:
			executor = f.CreateFollowupGenerator(agentConfig.Prompt)
		case RoleAnalyzer:
			executor = f.CreateAnalyzer(agentConfig.Prompt)
		default:
			executor = NewLLMAgentExecutor(name, agentConfig.Role, f.provider, agentConfig.Prompt)
		}

		agents[name] = executor
	}

	return agents
}

// ============================================================
// 交互式 Session
// ============================================================

// InteractiveSession 交互式会话
type InteractiveSession struct {
	engine   *DefaultRoutineEngine
	instance *RoutineInstance
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewInteractiveSession 创建交互式会话
func NewInteractiveSession(engine *DefaultRoutineEngine, config RoutineConfig) (*InteractiveSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.Settings.Timeout*time.Minute)

	instance, err := engine.Create(ctx, config)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create routine: %w", err)
	}

	return &InteractiveSession{
		engine:   engine,
		instance: instance,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Start 启动会话
func (s *InteractiveSession) Start() error {
	return s.engine.Start(s.ctx, s.instance.ID)
}

// SubmitAnswer 提交回答
func (s *InteractiveSession) SubmitAnswer(answer string) error {
	return s.engine.SubmitAnswer(s.ctx, s.instance.ID, answer)
}

// GetQuestion 获取当前问题
func (s *InteractiveSession) GetQuestion() (string, error) {
	return s.engine.GetNextQuestion(s.ctx, s.instance.ID)
}

// GetReport 获取报告
func (s *InteractiveSession) GetReport() (*FinalReport, error) {
	return s.engine.GetReport(s.ctx, s.instance.ID)
}

// Stop 停止会话
func (s *InteractiveSession) Stop() error {
	s.cancel()
	return s.engine.Stop(s.ctx, s.instance.ID)
}

// GetStatus 获取状态
func (s *InteractiveSession) GetStatus() RoutineStatus {
	return s.instance.Status
}

// GetRound 获取当前轮次
func (s *InteractiveSession) GetRound() int {
	return s.instance.Round
}

// GetHistory 获取历史
func (s *InteractiveSession) GetHistory() []Message {
	return s.instance.GetHistory()
}

// GetScores 获取评分
func (s *InteractiveSession) GetScores() []RoundScore {
	return s.instance.Scores
}

// GetAverageScore 获取平均分
func (s *InteractiveSession) GetAverageScore() Score {
	return s.instance.GetAverageScore()
}
