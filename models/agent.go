package models

import "time"

// ============================================================
// Agent 注册与能力
// ============================================================

// AgentStatus Agent 状态
type AgentStatus string

const (
	AgentStatusIdle      AgentStatus = "idle"
	AgentStatusBusy      AgentStatus = "busy"
	AgentStatusOffline   AgentStatus = "offline"
	AgentStatusError     AgentStatus = "error"
)

// AgentRole Agent 角色
type AgentRole string

const (
	AgentRoleWorker    AgentRole = "worker"    // 执行具体任务
	AgentRoleReviewer  AgentRole = "reviewer"  // 审查其他 Agent 的输出
	AgentRoleLead      AgentRole = "lead"      // 领导协作，分解任务
	AgentRoleObserver  AgentRole = "observer"  // 观察和学习
)

// Agent 注册的 Agent
type Agent struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Adapter      string            `json:"adapter"`       // 使用的适配器: claude-code / hermes / codex-cli
	Role         AgentRole         `json:"role"`
	Status       AgentStatus       `json:"status"`
	Capabilities []Capability      `json:"capabilities"`
	Constraints  []Constraint      `json:"constraints,omitempty"`  // Agent 级约束
	Metadata     map[string]any    `json:"metadata,omitempty"`
	RegisteredAt time.Time         `json:"registered_at"`
	LastActiveAt time.Time         `json:"last_active_at"`
	CurrentTask  string            `json:"current_task,omitempty"` // 当前正在执行的任务 ID
	MaxConcurrent int              `json:"max_concurrent"`         // 最大并发任务数
}

// Capability Agent 能力声明
type Capability struct {
	Name        string   `json:"name"`        // 能力名称: implement / review / test / deploy / refactor
	Confidence  float64  `json:"confidence"`  // 自评置信度 0.0-1.0
	Languages   []string `json:"languages,omitempty"`  // 支持的编程语言
	Domains     []string `json:"domains,omitempty"`    // 擅长领域: backend / frontend / devops / ml
	MaxTaskSize int      `json:"max_task_size"`         // 能处理的最大任务复杂度
}

// ============================================================
// 任务分解 DAG
// ============================================================

// SubTaskStatus 子任务状态
type SubTaskStatus string

const (
	SubTaskPending    SubTaskStatus = "pending"
	SubTaskReady      SubTaskStatus = "ready"      // 依赖已满足，可执行
	SubTaskAssigned   SubTaskStatus = "assigned"
	SubTaskInProgress SubTaskStatus = "in_progress"
	SubTaskCompleted  SubTaskStatus = "completed"
	SubTaskFailed     SubTaskStatus = "failed"
	SubTaskSkipped    SubTaskStatus = "skipped"     // 因上游失败而跳过
)

