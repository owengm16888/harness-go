package routine

import (
	"context"
	"testing"
	"time"
)

// ============================================================
// Routine 模型测试
// ============================================================

func TestNewRoutineInstance(t *testing.T) {
	config := RoutineConfig{
		Name:        "test-routine",
		Description: "Test routine",
		Type:        TypeInterview,
		Settings: RoutineSettings{
			MaxRounds: 5,
			Timeout:   30 * time.Minute,
		},
	}

	instance := NewRoutineInstance(config)

	if instance.ID == "" {
		t.Error("Expected non-empty ID")
	}

	if instance.Status != StatusPending {
		t.Errorf("Expected status pending, got %s", instance.Status)
	}

	if instance.Config.Name != "test-routine" {
		t.Errorf("Expected name test-routine, got %s", instance.Config.Name)
	}

	if instance.Round != 0 {
		t.Errorf("Expected round 0, got %d", instance.Round)
	}
}

func TestRoutineInstance_AddMessage(t *testing.T) {
	instance := NewRoutineInstance(RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	})

	msg := Message{
		Role:    "interviewer",
		Content: "Hello",
		Round:   0,
	}

	instance.AddMessage(msg)

	history := instance.GetHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 message, got %d", len(history))
	}

	if history[0].Role != "interviewer" {
		t.Errorf("Expected role interviewer, got %s", history[0].Role)
	}

	if history[0].Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestRoutineInstance_AddScore(t *testing.T) {
	instance := NewRoutineInstance(RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	})

	score := RoundScore{
		Round:    1,
		Question: "What is GMP?",
		Score: Score{
			Correctness: 8,
			Depth:       7,
			Clarity:     9,
			Practical:   6,
			Total:       75,
		},
	}

	instance.AddScore(score)

	if len(instance.Scores) != 1 {
		t.Errorf("Expected 1 score, got %d", len(instance.Scores))
	}

	if instance.Scores[0].Score.Total != 75 {
		t.Errorf("Expected total 75, got %.1f", instance.Scores[0].Score.Total)
	}
}

func TestRoutineInstance_GetAverageScore(t *testing.T) {
	instance := NewRoutineInstance(RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	})

	// 空评分
	avg := instance.GetAverageScore()
	if avg.Total != 0 {
		t.Errorf("Expected 0, got %.1f", avg.Total)
	}

	// 添加评分
	instance.AddScore(RoundScore{
		Round: 1,
		Score: Score{Correctness: 8, Depth: 7, Clarity: 9, Practical: 6, Total: 75},
	})
	instance.AddScore(RoundScore{
		Round: 2,
		Score: Score{Correctness: 9, Depth: 8, Clarity: 8, Practical: 7, Total: 80},
	})

	avg = instance.GetAverageScore()
	if avg.Correctness != 8 {
		t.Errorf("Expected correctness 8, got %d", avg.Correctness)
	}
	if avg.Depth != 7 {
		t.Errorf("Expected depth 7, got %d", avg.Depth)
	}
}

func TestRoutineInstance_SetStatus(t *testing.T) {
	instance := NewRoutineInstance(RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	})

	instance.SetStatus(StatusRunning)
	if instance.Status != StatusRunning {
		t.Errorf("Expected running, got %s", instance.Status)
	}

	if instance.EndTime != nil {
		t.Error("Expected nil end time for running status")
	}

	instance.SetStatus(StatusCompleted)
	if instance.Status != StatusCompleted {
		t.Errorf("Expected completed, got %s", instance.Status)
	}

	if instance.EndTime == nil {
		t.Error("Expected non-nil end time for completed status")
	}
}

func TestRoutineInstance_GetLastQuestion(t *testing.T) {
	instance := NewRoutineInstance(RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	})

	// 空历史
	if q := instance.GetLastQuestion(); q != "" {
		t.Errorf("Expected empty, got %s", q)
	}

	// 添加消息
	instance.AddMessage(Message{Role: "candidate", Content: "Hi"})
	instance.AddMessage(Message{Role: "interviewer", Content: "What is GMP?"})

	q := instance.GetLastQuestion()
	if q != "What is GMP?" {
		t.Errorf("Expected 'What is GMP?', got %s", q)
	}
}

