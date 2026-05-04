package routine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// DefaultRoutineEngine 默认 Routine 引擎实现
type DefaultRoutineEngine struct {
	mu        sync.RWMutex
	instances map[string]*RoutineInstance
	agents    map[string]AgentExecutor
	config    EngineConfig
	llm       LLMProvider
}

// EngineConfig 引擎配置
type EngineConfig struct {
	MaxConcurrent int           `yaml:"max_concurrent"`
	DefaultTimeout time.Duration `yaml:"default_timeout"`
	EnableScoring  bool          `yaml:"enable_scoring"`
	EnableFollowup bool          `yaml:"enable_followup"`
}

// NewRoutineEngine 创建 Routine 引擎
func NewRoutineEngine(config EngineConfig) *DefaultRoutineEngine {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 60 * time.Minute
	}

	engine := &DefaultRoutineEngine{
		instances: make(map[string]*RoutineInstance),
		agents:    make(map[string]AgentExecutor),
		config:    config,
	}

	// 注册内置 Agent
	engine.registerBuiltinAgents()

	return engine
}

// registerBuiltinAgents 注册内置 Agent
func (e *DefaultRoutineEngine) registerBuiltinAgents() {
	e.agents["interviewer"] = NewInterviewerAgent()
	e.agents["evaluator"] = NewEvaluatorAgent()
	e.agents["followup_generator"] = NewFollowupAgent()
	e.agents["knowledge_gap_analyzer"] = NewAnalyzerAgent()
}

// Create 创建 Routine 实例
func (e *DefaultRoutineEngine) Create(ctx context.Context, config RoutineConfig) (*RoutineInstance, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 设置默认值 (agent 配置 + workflow)
	loader := NewConfigLoader("")
	loader.setDefaults(&config)

	instance := NewRoutineInstance(config)

	if config.Settings.MaxRounds <= 0 {
		config.Settings.MaxRounds = 10
	}
	if config.Settings.Timeout <= 0 {
		config.Settings.Timeout = e.config.DefaultTimeout
	}

	e.instances[instance.ID] = instance

	slog.Info("routine created",
		"id", instance.ID,
		"name", config.Name,
		"type", config.Type,
	)

	return instance, nil
}

// Start 启动 Routine
func (e *DefaultRoutineEngine) Start(ctx context.Context, instanceID string) error {
	e.mu.Lock()
	instance, exists := e.instances[instanceID]
	if !exists {
		e.mu.Unlock()
		return fmt.Errorf("routine not found: %s", instanceID)
	}

	if instance.Status != StatusPending {
		e.mu.Unlock()
		return fmt.Errorf("routine is not in pending status: %s", instance.Status)
	}

	instance.SetStatus(StatusRunning)
	instance.StartTime = time.Now()
	e.mu.Unlock()

	// 执行第一步
	go e.executeWorkflow(ctx, instance)

	return nil
}

// executeWorkflow 执行工作流
func (e *DefaultRoutineEngine) executeWorkflow(ctx context.Context, instance *RoutineInstance) {
	// 创建超时上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, instance.Config.Settings.Timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			instance.SetStatus(StatusFailed)
			slog.Warn("routine timeout", "id", instance.ID)
			return
		default:
		}

		// 检查状态
		if instance.Status != StatusRunning {
			return
		}

		// 获取当前步骤
		if instance.CurrentStep >= len(instance.Config.Workflow) {
			// 工作流完成一轮 — 检查是否需要继续
			if instance.Round >= instance.Config.Settings.MaxRounds {
				instance.SetStatus(StatusCompleted)
				e.generateFinalReport(ctx, instance)
				return
			}
			// 重置到第一步开始新一轮
			instance.CurrentStep = 0
		}

		step := instance.Config.Workflow[instance.CurrentStep]

		// 执行步骤
		done, err := e.executeStep(timeoutCtx, instance, step)
		if err != nil {
			slog.Error("step execution failed",
				"routine", instance.ID,
				"step", step.Name,
				"error", err,
			)
			instance.SetStatus(StatusFailed)
			return
		}

		if done {
			instance.SetStatus(StatusCompleted)
			e.generateFinalReport(ctx, instance)
			return
		}

		// 处理循环
		if step.Until != "" {
			// 检查终止条件
			if e.checkUntilCondition(instance, step.Until) {
				instance.CurrentStep++ // 超过最大轮次，进入下一步 (final_review)
			}
			// 否则继续当前步骤 (repeat_loop)
		} else {
			instance.CurrentStep++
		}

		// 如果需要等待候选人回答，暂停执行
		if e.needsCandidateInput(step) {
			return
		}
	}
}

