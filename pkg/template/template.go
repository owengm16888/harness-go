package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// TaskTemplate 任务模板
type TaskTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Type        string            `json:"type"`
	Language    string            `json:"language"`
	Tags        []string          `json:"tags"`
	Variables   []Variable        `json:"variables"`
	Constraints []Constraint      `json:"constraints"`
	Context     map[string]string `json:"context"`
	Examples    []Example         `json:"examples"`
	Content     string            `json:"content"`
}

// Variable 模板变量
type Variable struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Type         string `json:"type"` // string, int, bool, select
	Required     bool   `json:"required"`
	DefaultValue string `json:"default_value"`
	Options      []string `json:"options"` // 用于 select 类型
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
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Input       map[string]string `json:"input"`
	Output      string            `json:"output"`
}

// TemplateManager 模板管理器
type TemplateManager struct {
	templatesDir string
	templates    map[string]*TaskTemplate
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager(templatesDir string) *TemplateManager {
	return &TemplateManager{
		templatesDir: templatesDir,
		templates:    make(map[string]*TaskTemplate),
	}
}

// Load 加载模板
func (m *TemplateManager) Load() error {
	// 加载内置模板
	m.loadBuiltinTemplates()

	// 加载自定义模板
	if err := m.loadCustomTemplates(); err != nil {
		return fmt.Errorf("failed to load custom templates: %w", err)
	}

	return nil
}

// loadBuiltinTemplates 加载内置模板
func (m *TemplateManager) loadBuiltinTemplates() {
	// Go 实现模板
	m.templates["go-implement"] = &TaskTemplate{
		ID:          "go-implement",
		Name:        "Go 代码实现",
		Description: "实现 Go 语言功能模块",
		Category:    "implementation",
		Type:        "implement",
		Language:    "go",
		Tags:        []string{"go", "implement", "feature"},
		Variables: []Variable{
			{Name: "module_name", Description: "模块名称", Type: "string", Required: true},
			{Name: "package_name", Description: "包名", Type: "string", Required: true, DefaultValue: "main"},
			{Name: "with_tests", Description: "是否生成测试", Type: "bool", Required: false, DefaultValue: "true"},
		},
		Constraints: []Constraint{
			{Type: "quality", Rule: "require-tests", Severity: "warning", Message: "需要编写测试"},
			{Type: "quality", Rule: "max-file-length", Severity: "warning", Message: "文件不超过 500 行"},
			{Type: "architecture", Rule: "no-circular-imports", Severity: "error", Message: "避免循环导入"},
		},
		Content: `实现 {{.module_name}} 模块

要求:
1. 包名: {{.package_name}}
2. 遵循 Go 编码规范
3. 添加适当的注释
{{if .with_tests}}4. 编写单元测试{{end}}

约束:
{{range .constraints}}- [{{.Severity}}] {{.Rule}}: {{.Message}}
{{end}}`,
		Examples: []Example{
			{
				Name:        "用户认证模块",
				Description: "实现用户登录、注册、JWT 认证",
				Input: map[string]string{
					"module_name":  "auth",
					"package_name": "auth",
					"with_tests":   "true",
				},
				Output: "auth/auth.go, auth/auth_test.go",
			},
		},
	}

	// Go Review 模板
	m.templates["go-review"] = &TaskTemplate{
		ID:          "go-review",
		Name:        "Go 代码审查",
		Description: "审查 Go 代码质量",
		Category:    "review",
		Type:        "review",
		Language:    "go",
		Tags:        []string{"go", "review", "quality"},
		Variables: []Variable{
			{Name: "target_path", Description: "审查目标路径", Type: "string", Required: true},
			{Name: "focus", Description: "审查重点", Type: "select", Required: false, Options: []string{"security", "performance", "readability", "all"}},
		},
		Constraints: []Constraint{
			{Type: "security", Rule: "no-hardcoded-secrets", Severity: "error", Message: "检查硬编码密钥"},
			{Type: "security", Rule: "no-unsafe-imports", Severity: "error", Message: "检查不安全导入"},
			{Type: "quality", Rule: "error-handling", Severity: "warning", Message: "检查错误处理"},
		},
		Content: `审查 {{.target_path}} 的代码质量

审查重点: {{.focus}}

检查项:
1. 安全性
2. 性能
3. 可读性
4. 错误处理
5. 测试覆盖`,
	}

	// Go Test 模板
	m.templates["go-test"] = &TaskTemplate{
		ID:          "go-test",
		Name:        "Go 测试生成",
		Description: "生成 Go 单元测试",
		Category:    "testing",
		Type:        "test",
		Language:    "go",
		Tags:        []string{"go", "test", "unit"},
		Variables: []Variable{
			{Name: "source_file", Description: "源文件路径", Type: "string", Required: true},
			{Name: "coverage_target", Description: "覆盖率目标", Type: "int", Required: false, DefaultValue: "80"},
			{Name: "test_type", Description: "测试类型", Type: "select", Required: false, Options: []string{"unit", "integration", "benchmark"}},
		},
		Constraints: []Constraint{
			{Type: "quality", Rule: "min-coverage", Severity: "warning", Message: "测试覆盖率不低于 80%"},
		},
		Content: `为 {{.source_file}} 生成单元测试

测试类型: {{.test_type}}
覆盖率目标: {{.coverage_target}}%

要求:
1. 测试所有公开函数
2. 测试边界条件
3. 测试错误情况
4. 使用表驱动测试`,
	}

	// Go Refactor 模板
	m.templates["go-refactor"] = &TaskTemplate{
		ID:          "go-refactor",
		Name:        "Go 代码重构",
		Description: "重构 Go 代码",
		Category:    "refactoring",
		Type:        "implement",
		Language:    "go",
		Tags:        []string{"go", "refactor", "clean-code"},
		Variables: []Variable{
			{Name: "target_path", Description: "重构目标路径", Type: "string", Required: true},
			{Name: "refactor_type", Description: "重构类型", Type: "select", Required: true, Options: []string{"extract-function", "extract-interface", "simplify", "optimize"}},
			{Name: "preserve_api", Description: "保持 API 兼容", Type: "bool", Required: false, DefaultValue: "true"},
		},
		Constraints: []Constraint{
			{Type: "architecture", Rule: "no-circular-imports", Severity: "error", Message: "避免循环导入"},
			{Type: "quality", Rule: "require-tests", Severity: "warning", Message: "保持测试覆盖"},
		},
		Content: `重构 {{.target_path}}

重构类型: {{.refactor_type}}
保持 API 兼容: {{.preserve_api}}

要求:
1. 提高代码可读性
2. 减少复杂度
3. 保持功能不变
4. 更新相关测试`,
	}

	// Go Interview 模板
	m.templates["go-interview"] = &TaskTemplate{
		ID:          "go-interview",
		Name:        "Go 面试题",
		Description: "生成 Go 面试题",
		Category:    "interview",
		Type:        "interview",
		Language:    "go",
		Tags:        []string{"go", "interview", "practice"},
		Variables: []Variable{
			{Name: "topic", Description: "面试主题", Type: "select", Required: true, Options: []string{"goroutine", "channel", "interface", "error-handling", "memory", "gc", "concurrency", "all"}},
			{Name: "difficulty", Description: "难度", Type: "select", Required: false, Options: []string{"junior", "middle", "senior"}},
			{Name: "count", Description: "题目数量", Type: "int", Required: false, DefaultValue: "5"},
		},
		Content: `生成 Go 面试题

主题: {{.topic}}
难度: {{.difficulty}}
数量: {{.count}}

要求:
1. 覆盖核心知识点
2. 包含代码示例
3. 提供详细解答
4. 解释底层原理`,
	}

	// API 实现模板
	m.templates["api-implement"] = &TaskTemplate{
		ID:          "api-implement",
		Name:        "API 实现",
		Description: "实现 RESTful API",
		Category:    "api",
		Type:        "implement",
		Language:    "go",
		Tags:        []string{"api", "rest", "http"},
		Variables: []Variable{
			{Name: "resource_name", Description: "资源名称", Type: "string", Required: true},
			{Name: "endpoint", Description: "API 端点", Type: "string", Required: true},
			{Name: "methods", Description: "HTTP 方法", Type: "string", Required: false, DefaultValue: "GET,POST,PUT,DELETE"},
			{Name: "auth_required", Description: "需要认证", Type: "bool", Required: false, DefaultValue: "true"},
		},
		Constraints: []Constraint{
			{Type: "security", Rule: "input-validation", Severity: "error", Message: "验证所有输入"},
			{Type: "quality", Rule: "error-handling", Severity: "error", Message: "统一错误处理"},
		},
		Content: `实现 {{.resource_name}} API

端点: {{.endpoint}}
方法: {{.methods}}
需要认证: {{.auth_required}}

要求:
1. RESTful 设计
2. 输入验证
3. 错误处理
4. 日志记录
5. 单元测试`,
	}

	// Database 模板
	m.templates["database-implement"] = &TaskTemplate{
		ID:          "database-implement",
		Name:        "数据库层实现",
		Description: "实现数据库访问层",
		Category:    "database",
		Type:        "implement",
		Language:    "go",
		Tags:        []string{"database", "repository", "orm"},
		Variables: []Variable{
			{Name: "model_name", Description: "模型名称", Type: "string", Required: true},
			{Name: "database_type", Description: "数据库类型", Type: "select", Required: true, Options: []string{"sqlite", "postgres", "mysql"}},
			{Name: "use_orm", Description: "使用 ORM", Type: "bool", Required: false, DefaultValue: "false"},
		},
		Constraints: []Constraint{
			{Type: "security", Rule: "sql-injection", Severity: "error", Message: "防止 SQL 注入"},
			{Type: "quality", Rule: "transaction", Severity: "warning", Message: "使用事务"},
		},
		Content: `实现 {{.model_name}} 数据库层

数据库: {{.database_type}}
使用 ORM: {{.use_orm}}

要求:
1. CRUD 操作
2. 事务支持
3. 错误处理
4. 连接池管理
5. 单元测试`,
	}
}

// loadCustomTemplates 加载自定义模板
func (m *TemplateManager) loadCustomTemplates() error {
	if m.templatesDir == "" {
		return nil
	}

	// 检查目录是否存在
	if _, err := os.Stat(m.templatesDir); os.IsNotExist(err) {
		// 创建目录
		if err := os.MkdirAll(m.templatesDir, 0755); err != nil {
			return fmt.Errorf("failed to create templates directory: %w", err)
		}
		return nil
	}

	// 遍历目录
	entries, err := os.ReadDir(m.templatesDir)
	if err != nil {
		return fmt.Errorf("failed to read templates directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(m.templatesDir, entry.Name())
		if err := m.loadTemplate(path); err != nil {
			fmt.Printf("Warning: failed to load template %s: %v\n", path, err)
		}
	}

	return nil
}

// loadTemplate 加载单个模板
func (m *TemplateManager) loadTemplate(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	var tmpl TaskTemplate
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	m.templates[tmpl.ID] = &tmpl
	return nil
}

// Get 获取模板
func (m *TemplateManager) Get(id string) (*TaskTemplate, error) {
	tmpl, exists := m.templates[id]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return tmpl, nil
}

// List 列出所有模板
func (m *TemplateManager) List() []*TaskTemplate {
	templates := make([]*TaskTemplate, 0, len(m.templates))
	for _, tmpl := range m.templates {
		templates = append(templates, tmpl)
	}
	return templates
}

// ListByCategory 按类别列出模板
func (m *TemplateManager) ListByCategory(category string) []*TaskTemplate {
	templates := make([]*TaskTemplate, 0)
	for _, tmpl := range m.templates {
		if tmpl.Category == category {
			templates = append(templates, tmpl)
		}
	}
	return templates
}

// ListByLanguage 按语言列出模板
func (m *TemplateManager) ListByLanguage(language string) []*TaskTemplate {
	templates := make([]*TaskTemplate, 0)
	for _, tmpl := range m.templates {
		if tmpl.Language == language {
			templates = append(templates, tmpl)
		}
	}
	return templates
}

// Render 渲染模板
func (m *TemplateManager) Render(id string, variables map[string]string) (string, error) {
	tmpl, err := m.Get(id)
	if err != nil {
		return "", err
	}

	// 验证必填变量
	for _, v := range tmpl.Variables {
		if v.Required {
			if _, exists := variables[v.Name]; !exists {
				if v.DefaultValue == "" {
					return "", fmt.Errorf("required variable missing: %s", v.Name)
				}
				variables[v.Name] = v.DefaultValue
			}
		} else if _, exists := variables[v.Name]; !exists {
			variables[v.Name] = v.DefaultValue
		}
	}

	// 渲染模板
	t, err := template.New("task").Parse(tmpl.Content)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var result strings.Builder
	if err := t.Execute(&result, variables); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return result.String(), nil
}

// Save 保存模板
func (m *TemplateManager) Save(tmpl *TaskTemplate) error {
	if m.templatesDir == "" {
		return fmt.Errorf("templates directory not configured")
	}

	// 确保目录存在
	if err := os.MkdirAll(m.templatesDir, 0755); err != nil {
		return fmt.Errorf("failed to create templates directory: %w", err)
	}

	// 序列化模板
	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	// 写入文件
	path := filepath.Join(m.templatesDir, tmpl.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write template: %w", err)
	}

	// 添加到内存
	m.templates[tmpl.ID] = tmpl

	return nil
}

// Delete 删除模板
func (m *TemplateManager) Delete(id string) error {
	// 检查是否为内置模板
	builtinTemplates := map[string]bool{
		"go-implement":       true,
		"go-review":          true,
		"go-test":            true,
		"go-refactor":        true,
		"go-interview":       true,
		"api-implement":      true,
		"database-implement": true,
	}

	if builtinTemplates[id] {
		return fmt.Errorf("cannot delete builtin template: %s", id)
	}

	// 从内存删除
	delete(m.templates, id)

	// 从文件删除
	if m.templatesDir != "" {
		path := filepath.Join(m.templatesDir, id+".json")
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete template file: %w", err)
		}
	}

	return nil
}

// Export 导出模板
func (m *TemplateManager) Export(id string, path string) error {
	tmpl, err := m.Get(id)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Import 导入模板
func (m *TemplateManager) Import(path string) error {
	return m.loadTemplate(path)
}
