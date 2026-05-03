package models

import (
	"encoding/json"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// Severity 严重程度
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Task 表示一个任务
type Task struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Context     map[string]any    `json:"context"`
	Constraints []Constraint      `json:"constraints"`
	Priority    int               `json:"priority"`
	Deadline    *time.Time        `json:"deadline,omitempty"`
}

// Constraint 表示约束条件
type Constraint struct {
	Type      string         `json:"type"`
	Rule      string         `json:"rule"`
	Severity  Severity       `json:"severity"`
	Message   string         `json:"message"`
	Validator func(any) bool `json:"-"`
}

// Result 表示任务结果
type Result struct {
	TaskID   string   `json:"task_id"`
	Status   TaskStatus `json:"status"`
	Output   any      `json:"output"`
	Evidence []Evidence `json:"evidence"`
	Metrics  Metrics  `json:"metrics"`
	Errors   []Error  `json:"errors,omitempty"`
}

// Evidence 表示证据
type Evidence struct {
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
	Verified  bool      `json:"verified"`
}

// Metrics 表示指标
type Metrics struct {
	Duration   time.Duration `json:"duration"`
	TokenCount int           `json:"token_count"`
	ToolUses   int           `json:"tool_uses"`
	ErrorCount int           `json:"error_count"`
	RetryCount int           `json:"retry_count"`
}

// Error 表示错误
type Error struct {
	Code        string    `json:"code"`
	Message     string    `json:"message"`
	Details     any       `json:"details,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Recoverable bool      `json:"recoverable"`
}

// State 表示系统状态
type State struct {
	SessionID   string         `json:"session_id"`
	Environment string         `json:"environment"`
	Tasks       []Task         `json:"tasks"`
	Context     map[string]any `json:"context"`
	Timestamp   time.Time      `json:"timestamp"`
}

// Pattern 表示模式
type Pattern struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Trigger     string    `json:"trigger"`
	Actions     []Action  `json:"actions"`
	Metadata    map[string]any `json:"metadata"`
	SuccessRate float64   `json:"success_rate"`
	UsageCount  int       `json:"usage_count"`
	LastUsed    time.Time `json:"last_used"`
}

// Action 表示动作
type Action struct {
	Type       string         `json:"type"`
	Parameters map[string]any `json:"parameters"`
	Timeout    time.Duration  `json:"timeout"`
	Retryable  bool           `json:"retryable"`
}

// Violation 表示违规
type Violation struct {
	Rule      string    `json:"rule"`
	Severity  Severity  `json:"severity"`
	Message   string    `json:"message"`
	Evidence  Evidence  `json:"evidence"`
	Fixable   bool      `json:"fixable"`
	Timestamp time.Time `json:"timestamp"`
}

// FixResult 修复结果
type FixResult struct {
	Success   bool      `json:"success"`
	Changes   []Change  `json:"changes"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Change 表示变更
type Change struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// FeedbackResult 反馈结果
type FeedbackResult struct {
	TaskID     string      `json:"task_id"`
	Status     string      `json:"status"`
	Violations []Violation `json:"violations"`
	Fixes      []FixResult `json:"fixes"`
	Timestamp  time.Time   `json:"timestamp"`
}

// KnowledgeEntry 知识条目
type KnowledgeEntry struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
	References  []Reference    `json:"references"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	AccessCount int            `json:"access_count"`
}

// Reference 引用
type Reference struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	URL  string `json:"url,omitempty"`
}

// Observation 观察
type Observation struct {
	Task      Task      `json:"task"`
	Result    Result    `json:"result"`
	Pattern   string    `json:"pattern"`
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
}

// Prediction 预测
type Prediction struct {
	PatternID       string  `json:"pattern_id"`
	Confidence      float64 `json:"confidence"`
	ExpectedOutcome Outcome `json:"expected_outcome"`
	Recommendation  string  `json:"recommendation"`
}

// Outcome 结果
type Outcome struct {
	Success  float64 `json:"success"`  // 成功率 0.0-1.0
	Duration float64 `json:"duration"`
	Quality  float64 `json:"quality"`
}

// Event 事件
type Event struct {
	Type      string    `json:"type"`
	SessionID string    `json:"session_id"`
	Data      any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

// StateUpdate 状态更新
type StateUpdate struct {
	Type   string          `json:"type"`
	Data   json.RawMessage `json:"data"`
	Reason string          `json:"reason"`
}

// TaskUpdate 任务更新
type TaskUpdate struct {
	ID   string `json:"id"`
	Task Task   `json:"task"`
}

// ContextUpdate 上下文更新
type ContextUpdate map[string]any

// TaskFilter 任务过滤器
type TaskFilter struct {
	Status string
	Type   string
}

// Match 匹配任务
func (f TaskFilter) Match(state *TaskState) bool {
	if f.Status != "" && string(state.Status) != f.Status {
		return false
	}
	if f.Type != "" && state.Task.Type != f.Type {
		return false
	}
	return true
}

// TaskState 任务状态
type TaskState struct {
	Task      Task            `json:"task"`
	Status    TaskStatus      `json:"status"`
	Result    *Result         `json:"result,omitempty"`
	History   []StatusChange  `json:"history"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// StatusChange 状态变更记录
type StatusChange struct {
	From      TaskStatus `json:"from"`
	To        TaskStatus `json:"to"`
	Reason    string     `json:"reason"`
	Timestamp time.Time  `json:"timestamp"`
}

// CreateSessionRequest 创建会话请求
type CreateSessionRequest struct {
	Environment string         `json:"environment"`
	Config      map[string]any `json:"config,omitempty"`
}

// CreateSessionResponse 创建会话响应
type CreateSessionResponse struct {
	ID string `json:"id"`
}

// TaskRequest 任务请求
type TaskRequest struct {
	SessionID   string `json:"session_id"`
	Task        Task   `json:"task"`
	Environment string `json:"environment"`
}

// KnowledgeUpdate 知识更新
type KnowledgeUpdate struct {
	Title    string         `json:"title,omitempty"`
	Content  string         `json:"content,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PatternUpdate 模式更新
type PatternUpdate struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Trigger     string         `json:"trigger,omitempty"`
	Actions     []Action       `json:"actions,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// FeedbackConfig 反馈配置
type FeedbackConfig struct {
	MaxRetries      int           `json:"max_retries"`
	RetryDelay      time.Duration `json:"retry_delay"`
	AutoFix         bool          `json:"auto_fix"`
	NotifyOnFailure bool          `json:"notify_on_failure"`
}
