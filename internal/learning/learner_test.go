package learning

import (
	"context"
	"testing"
	"time"

	"github.com/harness-engineering/harness/models"
)

func TestLearner_Observe(t *testing.T) {
	learner := NewLearner(3, 0.7)

	ctx := context.Background()

	// 观察一个成功的实现任务
	err := learner.Observe(ctx, models.Observation{
		Task: models.Task{
			ID:          "task-1",
			Type:        "implement",
			Description: "Implement auth",
			Context:     map[string]any{"environment": "production"},
		},
		Result: models.Result{
			TaskID: "task-1",
			Status: models.TaskStatusCompleted,
			Metrics: models.Metrics{
				Duration: 2 * time.Second,
			},
		},
		Success:   true,
		Timestamp: time.Now(),
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 应该创建了一个模式
	if len(learner.patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(learner.patterns))
	}
}

func TestLearner_Predict(t *testing.T) {
	learner := NewLearner(2, 0.5) // 低阈值，方便测试
	ctx := context.Background()

	// 观察多个相似任务
	for i := 0; i < 5; i++ {
		learner.Observe(ctx, models.Observation{
			Task: models.Task{
				ID:          "task-" + string(rune('a'+i)),
				Type:        "implement",
				Description: "Implement feature",
				Context:     map[string]any{"environment": "production"},
			},
			Result: models.Result{
				Metrics: models.Metrics{Duration: time.Second},
			},
			Success:   true,
			Timestamp: time.Now(),
		})
	}

	// 预测相似任务
	prediction, err := learner.Predict(ctx, models.Task{
		Type:        "implement",
		Description: "New feature",
		Context:     map[string]any{"environment": "production"},
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if prediction == nil {
		t.Fatal("Expected prediction, got nil")
	}
	if prediction.Confidence <= 0 {
		t.Errorf("Expected positive confidence, got %f", prediction.Confidence)
	}
}

func TestLearner_PredictNoPattern(t *testing.T) {
	learner := NewLearner(3, 0.7)
	ctx := context.Background()

	prediction, err := learner.Predict(ctx, models.Task{
		Type:        "unknown",
		Description: "Unknown task",
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if prediction.Confidence != 0 {
		t.Errorf("Expected 0 confidence, got %f", prediction.Confidence)
	}
	if prediction.Recommendation != "no pattern found" {
		t.Errorf("Expected 'no pattern found', got '%s'", prediction.Recommendation)
	}
}

func TestLearner_BayesianConfidence(t *testing.T) {
	learner := NewLearner(3, 0.7) // minSamples=3
	ctx := context.Background()

	// 观察 5 个成功任务
	for i := 0; i < 5; i++ {
		learner.Observe(ctx, models.Observation{
			Task: models.Task{
				ID:   "t",
				Type: "test",
				Context: map[string]any{"environment": "dev"},
			},
			Result:    models.Result{Metrics: models.Metrics{Duration: time.Second}},
			Success:   true,
			Timestamp: time.Now(),
		})
	}

	// 找到模式并检查置信度
	for _, p := range learner.patterns {
		// Beta(1,1) 先验 + 5/5 成功 → (1+5)/(2+5) = 6/7 ≈ 0.857
		expected := 6.0 / 7.0
		if abs(p.Confidence-expected) > 0.01 {
			t.Errorf("Expected confidence ~%.3f, got %.3f", expected, p.Confidence)
		}
		break
	}
}

func TestLearner_FeatureSimilarity(t *testing.T) {
	learner := NewLearner(1, 0.5)

	features1 := []Feature{
		{Name: "task_type", Value: "implement", Weight: 1.0},
		{Name: "environment", Value: "production", Weight: 0.8},
	}

	features2 := []Feature{
		{Name: "task_type", Value: "implement", Weight: 1.0},
		{Name: "environment", Value: "production", Weight: 0.8},
	}

	sim := learner.calculateSimilarity(features1, features2)
	if sim != 1.0 {
		t.Errorf("Expected similarity 1.0, got %f", sim)
	}

	// 不同环境
	features3 := []Feature{
		{Name: "task_type", Value: "implement", Weight: 1.0},
		{Name: "environment", Value: "staging", Weight: 0.8},
	}

	sim = learner.calculateSimilarity(features1, features3)
	if sim >= 1.0 {
		t.Errorf("Expected similarity < 1.0 for different environment, got %f", sim)
	}
}

func TestLearner_ComplexityCalculation(t *testing.T) {
	learner := NewLearner(1, 0.5)

	// 简单任务
	simple := models.Task{
		ID:          "simple",
		Type:        "test",
		Description: "short",
	}
	c1 := learner.calculateComplexity(simple)

	// 复杂任务
	complex := models.Task{
		ID:          "complex",
		Type:        "implement",
		Description: "A very long description that goes on and on and on and adds complexity",
		Constraints: []models.Constraint{
			{Type: "security", Rule: "no-secrets"},
			{Type: "quality", Rule: "require-tests"},
			{Type: "architecture", Rule: "no-circular"},
		},
		Context: map[string]any{
			"env":  "production",
			"team": "backend",
			"lang": "go",
		},
	}
	c2 := learner.calculateComplexity(complex)

	if c2 <= c1 {
		t.Errorf("Expected complex task (%.3f) > simple task (%.3f)", c2, c1)
	}
	if c2 > 1.0 {
		t.Errorf("Complexity should be capped at 1.0, got %f", c2)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
