package collaboration

import (
	"context"
	"testing"
	"time"

	"github.com/harness-engineering/harness/models"
)

// ============================================================
// Registry 测试
// ============================================================

func TestRegistry_Register(t *testing.T) {
	reg := NewRegistry()

	agent := &models.Agent{
		ID:      "agent-1",
		Name:    "Claude Worker",
		Adapter: "claude-code",
		Role:    models.AgentRoleWorker,
		Capabilities: []models.Capability{
			{Name: "implement", Confidence: 0.9, Languages: []string{"go", "python"}},
			{Name: "review", Confidence: 0.8},
		},
	}

	err := reg.Register(agent)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := reg.Get("agent-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "Claude Worker" {
		t.Errorf("Expected name 'Claude Worker', got '%s'", got.Name)
	}
	if got.Status != models.AgentStatusIdle {
		t.Errorf("Expected idle status, got %s", got.Status)
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	reg := NewRegistry()

	agent := &models.Agent{ID: "dup", Name: "A", Adapter: "test"}
	reg.Register(agent)

	err := reg.Register(&models.Agent{ID: "dup", Name: "B", Adapter: "test"})
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}
}

func TestRegistry_FindByCapability(t *testing.T) {
	reg := NewRegistry()

	reg.Register(&models.Agent{
		ID: "w1", Name: "Worker 1", Adapter: "claude-code",
		Capabilities: []models.Capability{
			{Name: "implement", Confidence: 0.9},
		},
	})
	reg.Register(&models.Agent{
		ID: "w2", Name: "Worker 2", Adapter: "hermes",
		Capabilities: []models.Capability{
			{Name: "implement", Confidence: 0.7},
		},
	})
	reg.Register(&models.Agent{
		ID: "r1", Name: "Reviewer 1", Adapter: "codex-cli",
		Capabilities: []models.Capability{
			{Name: "review", Confidence: 0.85},
		},
	})

	// 查找 implement 能力
	implementers := reg.FindByCapability("implement", 10)
	if len(implementers) != 2 {
		t.Errorf("Expected 2 implementers, got %d", len(implementers))
	}
	// 应该按置信度排序
	if implementers[0].ID != "w1" {
		t.Errorf("Expected w1 first (highest confidence), got %s", implementers[0].ID)
	}

	// 查找 review 能力
	reviewers := reg.FindByCapability("review", 1)
	if len(reviewers) != 1 {
		t.Errorf("Expected 1 reviewer, got %d", len(reviewers))
	}
}

func TestRegistry_AssignAndRelease(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&models.Agent{ID: "a1", Name: "A", Adapter: "test"})

	// 分配任务
	err := reg.AssignTask("a1", "task-1")
	if err != nil {
		t.Fatalf("AssignTask failed: %v", err)
	}

	agent, _ := reg.Get("a1")
	if agent.Status != models.AgentStatusBusy {
		t.Errorf("Expected busy, got %s", agent.Status)
	}
	if agent.CurrentTask != "task-1" {
		t.Errorf("Expected task-1, got %s", agent.CurrentTask)
	}

	// 忙碌时不能再分配
	err = reg.AssignTask("a1", "task-2")
	if err == nil {
		t.Error("Expected error when assigning to busy agent")
	}

	// 释放
	reg.ReleaseTask("a1")
	agent, _ = reg.Get("a1")
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Expected idle after release, got %s", agent.Status)
	}
}

func TestRegistry_AutoSelect(t *testing.T) {
	reg := NewRegistry()

	reg.Register(&models.Agent{
		ID: "impl", Name: "Implementer", Adapter: "claude-code",
		Capabilities: []models.Capability{
			{Name: "implement", Confidence: 0.9, Domains: []string{"backend"}},
		},
	})
	reg.Register(&models.Agent{
		ID: "rev", Name: "Reviewer", Adapter: "hermes",
		Capabilities: []models.Capability{
			{Name: "review", Confidence: 0.85},
		},
	})

	task := models.Task{
		ID: "t1", Type: "implement", Description: "Build API",
		Context: map[string]any{"domain": "backend"},
	}

	selected := reg.AutoSelectAgents(task, 1)
	if len(selected) != 1 {
		t.Fatalf("Expected 1 agent, got %d", len(selected))
	}
	if selected[0].ID != "impl" {
		t.Errorf("Expected 'impl', got '%s'", selected[0].ID)
	}
}

