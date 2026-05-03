package collaboration

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/harness-engineering/harness/models"
)

// ExecutorFunc 任务执行函数（由 Engine 提供）
type ExecutorFunc func(ctx context.Context, adapterName string, task models.Task) (*models.Result, error)

// Orchestrator 协作编排器
type Orchestrator struct {
	mu          sync.RWMutex
	registry    *Registry
	bus         *MessageBus
	decomposer  *Decomposer
	executor    ExecutorFunc
	collabs     map[string]*models.Collaboration // collabID -> Collaboration
}

// NewOrchestrator 创建编排器
func NewOrchestrator(registry *Registry, bus *MessageBus, executor ExecutorFunc) *Orchestrator {
	return &Orchestrator{
		registry:   registry,
		bus:        bus,
		decomposer: NewDecomposer(),
		executor:   executor,
		collabs:    make(map[string]*models.Collaboration),
	}
}

// StartCollaboration 发起协作
func (o *Orchestrator) StartCollaboration(ctx context.Context, req models.StartCollaborationRequest) (*models.Collaboration, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 分解任务
	graph, err := o.decomposer.Decompose(req.Task, req.Protocol)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: task decomposition failed: %w", err)
	}

	// 选择 Agent
	var agentIDs []string
	if len(req.AgentIDs) > 0 {
		agentIDs = req.AgentIDs
	} else {
		agents := o.registry.AutoSelectAgents(req.Task, 3)
		for _, a := range agents {
			agentIDs = append(agentIDs, a.ID)
		}
	}

	if len(agentIDs) == 0 {
		return nil, fmt.Errorf("orchestrator: no available agents for collaboration")
	}

	// 创建协作会话
	collab := &models.Collaboration{
		ID:        fmt.Sprintf("collab-%s-%d", req.TaskID, time.Now().UnixNano()),
		TaskID:    req.TaskID,
		Protocol:  req.Protocol,
		Graph:     graph,
		Agents:    agentIDs,
		Status:    models.CollabStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	o.collabs[collab.ID] = collab

	slog.Info("collaboration started",
		"collab_id", collab.ID,
		"strategy", req.Protocol.Strategy,
		"agents", agentIDs,
		"subtasks", len(graph.SubTasks),
	)

	return collab, nil
}

// Run 执行协作（主循环）
func (o *Orchestrator) Run(ctx context.Context, collabID string) error {
	o.mu.Lock()
	collab, exists := o.collabs[collabID]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("orchestrator: collaboration not found: %s", collabID)
	}
	collab.Status = models.CollabStatusRunning
	collab.UpdatedAt = time.Now()
	o.mu.Unlock()

	slog.Info("collaboration running", "collab_id", collabID, "strategy", collab.Protocol.Strategy)

	// 根据策略执行
	switch collab.Protocol.Strategy {
	case models.StrategyPipeline, models.StrategyReview:
		return o.runSequential(ctx, collab)
	case models.StrategyFanOut:
		return o.runParallel(ctx, collab)
	case models.StrategyDiscussion, models.StrategyDebate:
		return o.runDiscussion(ctx, collab)
	default:
		return o.runSequential(ctx, collab)
	}
}

// runSequential 顺序执行（Pipeline / Review）
func (o *Orchestrator) runSequential(ctx context.Context, collab *models.Collaboration) error {
	graph := collab.Graph

	for {
		// 检查是否完成
		if graph.IsComplete() {
			return o.completeCollaboration(collab)
		}
		if graph.HasFailed() {
			return o.failCollaboration(collab, "subtask failed")
		}

		// 获取可执行的子任务
		ready := graph.GetReadyTasks()
		if len(ready) == 0 {
			// 没有可执行的任务，也没有完成 → 等待上下文取消或超时
			select {
			case <-ctx.Done():
				return o.failCollaboration(collab, "context cancelled")
			case <-time.After(50 * time.Millisecond):
				continue
			}
		}

		// 顺序执行（取第一个）
		subTask := ready[0]
		if err := o.executeSubTask(ctx, collab, subTask); err != nil {
			slog.Error("subtask failed", "subtask_id", subTask.ID, "error", err)

			// 检查失败策略
			if collab.Protocol.FailPolicy == models.FailPolicyAbort {
				subTask.Status = models.SubTaskFailed
				return o.failCollaboration(collab, err.Error())
			}

			// 重试
			if subTask.Retries < subTask.MaxRetries {
				subTask.Retries++
				subTask.Status = models.SubTaskPending
				continue
			}

			if collab.Protocol.FailPolicy == models.FailPolicySkip {
				subTask.Status = models.SubTaskSkipped
				o.skipDownstream(graph, subTask.ID)
			} else {
				subTask.Status = models.SubTaskFailed
			}
		}
	}
}