func TestRoutineInstance_GetCandidateAnswers(t *testing.T) {
	instance := NewRoutineInstance(RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	})

	instance.AddMessage(Message{Role: "interviewer", Content: "Q1"})
	instance.AddMessage(Message{Role: "candidate", Content: "A1"})
	instance.AddMessage(Message{Role: "interviewer", Content: "Q2"})
	instance.AddMessage(Message{Role: "candidate", Content: "A2"})

	answers := instance.GetCandidateAnswers()
	if len(answers) != 2 {
		t.Errorf("Expected 2 answers, got %d", len(answers))
	}

	if answers[0].Content != "A1" {
		t.Errorf("Expected A1, got %s", answers[0].Content)
	}
}

// ============================================================
// Agent 测试
// ============================================================

func TestInterviewerAgent(t *testing.T) {
	agent := NewInterviewerAgent()

	if agent.Name() != "interviewer" {
		t.Errorf("Expected interviewer, got %s", agent.Name())
	}

	if agent.Role() != RoleInterviewer {
		t.Errorf("Expected interviewer role, got %s", agent.Role())
	}

	// 第一轮
	input := AgentInput{
		Round:    0,
		Context:  map[string]any{"focus": "concurrency"},
		MaxRounds: 5,
	}

	output, err := agent.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Content == "" {
		t.Error("Expected non-empty content")
	}

	if output.NextAction != "wait_answer" {
		t.Errorf("Expected wait_answer, got %s", output.NextAction)
	}
}

func TestEvaluatorAgent(t *testing.T) {
	agent := NewEvaluatorAgent()

	if agent.Name() != "evaluator" {
		t.Errorf("Expected evaluator, got %s", agent.Name())
	}

	input := AgentInput{
		Question: "What is GMP?",
		Answer:   "GMP is Go's scheduler with G goroutine, M machine, P processor",
	}

	output, err := agent.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Score == nil {
		t.Error("Expected score")
	}

	if output.Score.Total <= 0 {
		t.Errorf("Expected positive total, got %.1f", output.Score.Total)
	}
}

func TestFollowupAgent(t *testing.T) {
	agent := NewFollowupAgent()

	if agent.Name() != "followup_generator" {
		t.Errorf("Expected followup_generator, got %s", agent.Name())
	}

	input := AgentInput{
		Question: "What is GMP?",
		Answer:   "GMP is Go's scheduler",
		Score: &Score{
			Missing: []string{"P processor details"},
		},
	}

	output, err := agent.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Content == "" {
		t.Error("Expected non-empty content")
	}
}

func TestAnalyzerAgent(t *testing.T) {
	agent := NewAnalyzerAgent()

	if agent.Name() != "knowledge_gap_analyzer" {
		t.Errorf("Expected knowledge_gap_analyzer, got %s", agent.Name())
	}

	input := AgentInput{
		Scores: []RoundScore{
			{Round: 1, Score: Score{Correctness: 8, Depth: 7, Clarity: 9, Practical: 6, Total: 75}},
			{Round: 2, Score: Score{Correctness: 9, Depth: 8, Clarity: 8, Practical: 7, Total: 80}},
		},
		MaxRounds: 5,
	}

	output, err := agent.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Analysis == nil {
		t.Error("Expected analysis")
	}

	if !output.Done {
		t.Error("Expected done=true")
	}

	if output.Analysis.Level == "" {
		t.Error("Expected non-empty level")
	}
}

// ============================================================
// 引擎测试
// ============================================================

func TestRoutineEngine_Create(t *testing.T) {
	engine := NewRoutineEngine(EngineConfig{})

	config := RoutineConfig{
		Name: "test",
		Type: TypeInterview,
		Settings: RoutineSettings{
			MaxRounds: 5,
		},
	}

	instance, err := engine.Create(context.Background(), config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if instance.ID == "" {
		t.Error("Expected non-empty ID")
	}

	if instance.Status != StatusPending {
		t.Errorf("Expected pending, got %s", instance.Status)
	}
}

func TestRoutineEngine_GetInstance(t *testing.T) {
	engine := NewRoutineEngine(EngineConfig{})

	config := RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	}

	instance, _ := engine.Create(context.Background(), config)

	retrieved, err := engine.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}

	if retrieved.ID != instance.ID {
		t.Errorf("Expected %s, got %s", instance.ID, retrieved.ID)
	}
}

