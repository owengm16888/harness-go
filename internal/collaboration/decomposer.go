package collaboration

import (
	"fmt"
	"time"

	"github.com/harness-engineering/harness/models"
)

// Decomposer 任务分解器 — 将复杂任务分解为子任务 DAG
type Decomposer struct {
	strategies map[models.StrategyType]DecomposeFunc
}

// DecomposeFunc 分解函数
type DecomposeFunc func(task models.Task, protocol models.CollaborationProtocol) (*models.TaskGraph, error)

// NewDecomposer 创建分解器
func NewDecomposer() *Decomposer {
	d := &Decomposer{
		strategies: make(map[models.StrategyType]DecomposeFunc),
	}

	// 注册内置策略分解器
	d.strategies[models.StrategyPipeline] = d.decomposePipeline
	d.strategies[models.StrategyFanOut] = d.decomposeFanOut
	d.strategies[models.StrategyReview] = d.decomposeReview
	d.strategies[models.StrategyDiscussion] = d.decomposeDiscussion
	d.strategies[models.StrategyDebate] = d.decomposeDebate
	d.strategies[models.StrategyAuto] = d.decomposeAuto

	return d
}

// Decompose 分解任务
func (d *Decomposer) Decompose(task models.Task, protocol models.CollaborationProtocol) (*models.TaskGraph, error) {
	fn, exists := d.strategies[protocol.Strategy]
	if !exists {
		return nil, fmt.Errorf("unsupported strategy: %s", protocol.Strategy)
	}

	graph, err := fn(task, protocol)
	if err != nil {
		return nil, err
	}

	graph.ParentID = task.ID
	graph.Strategy = protocol.Strategy
	graph.Status = "pending"
	graph.CreatedAt = time.Now()

	return graph, nil
}

// decomposePipeline 流水线分解: A → B → C
//
// 典型流程: implement → review → test
func (d *Decomposer) decomposePipeline(task models.Task, protocol models.CollaborationProtocol) (*models.TaskGraph, error) {
	graph := &models.TaskGraph{
		ID:       fmt.Sprintf("graph-%s-pipeline", task.ID),
		SubTasks: make(map[string]*models.SubTask),
	}

	// 根据任务类型决定流水线阶段
	stages := d.getPipelineStages(task.Type)

	var prevID string
	for i, stage := range stages {
		subTaskID := fmt.Sprintf("%s-stage-%d-%s", task.ID, i, stage)
		subTask := &models.SubTask{
			ID:          subTaskID,
			ParentID:    task.ID,
			Name:        fmt.Sprintf("Stage %d: %s", i+1, stage),
			Description: fmt.Sprintf("%s: %s", stage, task.Description),
			Type:        stage,
			Status:      models.SubTaskPending,
			Priority:    task.Priority,
			Constraints: task.Constraints,
			Context:     copyMap(task.Context),
			MaxRetries:  2,
			Timeout:     protocol.TimeoutPerTask,
			CreatedAt:   time.Now(),
		}

		if prevID != "" {
			subTask.DependsOn = []string{prevID}
		} else {
			graph.RootIDs = append(graph.RootIDs, subTaskID)
		}

		graph.SubTasks[subTaskID] = subTask
		prevID = subTaskID
	}

	return graph, nil
}

// decomposeFanOut 扇出分解: A → [B, C, D] → aggregate
//
// 将任务拆分为多个并行子任务，最后聚合结果
func (d *Decomposer) decomposeFanOut(task models.Task, protocol models.CollaborationProtocol) (*models.TaskGraph, error) {
	graph := &models.TaskGraph{
		ID:       fmt.Sprintf("graph-%s-fanout", task.ID),
		SubTasks: make(map[string]*models.SubTask),
	}

	// 从上下文获取拆分维度
	dimensions := d.getFanOutDimensions(task)
	if len(dimensions) == 0 {
		// 默认拆分为 3 个并行子任务
		dimensions = []string{"part-1", "part-2", "part-3"}
	}

	// Phase 1: 并行执行
	var parallelIDs []string
	for i, dim := range dimensions {
		subTaskID := fmt.Sprintf("%s-parallel-%d-%s", task.ID, i, dim)
		subTask := &models.SubTask{
			ID:          subTaskID,
			ParentID:    task.ID,
			Name:        fmt.Sprintf("Parallel: %s", dim),
			Description: fmt.Sprintf("[%s] %s", dim, task.Description),
			Type:        task.Type,
			Status:      models.SubTaskPending,
			Priority:    task.Priority,
			Constraints: task.Constraints,
			Context:     mergeMap(copyMap(task.Context), map[string]any{"dimension": dim}),
			MaxRetries:  2,
			Timeout:     protocol.TimeoutPerTask,
			CreatedAt:   time.Now(),
		}
		graph.SubTasks[subTaskID] = subTask
		graph.RootIDs = append(graph.RootIDs, subTaskID)
		parallelIDs = append(parallelIDs, subTaskID)
	}

	// Phase 2: 聚合
	aggregateID := fmt.Sprintf("%s-aggregate", task.ID)
	aggregate := &models.SubTask{
		ID:          aggregateID,
		ParentID:    task.ID,
		Name:        "Aggregate results",
		Description: fmt.Sprintf("Aggregate results from %d parallel tasks: %s", len(dimensions), task.Description),
		Type:        "aggregate",
		Status:      models.SubTaskPending,
		Priority:    task.Priority,
		DependsOn:   parallelIDs,
		Constraints: task.Constraints,
		Context:     copyMap(task.Context),
		MaxRetries:  1,
		Timeout:     protocol.TimeoutPerTask,
		CreatedAt:   time.Now(),
	}
	graph.SubTasks[aggregateID] = aggregate

	return graph, nil
}

