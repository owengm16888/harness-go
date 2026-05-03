package routine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ============================================================
// Routine 核心模型
// ============================================================

// RoutineStatus Routine 状态
type RoutineStatus string

const (
	StatusPending   RoutineStatus = "pending"
	StatusRunning   RoutineStatus = "running"
	StatusPaused    RoutineStatus = "paused"
	StatusCompleted RoutineStatus = "completed"
	StatusFailed    RoutineStatus = "failed"
)

// RoutineType Routine 类型
type RoutineType string

const (
	TypeInterview    RoutineType = "interview"
	TypeCodeReview   RoutineType = "code_review"
	TypeDebugging    RoutineType = "debugging"
	TypeArchitecture RoutineType = "architecture"
)

// RoutineConfig Routine 配置（从 YAML 加载）
type RoutineConfig struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Type        RoutineType       `yaml:"type"`
	Schedule    ScheduleConfig    `yaml:"schedule"`
	Input       map[string]any    `yaml:"input"`
	Agents      map[string]Agent  `yaml:"agents"`
	Workflow    []WorkflowStep    `yaml:"workflow"`
	Settings    RoutineSettings   `yaml:"settings"`
}

// ScheduleConfig 调度配置
type ScheduleConfig struct {
	Type     string `yaml:"type"`     // manual, cron, trigger
	Cron     string `yaml:"cron"`     // cron 表达式
	Trigger  string `yaml:"trigger"`  // 触发条件
}

// RoutineSettings Routine 设置
type RoutineSettings struct {
	MaxRounds       int           `yaml:"max_rounds"`       // 最大轮次
	Timeout         time.Duration `yaml:"timeout"`          // 超时时间
	AutoEvaluate    bool          `yaml:"auto_evaluate"`    // 自动评估
	EnableFollowup  bool          `yaml:"enable_followup"`  // 启用追问
	EnableScoring   bool          `yaml:"enable_scoring"`   // 启用评分
	OutputFormat    string        `yaml:"output_format"`    // 输出格式
}

// ============================================================
// Agent 模型
// ============================================================

// AgentRole Agent 角色
type AgentRole string

const (
	RoleInterviewer    AgentRole = "interviewer"
	RoleEvaluator      AgentRole = "evaluator"
	RoleFollowup       AgentRole = "followup_generator"
	RoleAnalyzer       AgentRole = "knowledge_gap_analyzer"
	RoleSystem         AgentRole = "system"
)

// Agent Agent 定义
type Agent struct {
	Role    AgentRole `yaml:"role"`
	Name    string    `yaml:"name"`
	Prompt  string    `yaml:"prompt"`
	Model   string    `yaml:"model"`   // 使用的模型
}

// ============================================================
// Workflow 模型
// ============================================================

// WorkflowStep 工作流步骤
type WorkflowStep struct {
	Name        string         `yaml:"name"`
	Agent       string         `yaml:"agent"`       // 引用 agent 名称
	Action      string         `yaml:"action"`      // 动作类型
	Input       map[string]any `yaml:"input"`       // 步骤输入
	Output      string         `yaml:"output"`      // 输出模板
	Condition   string         `yaml:"condition"`   // 条件表达式
	Until       string         `yaml:"until"`       // 循环终止条件
	MaxIter     int            `yaml:"max_iter"`    // 最大迭代次数
	Next        string         `yaml:"next"`        // 下一步（条件分支）
}

// ============================================================
// 运行时模型
// ============================================================