// ============================================================
// MessageBus 测试
// ============================================================

func TestMessageBus_Publish(t *testing.T) {
	bus := NewMessageBus(100)

	ch := bus.Subscribe("agent-1")

	msg := models.Message{
		From: "orchestrator",
		To:   "agent-1",
		Type: models.MessageTypeTaskAssign,
		Payload: models.TaskAssignPayload{
			SubTask: models.SubTask{ID: "st-1"},
		},
	}

	err := bus.Publish(msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case received := <-ch:
		if received.Type != models.MessageTypeTaskAssign {
			t.Errorf("Expected task_assign, got %s", received.Type)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestMessageBus_Broadcast(t *testing.T) {
	bus := NewMessageBus(100)

	ch1 := bus.SubscribeBroadcast()
	ch2 := bus.SubscribeBroadcast()

	bus.Publish(models.Message{
		From: "system",
		Type: models.MessageTypeStatusUpdate,
	})

	// 两个订阅者都应该收到
	for _, ch := range []chan models.Message{ch1, ch2} {
		select {
		case <-ch:
			// OK
		case <-time.After(time.Second):
			t.Error("Timeout waiting for broadcast")
		}
	}
}

func TestMessageBus_History(t *testing.T) {
	bus := NewMessageBus(100)

	bus.Subscribe("a1")
	bus.Subscribe("a2")

	bus.Publish(models.Message{From: "a1", To: "a2", Type: models.MessageTypeTaskAssign, CollabID: "c1"})
	bus.Publish(models.Message{From: "a2", To: "a1", Type: models.MessageTypeTaskResult, CollabID: "c1"})
	bus.Publish(models.Message{From: "a1", To: "a2", Type: models.MessageTypeFeedback, CollabID: "c2"})

	// 按 collab 过滤
	c1History := bus.GetHistory("c1")
	if len(c1History) != 2 {
		t.Errorf("Expected 2 messages for c1, got %d", len(c1History))
	}

	// 全部历史
	all := bus.GetHistory("")
	if len(all) != 3 {
		t.Errorf("Expected 3 total messages, got %d", len(all))
	}
}

// ============================================================
// Decomposer 测试
// ============================================================

func TestDecomposer_Pipeline(t *testing.T) {
	d := NewDecomposer()

	task := models.Task{
		ID: "t1", Type: "implement", Description: "Build auth",
	}
	protocol := models.CollaborationProtocol{
		Strategy:       models.StrategyPipeline,
		TimeoutPerTask: time.Minute,
	}

	graph, err := d.Decompose(task, protocol)
	if err != nil {
		t.Fatalf("Decompose failed: %v", err)
	}

	// implement → review → test = 3 stages
	if len(graph.SubTasks) != 3 {
		t.Errorf("Expected 3 subtasks, got %d", len(graph.SubTasks))
	}

	// 检查依赖链
	var implement, review, test *models.SubTask
	for _, st := range graph.SubTasks {
		switch st.Type {
		case "implement":
			implement = st
		case "review":
			review = st
		case "test":
			test = st
		}
	}

	if implement == nil || review == nil || test == nil {
		t.Fatal("Missing expected subtask types")
	}

	if len(implement.DependsOn) != 0 {
		t.Errorf("Implement should have no dependencies")
	}
	if len(review.DependsOn) != 1 || review.DependsOn[0] != implement.ID {
		t.Errorf("Review should depend on implement")
	}
	if len(test.DependsOn) != 1 || test.DependsOn[0] != review.ID {
		t.Errorf("Test should depend on review")
	}
}

func TestDecomposer_FanOut(t *testing.T) {
	d := NewDecomposer()

	task := models.Task{
		ID: "t1", Type: "implement", Description: "Build API",
		Context: map[string]any{
			"dimensions": []string{"auth", "users", "posts"},
		},
	}
	protocol := models.CollaborationProtocol{
		Strategy:       models.StrategyFanOut,
		TimeoutPerTask: time.Minute,
	}

	graph, err := d.Decompose(task, protocol)
	if err != nil {
		t.Fatalf("Decompose failed: %v", err)
	}

	// 3 parallel + 1 aggregate = 4
	if len(graph.SubTasks) != 4 {
		t.Errorf("Expected 4 subtasks, got %d", len(graph.SubTasks))
	}

	// 应该有 3 个根任务
	if len(graph.RootIDs) != 3 {
		t.Errorf("Expected 3 root tasks, got %d", len(graph.RootIDs))
	}
}

func TestDecomposer_Review(t *testing.T) {
	d := NewDecomposer()

	task := models.Task{ID: "t1", Type: "implement", Description: "Build feature"}
	protocol := models.CollaborationProtocol{
		Strategy:       models.StrategyReview,
		TimeoutPerTask: time.Minute,
	}

	graph, _ := d.Decompose(task, protocol)

	if len(graph.SubTasks) != 2 {
		t.Errorf("Expected 2 subtasks (implement + review), got %d", len(graph.SubTasks))
	}
}

func TestDecomposer_Discussion(t *testing.T) {
	d := NewDecomposer()

	task := models.Task{ID: "t1", Type: "implement", Description: "Design architecture"}
	protocol := models.CollaborationProtocol{
		Strategy:       models.StrategyDiscussion,
		MaxRounds:      3,
		TimeoutPerTask: time.Minute,
	}

	graph, _ := d.Decompose(task, protocol)

	// 1 proposal + 2 discussion rounds + 1 consensus = 4
	if len(graph.SubTasks) != 4 {
		t.Errorf("Expected 4 subtasks, got %d", len(graph.SubTasks))
	}
}

func TestDecomposer_Debate(t *testing.T) {
	d := NewDecomposer()

	task := models.Task{ID: "t1", Type: "implement", Description: "Choose framework"}
	protocol := models.CollaborationProtocol{
		Strategy:       models.StrategyDebate,
		TimeoutPerTask: time.Minute,
	}

	graph, _ := d.Decompose(task, protocol)

	// pro + con + judge = 3
	if len(graph.SubTasks) != 3 {
		t.Errorf("Expected 3 subtasks, got %d", len(graph.SubTasks))
	}
}

func TestDecomposer_Auto(t *testing.T) {
	d := NewDecomposer()

	// "compare" → debate
	task := models.Task{ID: "t1", Type: "implement", Description: "Compare React and Vue"}
	protocol := models.CollaborationProtocol{
		Strategy:       models.StrategyAuto,
		TimeoutPerTask: time.Minute,
	}

	graph, _ := d.Decompose(task, protocol)
	if graph.Strategy != models.StrategyDebate {
		t.Errorf("Expected debate strategy for 'compare', got %s", graph.Strategy)
	}

	// "design" → discussion
	task.Description = "Design the system architecture"
	graph, _ = d.Decompose(task, protocol)
	if graph.Strategy != models.StrategyDiscussion {
		t.Errorf("Expected discussion strategy for 'design', got %s", graph.Strategy)
	}
}

// ============================================================
// TaskGraph 测试
// ============================================================

func TestTaskGraph_GetReadyTasks(t *testing.T) {
	graph := &models.TaskGraph{
		SubTasks: map[string]*models.SubTask{
			"a": {ID: "a", Status: models.SubTaskPending, DependsOn: []string{}},
			"b": {ID: "b", Status: models.SubTaskPending, DependsOn: []string{"a"}},
			"c": {ID: "c", Status: models.SubTaskPending, DependsOn: []string{"a"}},
			"d": {ID: "d", Status: models.SubTaskPending, DependsOn: []string{"b", "c"}},
		},
	}

	// 初始：只有 a 是 ready
	ready := graph.GetReadyTasks()
	if len(ready) != 1 || ready[0].ID != "a" {
		t.Errorf("Expected only 'a' ready, got %v", ready)
	}

	// 完成 a 后：b 和 c 是 ready
	graph.SubTasks["a"].Status = models.SubTaskCompleted
	ready = graph.GetReadyTasks()
	if len(ready) != 2 {
		t.Errorf("Expected 2 ready after a completed, got %d", len(ready))
	}

	// 完成 b 后：只有 c 是 ready（d 还依赖 c）
	graph.SubTasks["b"].Status = models.SubTaskCompleted
	ready = graph.GetReadyTasks()
	if len(ready) != 1 || ready[0].ID != "c" {
		t.Errorf("Expected only 'c' ready, got %v", ready)
	}

	// 完成 c 后：d 是 ready
	graph.SubTasks["c"].Status = models.SubTaskCompleted
	ready = graph.GetReadyTasks()
	if len(ready) != 1 || ready[0].ID != "d" {
		t.Errorf("Expected only 'd' ready, got %v", ready)
	}
}

func TestTaskGraph_IsComplete(t *testing.T) {
	graph := &models.TaskGraph{
		SubTasks: map[string]*models.SubTask{
			"a": {ID: "a", Status: models.SubTaskCompleted},
			"b": {ID: "b", Status: models.SubTaskCompleted},
		},
	}

	if !graph.IsComplete() {
		t.Error("Expected complete")
	}

	graph.SubTasks["b"].Status = models.SubTaskFailed
	if graph.IsComplete() {
		t.Error("Expected not complete with failed task")
	}

	graph.SubTasks["b"].Status = models.SubTaskSkipped
	if !graph.IsComplete() {
		t.Error("Expected complete with skipped task")
	}
}

// ============================================================
// Orchestrator 集成测试
// ============================================================

func TestOrchestrator_PipelineExecution(t *testing.T) {
	registry := NewRegistry()
	bus := NewMessageBus(100)

	// 注册 Agent
	registry.Register(&models.Agent{
		ID: "impl-agent", Name: "Implementer", Adapter: "claude-code",
		Capabilities: []models.Capability{
			{Name: "implement", Confidence: 0.9},
			{Name: "review", Confidence: 0.7},
			{Name: "test", Confidence: 0.6},
		},
	})

	// Mock 执行器
	executor := func(ctx context.Context, adapter string, task models.Task) (*models.Result, error) {
		return &models.Result{
			TaskID: task.ID,
			Status: models.TaskStatusCompleted,
			Output: "Done: " + task.Description,
			Metrics: models.Metrics{
				Duration:   100 * time.Millisecond,
				TokenCount: 50,
			},
		}, nil
	}

	orch := NewOrchestrator(registry, bus, executor)

	ctx := context.Background()
	task := models.Task{
		ID: "task-1", Type: "implement", Description: "Build REST API",
	}

	collab, err := orch.StartCollaboration(ctx, models.StartCollaborationRequest{
		TaskID: task.ID,
		Task:   task,
		Protocol: models.CollaborationProtocol{
			Strategy:       models.StrategyPipeline,
			TimeoutPerTask: time.Minute,
		},
	})
	if err != nil {
		t.Fatalf("StartCollaboration failed: %v", err)
	}

	// 执行
	err = orch.Run(ctx, collab.ID)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// 验证
	result, _ := orch.GetCollaboration(collab.ID)
	if result.Status != models.CollabStatusCompleted {
		t.Errorf("Expected completed, got %s", result.Status)
	}

	// 所有子任务应该都完成了
	for _, st := range result.Graph.SubTasks {
		if st.Status != models.SubTaskCompleted {
			t.Errorf("Subtask %s not completed: %s", st.ID, st.Status)
		}
	}
}

func TestOrchestrator_ReviewExecution(t *testing.T) {
	registry := NewRegistry()
	bus := NewMessageBus(100)

	registry.Register(&models.Agent{
		ID: "worker", Name: "Worker", Adapter: "claude-code",
		Capabilities: []models.Capability{
			{Name: "implement", Confidence: 0.9},
			{Name: "review", Confidence: 0.8},
		},
	})

	executor := func(ctx context.Context, adapter string, task models.Task) (*models.Result, error) {
		return &models.Result{
			TaskID: task.ID, Status: models.TaskStatusCompleted,
			Output: "OK", Metrics: models.Metrics{Duration: time.Millisecond},
		}, nil
	}

	orch := NewOrchestrator(registry, bus, executor)

	collab, _ := orch.StartCollaboration(context.Background(), models.StartCollaborationRequest{
		TaskID: "t1",
		Task:   models.Task{ID: "t1", Type: "implement", Description: "Feature"},
		Protocol: models.CollaborationProtocol{
			Strategy: models.StrategyReview, TimeoutPerTask: time.Minute,
		},
	})

	err := orch.Run(context.Background(), collab.ID)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	result, _ := orch.GetCollaboration(collab.ID)
	if result.Status != models.CollabStatusCompleted {
		t.Errorf("Expected completed, got %s", result.Status)
	}
}
