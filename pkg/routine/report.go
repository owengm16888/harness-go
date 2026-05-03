package routine

import (
	"fmt"
	"strings"
	"time"
)

// ============================================================
// 可视化报告生成器
// ============================================================

// ReportGenerator 报告生成器
type ReportGenerator struct {
	format string
}

// NewReportGenerator 创建报告生成器
func NewReportGenerator(format string) *ReportGenerator {
	if format == "" {
		format = "text"
	}
	return &ReportGenerator{format: format}
}

// Generate 生成报告
func (g *ReportGenerator) Generate(instance *RoutineInstance) string {
	switch g.format {
	case "markdown":
		return g.generateMarkdown(instance)
	case "html":
		return g.generateHTML(instance)
	case "json":
		return g.generateJSON(instance)
	default:
		return g.generateText(instance)
	}
}

// generateText 生成文本报告
func (g *ReportGenerator) generateText(instance *RoutineInstance) string {
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║              Harness Engineering Routine 报告               ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════╝\n\n")

	// 基本信息
	sb.WriteString("━━━ 基本信息 ━━━\n")
	sb.WriteString(fmt.Sprintf("  名称:     %s\n", instance.Config.Name))
	sb.WriteString(fmt.Sprintf("  类型:     %s\n", instance.Config.Type))
	sb.WriteString(fmt.Sprintf("  状态:     %s\n", instance.Status))
	sb.WriteString(fmt.Sprintf("  轮次:     %d / %d\n", instance.Round, instance.Config.Settings.MaxRounds))
	sb.WriteString(fmt.Sprintf("  开始时间: %s\n", instance.StartTime.Format("2006-01-02 15:04:05")))

	if instance.EndTime != nil {
		duration := instance.EndTime.Sub(instance.StartTime)
		sb.WriteString(fmt.Sprintf("  结束时间: %s\n", instance.EndTime.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("  持续时间: %s\n", formatDuration(duration)))
	}

	sb.WriteString("\n")

	// 评分统计
	if len(instance.Scores) > 0 {
		sb.WriteString("━━━ 评分统计 ━━━\n")
		avg := instance.GetAverageScore()
		sb.WriteString(fmt.Sprintf("  综合分:   %.1f / 100\n", avg.Total))
		sb.WriteString(fmt.Sprintf("  正确性:   %d / 10\n", avg.Correctness))
		sb.WriteString(fmt.Sprintf("  深度:     %d / 10\n", avg.Depth))
		sb.WriteString(fmt.Sprintf("  清晰度:   %d / 10\n", avg.Clarity))
		sb.WriteString(fmt.Sprintf("  实用性:   %d / 10\n", avg.Practical))
		sb.WriteString("\n")

		// 每轮详情
		sb.WriteString("━━━ 每轮详情 ━━━\n")
		for i, score := range instance.Scores {
			sb.WriteString(fmt.Sprintf("  第 %d 轮: %.1f 分\n", i+1, score.Score.Total))
			sb.WriteString(fmt.Sprintf("    问题: %s\n", truncate(score.Question, 50)))
			if len(score.Score.Strengths) > 0 {
				sb.WriteString(fmt.Sprintf("    优点: %s\n", strings.Join(score.Score.Strengths, "、")))
			}
			if len(score.Score.Weaknesses) > 0 {
				sb.WriteString(fmt.Sprintf("    不足: %s\n", strings.Join(score.Score.Weaknesses, "、")))
			}
		}
		sb.WriteString("\n")
	}

	// 最终报告
	if instance.FinalReport != nil {
		report := instance.FinalReport
		sb.WriteString("━━━ 最终评估 ━━━\n")
		sb.WriteString(fmt.Sprintf("  技术评级: %s\n", report.Level))

		if report.Pass {
			sb.WriteString("  面试结果: 通过 ✓\n")
		} else {
			sb.WriteString("  面试结果: 未通过 ✗\n")
		}

		sb.WriteString(fmt.Sprintf("  总分:     %.1f / %.1f\n", report.TotalScore, report.MaxScore))

		if len(report.StrongAreas) > 0 {
			sb.WriteString("\n  优势领域:\n")
			for _, area := range report.StrongAreas {
				sb.WriteString(fmt.Sprintf("    ✓ %s\n", area))
			}
		}

		if len(report.WeakAreas) > 0 {
			sb.WriteString("\n  薄弱环节:\n")
			for _, area := range report.WeakAreas {
				sb.WriteString(fmt.Sprintf("    ✗ %s\n", area))
			}
		}

		if len(report.StudyPlan) > 0 {
			sb.WriteString("\n  学习计划:\n")
			for i, item := range report.StudyPlan {
				sb.WriteString(fmt.Sprintf("    %d. %s\n", i+1, item.Topic))
				sb.WriteString(fmt.Sprintf("       原因: %s\n", item.Why))
				sb.WriteString(fmt.Sprintf("       资源: %s\n", item.Resource))
			}
		}

		if report.Summary != "" {
			sb.WriteString(fmt.Sprintf("\n  总结: %s\n", report.Summary))
		}
	}

	// 对话历史
	if len(instance.History) > 0 {
		sb.WriteString("\n━━━ 对话历史 ━━━\n")
		for _, msg := range instance.History {
			role := formatRole(msg.Role)
			sb.WriteString(fmt.Sprintf("\n  [%s] %s\n", role, msg.Timestamp.Format("15:04:05")))
			sb.WriteString(fmt.Sprintf("  %s\n", indent(msg.Content, 2)))
		}
	}

	sb.WriteString("\n═══════════════════════════════════════════════════════════════\n")

	return sb.String()
}

// generateMarkdown 生成 Markdown 报告
func (g *ReportGenerator) generateMarkdown(instance *RoutineInstance) string {
	var sb strings.Builder

	sb.WriteString("# Harness Engineering Routine 报告\n\n")

	// 基本信息
	sb.WriteString("## 基本信息\n\n")
	sb.WriteString("| 项目 | 值 |\n")
	sb.WriteString("|------|-----|\n")
	sb.WriteString(fmt.Sprintf("| 名称 | %s |\n", instance.Config.Name))
	sb.WriteString(fmt.Sprintf("| 类型 | %s |\n", instance.Config.Type))
	sb.WriteString(fmt.Sprintf("| 状态 | %s |\n", instance.Status))
	sb.WriteString(fmt.Sprintf("| 轮次 | %d / %d |\n", instance.Round, instance.Config.Settings.MaxRounds))
	sb.WriteString(fmt.Sprintf("| 开始时间 | %s |\n", instance.StartTime.Format("2006-01-02 15:04:05")))

	if instance.EndTime != nil {
		duration := instance.EndTime.Sub(instance.StartTime)
		sb.WriteString(fmt.Sprintf("| 持续时间 | %s |\n", formatDuration(duration)))
	}

	sb.WriteString("\n")

	// 评分统计
	if len(instance.Scores) > 0 {
		sb.WriteString("## 评分统计\n\n")
		avg := instance.GetAverageScore()

		sb.WriteString("### 综合评分\n\n")
		sb.WriteString(fmt.Sprintf("**%.1f** / 100\n\n", avg.Total))

		sb.WriteString("### 维度评分\n\n")
		sb.WriteString("```")
		sb.WriteString(fmt.Sprintf("正确性: %s %d/10\n", renderBar(avg.Correctness, 10), avg.Correctness))
		sb.WriteString(fmt.Sprintf("深  度: %s %d/10\n", renderBar(avg.Depth, 10), avg.Depth))
		sb.WriteString(fmt.Sprintf("清晰度: %s %d/10\n", renderBar(avg.Clarity, 10), avg.Clarity))
		sb.WriteString(fmt.Sprintf("实用性: %s %d/10\n", renderBar(avg.Practical, 10), avg.Practical))
		sb.WriteString("```\n\n")

		// 每轮详情
		sb.WriteString("### 每轮详情\n\n")
		for i, score := range instance.Scores {
			sb.WriteString(fmt.Sprintf("#### 第 %d 轮 (%.1f 分)\n\n", i+1, score.Score.Total))
			sb.WriteString(fmt.Sprintf("**问题:** %s\n\n", score.Question))

			if len(score.Score.Strengths) > 0 {
				sb.WriteString("**优点:**\n")
				for _, s := range score.Score.Strengths {
					sb.WriteString(fmt.Sprintf("- ✓ %s\n", s))
				}
				sb.WriteString("\n")
			}

			if len(score.Score.Weaknesses) > 0 {
				sb.WriteString("**不足:**\n")
				for _, w := range score.Score.Weaknesses {
					sb.WriteString(fmt.Sprintf("- ✗ %s\n", w))
				}
				sb.WriteString("\n")
			}
		}
	}

	// 最终报告
	if instance.FinalReport != nil {
		report := instance.FinalReport
		sb.WriteString("## 最终评估\n\n")

		result := "通过 ✓"
		if !report.Pass {
			result = "未通过 ✗"
		}

		sb.WriteString(fmt.Sprintf("- **技术评级:** %s\n", report.Level))
		sb.WriteString(fmt.Sprintf("- **面试结果:** %s\n", result))
		sb.WriteString(fmt.Sprintf("- **总分:** %.1f / %.1f\n\n", report.TotalScore, report.MaxScore))

		if len(report.StrongAreas) > 0 {
			sb.WriteString("### 优势领域\n\n")
			for _, area := range report.StrongAreas {
				sb.WriteString(fmt.Sprintf("- ✓ %s\n", area))
			}
			sb.WriteString("\n")
		}

		if len(report.WeakAreas) > 0 {
			sb.WriteString("### 薄弱环节\n\n")
			for _, area := range report.WeakAreas {
				sb.WriteString(fmt.Sprintf("- ✗ %s\n", area))
			}
			sb.WriteString("\n")
		}

		if len(report.StudyPlan) > 0 {
			sb.WriteString("### 学习计划\n\n")
			for i, item := range report.StudyPlan {
				sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, item.Topic))
				sb.WriteString(fmt.Sprintf("   - 原因: %s\n", item.Why))
				sb.WriteString(fmt.Sprintf("   - 资源: %s\n", item.Resource))
			}
			sb.WriteString("\n")
		}

		if report.Summary != "" {
			sb.WriteString("### 总结\n\n")
			sb.WriteString(report.Summary + "\n")
		}
	}

	// 对话历史
	if len(instance.History) > 0 {
		sb.WriteString("## 对话历史\n\n")
		for _, msg := range instance.History {
			role := formatRole(msg.Role)
			sb.WriteString(fmt.Sprintf("### %s (%s)\n\n", role, msg.Timestamp.Format("15:04:05")))
			sb.WriteString("```\n")
			sb.WriteString(msg.Content + "\n")
			sb.WriteString("```\n\n")
		}
	}

	return sb.String()
}