func TestRoutineEngine_ListInstances(t *testing.T) {
	engine := NewRoutineEngine(EngineConfig{})

	engine.Create(context.Background(), RoutineConfig{Name: "test1", Type: TypeInterview})
	engine.Create(context.Background(), RoutineConfig{Name: "test2", Type: TypeCodeReview})

	instances, err := engine.ListInstances(context.Background())
	if err != nil {
		t.Fatalf("ListInstances failed: %v", err)
	}

	if len(instances) != 2 {
		t.Errorf("Expected 2 instances, got %d", len(instances))
	}
}

func TestRoutineEngine_SubmitAnswer(t *testing.T) {
	engine := NewRoutineEngine(EngineConfig{
		EnableScoring: true,
	})

	config := RoutineConfig{
		Name: "test",
		Type: TypeInterview,
		Workflow: []WorkflowStep{
			{Name: "ask", Agent: "interviewer", Action: "ask_question"},
			{Name: "eval", Agent: "evaluator", Action: "evaluate_answer"},
		},
		Settings: RoutineSettings{
			MaxRounds: 3,
		},
	}

	instance, _ := engine.Create(context.Background(), config)
	engine.Start(context.Background(), instance.ID)

	err := engine.SubmitAnswer(context.Background(), instance.ID, "GMP is Go's scheduler")
	if err != nil {
		t.Fatalf("SubmitAnswer failed: %v", err)
	}

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	history := instance.GetHistory()
	if len(history) == 0 {
		t.Error("Expected messages in history")
	}
}

// ============================================================
// 报告测试
// ============================================================

func TestReportGenerator_Text(t *testing.T) {
	gen := NewReportGenerator("text")

	instance := NewRoutineInstance(RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	})

	instance.AddScore(RoundScore{
		Round: 1,
		Score: Score{Correctness: 8, Depth: 7, Clarity: 9, Practical: 6, Total: 75},
	})

	report := gen.Generate(instance)

	if report == "" {
		t.Error("Expected non-empty report")
	}

	if !contains(report, "test") {
		t.Error("Expected report to contain routine name")
	}
}

func TestReportGenerator_Markdown(t *testing.T) {
	gen := NewReportGenerator("markdown")

	instance := NewRoutineInstance(RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	})

	report := gen.Generate(instance)

	if report == "" {
		t.Error("Expected non-empty report")
	}

	if !contains(report, "#") {
		t.Error("Expected markdown headers")
	}
}

func TestReportGenerator_HTML(t *testing.T) {
	gen := NewReportGenerator("html")

	instance := NewRoutineInstance(RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	})

	report := gen.Generate(instance)

	if report == "" {
		t.Error("Expected non-empty report")
	}

	if !contains(report, "<!DOCTYPE html>") {
		t.Error("Expected HTML tags")
	}
}

// ============================================================
// 存储测试
// ============================================================

func TestMemoryRoutineStore(t *testing.T) {
	store := NewMemoryRoutineStore()

	instance := NewRoutineInstance(RoutineConfig{
		Name: "test",
		Type: TypeInterview,
	})

	// 保存
	err := store.Save(context.Background(), instance)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 获取
	retrieved, err := store.Get(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != instance.ID {
		t.Errorf("Expected %s, got %s", instance.ID, retrieved.ID)
	}

	// 列表
	instances, err := store.List(context.Background(), StoreFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(instances) != 1 {
		t.Errorf("Expected 1, got %d", len(instances))
	}

	// 删除
	err = store.Delete(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(context.Background(), instance.ID)
	if err == nil {
		t.Error("Expected error after delete")
	}
}

// ============================================================
// 辅助函数
// ============================================================

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
