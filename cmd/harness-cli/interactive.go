package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// TaskTemplate 任务模板
type TaskTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Constraints []Constraint      `json:"constraints"`
	Context     map[string]string `json:"context"`
	Examples    []Example         `json:"examples"`
}

// Constraint 约束
type Constraint struct {
	Type     string `json:"type"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Example 示例
type Example struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

// TaskRequest 任务请求
type TaskRequest struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Constraints []Constraint      `json:"constraints"`
	Context     map[string]string `json:"context"`
	Adapter     string            `json:"adapter"`
	Priority    string            `json:"priority"`
}

// InteractiveTaskCreator 交互式任务创建器
type InteractiveTaskCreator struct {
	reader    *bufio.Reader
	templates map[string]TaskTemplate
}

// NewInteractiveTaskCreator 创建交互式任务创建器
func NewInteractiveTaskCreator() *InteractiveTaskCreator {
	creator := &InteractiveTaskCreator{
		reader:    bufio.NewReader(os.Stdin),
		templates: make(map[string]TaskTemplate),
	}
	creator.loadDefaultTemplates()
	return creator
}

// loadDefaultTemplates 加载默认模板
func (c *InteractiveTaskCreator) loadDefaultTemplates() {
	c.templates["implement"] = TaskTemplate{
		ID:          "implement",
		Name:        "代码实现",
		Description: "实现新功能或模块",
		Type:        "implement",
		Constraints: []Constraint{
			{Type: "quality", Rule: "require-tests", Severity: "warning", Message: "需要编写测试"},
			{Type: "quality", Rule: "max-file-length", Severity: "warning", Message: "文件不超过 500 行"},
		},
		Examples: []Example{
			{Input: "实现用户认证功能", Output: "创建 auth.go, auth_test.go"},
		},
	}

	c.templates["review"] = TaskTemplate{
		ID:          "review",
		Name:        "代码审查",
		Description: "审查代码质量和安全性",
		Type:        "review",
		Constraints: []Constraint{
			{Type: "security", Rule: "no-hardcoded-secrets", Severity: "error", Message: "检查硬编码密钥"},
			{Type: "security", Rule: "no-unsafe-imports", Severity: "error", Message: "检查不安全导入"},
		},
		Examples: []Example{
			{Input: "审查 auth.go 的安全性", Output: "安全审查报告"},
		},
	}

	c.templates["test"] = TaskTemplate{
		ID:          "test",
		Name:        "测试生成",
		Description: "生成和运行测试",
		Type:        "test",
		Constraints: []Constraint{
			{Type: "quality", Rule: "min-coverage", Severity: "warning", Message: "测试覆盖率不低于 80%"},
		},
		Examples: []Example{
			{Input: "为 auth.go 生成单元测试", Output: "auth_test.go"},
		},
	}

	c.templates["refactor"] = TaskTemplate{
		ID:          "refactor",
		Name:        "代码重构",
		Description: "重构现有代码",
		Type:        "implement",
		Constraints: []Constraint{
			{Type: "architecture", Rule: "no-circular-imports", Severity: "error", Message: "避免循环导入"},
			{Type: "quality", Rule: "require-tests", Severity: "warning", Message: "保持测试覆盖"},
		},
		Examples: []Example{
			{Input: "重构 database 包，使用接口抽象", Output: "重构后的代码"},
		},
	}

	c.templates["interview"] = TaskTemplate{
		ID:          "interview",
		Name:        "Go 面试",
		Description: "生成 Go 面试题",
		Type:        "interview",
		Context: map[string]string{
			"topic": "Go 语言",
		},
		Examples: []Example{
			{Input: "Go 并发编程面试题", Output: "面试题和答案"},
		},
	}
}

// Create 交互式创建任务
func (c *InteractiveTaskCreator) Create() (*TaskRequest, error) {
	fmt.Println("=== Harness 任务创建向导 ===")
	fmt.Println()

	// 选择模板
	template, err := c.selectTemplate()
	if err != nil {
		return nil, err
	}

	// 输入任务 ID
	id, err := c.inputTaskID()
	if err != nil {
		return nil, err
	}

	// 输入任务描述
	description, err := c.inputDescription(template)
	if err != nil {
		return nil, err
	}

	// 选择适配器
	adapter, err := c.selectAdapter()
	if err != nil {
		return nil, err
	}

	// 选择优先级
	priority, err := c.selectPriority()
	if err != nil {
		return nil, err
	}

	// 自定义约束
	constraints, err := c.customizeConstraints(template)
	if err != nil {
		return nil, err
	}

	// 添加上下文
	context, err := c.addContext(template)
	if err != nil {
		return nil, err
	}

	task := &TaskRequest{
		ID:          id,
		Type:        template.Type,
		Description: description,
		Constraints: constraints,
		Context:     context,
		Adapter:     adapter,
		Priority:    priority,
	}

	return task, nil
}

// selectTemplate 选择模板
func (c *InteractiveTaskCreator) selectTemplate() (*TaskTemplate, error) {
	fmt.Println("可用模板:")
	fmt.Println()

	templates := make([]string, 0, len(c.templates))
	for name, tmpl := range c.templates {
		templates = append(templates, name)
		fmt.Printf("  %s - %s\n", name, tmpl.Description)
	}

	fmt.Println()
	fmt.Print("选择模板 (输入名称，或按回车跳过): ")
	input, _ := c.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		// 默认使用 implement 模板
		tmpl := c.templates["implement"]
		return &tmpl, nil
	}

	if tmpl, exists := c.templates[input]; exists {
		return &tmpl, nil
	}

	return nil, fmt.Errorf("未知模板: %s", input)
}

// inputTaskID 输入任务 ID
func (c *InteractiveTaskCreator) inputTaskID() (string, error) {
	fmt.Print("任务 ID (按回车自动生成): ")
	input, _ := c.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		input = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}

	return input, nil
}

// inputDescription 输入任务描述
func (c *InteractiveTaskCreator) inputDescription(template *TaskTemplate) (string, error) {
	fmt.Printf("任务描述 (示例: %s): ", template.Examples[0].Input)
	input, _ := c.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return "", fmt.Errorf("任务描述不能为空")
	}

	return input, nil
}

// selectAdapter 选择适配器
func (c *InteractiveTaskCreator) selectAdapter() (string, error) {
	fmt.Println()
	fmt.Println("可用适配器:")
	fmt.Println("  claude-code - Claude Code CLI")
	fmt.Println("  hermes - Hermes Agent")
	fmt.Println("  codex-cli - Codex CLI")
	fmt.Println()
	fmt.Print("选择适配器 (默认: claude-code): ")
	input, _ := c.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return "claude-code", nil
	}

	validAdapters := map[string]bool{
		"claude-code": true,
		"hermes":      true,
		"codex-cli":   true,
	}

	if !validAdapters[input] {
		return "", fmt.Errorf("未知适配器: %s", input)
	}

	return input, nil
}

// selectPriority 选择优先级
func (c *InteractiveTaskCreator) selectPriority() (string, error) {
	fmt.Println()
	fmt.Println("优先级:")
	fmt.Println("  low - 低")
	fmt.Println("  medium - 中")
	fmt.Println("  high - 高")
	fmt.Println("  critical - 紧急")
	fmt.Println()
	fmt.Print("选择优先级 (默认: medium): ")
	input, _ := c.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return "medium", nil
	}

	validPriorities := map[string]bool{
		"low":      true,
		"medium":   true,
		"high":     true,
		"critical": true,
	}

	if !validPriorities[input] {
		return "", fmt.Errorf("未知优先级: %s", input)
	}

	return input, nil
}

// customizeConstraints 自定义约束
func (c *InteractiveTaskCreator) customizeConstraints(template *TaskTemplate) ([]Constraint, error) {
	fmt.Println()
	fmt.Println("默认约束:")
	for i, constraint := range template.Constraints {
		fmt.Printf("  %d. [%s] %s: %s\n", i+1, constraint.Severity, constraint.Rule, constraint.Message)
	}

	fmt.Println()
	fmt.Print("是否自定义约束? (y/N): ")
	input, _ := c.reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input != "y" {
		return template.Constraints, nil
	}

	// 允许用户添加新约束
	constraints := make([]Constraint, len(template.Constraints))
	copy(constraints, template.Constraints)

	for {
		fmt.Println()
		fmt.Print("添加约束 (输入 'done' 完成): ")
		input, _ := c.reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "done" {
			break
		}

		fmt.Print("约束类型 (security/quality/architecture): ")
		typeInput, _ := c.reader.ReadString('\n')
		typeInput = strings.TrimSpace(typeInput)

		fmt.Print("约束规则: ")
		ruleInput, _ := c.reader.ReadString('\n')
		ruleInput = strings.TrimSpace(ruleInput)

		fmt.Print("严重程度 (error/warning): ")
		severityInput, _ := c.reader.ReadString('\n')
		severityInput = strings.TrimSpace(severityInput)

		fmt.Print("错误消息: ")
		messageInput, _ := c.reader.ReadString('\n')
		messageInput = strings.TrimSpace(messageInput)

		constraints = append(constraints, Constraint{
			Type:     typeInput,
			Rule:     ruleInput,
			Severity: severityInput,
			Message:  messageInput,
		})
	}

	return constraints, nil
}

// addContext 添加上下文
func (c *InteractiveTaskCreator) addContext(template *TaskTemplate) (map[string]string, error) {
	context := make(map[string]string)

	// 复制模板上下文
	for k, v := range template.Context {
		context[k] = v
	}

	fmt.Println()
	fmt.Print("是否添加上下文信息? (y/N): ")
	input, _ := c.reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input != "y" {
		return context, nil
	}

	for {
		fmt.Println()
		fmt.Print("添加上下文 (输入 'done' 完成): ")
		input, _ := c.reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "done" {
			break
		}

		fmt.Print("键: ")
		key, _ := c.reader.ReadString('\n')
		key = strings.TrimSpace(key)

		fmt.Print("值: ")
		value, _ := c.reader.ReadString('\n')
		value = strings.TrimSpace(value)

		context[key] = value
	}

	return context, nil
}

// FormatTask 格式化任务
func (t *TaskRequest) FormatTask() string {
	data, _ := json.MarshalIndent(t, "", "  ")
	return string(data)
}

// SaveToFile 保存到文件
func (t *TaskRequest) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func RunInteractive() {
	creator := NewInteractiveTaskCreator()

	task, err := creator.Create()
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("=== 任务创建完成 ===")
	fmt.Println()
	fmt.Println(task.FormatTask())

	// 保存到文件
	fmt.Println()
	fmt.Print("保存到文件? (输入文件名，或按回车跳过): ")
	reader := bufio.NewReader(os.Stdin)
	filename, _ := reader.ReadString('\n')
	filename = strings.TrimSpace(filename)

	if filename != "" {
		if err := task.SaveToFile(filename); err != nil {
			fmt.Printf("保存失败: %v\n", err)
		} else {
			fmt.Printf("已保存到 %s\n", filename)
		}
	}
}