// executeStep 执行单个步骤
func (e *DefaultRoutineEngine) executeStep(ctx context.Context, instance *RoutineInstance, step WorkflowStep) (bool, error) {
	// 获取 Agent
	agent, exists := e.agents[step.Agent]
	if !exists {
		return false, fmt.Errorf("agent not found: %s", step.Agent)
	}

	// 准备输入
	input := e.prepareAgentInput(instance, step)

	// 执行 Agent
	output, err := agent.Execute(ctx, input)
	if err != nil {
		return false, fmt.Errorf("agent execution failed: %w", err)
	}

	// 处理输出
	e.processAgentOutput(instance, step, output)

	// 检查是否结束
	if output.Done {
		return true, nil
	}

	return false, nil
}

// prepareAgentInput 准备 Agent 输入
func (e *DefaultRoutineEngine) prepareAgentInput(instance *RoutineInstance, step WorkflowStep) AgentInput {
	input := AgentInput{
		History:   instance.GetHistory(),
		Context:   instance.Context,
		Config:    instance.Config.Agents[step.Agent],
		Round:     instance.Round,
		MaxRounds: instance.Config.Settings.MaxRounds,
	}

	// 根据步骤类型填充输入
	switch step.Action {
	case "ask_question":
		// 面试官提问
		if instance.Round == 0 {
			input.Context["is_first"] = true
		}
	case "evaluate_answer":
		// 评估回答
		input.Question = instance.GetLastQuestion()
		answers := instance.GetCandidateAnswers()
		if len(answers) > 0 {
			input.Answer = answers[len(answers)-1].Content
		}
	case "generate_followup":
		// 生成追问
		input.Question = instance.GetLastQuestion()
		answers := instance.GetCandidateAnswers()
		if len(answers) > 0 {
			input.Answer = answers[len(answers)-1].Content
		}
		if len(instance.Scores) > 0 {
			input.Score = &instance.Scores[len(instance.Scores)-1].Score
		}
	case "final_review":
		// 最终评审
		input.Scores = instance.Scores
	}

	return input
}

// processAgentOutput 处理 Agent 输出
func (e *DefaultRoutineEngine) processAgentOutput(instance *RoutineInstance, step WorkflowStep, output *AgentOutput) {
	// 添加消息到历史
	instance.AddMessage(Message{
		Role:    string(instance.Config.Agents[step.Agent].Role),
		Content: output.Content,
		Round:   instance.Round,
		Step:    step.Name,
	})

	// 处理评分
	if output.Score != nil && instance.Config.Settings.EnableScoring {
		instance.AddScore(RoundScore{
			Round:    instance.Round,
			Question: instance.GetLastQuestion(),
			Score:    *output.Score,
		})
	}

	// 更新上下文
	if output.Context != nil {
		for k, v := range output.Context {
			instance.Context[k] = v
		}
	}

	// 处理 NextAction
	switch output.NextAction {
	case "next_round":
		instance.Round++
		instance.CurrentStep = 0 // 回到工作流开始
	case "next_step":
		// followup_generator 完成 = 一轮结束，递增轮次
		if step.Agent == "followup_generator" {
			instance.Round++
		}
	case "wait_answer":
		// 等待候选人回答 (由 needsCandidateInput 暂停)
	case "end":
		// 面试结束
	}
}

// needsCandidateInput 检查是否需要候选人输入
func (e *DefaultRoutineEngine) needsCandidateInput(step WorkflowStep) bool {
	return step.Action == "ask_question" || step.Action == "wait_answer"
}

// checkUntilCondition 检查终止条件
func (e *DefaultRoutineEngine) checkUntilCondition(instance *RoutineInstance, condition string) bool {
	switch condition {
	case "max_rounds_reached":
		return instance.Round >= instance.Config.Settings.MaxRounds
	case "interview_end":
		return instance.Round >= instance.Config.Settings.MaxRounds
	case "candidate_pass":
		avg := instance.GetAverageScore()
		return avg.Total >= 70
	case "candidate_fail":
		avg := instance.GetAverageScore()
		return avg.Total < 40
	default:
		return false
	}
}

// Pause 暂停 Routine
func (e *DefaultRoutineEngine) Pause(ctx context.Context, instanceID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.instances[instanceID]
	if !exists {
		return fmt.Errorf("routine not found: %s", instanceID)
	}

	if instance.Status != StatusRunning {
		return fmt.Errorf("routine is not running: %s", instance.Status)
	}

	instance.SetStatus(StatusPaused)
	return nil
}