// generateHTML 生成 HTML 报告
func (g *ReportGenerator) generateHTML(instance *RoutineInstance) string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <title>Harness Engineering Routine 报告</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; background: #f5f5f5; }
        .card { background: white; border-radius: 10px; padding: 20px; margin-bottom: 20px; box-shadow: 0 2px 10px rgba(0,0,0,0.05); }
        h1 { color: #333; border-bottom: 2px solid #667eea; padding-bottom: 10px; }
        h2 { color: #667eea; }
        .stat { display: inline-block; margin: 10px 20px 10px 0; }
        .stat-value { font-size: 2rem; font-weight: bold; color: #667eea; }
        .stat-label { font-size: 0.9rem; color: #666; }
        .bar { background: #eee; border-radius: 10px; height: 20px; width: 200px; display: inline-block; }
        .bar-fill { background: #667eea; border-radius: 10px; height: 100%; }
        .pass { color: #28a745; }
        .fail { color: #dc3545; }
        .message { padding: 10px; margin: 10px 0; border-radius: 5px; }
        .interviewer { background: #e3f2fd; }
        .candidate { background: #f3e5f5; }
        .evaluator { background: #fff3e0; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #eee; }
        th { background: #667eea; color: white; }
    </style>
</head>
<body>
    <h1>Harness Engineering Routine 报告</h1>
`)

	// 基本信息
	sb.WriteString(`    <div class="card">
        <h2>基本信息</h2>
        <div class="stat">
            <div class="stat-value">` + instance.Config.Name + `</div>
            <div class="stat-label">名称</div>
        </div>
        <div class="stat">
            <div class="stat-value">` + string(instance.Config.Type) + `</div>
            <div class="stat-label">类型</div>
        </div>
        <div class="stat">
            <div class="stat-value">` + fmt.Sprintf("%d/%d", instance.Round, instance.Config.Settings.MaxRounds) + `</div>
            <div class="stat-label">轮次</div>
        </div>
    </div>
`)

	// 评分
	if len(instance.Scores) > 0 {
		avg := instance.GetAverageScore()
		sb.WriteString(`    <div class="card">
        <h2>评分统计</h2>
        <div class="stat">
            <div class="stat-value">` + fmt.Sprintf("%.1f", avg.Total) + `</div>
            <div class="stat-label">综合分 / 100</div>
        </div>
        <p>正确性: <span class="bar"><span class="bar-fill" style="width:` + fmt.Sprintf("%d", avg.Correctness*10) + `%"></span></span> ` + fmt.Sprintf("%d/10", avg.Correctness) + `</p>
        <p>深度: <span class="bar"><span class="bar-fill" style="width:` + fmt.Sprintf("%d", avg.Depth*10) + `%"></span></span> ` + fmt.Sprintf("%d/10", avg.Depth) + `</p>
        <p>清晰度: <span class="bar"><span class="bar-fill" style="width:` + fmt.Sprintf("%d", avg.Clarity*10) + `%"></span></span> ` + fmt.Sprintf("%d/10", avg.Clarity) + `</p>
        <p>实用性: <span class="bar"><span class="bar-fill" style="width:` + fmt.Sprintf("%d", avg.Practical*10) + `%"></span></span> ` + fmt.Sprintf("%d/10", avg.Practical) + `</p>
    </div>
`)
	}

	// 最终报告
	if instance.FinalReport != nil {
		report := instance.FinalReport
		resultClass := "pass"
		resultText := "通过 ✓"
		if !report.Pass {
			resultClass = "fail"
			resultText = "未通过 ✗"
		}

		sb.WriteString(`    <div class="card">
        <h2>最终评估</h2>
        <p><strong>技术评级:</strong> ` + report.Level + `</p>
        <p><strong>面试结果:</strong> <span class="` + resultClass + `">` + resultText + `</span></p>
        <p><strong>总分:</strong> ` + fmt.Sprintf("%.1f / %.1f", report.TotalScore, report.MaxScore) + `</p>
    </div>
`)

		if len(report.StrongAreas) > 0 {
			sb.WriteString(`    <div class="card">
        <h2>优势领域</h2>
        <ul>`)
			for _, area := range report.StrongAreas {
				sb.WriteString("\n            <li>✓ " + area + "</li>")
			}
			sb.WriteString(`
        </ul>
    </div>
`)
		}

		if len(report.WeakAreas) > 0 {
			sb.WriteString(`    <div class="card">
        <h2>薄弱环节</h2>
        <ul>`)
			for _, area := range report.WeakAreas {
				sb.WriteString("\n            <li>✗ " + area + "</li>")
			}
			sb.WriteString(`
        </ul>
    </div>
`)
		}
	}

	sb.WriteString(`
</body>
</html>`)

	return sb.String()
}

// generateJSON 生成 JSON 报告
func (g *ReportGenerator) generateJSON(instance *RoutineInstance) string {
	// 简化的 JSON 输出
	return fmt.Sprintf(`{
  "id": "%s",
  "name": "%s",
  "type": "%s",
  "status": "%s",
  "round": %d,
  "max_rounds": %d,
  "start_time": "%s",
  "total_score": %.1f,
  "pass": %v
}`, instance.ID, instance.Config.Name, instance.Config.Type,
		instance.Status, instance.Round, instance.Config.Settings.MaxRounds,
		instance.StartTime.Format(time.RFC3339),
		instance.GetAverageScore().Total,
		instance.FinalReport != nil && instance.FinalReport.Pass)
}

// ============================================================
// 辅助函数
// ============================================================

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d 秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分 %d 秒", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d 时 %d 分", int(d.Hours()), int(d.Minutes())%60)
}

func formatRole(role string) string {
	switch role {
	case "interviewer":
		return "面试官"
	case "candidate":
		return "候选人"
	case "evaluator":
		return "评估官"
	case "system":
		return "系统"
	default:
		return role
	}
}

func renderBar(value, max int) string {
	filled := value * 20 / max
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
	return bar
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func indent(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// Scores 辅助方法

// GetScoreDistribution 获取分数分布
func GetScoreDistribution(scores []RoundScore) map[string]int {
	dist := map[string]int{
		"excellent": 0, // 90-100
		"good":      0, // 70-89
		"average":   0, // 50-69
		"poor":      0, // 0-49
	}

	for _, score := range scores {
		switch {
		case score.Score.Total >= 90:
			dist["excellent"]++
		case score.Score.Total >= 70:
			dist["good"]++
		case score.Score.Total >= 50:
			dist["average"]++
		default:
			dist["poor"]++
		}
	}

	return dist
}

// GetScoreTrend 获取分数趋势
func GetScoreTrend(scores []RoundScore) string {
	if len(scores) < 2 {
		return "insufficient_data"
	}

	first := scores[0].Score.Total
	last := scores[len(scores)-1].Score.Total

	diff := last - first
	if diff > 10 {
		return "improving"
	}
	if diff < -10 {
		return "declining"
	}
	return "stable"
}
