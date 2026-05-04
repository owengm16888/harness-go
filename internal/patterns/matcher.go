package patterns

import (
	"context"
	"strings"

	"github.com/harness-engineering/harness/models"
)

// Matcher 模式匹配器
type Matcher struct {
	rules []MatchRule
}

// MatchRule 匹配规则
type MatchRule struct {
	Name      string
	Type      string
	Condition func(ctx context.Context, task models.Task, pattern *models.Pattern) bool
	Priority  int
}

// NewMatcher 创建匹配器
func NewMatcher() *Matcher {
	m := &Matcher{
		rules: []MatchRule{},
	}

	// 注册默认规则
	m.registerDefaultRules()

	return m
}

// Match 匹配模式
func (m *Matcher) Match(ctx context.Context, task models.Task, pattern *models.Pattern) bool {
	for _, rule := range m.rules {
		if rule.Condition(ctx, task, pattern) {
			return true
		}
	}
	return false
}

// registerDefaultRules 注册默认规则
func (m *Matcher) registerDefaultRules() {
	// 类型匹配规则
	m.rules = append(m.rules, MatchRule{
		Name: "type_match",
		Type: "string",
		Condition: func(ctx context.Context, task models.Task, pattern *models.Pattern) bool {
			// 先查 pattern.Metadata 里的 task_type
			if taskType, ok := pattern.Metadata["task_type"]; ok {
				return task.Type == taskType
			}
			// 再用 trigger 和 task.Type 做模糊匹配
			if pattern.Trigger != "" {
				triggerLower := strings.ToLower(pattern.Trigger)
				taskTypeLower := strings.ToLower(task.Type)
				descLower := strings.ToLower(task.Description)
				// trigger 匹配 task.Type 或 task.Description
				if strings.Contains(taskTypeLower, triggerLower) || strings.Contains(triggerLower, taskTypeLower) {
					return true
				}
				if strings.Contains(descLower, triggerLower) {
					return true
				}
			}
			return false
		},
		Priority: 9,
	})

	// 触发器匹配规则 (保留原有逻辑)
	m.rules = append(m.rules, MatchRule{
		Name: "trigger_match",
		Type: "string",
		Condition: func(ctx context.Context, task models.Task, pattern *models.Pattern) bool {
			return m.matchTrigger(task, pattern.Trigger)
		},
		Priority: 10,
	})

	// 上下文匹配规则
	m.rules = append(m.rules, MatchRule{
		Name: "context_match",
		Type: "map",
		Condition: func(ctx context.Context, task models.Task, pattern *models.Pattern) bool {
			if patternContext, ok := pattern.Metadata["context"].(map[string]any); ok {
				return m.matchContext(task.Context, patternContext)
			}
			return false
		},
		Priority: 6,
	})

	// 约束匹配规则
	m.rules = append(m.rules, MatchRule{
		Name: "constraint_match",
		Type: "array",
		Condition: func(ctx context.Context, task models.Task, pattern *models.Pattern) bool {
			if patternConstraints, ok := pattern.Metadata["constraints"].([]models.Constraint); ok {
				return m.matchConstraints(task.Constraints, patternConstraints)
			}
			return false
		},
		Priority: 4,
	})
}

// matchTrigger 匹配触发器
func (m *Matcher) matchTrigger(task models.Task, trigger string) bool {
	// 精确匹配
	if task.Description == trigger {
		return true
	}

	// 关键词匹配
	keywords := strings.Split(trigger, "|")
	for _, keyword := range keywords {
		if strings.Contains(task.Description, strings.TrimSpace(keyword)) {
			return true
		}
	}

	return false
}

// matchContext 匹配上卜文
func (m *Matcher) matchContext(taskContext, patternContext map[string]any) bool {
	for key, patternValue := range patternContext {
		taskValue, exists := taskContext[key]
		if !exists {
			return false
		}
		if taskValue != patternValue {
			return false
		}
	}
	return true
}

// matchConstraints 匹配约束
func (m *Matcher) matchConstraints(taskConstraints, patternConstraints []models.Constraint) bool {
	for _, patternConstraint := range patternConstraints {
		found := false
		for _, taskConstraint := range taskConstraints {
			if taskConstraint.Rule == patternConstraint.Rule {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