// Resume 恢复 Routine
func (e *DefaultRoutineEngine) Resume(ctx context.Context, instanceID string) error {
	e.mu.Lock()
	instance, exists := e.instances[instanceID]
	if !exists {
		e.mu.Unlock()
		return fmt.Errorf("routine not found: %s", instanceID)
	}

	if instance.Status != StatusPaused {
		e.mu.Unlock()
		return fmt.Errorf("routine is not paused: %s", instance.Status)
	}

	instance.SetStatus(StatusRunning)
	e.mu.Unlock()

	// 继续执行工作流
	go e.executeWorkflow(ctx, instance)

	return nil
}

// Stop 停止 Routine
func (e *DefaultRoutineEngine) Stop(ctx context.Context, instanceID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.instances[instanceID]
	if !exists {
		return fmt.Errorf("routine not found: %s", instanceID)
	}

	instance.SetStatus(StatusCompleted)
	e.generateFinalReport(ctx, instance)

	return nil
}

// SubmitAnswer 提交候选人回答
func (e *DefaultRoutineEngine) SubmitAnswer(ctx context.Context, instanceID string, answer string) error {
	e.mu.Lock()
	instance, exists := e.instances[instanceID]
	if !exists {
		e.mu.Unlock()
		return fmt.Errorf("routine not found: %s", instanceID)
	}

	if instance.Status != StatusRunning {
		e.mu.Unlock()
		return fmt.Errorf("routine is not running: %s", instance.Status)
	}

	// 添加候选人回答
	instance.AddMessage(Message{
		Role:    "candidate",
		Content: answer,
		Round:   instance.Round,
	})
	e.mu.Unlock()

	// 继续执行工作流
	go e.executeWorkflow(ctx, instance)

	return nil
}

// GetNextQuestion 获取下一个问题
func (e *DefaultRoutineEngine) GetNextQuestion(ctx context.Context, instanceID string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	instance, exists := e.instances[instanceID]
	if !exists {
		return "", fmt.Errorf("routine not found: %s", instanceID)
	}

	return instance.GetLastQuestion(), nil
}

// GetInstance 获取实例
func (e *DefaultRoutineEngine) GetInstance(ctx context.Context, instanceID string) (*RoutineInstance, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	instance, exists := e.instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("routine not found: %s", instanceID)
	}

	return instance, nil
}

// ListInstances 列出实例
func (e *DefaultRoutineEngine) ListInstances(ctx context.Context) ([]*RoutineInstance, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	instances := make([]*RoutineInstance, 0, len(e.instances))
	for _, instance := range e.instances {
		instances = append(instances, instance)
	}

	return instances, nil
}

// GetReport 获取报告
func (e *DefaultRoutineEngine) GetReport(ctx context.Context, instanceID string) (*FinalReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	instance, exists := e.instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("routine not found: %s", instanceID)
	}

	if instance.FinalReport == nil {
		return nil, fmt.Errorf("report not ready")
	}

	return instance.FinalReport, nil
}

// generateFinalReport 生成最终报告
func (e *DefaultRoutineEngine) generateFinalReport(ctx context.Context, instance *RoutineInstance) {
	// 使用 analyzer agent 生成报告
	analyzer, exists := e.agents["knowledge_gap_analyzer"]
	if !exists {
		return
	}

	input := AgentInput{
		History:   instance.GetHistory(),
		Scores:    instance.Scores,
		Context:   instance.Context,
		Config:    instance.Config.Agents["knowledge_gap_analyzer"],
		Round:     instance.Round,
		MaxRounds: instance.Config.Settings.MaxRounds,
	}

	output, err := analyzer.Execute(ctx, input)
	if err != nil {
		slog.Error("failed to generate final report", "error", err)
		return
	}

	if output.Analysis != nil {
		instance.FinalReport = &FinalReport{
			Level:       output.Analysis.Level,
			Pass:        output.Analysis.Pass,
			StrongAreas: output.Analysis.StrongAreas,
			WeakAreas:   output.Analysis.WeakAreas,
			StudyPlan:   output.Analysis.StudyPlan,
			Summary:     output.Analysis.Summary,
			Duration:    time.Since(instance.StartTime),
			TotalRounds: instance.Round,
			TotalScore:  instance.GetAverageScore().Total,
			MaxScore:    100,
		}
	}
}

// RegisterAgent 注册 Agent
// SetLLMProvider 设置 LLM 提供者，自动注入到所有支持 LLM 的 Agent
func (e *DefaultRoutineEngine) SetLLMProvider(p LLMProvider) {
	e.llm = p
	for _, agent := range e.agents {
		switch a := agent.(type) {
		case *InterviewerAgent:
			a.provider = p
		case *EvaluatorAgent:
			a.provider = p
		case *FollowupAgent:
			a.provider = p
		case *AnalyzerAgent:
			a.provider = p
		}
	}
}

func (e *DefaultRoutineEngine) RegisterAgent(name string, agent AgentExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.agents[name] = agent
}