// decomposeReview 实现+审查分解: implement → review
func (d *Decomposer) decomposeReview(task models.Task, protocol models.CollaborationProtocol) (*models.TaskGraph, error) {
	graph := &models.TaskGraph{
		ID:       fmt.Sprintf("graph-%s-review", task.ID),
		SubTasks: make(map[string]*models.SubTask),
	}

	// 阶段 1: 实现
	implementID := fmt.Sprintf("%s-implement", task.ID)
	graph.RootIDs = []string{implementID}
	graph.SubTasks[implementID] = &models.SubTask{
		ID:          implementID,
		ParentID:    task.ID,
		Name:        "Implement",
		Description: task.Description,
		Type:        "implement",
		Status:      models.SubTaskPending,
		Priority:    task.Priority,
		Constraints: task.Constraints,
		Context:     copyMap(task.Context),
		MaxRetries:  2,
		Timeout:     protocol.TimeoutPerTask,
		CreatedAt:   time.Now(),
	}

	// 阶段 2: 审查
	reviewID := fmt.Sprintf("%s-review", task.ID)
	graph.SubTasks[reviewID] = &models.SubTask{
		ID:          reviewID,
		ParentID:    task.ID,
		Name:        "Code Review",
		Description: fmt.Sprintf("Review implementation: %s", task.Description),
		Type:        "review",
		Status:      models.SubTaskPending,
		Priority:    task.Priority,
		DependsOn:   []string{implementID},
		Constraints: task.Constraints,
		Context:     copyMap(task.Context),
		MaxRetries:  1,
		Timeout:     protocol.TimeoutPerTask,
		CreatedAt:   time.Now(),
	}

	return graph, nil
}

// decomposeDiscussion 讨论分解: 多 Agent 讨论达成共识
func (d *Decomposer) decomposeDiscussion(task models.Task, protocol models.CollaborationProtocol) (*models.TaskGraph, error) {
	graph := &models.TaskGraph{
		ID:       fmt.Sprintf("graph-%s-discussion", task.ID),
		SubTasks: make(map[string]*models.SubTask),
	}

	// 阶段 1: 各 Agent 独立提案
	rounds := protocol.MaxRounds
	if rounds <= 0 {
		rounds = 3
	}

	// 第一轮: 独立提案
	proposalID := fmt.Sprintf("%s-round-1-proposals", task.ID)
	graph.RootIDs = []string{proposalID}
	graph.SubTasks[proposalID] = &models.SubTask{
		ID:          proposalID,
		ParentID:    task.ID,
		Name:        "Round 1: Independent Proposals",
		Description: fmt.Sprintf("Each agent independently proposes a solution: %s", task.Description),
		Type:        "proposal",
		Status:      models.SubTaskPending,
		Priority:    task.Priority,
		Context:     mergeMap(copyMap(task.Context), map[string]any{"round": 1, "max_rounds": rounds}),
		MaxRetries:  1,
		Timeout:     protocol.TimeoutPerTask,
		CreatedAt:   time.Now(),
	}

	// 后续轮次: 讨论
	prevID := proposalID
	for round := 2; round <= rounds; round++ {
		discussID := fmt.Sprintf("%s-round-%d-discuss", task.ID, round)
		graph.SubTasks[discussID] = &models.SubTask{
			ID:          discussID,
			ParentID:    task.ID,
			Name:        fmt.Sprintf("Round %d: Discussion", round),
			Description: fmt.Sprintf("Discuss and refine proposals (round %d/%d)", round, rounds),
			Type:        "discussion",
			Status:      models.SubTaskPending,
			Priority:    task.Priority,
			DependsOn:   []string{prevID},
			Context:     mergeMap(copyMap(task.Context), map[string]any{"round": round, "max_rounds": rounds}),
			MaxRetries:  1,
			Timeout:     protocol.TimeoutPerTask,
			CreatedAt:   time.Now(),
		}
		prevID = discussID
	}

	// 最终共识
	consensusID := fmt.Sprintf("%s-consensus", task.ID)
	graph.SubTasks[consensusID] = &models.SubTask{
		ID:          consensusID,
		ParentID:    task.ID,
		Name:        "Final Consensus",
		Description: "Synthesize final answer from discussion",
		Type:        "consensus",
		Status:      models.SubTaskPending,
		Priority:    task.Priority,
		DependsOn:   []string{prevID},
		Context:     copyMap(task.Context),
		MaxRetries:  1,
		Timeout:     protocol.TimeoutPerTask,
		CreatedAt:   time.Now(),
	}

	return graph, nil
}