// SubTask 分解后的子任务
type SubTask struct {
	ID          string            `json:"id"`
	ParentID    string            `json:"parent_id"`    // 原始任务 ID
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"`         // implement / review / test / etc.
	Status      SubTaskStatus     `json:"status"`
	Priority    int               `json:"priority"`
	Constraints []Constraint      `json:"constraints,omitempty"`
	Context     map[string]any    `json:"context,omitempty"`
	DependsOn   []string          `json:"depends_on"`   // 依赖的子任务 ID 列表
	AssignedTo  string            `json:"assigned_to"`  // 分配给哪个 Agent
	Result      *Result           `json:"result,omitempty"`
	Retries     int               `json:"retries"`
	MaxRetries  int               `json:"max_retries"`
	Timeout     time.Duration     `json:"timeout"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

// TaskGraph 子任务 DAG
type TaskGraph struct {
	ID        string            `json:"id"`
	ParentID  string            `json:"parent_id"`     // 原始任务 ID
	Strategy  StrategyType      `json:"strategy"`      // 使用的协作策略
	SubTasks  map[string]*SubTask `json:"sub_tasks"`   // ID → SubTask
	RootIDs   []string          `json:"root_ids"`      // 无依赖的起始任务
	Status    string            `json:"status"`        // pending / running / completed / failed
	CreatedAt time.Time         `json:"created_at"`
}

// GetReadyTasks 获取所有依赖已满足的子任务
func (g *TaskGraph) GetReadyTasks() []*SubTask {
	var ready []*SubTask
	for _, task := range g.SubTasks {
		if task.Status != SubTaskPending {
			continue
		}
		allDepsMet := true
		for _, depID := range task.DependsOn {
			dep, exists := g.SubTasks[depID]
			if !exists || dep.Status != SubTaskCompleted {
				allDepsMet = false
				break
			}
		}
		if allDepsMet {
			ready = append(ready, task)
		}
	}
	return ready
}

// IsComplete 检查 DAG 是否全部完成
func (g *TaskGraph) IsComplete() bool {
	for _, task := range g.SubTasks {
		if task.Status != SubTaskCompleted && task.Status != SubTaskSkipped {
			return false
		}
	}
	return true
}

// HasFailed 检查 DAG 是否有失败且无法恢复的任务
func (g *TaskGraph) HasFailed() bool {
	for _, task := range g.SubTasks {
		if task.Status == SubTaskFailed && task.Retries >= task.MaxRetries {
			return true
		}
	}
	return false
}

// ============================================================
// 协作协议与策略
// ============================================================

// StrategyType 协作策略类型
type StrategyType string

const (
	StrategyPipeline   StrategyType = "pipeline"    // A → B → C 顺序执行
	StrategyFanOut     StrategyType = "fan_out"     // A → [B,C,D] → 聚合
	StrategyDiscussion StrategyType = "discussion"   // 多 Agent 讨论达成共识
	StrategyReview     StrategyType = "review"       // 实现 + 审查
	StrategyDebate     StrategyType = "debate"       // 正反方辩论
	StrategyAuto       StrategyType = "auto"         // 自动选择策略
)

// CollaborationProtocol 协作协议配置
type CollaborationProtocol struct {
	Strategy       StrategyType      `json:"strategy"`
	MaxRounds      int               `json:"max_rounds"`       // 最大轮次（discussion/debate）
	ConsensusThreshold float64       `json:"consensus_threshold"` // 共识阈值
	ReviewRequired bool              `json:"review_required"`   // 是否强制审查
	TimeoutPerTask time.Duration     `json:"timeout_per_task"`
	FailPolicy     FailPolicy        `json:"fail_policy"`      // 失败策略
}

// FailPolicy 失败处理策略
type FailPolicy string

const (
	FailPolicyAbort    FailPolicy = "abort"     // 一个失败全部中止
	FailPolicyRetry    FailPolicy = "retry"     // 重试失败的任务
	FailPolicySkip     FailPolicy = "skip"      // 跳过失败，继续执行
	FailPolicyFallback FailPolicy = "fallback"  // 使用备用 Agent 重试
)

// ============================================================
// Agent 间消息
// ============================================================

// MessageType 消息类型
type MessageType string

const (
	MessageTypeTaskAssign   MessageType = "task_assign"    // 分配任务
	MessageTypeTaskResult   MessageType = "task_result"    // 返回结果
	MessageTypeFeedback     MessageType = "feedback"       // 反馈意见
	MessageTypeProposal     MessageType = "proposal"       // 提案（discussion）
	MessageTypeVote         MessageType = "vote"           // 投票
	MessageTypeConsensus    MessageType = "consensus"      // 达成共识
	MessageTypeRequestHelp  MessageType = "request_help"   // 请求帮助
	MessageTypeProvideHelp  MessageType = "provide_help"   // 提供帮助
	MessageTypeStatusUpdate MessageType = "status_update"  // 状态更新
	MessageTypeError        MessageType = "error"          // 错误通知
)

// Message Agent 间消息
type Message struct {
	ID         string      `json:"id"`
	From       string      `json:"from"`         // 发送者 Agent ID
	To         string      `json:"to"`           // 接收者 Agent ID（"" = 广播）
	Type       MessageType `json:"type"`
	CollabID   string      `json:"collab_id"`    // 协作会话 ID
	SubTaskID  string      `json:"sub_task_id"`  // 关联的子任务
	Payload    any         `json:"payload"`      // 消息载荷
	Timestamp  time.Time   `json:"timestamp"`
	ReplyTo    string      `json:"reply_to,omitempty"`  // 回复哪条消息
}

// ============================================================
// 消息载荷类型
// ============================================================

// TaskAssignPayload 任务分配载荷
type TaskAssignPayload struct {
	SubTask     SubTask  `json:"sub_task"`
	Constraints []Constraint `json:"constraints,omitempty"`
}

// TaskResultPayload 任务结果载荷
type TaskResultPayload struct {
	SubTaskID string   `json:"sub_task_id"`
	Result    Result   `json:"result"`
	Success   bool     `json:"success"`
}

// FeedbackPayload 反馈载荷
type FeedbackPayload struct {
	SubTaskID  string     `json:"sub_task_id"`
	Score      float64    `json:"score"`       // 0.0-1.0
	Comments   string     `json:"comments"`
	Violations []Violation `json:"violations,omitempty"`
	Suggestions []string  `json:"suggestions,omitempty"`
}

// ProposalPayload 提案载荷（discussion 策略）
type ProposalPayload struct {
	Round      int    `json:"round"`       // 第几轮
	Content    string `json:"content"`     // 提案内容
	Reasoning  string `json:"reasoning"`   // 理由
	Confidence float64 `json:"confidence"` // 置信度
}

// VotePayload 投票载荷
type VotePayload struct {
	ProposalID string  `json:"proposal_id"`
	Accept     bool    `json:"accept"`
	Comments   string  `json:"comments,omitempty"`
	Score      float64 `json:"score"` // 0.0-1.0
}

// ConsensusPayload 共识载荷
type ConsensusPayload struct {
	Round       int    `json:"round"`
	Content     string `json:"content"`      // 最终共识内容
	VoteCount   int    `json:"vote_count"`
	TotalVotes  int    `json:"total_votes"`
	Agreed      bool   `json:"agreed"`       // 是否达成共识
}

// ============================================================
// 协作会话
// ============================================================

// CollaborationStatus 协作状态
type CollaborationStatus string

const (
	CollabStatusPending   CollaborationStatus = "pending"
	CollabStatusRunning   CollaborationStatus = "running"
	CollabStatusCompleted CollaborationStatus = "completed"
	CollabStatusFailed    CollaborationStatus = "failed"
	CollabStatusCancelled CollaborationStatus = "cancelled"
)

// Collaboration 协作会话
type Collaboration struct {
	ID         string                `json:"id"`
	TaskID     string                `json:"task_id"`       // 原始任务 ID
	Protocol   CollaborationProtocol `json:"protocol"`
	Graph      *TaskGraph            `json:"graph"`
	Agents     []string              `json:"agents"`        // 参与的 Agent ID 列表
	Status     CollaborationStatus   `json:"status"`
	Messages   []Message             `json:"messages"`      // 通信历史
	Result     *Result               `json:"result,omitempty"`
	Round      int                   `json:"round"`         // 当前轮次
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
	CompletedAt *time.Time           `json:"completed_at,omitempty"`
}

// ============================================================
// 请求/响应类型
// ============================================================

// StartCollaborationRequest 发起协作请求
type StartCollaborationRequest struct {
	TaskID   string                `json:"task_id"`
	Task     Task                  `json:"task"`
	Protocol CollaborationProtocol `json:"protocol"`
	AgentIDs []string              `json:"agent_ids,omitempty"` // 指定参与 Agent（空=自动选择）
}

// RegisterAgentRequest 注册 Agent 请求
type RegisterAgentRequest struct {
	Name         string       `json:"name"`
	Adapter      string       `json:"adapter"`
	Role         AgentRole    `json:"role"`
	Capabilities []Capability `json:"capabilities"`
	Constraints  []Constraint `json:"constraints,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	MaxConcurrent int         `json:"max_concurrent"`
}

// CollaborationResponse 协作响应
type CollaborationResponse struct {
	CollaborationID string              `json:"collaboration_id"`
	Status          CollaborationStatus `json:"status"`
	Message         string              `json:"message"`
}
