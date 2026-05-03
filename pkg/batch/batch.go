package batch

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Task 任务
type Task struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Adapter     string            `json:"adapter"`
	Priority    string            `json:"priority"`
	Status      string            `json:"status"`
	Constraints []Constraint      `json:"constraints"`
	Context     map[string]string `json:"context"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Constraint 约束
type Constraint struct {
	Type     string `json:"type"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// ExportFormat 导出格式
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
	FormatYAML ExportFormat = "yaml"
)

// BatchManager 批量管理器
type BatchManager struct {
	exportDir string
	importDir string
}

// NewBatchManager 创建批量管理器
func NewBatchManager(exportDir, importDir string) *BatchManager {
	return &BatchManager{
		exportDir: exportDir,
		importDir: importDir,
	}
}

// Export 导出任务
func (m *BatchManager) Export(tasks []Task, format ExportFormat, filename string) error {
	// 确保导出目录存在
	if err := os.MkdirAll(m.exportDir, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	// 生成文件路径
	var path string
	if filename == "" {
		timestamp := time.Now().Format("20060102_150405")
		filename = fmt.Sprintf("tasks_%s.%s", timestamp, format)
	}
	path = filepath.Join(m.exportDir, filename)

	// 根据格式导出
	switch format {
	case FormatJSON:
		return m.exportJSON(tasks, path)
	case FormatCSV:
		return m.exportCSV(tasks, path)
	case FormatYAML:
		return m.exportYAML(tasks, path)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// exportJSON 导出为 JSON
func (m *BatchManager) exportJSON(tasks []Task, path string) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// exportCSV 导出为 CSV
func (m *BatchManager) exportCSV(tasks []Task, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	header := []string{"ID", "Type", "Description", "Adapter", "Priority", "Status", "CreatedAt"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// 写入数据
	for _, task := range tasks {
		row := []string{
			task.ID,
			task.Type,
			task.Description,
			task.Adapter,
			task.Priority,
			task.Status,
			task.CreatedAt.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

// exportYAML 导出为 YAML
func (m *BatchManager) exportYAML(tasks []Task, path string) error {
	// 使用 JSON 转换为 YAML 格式
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	// 简单的 JSON 到 YAML 转换
	yamlData := jsonToYAML(string(data))

	return os.WriteFile(path, []byte(yamlData), 0644)
}

// Import 导入任务
func (m *BatchManager) Import(format ExportFormat, filename string) ([]Task, error) {
	// 生成文件路径
	path := filepath.Join(m.importDir, filename)

	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	// 根据格式导入
	switch format {
	case FormatJSON:
		return m.importJSON(path)
	case FormatCSV:
		return m.importCSV(path)
	case FormatYAML:
		return m.importYAML(path)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// importJSON 从 JSON 导入
func (m *BatchManager) importJSON(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tasks: %w", err)
	}

	return tasks, nil
}

// importCSV 从 CSV 导入
func (m *BatchManager) importCSV(path string) ([]Task, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// 读取表头
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// 创建列索引映射
	columnIndex := make(map[string]int)
	for i, col := range header {
		columnIndex[col] = i
	}

	// 读取数据
	var tasks []Task
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}

		task := Task{
			ID:          getColumn(row, columnIndex, "ID"),
			Type:        getColumn(row, columnIndex, "Type"),
			Description: getColumn(row, columnIndex, "Description"),
			Adapter:     getColumn(row, columnIndex, "Adapter"),
			Priority:    getColumn(row, columnIndex, "Priority"),
			Status:      getColumn(row, columnIndex, "Status"),
		}

		// 解析时间
		createdAtStr := getColumn(row, columnIndex, "CreatedAt")
		if createdAtStr != "" {
			task.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// importYAML 从 YAML 导入
func (m *BatchManager) importYAML(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 简单的 YAML 到 JSON 转换
	jsonData := yamlToJSON(string(data))

	var tasks []Task
	if err := json.Unmarshal([]byte(jsonData), &tasks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tasks: %w", err)
	}

	return tasks, nil
}

// ImportFromURL 从 URL 导入
func (m *BatchManager) ImportFromURL(url string) ([]Task, error) {
	// TODO: 实现从 URL 导入
	return nil, fmt.Errorf("not implemented")
}

// ExportToURL 导出到 URL
func (m *BatchManager) ExportToURL(tasks []Task, url string) error {
	// TODO: 实现导出到 URL
	return fmt.Errorf("not implemented")
}

// ValidateTasks 验证任务
func (m *BatchManager) ValidateTasks(tasks []Task) []ValidationError {
	var errors []ValidationError

	for i, task := range tasks {
		if task.ID == "" {
			errors = append(errors, ValidationError{
				Index:   i,
				Field:   "ID",
				Message: "ID is required",
			})
		}

		if task.Type == "" {
			errors = append(errors, ValidationError{
				Index:   i,
				Field:   "Type",
				Message: "Type is required",
			})
		}

		if task.Description == "" {
			errors = append(errors, ValidationError{
				Index:   i,
				Field:   "Description",
				Message: "Description is required",
			})
		}
	}

	return errors
}

// ValidationError 验证错误
type ValidationError struct {
	Index   int    `json:"index"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error 实现 error 接口
func (e ValidationError) Error() string {
	return fmt.Sprintf("task %d: %s: %s", e.Index, e.Field, e.Message)
}

// MergeTasks 合并任务
func (m *BatchManager) MergeTasks(existing []Task, imported []Task, strategy MergeStrategy) []Task {
	switch strategy {
	case MergeStrategyReplace:
		return imported
	case MergeStrategyAppend:
		return append(existing, imported...)
	case MergeStrategyUpdate:
		// 创建索引
		index := make(map[string]int)
		for i, task := range existing {
			index[task.ID] = i
		}

		// 更新或添加
		for _, task := range imported {
			if i, exists := index[task.ID]; exists {
				existing[i] = task
			} else {
				existing = append(existing, task)
			}
		}

		return existing
	default:
		return existing
	}
}

// MergeStrategy 合并策略
type MergeStrategy string

const (
	MergeStrategyReplace MergeStrategy = "replace"
	MergeStrategyAppend  MergeStrategy = "append"
	MergeStrategyUpdate  MergeStrategy = "update"
)

// FilterTasks 过滤任务
func (m *BatchManager) FilterTasks(tasks []Task, filter TaskFilter) []Task {
	var filtered []Task

	for _, task := range tasks {
		if m.matchesFilter(task, filter) {
			filtered = append(filtered, task)
		}
	}

	return filtered
}

// TaskFilter 任务过滤器
type TaskFilter struct {
	Type     string `json:"type"`
	Adapter  string `json:"adapter"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

// matchesFilter 检查是否匹配过滤器
func (m *BatchManager) matchesFilter(task Task, filter TaskFilter) bool {
	if filter.Type != "" && task.Type != filter.Type {
		return false
	}

	if filter.Adapter != "" && task.Adapter != filter.Adapter {
		return false
	}

	if filter.Priority != "" && task.Priority != filter.Priority {
		return false
	}

	if filter.Status != "" && task.Status != filter.Status {
		return false
	}

	return true
}

// SortTasks 排序任务
func (m *BatchManager) SortTasks(tasks []Task, sortBy SortBy, order SortOrder) []Task {
	// 使用简单的排序算法
	for i := 0; i < len(tasks); i++ {
		for j := i + 1; j < len(tasks); j++ {
			if m.shouldSwap(tasks[i], tasks[j], sortBy, order) {
				tasks[i], tasks[j] = tasks[j], tasks[i]
			}
		}
	}

	return tasks
}

// SortBy 排序字段
type SortBy string

const (
	SortByID        SortBy = "id"
	SortByType      SortBy = "type"
	SortByPriority  SortBy = "priority"
	SortByStatus    SortBy = "status"
	SortByCreatedAt SortBy = "created_at"
)

// SortOrder 排序顺序
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// shouldSwap 检查是否应该交换
func (m *BatchManager) shouldSwap(a, b Task, sortBy SortBy, order SortOrder) bool {
	var less bool

	switch sortBy {
	case SortByID:
		less = a.ID < b.ID
	case SortByType:
		less = a.Type < b.Type
	case SortByPriority:
		less = priorityValue(a.Priority) < priorityValue(b.Priority)
	case SortByStatus:
		less = a.Status < b.Status
	case SortByCreatedAt:
		less = a.CreatedAt.Before(b.CreatedAt)
	default:
		return false
	}

	if order == SortDesc {
		return !less
	}

	return less
}

// priorityValue 获取优先级值
func priorityValue(priority string) int {
	switch priority {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// 辅助函数

// getColumn 获取列值
func getColumn(row []string, columnIndex map[string]int, column string) string {
	if idx, exists := columnIndex[column]; exists && idx < len(row) {
		return row[idx]
	}
	return ""
}

// jsonToYAML 简单的 JSON 到 YAML 转换
func jsonToYAML(jsonStr string) string {
	// 这是一个简化的实现
	// 实际项目中应使用 yaml.v3 库
	return jsonStr
}

// yamlToJSON 简单的 YAML 到 JSON 转换
func yamlToJSON(yamlStr string) string {
	// 这是一个简化的实现
	// 实际项目中应使用 yaml.v3 库
	return yamlStr
}