// decomposeDebate 辩论分解: 正方 vs 反方
func (d *Decomposer) decomposeDebate(task models.Task, protocol models.CollaborationProtocol) (*models.TaskGraph, error) {
	graph := &models.TaskGraph{
		ID:       fmt.Sprintf("graph-%s-debate", task.ID),
		SubTasks: make(map[string]*models.SubTask),
	}

	// 正方论证
	proID := fmt.Sprintf("%s-pro", task.ID)
	graph.RootIDs = []string{proID}
	graph.SubTasks[proID] = &models.SubTask{
		ID:          proID,
		ParentID:    task.ID,
		Name:        "Pro Argument",
		Description: fmt.Sprintf("Argue FOR: %s", task.Description),
		Type:        "debate-pro",
		Status:      models.SubTaskPending,
		Priority:    task.Priority,
		Context:     mergeMap(copyMap(task.Context), map[string]any{"side": "pro"}),
		MaxRetries:  1,
		Timeout:     protocol.TimeoutPerTask,
		CreatedAt:   time.Now(),
	}

	// 反方论证
	conID := fmt.Sprintf("%s-con", task.ID)
	graph.SubTasks[conID] = &models.SubTask{
		ID:          conID,
		ParentID:    task.ID,
		Name:        "Con Argument",
		Description: fmt.Sprintf("Argue AGAINST: %s", task.Description),
		Type:        "debate-con",
		Status:      models.SubTaskPending,
		Priority:    task.Priority,
		Context:     mergeMap(copyMap(task.Context), map[string]any{"side": "con"}),
		MaxRetries:  1,
		Timeout:     protocol.TimeoutPerTask,
		CreatedAt:   time.Now(),
	}

	// 裁决
	judgeID := fmt.Sprintf("%s-judge", task.ID)
	graph.SubTasks[judgeID] = &models.SubTask{
		ID:          judgeID,
		ParentID:    task.ID,
		Name:        "Judge Decision",
		Description: "Evaluate both sides and make final decision",
		Type:        "judge",
		Status:      models.SubTaskPending,
		Priority:    task.Priority,
		DependsOn:   []string{proID, conID},
		Context:     copyMap(task.Context),
		MaxRetries:  1,
		Timeout:     protocol.TimeoutPerTask,
		CreatedAt:   time.Now(),
	}

	return graph, nil
}

// decomposeAuto 自动选择最佳策略
func (d *Decomposer) decomposeAuto(task models.Task, protocol models.CollaborationProtocol) (*models.TaskGraph, error) {
	// 根据任务特征自动选择策略
	strategy := d.selectBestStrategy(task)
	protocol.Strategy = strategy
	return d.Decompose(task, protocol)
}

// selectBestStrategy 根据任务特征选择最佳策略
func (d *Decomposer) selectBestStrategy(task models.Task) models.StrategyType {
	// 规则 1: 如果有 "review" 相关上下文，用 review 策略
	if _, ok := task.Context["require_review"]; ok {
		return models.StrategyReview
	}

	// 规则 2: 如果任务描述包含 "compare" 或 "evaluate"，用 debate
	desc := task.Description
	if containsWord(desc, "compare") || containsWord(desc, "evaluate") || containsWord(desc, "choose") {
		return models.StrategyDebate
	}

	// 规则 3: 如果任务描述包含 "design" 或 "architect"，用 discussion
	if containsWord(desc, "design") || containsWord(desc, "architect") || containsWord(desc, "plan") {
		return models.StrategyDiscussion
	}

	// 规则 4: 如果有多个维度，用 fan-out
	if dimensions, ok := task.Context["dimensions"]; ok {
		if dims, ok := dimensions.([]string); ok && len(dims) > 1 {
			return models.StrategyFanOut
		}
	}

	// 默认: pipeline
	return models.StrategyPipeline
}

// getPipelineStages 获取流水线阶段
func (d *Decomposer) getPipelineStages(taskType string) []string {
	switch taskType {
	case "implement":
		return []string{"implement", "review", "test"}
	case "refactor":
		return []string{"refactor", "review", "test"}
	case "review":
		return []string{"review"}
	case "test":
		return []string{"test"}
	default:
		return []string{"implement", "review", "test"}
	}
}

// getFanOutDimensions 获取扇出维度
func (d *Decomposer) getFanOutDimensions(task models.Task) []string {
	if dims, ok := task.Context["dimensions"]; ok {
		if d, ok := dims.([]string); ok {
			return d
		}
	}
	return nil
}

// --- 辅助函数 ---

func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func mergeMap(dst, src map[string]any) map[string]any {
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func containsWord(s, word string) bool {
	// 简单的子串匹配
	for i := 0; i <= len(s)-len(word); i++ {
		if s[i:i+len(word)] == word {
			return true
		}
	}
	return false
}