// runParallel 并行执行（Fan-out）— 带并发安全保护
func (o *Orchestrator) runParallel(ctx context.Context, collab *models.Collaboration) error {
	graph := collab.Graph
	var graphMu sync.Mutex // 保护 DAG 状态的并发访问

	for {
		graphMu.Lock()
		if graph.IsComplete() {
			graphMu.Unlock()
			return o.completeCollaboration(collab)
		}
		if graph.HasFailed() {
			graphMu.Unlock()
			return o.failCollaboration(collab, "subtask failed")
		}

		ready := graph.GetReadyTasks()
		graphMu.Unlock()

		if len(ready) == 0 {
			select {
			case <-ctx.Done():
				return o.failCollaboration(collab, "context cancelled")
			case <-time.After(50 * time.Millisecond):
				continue
			}
		}

		// 并行执行所有就绪任务
		var wg sync.WaitGroup
		errCh := make(chan error, len(ready))

		for _, subTask := range ready {
			wg.Add(1)
			go func(st *models.SubTask) {
				defer wg.Done()
				if err := o.executeSubTask(ctx, collab, st); err != nil {
					errCh <- fmt.Errorf("%s: %w", st.ID, err)
				}
			}(subTask)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			slog.Error("parallel subtask failed", "error", err)
			if collab.Protocol.FailPolicy == models.FailPolicyAbort {
				return o.failCollaboration(collab, err.Error())
			}
		}
	}
}

// runDiscussion 讨论执行（Discussion / Debate）
func (o *Orchestrator) runDiscussion(ctx context.Context, collab *models.Collaboration) error {
	graph := collab.Graph
	maxRounds := collab.Protocol.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 3
	}

	for round := 1; round <= maxRounds; round++ {
		collab.Round = round
		slog.Info("discussion round", "collab_id", collab.ID, "round", round, "max", maxRounds)

		// 获取当前轮次的子任务
		ready := graph.GetReadyTasks()
		if len(ready) == 0 {
			if graph.IsComplete() {
				return o.completeCollaboration(collab)
			}
			select {
			case <-ctx.Done():
				return o.failCollaboration(collab, "context cancelled")
			case <-time.After(50 * time.Millisecond):
				continue
			}
		}

		// 并行执行当前轮次
		var wg sync.WaitGroup
		for _, subTask := range ready {
			wg.Add(1)
			go func(st *models.SubTask) {
				defer wg.Done()
				o.executeSubTask(ctx, collab, st)
			}(subTask)
		}
		wg.Wait()

		// 检查共识
		if o.checkConsensus(collab) {
			slog.Info("consensus reached", "collab_id", collab.ID, "round", round)
			return o.completeCollaboration(collab)
		}
	}

	// 超过最大轮次，强制完成
	slog.Warn("max rounds reached, forcing consensus", "collab_id", collab.ID)
	return o.completeCollaboration(collab)
}

// executeSubTask 执行单个子任务（带超时保护）
func (o *Orchestrator) executeSubTask(ctx context.Context, collab *models.Collaboration, subTask *models.SubTask) error {
	// 应用子任务超时
	taskCtx := ctx
	if subTask.Timeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(ctx, subTask.Timeout)
		defer cancel()
	}

	// 选择 Agent
	agent := o.selectAgentForSubTask(collab, subTask)
	if agent == nil {
		return fmt.Errorf("orchestrator: no suitable agent for subtask %s (type: %s)", subTask.ID, subTask.Type)
	}

	// 更新状态
	subTask.Status = models.SubTaskInProgress
	subTask.AssignedTo = agent.ID
	now := time.Now()
	subTask.StartedAt = &now

	// 注册 Agent 为忙碌
	o.registry.AssignTask(agent.ID, subTask.ID)
	defer o.registry.ReleaseTask(agent.ID)

	// 构建任务
	task := models.Task{
		ID:          subTask.ID,
		Type:        subTask.Type,
		Description: subTask.Description,
		Constraints: subTask.Constraints,
		Context:     subTask.Context,
		Priority:    subTask.Priority,
	}

	// 如果子任务有上游结果，注入到上下文
	if upstreamResults := o.collectUpstreamResults(collab.Graph, subTask); len(upstreamResults) > 0 {
		if task.Context == nil {
			task.Context = make(map[string]any)
		}
		task.Context["upstream_results"] = upstreamResults
	}

	// 发送任务分配消息
	o.bus.Publish(models.Message{
		From:      "orchestrator",
		To:        agent.ID,
		Type:      models.MessageTypeTaskAssign,
		CollabID:  collab.ID,
		SubTaskID: subTask.ID,
		Payload: models.TaskAssignPayload{
			SubTask:     *subTask,
			Constraints: subTask.Constraints,
		},
	})

	// 执行任务（带超时上下文）
	result, err := o.executor(taskCtx, agent.Adapter, task)
	if err != nil {
		subTask.Status = models.SubTaskFailed
		now := time.Now()
		subTask.CompletedAt = &now

		o.bus.Publish(models.Message{
			From:      agent.ID,
			To:        "orchestrator",
			Type:      models.MessageTypeError,
			CollabID:  collab.ID,
			SubTaskID: subTask.ID,
			Payload:   err.Error(),
		})

		return err
	}

	// 成功
	subTask.Status = models.SubTaskCompleted
	subTask.Result = result
	now = time.Now()
	subTask.CompletedAt = &now

	o.bus.Publish(models.Message{
		From:      agent.ID,
		To:        "orchestrator",
		Type:      models.MessageTypeTaskResult,
		CollabID:  collab.ID,
		SubTaskID: subTask.ID,
		Payload: models.TaskResultPayload{
			SubTaskID: subTask.ID,
			Result:    *result,
			Success:   true,
		},
	})

	slog.Info("subtask completed",
		"subtask_id", subTask.ID,
		"agent", agent.ID,
		"duration", time.Since(now),
	)

	return nil
}

