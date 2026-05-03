package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/harness-engineering/harness/internal/learning"
	"github.com/harness-engineering/harness/internal/patterns"
	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
)

// PatternEngine 模式引擎
type PatternEngine struct {
	mu       sync.RWMutex
	patterns map[string]*models.Pattern
	store    storage.PatternStore
	matcher  *patterns.Matcher
	learner  *learning.Learner
}

// NewPatternEngine 创建模式引擎
func NewPatternEngine(store storage.PatternStore, minSamples int, threshold float64) *PatternEngine {
	return &PatternEngine{
		patterns: make(map[string]*models.Pattern),
		store:    store,
		matcher:  patterns.NewMatcher(),
		learner:  learning.NewLearner(minSamples, threshold),
	}
}

// LoadFromStorage 从持久化存储加载所有模式到内存
func (pe *PatternEngine) LoadFromStorage(ctx context.Context) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	patternsList, err := pe.store.ListPatterns(ctx)
	if err != nil {
		return fmt.Errorf("failed to load patterns from storage: %w", err)
	}

	for _, pattern := range patternsList {
		pe.patterns[pattern.ID] = pattern
	}

	return nil
}

// Match 匹配模式
func (pe *PatternEngine) Match(ctx context.Context, task models.Task) ([]*models.Pattern, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var matched []*models.Pattern

	for _, pattern := range pe.patterns {
		if pe.matcher.Match(ctx, task, pattern) {
			matched = append(matched, pattern)
		}
	}

	// 按成功率排序
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].SuccessRate > matched[j].SuccessRate
	})

	return matched, nil
}

// Learn 学习新模式
func (pe *PatternEngine) Learn(ctx context.Context, observation models.Observation) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	// 通过学习器观察
	if err := pe.learner.Observe(ctx, observation); err != nil {
		return fmt.Errorf("learner observe failed: %w", err)
	}

	// 更新现有模式
	if observation.Pattern != "" {
		if pattern, exists := pe.patterns[observation.Pattern]; exists {
			pe.updatePatternStats(pattern, observation)
		}
	}

	return nil
}

// Predict 预测任务结果
func (pe *PatternEngine) Predict(ctx context.Context, task models.Task) (*models.Prediction, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	return pe.learner.Predict(ctx, task)
}

// AddPattern 添加模式
func (pe *PatternEngine) AddPattern(ctx context.Context, pattern models.Pattern) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	// 保存到存储
	if err := pe.store.SavePattern(ctx, &pattern); err != nil {
		return fmt.Errorf("failed to save pattern: %w", err)
	}

	// 添加到内存
	pe.patterns[pattern.ID] = &pattern

	return nil
}

// UpdatePattern 更新模式
func (pe *PatternEngine) UpdatePattern(ctx context.Context, id string, update models.PatternUpdate) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	pattern, exists := pe.patterns[id]
	if !exists {
		return fmt.Errorf("pattern not found: %s", id)
	}

	// 应用更新
	if update.Name != "" {
		pattern.Name = update.Name
	}
	if update.Description != "" {
		pattern.Description = update.Description
	}
	if update.Trigger != "" {
		pattern.Trigger = update.Trigger
	}
	if update.Actions != nil {
		pattern.Actions = update.Actions
	}

	// 保存到存储
	if err := pe.store.SavePattern(ctx, pattern); err != nil {
		return fmt.Errorf("failed to save pattern: %w", err)
	}

	return nil
}

// DeletePattern 删除模式
func (pe *PatternEngine) DeletePattern(ctx context.Context, id string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	pattern, exists := pe.patterns[id]
	if !exists {
		return fmt.Errorf("pattern not found: %s", id)
	}

	// 从存储删除
	if err := pe.store.DeletePattern(ctx, id); err != nil {
		return fmt.Errorf("failed to delete pattern: %w", err)
	}

	// 从内存删除
	delete(pe.patterns, id)
	_ = pattern // suppress unused warning

	return nil
}

// ListPatterns 列出模式
func (pe *PatternEngine) ListPatterns(ctx context.Context) ([]*models.Pattern, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var patternsList []*models.Pattern
	for _, pattern := range pe.patterns {
		patternsList = append(patternsList, pattern)
	}

	// 按成功率排序
	sort.Slice(patternsList, func(i, j int) bool {
		return patternsList[i].SuccessRate > patternsList[j].SuccessRate
	})

	return patternsList, nil
}

// GetPattern 获取模式
func (pe *PatternEngine) GetPattern(ctx context.Context, id string) (*models.Pattern, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	pattern, exists := pe.patterns[id]
	if !exists {
		return nil, fmt.Errorf("pattern not found: %s", id)
	}

	return pattern, nil
}

// updatePatternStats 更新模式统计
func (pe *PatternEngine) updatePatternStats(pattern *models.Pattern, observation models.Observation) {
	pattern.UsageCount++
	pattern.LastUsed = time.Now()

	// 更新成功率（增量平均）
	if observation.Success {
		pattern.SuccessRate = (pattern.SuccessRate*float64(pattern.UsageCount-1) + 1) / float64(pattern.UsageCount)
	} else {
		pattern.SuccessRate = (pattern.SuccessRate * float64(pattern.UsageCount-1)) / float64(pattern.UsageCount)
	}
}
