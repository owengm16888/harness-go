package core

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/harness-engineering/harness/models"
)

// Monitor 监控器 — 收集并暴露指标
type Monitor struct {
	mu       sync.RWMutex
	metrics  *Metrics
	feedback []*models.FeedbackResult
	tasks    []models.Result
	startAt  time.Time
}

// Metrics 指标
type Metrics struct {
	TotalTasks      int           `json:"total_tasks"`
	SuccessTasks    int           `json:"success_tasks"`
	FailedTasks     int           `json:"failed_tasks"`
	CancelledTasks  int           `json:"cancelled_tasks"`
	TotalFeedback   int           `json:"total_feedback"`
	PassedFeedback  int           `json:"passed_feedback"`
	FixedFeedback   int           `json:"fixed_feedback"`
	ViolatedFeedback int          `json:"violated_feedback"`
	AverageDuration time.Duration `json:"average_duration"`
	TotalDuration   time.Duration `json:"total_duration"`
	TotalTokens     int           `json:"total_tokens"`
	TotalToolUses   int           `json:"total_tool_uses"`
	TotalErrors     int           `json:"total_errors"`
}

// NewMonitor 创建监控器
func NewMonitor() *Monitor {
	return &Monitor{
		metrics:  &Metrics{},
		feedback: []*models.FeedbackResult{},
		tasks:    []models.Result{},
		startAt:  time.Now(),
	}
}

// RecordTask 记录任务指标
func (m *Monitor) RecordTask(result models.Result) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks = append(m.tasks, result)
	m.metrics.TotalTasks++

	switch result.Status {
	case models.TaskStatusCompleted:
		m.metrics.SuccessTasks++
	case models.TaskStatusFailed:
		m.metrics.FailedTasks++
	case models.TaskStatusCancelled:
		m.metrics.CancelledTasks++
	}

	// 累加指标
	m.metrics.TotalDuration += result.Metrics.Duration
	m.metrics.TotalTokens += result.Metrics.TokenCount
	m.metrics.TotalToolUses += result.Metrics.ToolUses
	m.metrics.TotalErrors += result.Metrics.ErrorCount

	// 更新平均时长
	if m.metrics.TotalTasks > 0 {
		m.metrics.AverageDuration = m.metrics.TotalDuration / time.Duration(m.metrics.TotalTasks)
	}
}

// RecordFeedback 记录反馈指标
func (m *Monitor) RecordFeedback(result *models.FeedbackResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.feedback = append(m.feedback, result)
	m.metrics.TotalFeedback++

	switch result.Status {
	case "passed":
		m.metrics.PassedFeedback++
	case "fixed":
		m.metrics.FixedFeedback++
	case "violations_found", "partial_fix":
		m.metrics.ViolatedFeedback++
	}
}

// GetMetrics 获取指标
func (m *Monitor) GetMetrics() *Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// GetFeedback 获取任务的反馈
func (m *Monitor) GetFeedback(taskID string) []*models.FeedbackResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*models.FeedbackResult
	for _, fb := range m.feedback {
		if fb.TaskID == taskID {
			results = append(results, fb)
		}
	}
	return results
}

// Uptime 返回运行时长
func (m *Monitor) Uptime() time.Duration {
	return time.Since(m.startAt)
}

// ExportPrometheus 输出 Prometheus 文本格式指标
func (m *Monitor) ExportPrometheus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var b strings.Builder

	// 任务指标
	b.WriteString("# HELP harness_tasks_total Total number of tasks processed\n")
	b.WriteString("# TYPE harness_tasks_total counter\n")
	b.WriteString(fmt.Sprintf("harness_tasks_total %d\n", m.metrics.TotalTasks))

	b.WriteString("# HELP harness_tasks_success Total successful tasks\n")
	b.WriteString("# TYPE harness_tasks_success counter\n")
	b.WriteString(fmt.Sprintf("harness_tasks_success %d\n", m.metrics.SuccessTasks))

	b.WriteString("# HELP harness_tasks_failed Total failed tasks\n")
	b.WriteString("# TYPE harness_tasks_failed counter\n")
	b.WriteString(fmt.Sprintf("harness_tasks_failed %d\n", m.metrics.FailedTasks))

	b.WriteString("# HELP harness_tasks_cancelled Total cancelled tasks\n")
	b.WriteString("# TYPE harness_tasks_cancelled counter\n")
	b.WriteString(fmt.Sprintf("harness_tasks_cancelled %d\n", m.metrics.CancelledTasks))

	// 反馈指标
	b.WriteString("# HELP harness_feedback_total Total feedback entries\n")
	b.WriteString("# TYPE harness_feedback_total counter\n")
	b.WriteString(fmt.Sprintf("harness_feedback_total %d\n", m.metrics.TotalFeedback))

	b.WriteString("# HELP harness_feedback_passed Total passed feedback\n")
	b.WriteString("# TYPE harness_feedback_passed counter\n")
	b.WriteString(fmt.Sprintf("harness_feedback_passed %d\n", m.metrics.PassedFeedback))

	b.WriteString("# HELP harness_feedback_fixed Total auto-fixed feedback\n")
	b.WriteString("# TYPE harness_feedback_fixed counter\n")
	b.WriteString(fmt.Sprintf("harness_feedback_fixed %d\n", m.metrics.FixedFeedback))

	b.WriteString("# HELP harness_feedback_violated Total violated feedback\n")
	b.WriteString("# TYPE harness_feedback_violated counter\n")
	b.WriteString(fmt.Sprintf("harness_feedback_violated %d\n", m.metrics.ViolatedFeedback))

	// 性能指标
	b.WriteString("# HELP harness_duration_avg_seconds Average task duration\n")
	b.WriteString("# TYPE harness_duration_avg_seconds gauge\n")
	b.WriteString(fmt.Sprintf("harness_duration_avg_seconds %.3f\n", m.metrics.AverageDuration.Seconds()))

	b.WriteString("# HELP harness_duration_total_seconds Total task duration\n")
	b.WriteString("# TYPE harness_duration_total_seconds counter\n")
	b.WriteString(fmt.Sprintf("harness_duration_total_seconds %.3f\n", m.metrics.TotalDuration.Seconds()))

	// 资源指标
	b.WriteString("# HELP harness_tokens_total Total tokens consumed\n")
	b.WriteString("# TYPE harness_tokens_total counter\n")
	b.WriteString(fmt.Sprintf("harness_tokens_total %d\n", m.metrics.TotalTokens))

	b.WriteString("# HELP harness_tool_uses_total Total tool invocations\n")
	b.WriteString("# TYPE harness_tool_uses_total counter\n")
	b.WriteString(fmt.Sprintf("harness_tool_uses_total %d\n", m.metrics.TotalToolUses))

	b.WriteString("# HELP harness_errors_total Total errors\n")
	b.WriteString("# TYPE harness_errors_total counter\n")
	b.WriteString(fmt.Sprintf("harness_errors_total %d\n", m.metrics.TotalErrors))

	// 系统指标
	b.WriteString("# HELP harness_uptime_seconds Process uptime\n")
	b.WriteString("# TYPE harness_uptime_seconds gauge\n")
	b.WriteString(fmt.Sprintf("harness_uptime_seconds %.0f\n", m.Uptime().Seconds()))

	return b.String()
}