// selectAgentForSubTask 为子任务选择最佳 Agent
func (o *Orchestrator) selectAgentForSubTask(collab *models.Collaboration, subTask *models.SubTask) *models.Agent {
	// 如果已指定 Agent
	if subTask.AssignedTo != "" {
		agent, err := o.registry.Get(subTask.AssignedTo)
		if err == nil && agent.Status == models.AgentStatusIdle {
			return agent
		}
	}

	// 根据能力自动选择
	agents := o.registry.FindByCapability(subTask.Type, 1)
	if len(agents) > 0 {
		return agents[0]
	}

	// 从协作参与者中选择空闲的
	for _, agentID := range collab.Agents {
		agent, err := o.registry.Get(agentID)
		if err == nil && agent.Status == models.AgentStatusIdle {
			return agent
		}
	}

	return nil
}

// collectUpstreamResults 收集上游子任务的结果
func (o *Orchestrator) collectUpstreamResults(graph *models.TaskGraph, subTask *models.SubTask) map[string]any {
	results := make(map[string]any)
	for _, depID := range subTask.DependsOn {
		if dep, exists := graph.SubTasks[depID]; exists && dep.Result != nil {
			results[depID] = dep.Result.Output
		}
	}
	return results
}

// skipDownstream 跳过下游子任务
func (o *Orchestrator) skipDownstream(graph *models.TaskGraph, failedID string) {
	for _, task := range graph.SubTasks {
		for _, depID := range task.DependsOn {
			if depID == failedID && task.Status == models.SubTaskPending {
				task.Status = models.SubTaskSkipped
				o.skipDownstream(graph, task.ID)
			}
		}
	}
}

// checkConsensus 检查是否达成共识
func (o *Orchestrator) checkConsensus(collab *models.Collaboration) bool {
	// 简单实现：如果所有子任务都完成了，认为达成共识
	return collab.Graph.IsComplete()
}

// completeCollaboration 完成协作
func (o *Orchestrator) completeCollaboration(collab *models.Collaboration) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 聚合最终结果
	finalResult := o.aggregateResults(collab)

	collab.Status = models.CollabStatusCompleted
	collab.Result = finalResult
	now := time.Now()
	collab.CompletedAt = &now
	collab.UpdatedAt = now

	slog.Info("collaboration completed", "collab_id", collab.ID, "subtasks", len(collab.Graph.SubTasks))
	return nil
}

// failCollaboration 标记协作失败
func (o *Orchestrator) failCollaboration(collab *models.Collaboration, reason string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	collab.Status = models.CollabStatusFailed
	collab.UpdatedAt = time.Now()

	slog.Error("collaboration failed", "collab_id", collab.ID, "reason", reason)
	return fmt.Errorf("collaboration failed: %s", reason)
}

// aggregateResults 聚合子任务结果
func (o *Orchestrator) aggregateResults(collab *models.Collaboration) *models.Result {
	var outputs []any
	var allEvidence []models.Evidence
	totalDuration := time.Duration(0)
	totalTokens := 0
	allSuccess := true

	for _, subTask := range collab.Graph.SubTasks {
		if subTask.Result != nil {
			outputs = append(outputs, subTask.Result.Output)
			allEvidence = append(allEvidence, subTask.Result.Evidence...)
			totalDuration += subTask.Result.Metrics.Duration
			totalTokens += subTask.Result.Metrics.TokenCount
			if subTask.Status != models.SubTaskCompleted {
				allSuccess = false
			}
		}
	}

	status := models.TaskStatusCompleted
	if !allSuccess {
		status = models.TaskStatusFailed
	}

	return &models.Result{
		TaskID:   collab.TaskID,
		Status:   status,
		Output:   outputs,
		Evidence: allEvidence,
		Metrics: models.Metrics{
			Duration:   totalDuration,
			TokenCount: totalTokens,
		},
	}
}

// GetCollaboration 获取协作会话
func (o *Orchestrator) GetCollaboration(collabID string) (*models.Collaboration, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	collab, exists := o.collabs[collabID]
	if !exists {
		return nil, fmt.Errorf("collaboration not found: %s", collabID)
	}
	return collab, nil
}

// ListCollaborations 列出所有协作会话
func (o *Orchestrator) ListCollaborations() []*models.Collaboration {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var result []*models.Collaboration
	for _, c := range o.collabs {
		result = append(result, c)
	}
	return result
}
