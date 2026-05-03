package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/internal/adapters"
	"github.com/harness-engineering/harness/models"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// ============================================================
	// 1. 创建适配器
	// ============================================================

	claudeAdapter := adapters.NewClaudeCodeAdapter2()
	codexAdapter := adapters.NewCodexCLIAdapter2()

	// ============================================================
	// 2. 初始化配置
	// ============================================================

	cfg := config.AdapterConfig{
		RootDir:    ".",
		HooksPath:  ".claude/hooks.json",
		PlansPath:  "Plans.md",
		AgentsPath: "AGENTS.md",
	}

	if err := claudeAdapter.Initialize(ctx, cfg); err != nil {
		log.Printf("Claude Code 初始化失败: %v", err)
	}
	if err := codexAdapter.Initialize(ctx, cfg); err != nil {
		log.Printf("Codex CLI 初始化失败: %v", err)
	}

	// ============================================================
	// 3. 构建任务 (带 prompt + context + constraints)
	// ============================================================

	task := models.Task{
		ID:       "implement-cache-middleware-001",
		Type:     "implement",
		Priority: 1,
		Deadline: timePtr(time.Now().Add(10 * time.Minute)),

		// 核心 prompt
		Description: `
实现一个 Go 语言的 HTTP 缓存中间件，要求:
1. 支持 GET 请求的响应缓存
2. 使用 LRU 策略管理缓存大小
3. 支持 Cache-Control 头解析
4. 提供缓存命中率统计接口
`,

		// 上下文信息 (会注入到 prompt 中)
		Context: map[string]any{
			// 通用上下文
			"language":    "Go",
			"framework":   "gin",
			"go_version":  "1.21",
			"project":     "my-api-server",

			// Claude Code 专用
			"model":                 "claude-sonnet-4-20250514",
			"system_prompt":         "你是一个资深 Go 后端开发者，擅长设计高性能中间件",
			"append_system_prompt":  "请遵循项目的代码风格，使用 table-driven tests",

			// Codex 专用
			"approval_mode": "full-auto",

			// 代码上下文
			"existing_code": `
package main

import "github.com/gin-gonic/gin"

func main() {
    r := gin.Default()
    r.GET("/api/users", getUsers)
    r.Run(":8080")
}
`,
			"project_structure": `
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── middleware/
│   │   └── logger.go
│   └── handler/
│       └── user.go
├── go.mod
└── go.sum
`,
		},

		// 约束条件
		Constraints: []models.Constraint{
			{
				Type:     "quality",
				Rule:     "require-tests",
				Severity: models.SeverityError,
				Message:  "必须编写单元测试，覆盖率 > 80%",
			},
			{
				Type:     "quality",
				Rule:     "max-file-length",
				Severity: models.SeverityWarning,
				Message:  "单个文件不超过 500 行",
			},
			{
				Type:     "security",
				Rule:     "no-hardcoded-secrets",
				Severity: models.SeverityCritical,
				Message:  "禁止硬编码密钥或敏感信息",
			},
			{
				Type:     "architecture",
				Rule:     "no-circular-imports",
				Severity: models.SeverityError,
				Message:  "禁止循环导入",
			},
		},
	}

	// ============================================================
	// 4. 调用 Claude Code
	// ============================================================

	fmt.Println("=" + repeat("=", 60))
	fmt.Println("调用 Claude Code")
	fmt.Println("=" + repeat("=", 60))

	claudeResult, err := claudeAdapter.ExecuteTask(ctx, task)
	if err != nil {
		log.Printf("Claude Code 执行失败: %v", err)
	} else {
		printResult("Claude Code", claudeResult)
	}

	// ============================================================
	// 5. 调用 Codex CLI
	// ============================================================

	fmt.Println("\n" + "=" + repeat("=", 60))
	fmt.Println("调用 Codex CLI")
	fmt.Println("=" + repeat("=", 60))

	codexResult, err := codexAdapter.ExecuteTask(ctx, task)
	if err != nil {
		log.Printf("Codex CLI 执行失败: %v", err)
	} else {
		printResult("Codex CLI", codexResult)
	}

	// ============================================================
	// 6. 对比结果
	// ============================================================

	fmt.Println("\n" + "=" + repeat("=", 60))
	fmt.Println("结果对比")
	fmt.Println("=" + repeat("=", 60))

	fmt.Printf("%-20s %-15s %-15s %-10s\n", "Adapter", "Status", "Duration", "Tokens")
	fmt.Println(repeat("-", 60))
	fmt.Printf("%-20s %-15s %-15s %-10d\n",
		"Claude Code",
		claudeResult.Status,
		claudeResult.Metrics.Duration.Round(time.Millisecond),
		claudeResult.Metrics.TokenCount,
	)
	fmt.Printf("%-20s %-15s %-15s %-10d\n",
		"Codex CLI",
		codexResult.Status,
		codexResult.Metrics.Duration.Round(time.Millisecond),
		codexResult.Metrics.TokenCount,
	)
}

// printResult 打印结果
func printResult(name string, result models.Result) {
	fmt.Printf("\n[%s] Task ID: %s\n", name, result.TaskID)
	fmt.Printf("[%s] Status: %s\n", name, result.Status)
	fmt.Printf("[%s] Duration: %v\n", name, result.Metrics.Duration.Round(time.Millisecond))

	if result.Output != nil {
		output, _ := json.MarshalIndent(result.Output, "  ", "  ")
		fmt.Printf("[%s] Output:\n  %s\n", name, string(output))
	}

	if len(result.Evidence) > 0 {
		fmt.Printf("[%s] Evidence:\n", name)
		for _, e := range result.Evidence {
			fmt.Printf("  - Type: %s, Source: %s, Verified: %v\n",
				e.Type, e.Source, e.Verified)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Printf("[%s] Errors:\n", name)
		for _, e := range result.Errors {
			fmt.Printf("  - [%s] %s\n", e.Code, e.Message)
		}
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
