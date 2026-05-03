package collaboration

import (
	"fmt"
	"strings"

	"github.com/harness-engineering/harness/models"
)

// ExportMermaid 将 TaskGraph 导出为 Mermaid DAG 图
func ExportMermaid(graph *models.TaskGraph) string {
	var b strings.Builder

	b.WriteString("graph TD\n")
	b.WriteString(fmt.Sprintf("    title[\"%s — %s\"]\n", graph.ID, graph.Strategy))
	b.WriteString("    style title fill:#1a1a2e,stroke:#e94560,color:#fff\n\n")

	// 状态样式映射
	statusStyle := map[models.SubTaskStatus]string{
		models.SubTaskPending:    "fill:#2d2d2d,stroke:#666,color:#ccc",
		models.SubTaskReady:      "fill:#16213e,stroke:#0f3460,color:#e94560",
		models.SubTaskAssigned:   "fill:#1a1a2e,stroke:#533483,color:#fff",
		models.SubTaskInProgress: "fill:#0f3460,stroke:#e94560,color:#fff",
		models.SubTaskCompleted:  "fill:#1b4332,stroke:#2d6a4f,color:#95d5b2",
		models.SubTaskFailed:     "fill:#590d22,stroke:#a4133c,color:#ff8fa3",
		models.SubTaskSkipped:    "fill:#2d2d2d,stroke:#555,color:#888",
	}

	// 状态图标
	statusIcon := map[models.SubTaskStatus]string{
		models.SubTaskPending:    "⏳",
		models.SubTaskReady:      "🟢",
		models.SubTaskAssigned:   "📋",
		models.SubTaskInProgress: "🔄",
		models.SubTaskCompleted:  "✅",
		models.SubTaskFailed:     "❌",
		models.SubTaskSkipped:    "⏭️",
	}

	// 节点定义
	for id, task := range graph.SubTasks {
		icon := statusIcon[task.Status]
		label := fmt.Sprintf("%s %s", icon, task.Name)
		nodeID := sanitizeID(id)

		b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, label))

		style, ok := statusStyle[task.Status]
		if !ok {
			style = statusStyle[models.SubTaskPending]
		}
		b.WriteString(fmt.Sprintf("    style %s %s\n", nodeID, style))
	}

	b.WriteString("\n")

	// 边（依赖关系）
	for _, task := range graph.SubTasks {
		for _, depID := range task.DependsOn {
			from := sanitizeID(depID)
			to := sanitizeID(task.ID)
			b.WriteString(fmt.Sprintf("    %s --> %s\n", from, to))
		}
	}

	// 根节点标记
	if len(graph.RootIDs) > 0 {
		b.WriteString("\n")
		for _, rootID := range graph.RootIDs {
			b.WriteString(fmt.Sprintf("    %s -.->|start| %s\n", "title", sanitizeID(rootID)))
		}
	}

	return b.String()
}

// ExportMermaidWithDetails 带详细信息的 Mermaid 导出
func ExportMermaidWithDetails(graph *models.TaskGraph) string {
	var b strings.Builder

	b.WriteString("graph TD\n")
	b.WriteString(fmt.Sprintf("    title[\"%s\\nStrategy: %s\\nStatus: %s\"]\n",
		graph.ID, graph.Strategy, graph.Status))
	b.WriteString("    style title fill:#1a1a2e,stroke:#e94560,color:#fff,font-weight:bold\n\n")

	for id, task := range graph.SubTasks {
		nodeID := sanitizeID(id)

		// 构建详细标签
		var details []string
		details = append(details, task.Name)
		details = append(details, fmt.Sprintf("Status: %s", task.Status))
		if task.AssignedTo != "" {
			details = append(details, fmt.Sprintf("Agent: %s", task.AssignedTo))
		}
		if task.Result != nil {
			details = append(details, fmt.Sprintf("Duration: %s", task.Result.Metrics.Duration))
		}
		details = append(details, fmt.Sprintf("Priority: %d", task.Priority))

		label := strings.Join(details, "\\n")
		b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, label))

		// 根据状态着色
		switch task.Status {
		case models.SubTaskCompleted:
			b.WriteString(fmt.Sprintf("    style %s fill:#1b4332,stroke:#2d6a4f,color:#95d5b2\n", nodeID))
		case models.SubTaskFailed:
			b.WriteString(fmt.Sprintf("    style %s fill:#590d22,stroke:#a4133c,color:#ff8fa3\n", nodeID))
		case models.SubTaskInProgress:
			b.WriteString(fmt.Sprintf("    style %s fill:#0f3460,stroke:#e94560,color:#fff\n", nodeID))
		default:
			b.WriteString(fmt.Sprintf("    style %s fill:#2d2d2d,stroke:#666,color:#ccc\n", nodeID))
		}
	}

	// 边
	b.WriteString("\n")
	for _, task := range graph.SubTasks {
		for _, depID := range task.DependsOn {
			b.WriteString(fmt.Sprintf("    %s --> %s\n", sanitizeID(depID), sanitizeID(task.ID)))
		}
	}

	return b.String()
}

// sanitizeID 清理 Mermaid 节点 ID（只允许字母数字下划线）
func sanitizeID(id string) string {
	var result []rune
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result = append(result, r)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}
