package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/harness-engineering/harness/models"
)

// Validator 验证器接口
type Validator interface {
	Name() string
	Validate(ctx context.Context, result models.Result) ([]models.Violation, error)
}

// Fixer 修复器接口
type Fixer interface {
	Name() string
	CanFix(violation models.Violation) bool
	Fix(ctx context.Context, violation models.Violation) (*models.FixResult, error)
}

// FeedbackLoop 实现反馈循环
type FeedbackLoop struct {
	mu         sync.RWMutex
	validators []Validator
	fixers     []Fixer
	monitor    *Monitor
	config     models.FeedbackConfig
}

// NewFeedbackLoop 创建反馈循环
func NewFeedbackLoop(config models.FeedbackConfig, monitor *Monitor) *FeedbackLoop {
	return &FeedbackLoop{
		validators: []Validator{},
		fixers:     []Fixer{},
		monitor:    monitor,
		config:     config,
	}
}

// AddValidator 添加验证器
func (fl *FeedbackLoop) AddValidator(v Validator) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.validators = append(fl.validators, v)
}

// AddFixer 添加修复器
func (fl *FeedbackLoop) AddFixer(f Fixer) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.fixers = append(fl.fixers, f)
}

// Process 处理反馈循环
func (fl *FeedbackLoop) Process(ctx context.Context, result models.Result) (*models.FeedbackResult, error) {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	feedbackResult := &models.FeedbackResult{
		TaskID:    result.TaskID,
		Timestamp: time.Now(),
	}

	// 验证阶段
	violations, err := fl.validate(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	feedbackResult.Violations = violations

	// 如果没有违规，直接返回
	if len(violations) == 0 {
		feedbackResult.Status = "passed"
		return feedbackResult, nil
	}

	// 修复阶段
	if fl.config.AutoFix {
		fixes, err := fl.fix(ctx, violations)
		if err != nil {
			return nil, fmt.Errorf("fix failed: %w", err)
		}
		feedbackResult.Fixes = fixes

		// 检查是否所有违规都已修复
		allFixed := true
		for _, fix := range fixes {
			if !fix.Success {
				allFixed = false
				break
			}
		}

		if allFixed {
			feedbackResult.Status = "fixed"
		} else {
			feedbackResult.Status = "partial_fix"
		}
	} else {
		feedbackResult.Status = "violations_found"
	}

	// 记录指标
	fl.monitor.RecordFeedback(feedbackResult)

	return feedbackResult, nil
}

// validate 执行验证
func (fl *FeedbackLoop) validate(ctx context.Context, result models.Result) ([]models.Violation, error) {
	var violations []models.Violation

	for _, validator := range fl.validators {
		v, err := validator.Validate(ctx, result)
		if err != nil {
			return nil, fmt.Errorf("validator %s failed: %w", validator.Name(), err)
		}
		violations = append(violations, v...)
	}

	return violations, nil
}

// fix 执行修复
func (fl *FeedbackLoop) fix(ctx context.Context, violations []models.Violation) ([]models.FixResult, error) {
	var fixes []models.FixResult

	for _, violation := range violations {
		if !violation.Fixable {
			fixes = append(fixes, models.FixResult{
				Success: false,
				Message: "violation is not fixable",
			})
			continue
		}

		fixed := false
		for _, fixer := range fl.fixers {
			if fixer.CanFix(violation) {
				result, err := fixer.Fix(ctx, violation)
				if err != nil {
					fixes = append(fixes, models.FixResult{
						Success: false,
						Message: fmt.Sprintf("fix failed: %s", err.Error()),
					})
					continue
				}
				fixes = append(fixes, *result)
				fixed = true
				break
			}
		}

		if !fixed {
			fixes = append(fixes, models.FixResult{
				Success: false,
				Message: "no fixer available",
			})
		}
	}

	return fixes, nil
}