// RoutineInstance Routine 实例
type RoutineInstance struct {
	ID          string            `json:"id"`
	Config      RoutineConfig     `json:"config"`
	Status      RoutineStatus     `json:"status"`
	CurrentStep int               `json:"current_step"`
	Round       int               `json:"round"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     *time.Time        `json:"end_time,omitempty"`
	Context     map[string]any    `json:"context"`     // 运行时上下文
	History     []Message         `json:"history"`     // 消息历史
	Scores      []RoundScore      `json:"scores"`      // 每轮评分
	FinalReport *FinalReport      `json:"final_report,omitempty"`
	mu          sync.RWMutex
}

// Message 消息
type Message struct {
	Role      string    `json:"role"`       // interviewer, candidate, evaluator, system
	Content   string    `json:"content"`
	Round     int       `json:"round"`
	Step      string    `json:"step"`
	Timestamp time.Time `json:"timestamp"`
	Score     *Score    `json:"score,omitempty"`
}

// Score 评分
type Score struct {
	Correctness int      `json:"correctness"` // 1-10
	Depth       int      `json:"depth"`       // 1-10
	Clarity     int      `json:"clarity"`     // 1-10
	Practical   int      `json:"practical"`   // 1-10
	Total       float64  `json:"total"`       // 综合分
	Strengths   []string `json:"strengths"`
	Weaknesses  []string `json:"weaknesses"`
	Missing     []string `json:"missing_points"`
	Feedback    string   `json:"feedback"`
}

// RoundScore 每轮评分
type RoundScore struct {
	Round     int     `json:"round"`
	Question  string  `json:"question"`
	Answer    string  `json:"answer"`
	Score     Score   `json:"score"`
	Timestamp time.Time `json:"timestamp"`
}

// FinalReport 最终报告
type FinalReport struct {
	Level        string        `json:"level"`         // junior, mid, senior
	Pass         bool          `json:"pass"`
	TotalScore   float64       `json:"total_score"`
	MaxScore     float64       `json:"max_score"`
	StrongAreas  []string      `json:"strong_areas"`
	WeakAreas    []string      `json:"weak_areas"`
	StudyPlan    []StudyItem   `json:"study_plan"`
	Summary      string        `json:"summary"`
	Duration     time.Duration `json:"duration"`
	TotalRounds  int           `json:"total_rounds"`
}

// StudyItem 学习计划项
type StudyItem struct {
	Topic    string `json:"topic"`
	Why      string `json:"why"`
	Resource string `json:"resource"`
	Priority int    `json:"priority"`
}

// ============================================================
// 接口定义
// ============================================================

// RoutineEngine Routine 引擎接口
type RoutineEngine interface {
	// Create 创建 Routine 实例
	Create(ctx context.Context, config RoutineConfig) (*RoutineInstance, error)

	// Start 启动 Routine
	Start(ctx context.Context, instanceID string) error

	// Pause 暂停 Routine
	Pause(ctx context.Context, instanceID string) error

	// Resume 恢复 Routine
	Resume(ctx context.Context, instanceID string) error

	// Stop 停止 Routine
	Stop(ctx context.Context, instanceID string) error

	// GetInstance 获取实例
	GetInstance(ctx context.Context, instanceID string) (*RoutineInstance, error)

	// ListInstances 列出实例
	ListInstances(ctx context.Context) ([]*RoutineInstance, error)

	// SubmitAnswer 提交候选人回答
	SubmitAnswer(ctx context.Context, instanceID string, answer string) error

	// GetNextQuestion 获取下一个问题
	GetNextQuestion(ctx context.Context, instanceID string) (string, error)

	// GetReport 获取报告
	GetReport(ctx context.Context, instanceID string) (*FinalReport, error)
}

// AgentExecutor Agent 执行器接口
type AgentExecutor interface {
	// Execute 执行 Agent 任务
	Execute(ctx context.Context, input AgentInput) (*AgentOutput, error)

	// Name 返回 Agent 名称
	Name() string

	// Role 返回 Agent 角色
	Role() AgentRole
}

// AgentInput Agent 输入
type AgentInput struct {
	Question   string         `json:"question,omitempty"`
	Answer     string         `json:"answer,omitempty"`
	Score      *Score         `json:"score,omitempty"`
	Scores     []RoundScore   `json:"scores,omitempty"` // 所有轮次评分
	History    []Message      `json:"history,omitempty"`
	Context    map[string]any `json:"context,omitempty"`
	Config     Agent          `json:"config"`
	Round      int            `json:"round"`
	MaxRounds  int            `json:"max_rounds"`
}

// AgentOutput Agent 输出
type AgentOutput struct {
	Content    string         `json:"content"`
	Question   string         `json:"question,omitempty"`
	Score      *Score         `json:"score,omitempty"`
	Analysis   *Analysis      `json:"analysis,omitempty"`
	Done       bool           `json:"done"`        // 是否结束
	NextAction string         `json:"next_action"` // next_step, repeat, end
	Context    map[string]any `json:"context,omitempty"`
}

// Analysis 分析结果
type Analysis struct {
	StrongAreas []string    `json:"strong_areas"`
	WeakAreas   []string    `json:"weak_areas"`
	Level       string      `json:"level"`
	Pass        bool        `json:"pass"`
	StudyPlan   []StudyItem `json:"study_plan"`
	Summary     string      `json:"summary"`
}

// ============================================================
// 辅助方法
// ============================================================

// NewRoutineInstance 创建新的 Routine 实例
func NewRoutineInstance(config RoutineConfig) *RoutineInstance {
	return &RoutineInstance{
		ID:          fmt.Sprintf("routine-%d", time.Now().UnixNano()),
		Config:      config,
		Status:      StatusPending,
		CurrentStep: 0,
		Round:       0,
		Context:     make(map[string]any),
		History:     make([]Message, 0),
		Scores:      make([]RoundScore, 0),
	}
}

// AddMessage 添加消息
func (ri *RoutineInstance) AddMessage(msg Message) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	msg.Timestamp = time.Now()
	ri.History = append(ri.History, msg)
}

// AddScore 添加评分
func (ri *RoutineInstance) AddScore(score RoundScore) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	score.Timestamp = time.Now()
	ri.Scores = append(ri.Scores, score)
}

// GetAverageScore 获取平均分
func (ri *RoutineInstance) GetAverageScore() Score {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	if len(ri.Scores) == 0 {
		return Score{}
	}

	var total Score
	for _, rs := range ri.Scores {
		total.Correctness += rs.Score.Correctness
		total.Depth += rs.Score.Depth
		total.Clarity += rs.Score.Clarity
		total.Practical += rs.Score.Practical
	}

	n := float64(len(ri.Scores))
	return Score{
		Correctness: int(float64(total.Correctness) / n),
		Depth:       int(float64(total.Depth) / n),
		Clarity:     int(float64(total.Clarity) / n),
		Practical:   int(float64(total.Practical) / n),
		Total:       (float64(total.Correctness) + float64(total.Depth) + float64(total.Clarity) + float64(total.Practical)) / n / 4 * 10,
	}
}

// SetStatus 设置状态
func (ri *RoutineInstance) SetStatus(status RoutineStatus) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.Status = status
	if status == StatusCompleted || status == StatusFailed {
		now := time.Now()
		ri.EndTime = &now
	}
}

// GetHistory 获取历史
func (ri *RoutineInstance) GetHistory() []Message {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	result := make([]Message, len(ri.History))
	copy(result, ri.History)
	return result
}

// GetLastQuestion 获取最后一个问题
func (ri *RoutineInstance) GetLastQuestion() string {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	for i := len(ri.History) - 1; i >= 0; i-- {
		if ri.History[i].Role == "interviewer" || ri.History[i].Role == "system" {
			return ri.History[i].Content
		}
	}
	return ""
}

// GetCandidateAnswers 获取候选人回答
func (ri *RoutineInstance) GetCandidateAnswers() []Message {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	var answers []Message
	for _, msg := range ri.History {
		if msg.Role == "candidate" {
			answers = append(answers, msg)
		}
	}
	return answers
}
