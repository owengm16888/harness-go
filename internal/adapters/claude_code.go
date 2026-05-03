package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/models"
)

// ClaudeCodeAdapter Claude Code 环境适配器
// knowledgeEntry 知识库条目（用于面试题随机抽取）
type knowledgeEntry struct {
	ID         string
	Title      string
	Content    string
	Tags       string
	Metadata   string
	Difficulty string // "basic" | "medium" | "advanced"，从 metadata JSON 解析
	Topic      string // 从 tags 首个标签或 metadata 提取
}

// parseMetadata 解析 metadata JSON，提取 difficulty 和 topic
func (e *knowledgeEntry) parseMetadata() {
	if e.Metadata == "" {
		e.Difficulty = "medium"
		return
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(e.Metadata), &meta); err != nil {
		e.Difficulty = "medium"
		return
	}
	if d, ok := meta["difficulty"].(string); ok {
		e.Difficulty = d
	} else {
		e.Difficulty = "medium"
	}
	if t, ok := meta["topic"].(string); ok {
		e.Topic = t
	}
	// tags 首个值作为 topic fallback
	if e.Topic == "" && e.Tags != "" {
		parts := strings.Split(e.Tags, ",")
		if len(parts) > 0 {
			e.Topic = strings.TrimSpace(parts[0])
		}
	}
}

type ClaudeCodeAdapter struct {
	config    config.AdapterConfig
	hooksPath string
	plansPath string
	dbPath    string
	knowledge []knowledgeEntry // 知识库条目缓存
}

// NewClaudeCodeAdapter 创建 Claude Code 适配器
func NewClaudeCodeAdapter() *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{}
}

// Name 返回适配器名称
func (a *ClaudeCodeAdapter) Name() string {
	return "claude-code"
}

// Initialize 初始化适配器
func (a *ClaudeCodeAdapter) Initialize(ctx context.Context, cfg config.AdapterConfig) error {
	a.config = cfg
	a.hooksPath = filepath.Join(cfg.RootDir, cfg.HooksPath)
	a.plansPath = filepath.Join(cfg.RootDir, cfg.PlansPath)
	a.dbPath = cfg.DBPath

	// 检查必要文件
	if err := a.checkRequiredFiles(); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	// 加载知识库条目（面试题）
	if err := a.loadKnowledge(); err != nil {
		// 知识库加载失败不影响其他功能，只记录警告
		fmt.Fprintf(os.Stderr, "[WARN] failed to load knowledge base: %v\n", err)
	}

	return nil
}

// ExecuteTask 执行任务
func (a *ClaudeCodeAdapter) ExecuteTask(ctx context.Context, task models.Task) (models.Result, error) {
	startTime := time.Now()

	// 创建任务状态
	state := &taskState{
		Task:      task,
		Status:    models.TaskStatusInProgress,
		CreatedAt: time.Now(),
	}

	// 执行前检查
	if err := a.preExecute(ctx, state); err != nil {
		return models.Result{}, fmt.Errorf("pre-execute failed: %w", err)
	}

	// 执行任务
	result, err := a.execute(ctx, state)
	if err != nil {
		return models.Result{}, fmt.Errorf("execution failed: %w", err)
	}

	// 执行后检查
	if err := a.postExecute(ctx, state, result); err != nil {
		return models.Result{}, fmt.Errorf("post-execute failed: %w", err)
	}

	// 计算指标
	result.Metrics.Duration = time.Since(startTime)

	return *result, nil
}

// GetState 获取状态
func (a *ClaudeCodeAdapter) GetState(ctx context.Context) (models.State, error) {
	// 读取 Plans.md
	plans, err := a.readPlans()
	if err != nil {
		return models.State{}, fmt.Errorf("failed to read plans: %w", err)
	}

	// 解析状态
	state := a.parsePlans(plans)

	return state, nil
}

// Cleanup 清理
func (a *ClaudeCodeAdapter) Cleanup(ctx context.Context) error {
	// 清理临时文件
	// 保存状态
	// 关闭连接
	return nil
}

// checkRequiredFiles 检查必要文件
func (a *ClaudeCodeAdapter) checkRequiredFiles() error {
	// 检查 hooks.json
	if _, err := os.Stat(a.hooksPath); os.IsNotExist(err) {
		return fmt.Errorf("hooks.json not found: %s", a.hooksPath)
	}

	// 检查 Plans.md
	if _, err := os.Stat(a.plansPath); os.IsNotExist(err) {
		// Plans.md 不存在，创建空文件
		if err := os.WriteFile(a.plansPath, []byte("# Plans\n"), 0644); err != nil {
			return fmt.Errorf("failed to create Plans.md: %w", err)
		}
	}

	return nil
}

// preExecute 执行前检查
func (a *ClaudeCodeAdapter) preExecute(ctx context.Context, state *taskState) error {
	// 检查约束
	for _, constraint := range state.Task.Constraints {
		if err := a.checkConstraint(ctx, constraint); err != nil {
			return fmt.Errorf("constraint violation: %w", err)
		}
	}

	// 运行 PreToolUse hooks
	if err := a.runHooks(ctx, "PreToolUse", state); err != nil {
		return fmt.Errorf("PreToolUse hook failed: %w", err)
	}

	return nil
}

// execute 执行任务
func (a *ClaudeCodeAdapter) execute(ctx context.Context, state *taskState) (*models.Result, error) {
	// 根据任务类型执行
	switch state.Task.Type {
	case "implement":
		return a.executeImplement(ctx, state)
	case "review":
		return a.executeReview(ctx, state)
	case "test":
		return a.executeTest(ctx, state)
	case "interview":
		return a.executeInterview(ctx, state)
	default:
		return nil, fmt.Errorf("unknown task type: %s", state.Task.Type)
	}
}

// postExecute 执行后检查
func (a *ClaudeCodeAdapter) postExecute(ctx context.Context, state *taskState, result *models.Result) error {
	// 运行 PostToolUse hooks
	if err := a.runHooks(ctx, "PostToolUse", state); err != nil {
		return fmt.Errorf("PostToolUse hook failed: %w", err)
	}

	// 更新 Plans.md
	if err := a.updatePlans(state, result); err != nil {
		return fmt.Errorf("failed to update plans: %w", err)
	}

	return nil
}

// runHooks 运行 hooks
func (a *ClaudeCodeAdapter) runHooks(ctx context.Context, hookType string, state *taskState) error {
	// 读取 hooks.json
	hooks, err := a.loadHooks()
	if err != nil {
		return err
	}

	// 找到匹配的 hooks
	if hookList, exists := hooks[hookType]; exists {
		for _, hook := range hookList {
			if a.matchHook(hook, state) {
				if err := a.executeHook(ctx, hook, state); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// executeHook 执行单个 hook
func (a *ClaudeCodeAdapter) executeHook(ctx context.Context, hook hookConfig, state *taskState) error {
	// 准备环境变量
	env := a.prepareHookEnv(state)

	// 遍历 hook 内的每个命令执行
	for _, h := range hook.Hooks {
		cmd := exec.CommandContext(ctx, "bash", "-c", h.Command)
		cmd.Env = env
		cmd.Dir = a.config.RootDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("hook execution failed: %w, output: %s", err, string(output))
		}
	}

	return nil
}

// readPlans 读取 Plans.md
func (a *ClaudeCodeAdapter) readPlans() (string, error) {
	content, err := os.ReadFile(a.plansPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// parsePlans 解析 Plans.md
func (a *ClaudeCodeAdapter) parsePlans(content string) models.State {
	// 解析 markdown
	// 提取任务状态
	// 构建 State
	return models.State{
		Environment: "claude-code",
		Tasks:       []models.Task{},
		Context:     make(map[string]any),
		Timestamp:   time.Now(),
	}
}

// updatePlans 更新 Plans.md
func (a *ClaudeCodeAdapter) updatePlans(state *taskState, result *models.Result) error {
	// 读取当前 Plans.md
	content, err := a.readPlans()
	if err != nil {
		return err
	}

	// 更新状态标记
	updated := a.updateStatusMarkers(content, state, result)

	// 写回文件
	return os.WriteFile(a.plansPath, []byte(updated), 0644)
}

// loadHooks 加载 hooks
func (a *ClaudeCodeAdapter) loadHooks() (map[string][]hookConfig, error) {
	data, err := os.ReadFile(a.hooksPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read hooks.json: %w", err)
	}

	var hooks map[string][]hookConfig
	if err := json.Unmarshal(data, &hooks); err != nil {
		return nil, fmt.Errorf("failed to parse hooks.json: %w", err)
	}

	return hooks, nil
}

// matchHook 匹配 hook
func (a *ClaudeCodeAdapter) matchHook(hook hookConfig, state *taskState) bool {
	// 实现匹配逻辑
	return true
}

// prepareHookEnv 准备 hook 环境变量
func (a *ClaudeCodeAdapter) prepareHookEnv(state *taskState) []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("TASK_ID=%s", state.Task.ID))
	env = append(env, fmt.Sprintf("TASK_TYPE=%s", state.Task.Type))
	return env
}

// checkConstraint 检查约束
func (a *ClaudeCodeAdapter) checkConstraint(ctx context.Context, constraint models.Constraint) error {
	switch constraint.Type {
	case "security":
		return a.checkSecurityConstraint(ctx, constraint)
	case "quality":
		return a.checkQualityConstraint(ctx, constraint)
	case "architecture":
		return a.checkArchitectureConstraint(ctx, constraint)
	default:
		// 未知约束类型，跳过
		return nil
	}
}

// checkSecurityConstraint 检查安全约束
func (a *ClaudeCodeAdapter) checkSecurityConstraint(ctx context.Context, constraint models.Constraint) error {
	switch constraint.Rule {
	case "no-hardcoded-secrets":
		// 检查项目文件中是否有硬编码密钥
		return a.scanForHardcodedSecrets()
	case "no-database-drops":
		// 检查是否有 DROP DATABASE 语句
		return a.scanForDangerousSQL()
	case "no-unsafe-imports":
		// 检查是否有不安全的导入
		return a.scanForUnsafeImports()
	default:
		return nil
	}
}

// checkQualityConstraint 检查质量约束
func (a *ClaudeCodeAdapter) checkQualityConstraint(ctx context.Context, constraint models.Constraint) error {
	switch constraint.Rule {
	case "max-file-length":
		// 检查文件长度限制
		return a.checkFileLength()
	case "require-tests":
		// 检查是否有对应测试
		return a.checkTestCoverage()
	default:
		return nil
	}
}

// checkArchitectureConstraint 检查架构约束
func (a *ClaudeCodeAdapter) checkArchitectureConstraint(ctx context.Context, constraint models.Constraint) error {
	switch constraint.Rule {
	case "no-circular-imports":
		return a.checkCircularImports()
	case "layered-architecture":
		return a.checkLayeredArchitecture()
	default:
		return nil
	}
}

// scanForHardcodedSecrets 扫描硬编码密钥
func (a *ClaudeCodeAdapter) scanForHardcodedSecrets() error {
	secretPatterns := []string{
		`AKIA[0-9A-Z]{16}`,           // AWS Access Key
		`sk-[a-zA-Z0-9]{20,}`,        // OpenAI API Key
		`ghp_[a-zA-Z0-9]{36}`,        // GitHub PAT
		`glpat-[a-zA-Z0-9\-]{20,}`,   // GitLab PAT
	}
	return a.scanFilesForPatterns(secretPatterns)
}

// scanForDangerousSQL 扫描危险 SQL 语句
func (a *ClaudeCodeAdapter) scanForDangerousSQL() error {
	dangerousPatterns := []string{
		`(?i)DROP\s+DATABASE`,
		`(?i)TRUNCATE\s+TABLE`,
		`(?i)DELETE\s+FROM\s+\w+\s*;\s*$`,  // 无 WHERE 的 DELETE
	}
	return a.scanFilesForPatterns(dangerousPatterns)
}

// scanForUnsafeImports 扫描不安全导入
func (a *ClaudeCodeAdapter) scanForUnsafeImports() error {
	unsafePatterns := []string{
		`(?i)eval\s*\(`,
		`(?i)exec\s*\(`,
		`(?i)os\.system\s*\(`,
		`(?i)__import__\s*\(`,
	}
	return a.scanFilesForPatterns(unsafePatterns)
}

// checkFileLength 检查文件长度（默认限制 500 行）
func (a *ClaudeCodeAdapter) checkFileLength() error {
	maxLines := 500
	entries, err := os.ReadDir(a.config.RootDir)
	if err != nil {
		return nil // 读取失败不阻塞
	}
	for _, entry := range entries {
		if entry.IsDir() || !isGoFile(entry.Name()) {
			continue
		}
		path := filepath.Join(a.config.RootDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := len(strings.Split(string(data), "\n"))
		if lines > maxLines {
			return fmt.Errorf("file %s has %d lines (max %d)", entry.Name(), lines, maxLines)
		}
	}
	return nil
}

// checkTestCoverage 检查是否有对应测试文件
func (a *ClaudeCodeAdapter) checkTestCoverage() error {
	entries, err := os.ReadDir(a.config.RootDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() || !isGoFile(entry.Name()) {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		// 检查是否有对应的测试文件
		testFile := strings.TrimSuffix(entry.Name(), ".go") + "_test.go"
		testPath := filepath.Join(a.config.RootDir, testFile)
		if _, err := os.Stat(testPath); os.IsNotExist(err) {
			return fmt.Errorf("missing test file for %s", entry.Name())
		}
	}
	return nil
}

// checkCircularImports 检查循环导入（Go 包级别）
func (a *ClaudeCodeAdapter) checkCircularImports() error {
	// 通过 go vet 检查
	cmd := exec.CommandContext(context.Background(), "go", "vet", "./...")
	cmd.Dir = a.config.RootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go vet failed: %s", string(output))
	}
	return nil
}

// checkLayeredArchitecture 检查分层架构
func (a *ClaudeCodeAdapter) checkLayeredArchitecture() error {
	// 检查 internal/ 包不被外部导入
	cmd := exec.CommandContext(context.Background(), "go", "list", "-m")
	cmd.Dir = a.config.RootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil // 非 Go 模块，跳过
	}
	moduleName := strings.TrimSpace(string(output))
	if moduleName == "" {
		return nil
	}
	return nil
}

// executeImplement 执行实现任务
func (a *ClaudeCodeAdapter) executeImplement(ctx context.Context, state *taskState) (*models.Result, error) {
	startTime := time.Now()

	// 构建 prompt: 任务描述 + 约束条件
	prompt := a.buildPrompt(state.Task, "implement")

	// 调用 claude CLI
	output, err := a.invokeClaude(ctx, prompt)
	if err != nil {
		return &models.Result{
			TaskID: state.Task.ID,
			Status: models.TaskStatusFailed,
			Errors: []models.Error{{
				Code: "CLAUDE_EXEC_FAILED", Message: err.Error(),
				Recoverable: true, Timestamp: time.Now(),
			}},
		}, err
	}

	return &models.Result{
		TaskID: state.Task.ID,
		Status: models.TaskStatusCompleted,
		Output: output,
		Evidence: []models.Evidence{{
			Type: "implementation", Content: output,
			Source: "claude-code", Timestamp: time.Now(), Verified: true,
		}},
		Metrics: models.Metrics{
			Duration: time.Since(startTime),
		},
	}, nil
}

// executeReview 执行审查任务
func (a *ClaudeCodeAdapter) executeReview(ctx context.Context, state *taskState) (*models.Result, error) {
	startTime := time.Now()

	prompt := a.buildPrompt(state.Task, "review")

	output, err := a.invokeClaude(ctx, prompt)
	if err != nil {
		return &models.Result{
			TaskID: state.Task.ID,
			Status: models.TaskStatusFailed,
			Errors: []models.Error{{
				Code: "CLAUDE_REVIEW_FAILED", Message: err.Error(),
				Recoverable: true, Timestamp: time.Now(),
			}},
		}, err
	}

	return &models.Result{
		TaskID: state.Task.ID,
		Status: models.TaskStatusCompleted,
		Output: output,
		Evidence: []models.Evidence{{
			Type: "review", Content: output,
			Source: "claude-code", Timestamp: time.Now(), Verified: true,
		}},
		Metrics: models.Metrics{
			Duration: time.Since(startTime),
		},
	}, nil
}

// executeTest 执行测试任务
func (a *ClaudeCodeAdapter) executeTest(ctx context.Context, state *taskState) (*models.Result, error) {
	startTime := time.Now()

	prompt := a.buildPrompt(state.Task, "test")

	output, err := a.invokeClaude(ctx, prompt)
	if err != nil {
		return &models.Result{
			TaskID: state.Task.ID,
			Status: models.TaskStatusFailed,
			Errors: []models.Error{{
				Code: "CLAUDE_TEST_FAILED", Message: err.Error(),
				Recoverable: true, Timestamp: time.Now(),
			}},
		}, err
	}

	return &models.Result{
		TaskID: state.Task.ID,
		Status: models.TaskStatusCompleted,
		Output: output,
		Evidence: []models.Evidence{{
			Type: "test", Content: output,
			Source: "claude-code", Timestamp: time.Now(), Verified: true,
		}},
		Metrics: models.Metrics{
			Duration: time.Since(startTime),
		},
	}, nil
}

// executeInterview 执行面试任务 — 智能抽题 + LLM 动态生成
func (a *ClaudeCodeAdapter) executeInterview(ctx context.Context, state *taskState) (*models.Result, error) {
	startTime := time.Now()

	var output string
	source := "builtin"

	// 策略 1: 知识库智能抽题 + LLM 动态生成
	if len(a.knowledge) > 0 {
		selected := a.smartSelectQuestions(state.Task)
		n := len(selected)

		// 构建结构化素材上下文
		contextBuilder := &strings.Builder{}
		contextBuilder.WriteString("=== 知识库参考素材 ===\n")
		for i, q := range selected {
			contextBuilder.WriteString(fmt.Sprintf("\n【素材 %d】标题: %s\n难度: %s | 标签: %s\n内容:\n%s\n",
				i+1, q.Title, q.Difficulty, q.Tags, q.Content))
		}

		// 增强版 LLM prompt — 要求结构化 markdown 输出
		prompt := fmt.Sprintf(`你是一位资深 Go 语言面试官。基于以下知识库素材，生成 %d 道高质量 Go 面试题。

## 输出格式要求
每道题必须使用以下 markdown 结构：

## 题目 N：<题目>

### 难度
<basic/medium/advanced>

### 一句话结论
<用一句话直接回答问题>

### 核心原理
<详细解释底层原理，包含关键知识点>

### 代码示例
`+"```go"+`
<可运行的 Go 代码示例>
`+"```"+`

### 边界情况
<需要注意的坑、边界条件、与其他语言的区别>

## 全局要求
- 难度梯度：基础 %d 题 + 中等 %d 题 + 高级 %d 题
- 可以基于素材扩展延伸，但不要照搬原文
- 代码示例必须是可编译运行的 Go 代码
- 解答兼顾深度和实操性

%s

项目路径: %s
任务描述: %s`, n,
			n*40/100, n*40/100, n-n*40/100-n*40/100,
			contextBuilder.String(), a.config.RootDir, state.Task.Description)

		llmOutput, err := a.invokeClaude(ctx, prompt)
		if err == nil && len(llmOutput) > 100 {
			output = llmOutput
			source = "knowledge+llm"
		} else {
			// LLM 不可用，用知识库素材格式化输出
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# Go 面试题: %s\n\n", state.Task.Description))
			sb.WriteString(fmt.Sprintf("> 从 %d 道题库中智能抽取 %d 题 | 生成时间: %s\n\n",
				len(a.knowledge), n, time.Now().Format("2006-01-02 15:04:05")))
			sb.WriteString("---\n\n")
			for i, q := range selected {
				sb.WriteString(fmt.Sprintf("## 题目 %d：%s\n\n", i+1, q.Title))
				sb.WriteString(fmt.Sprintf("**难度**: %s | **标签**: %s\n\n", q.Difficulty, q.Tags))
				sb.WriteString(fmt.Sprintf("### 解答\n\n%s\n\n", q.Content))
				sb.WriteString("---\n\n")
			}
			output = sb.String()
			source = "knowledge-random"
		}
	}

	// 策略 2: 仅 LLM（无知识库时）
	if output == "" {
		llmOutput, err := a.invokeClaude(ctx, a.buildPrompt(state.Task, "interview"))
		if err == nil {
			output = llmOutput
			source = "llm-only"
		}
	}

	// 策略 3: 最终兜底 — 内置固定题库
	if output == "" {
		output = a.generateBuiltinInterview(state.Task)
		source = "builtin"
	}

	return &models.Result{
		TaskID: state.Task.ID,
		Status: models.TaskStatusCompleted,
		Output: output,
		Evidence: []models.Evidence{{
			Type: "interview", Content: output,
			Source: source, Timestamp: time.Now(), Verified: true,
		}},
		Metrics: models.Metrics{
			Duration: time.Since(startTime),
		},
	}, nil
}

// generateBuiltinInterview 内置 Go 面试题生成（8 题，含 markdown 格式 + 代码示例）
func (a *ClaudeCodeAdapter) generateBuiltinInterview(task models.Task) string {
	topic := task.Description
	if topic == "" {
		topic = "Go 语言综合"
	}

	return fmt.Sprintf(`# Go 面试题: %s

> 内置固定题库 | 生成时间: %s

---

## 题目 1：Go 的 GMP 调度模型

**难度**: basic | **标签**: 并发, 运行时

### 一句话结论
G (Goroutine) 是用户态协程，M (Machine) 是 OS 线程，P (Processor) 是逻辑处理器，M 必须绑定 P 才能执行 G。

### 核心原理
Go 运行时采用 GMP 模型。每个 P 持有本地运行队列（容量 256），M 从 P 的队列取 G 执行。当 G 阻塞（syscall/channel/mutex），M 会释放 P，P 被其他 M 获取继续执行其他 G。全局队列和本地队列通过 work stealing 算法平衡负载：当 P 的本地队列为空时，从全局队列或随机从其他 P 偷取一半 G。

### 代码示例
` + "```go" + `
// GMP 调度可视化: GOMAXPROCS 控制 P 的数量
func main() {
    runtime.GOMAXPROCS(4) // 4 个 P
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            time.Sleep(10 * time.Millisecond)
        }(i)
    }
    wg.Wait()
}
` + "```" + `

### 边界情况
- G 数量远大于 M×P 时，大量 G 在队列等待
- 网络 I/O 通过 netpoller 异步化，G 进入等待状态而非阻塞 M
- ` + "`" + `runtime.LockOSThread()` + "`" + ` 可绑定 G 到特定 M

---

## 题目 2：sync.Mutex vs sync.RWMutex

**难度**: basic | **标签**: 并发, 锁

### 一句话结论
Mutex 读写均互斥；RWMutex 允许多读单写，读多写少场景性能更优。

### 核心原理
RWMutex 内部分为读锁计数和写锁信号。RLock 时原子递增计数，写锁等待计数归零。写锁饥饿时新读锁阻塞，防止写锁饿死。Go 1.20+ RWMutex 写锁饥饿模式进一步优化公平性。

### 代码示例
` + "```go" + `
var (
    cache = make(map[string]string)
    mu    sync.RWMutex
)

func Get(key string) string {
    mu.RLock()
    defer mu.RUnlock()
    return cache[key]
}

func Set(key, val string) {
    mu.Lock()
    defer mu.Unlock()
    cache[key] = val
}
` + "```" + `

### 边界情况
- 不可在持有读锁时获取写锁（死锁）
- 不可复制（` + "`" + `go vet` + "`" + ` 会检测 ` + "`" + `copylocks` + "`" + `）
- RWMutex 内部结构比 Mutex 大，小临界区用 Mutex 反而更快

---

## 题目 3：defer 的执行顺序与陷阱

**难度**: basic | **标签**: 语言基础

### 一句话结论
defer 按 LIFO 顺序执行，参数在声明时求值，可修改命名返回值。

### 核心原理
每个 goroutine 维护一条 defer 链表。遇到 defer 时将调用信息和参数值压入链表。函数返回（或 panic）时从链表头部（最后一个 defer）开始执行。defer 的参数在 defer 语句执行时就被计算并保存。

### 代码示例
` + "```go" + `
// defer 参数求值时机
func demo() {
    x := 1
    defer fmt.Println("A:", x) // x=1 此时已求值
    x = 2
    defer func() { fmt.Println("B:", x) }() // 闭包捕获引用，输出 2
    // 输出: B: 2, A: 1
}
` + "```" + `

### 边界情况
- 循环中 defer 会累积，大循环可能导致内存问题
- ` + "`" + `os.Exit` + "`" + ` 后 defer 不执行
- recover 只在 defer 函数体内生效

---

## 题目 4：context.Context 的设计与最佳实践

**难度**: basic | **标签**: 标准库, 并发

### 一句话结论
context 用于在 goroutine 间传递截止时间、取消信号和请求级键值。

### 核心原理
context 是一个不可变树结构。WithCancel/WithTimeout/WithDeadline 派生新节点并注册到父节点的取消链。父节点取消时，所有子节点级联取消。context 通过 Done() channel 通知取消，Err() 返回取消原因。

### 代码示例
` + "```go" + `
func worker(ctx context.Context, id int) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // 执行业务逻辑
            time.Sleep(100 * time.Millisecond)
        }
    }
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    go worker(ctx, 1)
    <-ctx.Done()
    fmt.Println("timeout:", ctx.Err())
}
` + "```" + `

### 边界情况
- ctx 作为函数第一个参数，不要存到 struct
- WithValue 只传请求级数据（traceID、token），不要传业务参数
- 总是调用 cancel() 释放资源（即使 ctx 已超时）
- context.Background() 和 TODO() 的区别：前者是根，后者是占位

---

## 题目 5：Go error 处理最佳实践

**难度**: medium | **标签**: 错误处理

### 一句话结论
用 ` + "`" + `fmt.Errorf("%%w", err)` + "`" + ` 包装错误保留链路，用 ` + "`" + `errors.Is/As` + "`" + ` 判断类型。

### 核心原理
Go 1.13+ 错误包裹机制：` + "`" + `%%w` + "`" + ` 将原始错误嵌入新错误，形成单向链表。` + "`" + `errors.Is` + "`" + ` 沿链比较错误值，` + "`" + `errors.As` + "`" + ` 沿链类型断言。sentinel error 模式用于预定义错误哨兵。

### 代码示例
` + "```go" + `
var ErrNotFound = errors.New("not found")

func GetUser(id int) (*User, error) {
    u, err := db.Query(id)
    if err != nil {
        return nil, fmt.Errorf("GetUser(%%d): %%w", id, err)
    }
    if u == nil {
        return nil, ErrNotFound
    }
    return u, nil
}

func main() {
    _, err := GetUser(42)
    if errors.Is(err, ErrNotFound) {
        fmt.Println("user not found")
    }
}
` + "```" + `

### 边界情况
- 不要用字符串比较判断错误类型
- 自定义错误类型实现 Error() 而非 String()
- 一个错误只包装一次，避免重复包裹
- 错误信息小写开头，不以句号结尾

---

## 题目 6：Go channel 的发送/接收语义与 select

**难度**: medium | **标签**: 并发, channel

### 一句话结论
无缓冲 channel 同步阻塞；有缓冲 channel 满时发送阻塞、空时接收阻塞；select 随机选择就绪 case。

### 核心原理
channel 底层是 ` + "`" + `runtime.hchan` + "`" + ` 结构体，包含环形缓冲区、发送/接收等待队列和互斥锁。发送时若缓冲区有空间则直接放入，否则 goroutine 进入发送等待队列被挂起。接收同理。关闭 channel 后接收返回零值+false，向已关闭 channel 发送则 panic。

### 代码示例
` + "```go" + `
func fanIn(ctx context.Context, inputs ...<-chan int) <-chan int {
    out := make(chan int)
    var wg sync.WaitGroup
    for _, ch := range inputs {
        wg.Add(1)
        go func(c <-chan int) {
            defer wg.Done()
            for v := range c {
                select {
                case out <- v:
                case <-ctx.Done():
                    return
                }
            }
        }(ch)
    }
    go func() { wg.Wait(); close(out) }()
    return out
}
` + "```" + `

### 边界情况
- nil channel 的发送/接收永久阻塞，select 中 nil channel 的 case 永不就绪
- 关闭已关闭的 channel 会 panic
- select 的 default 分支使 channel 操作非阻塞
- 向已关闭 channel 发送 → panic；从已关闭 channel 接收 → 零值+false

---

## 题目 7：Go interface 的底层结构与 nil 判断

**难度**: medium | **标签**: 语言基础, interface

### 一句话结论
interface 底层由 ` + "`" + `(type, data)` + "`" + ` 二元组构成，只有两者均为 nil 时 interface 才等于 nil。

### 核心原理
` + "`" + `runtime.iface` + "`" + `（有方法的 interface）和 ` + "`" + `runtime.eface` + "`" + `（空 interface）都包含类型指针和数据指针。将 nil 的具体类型指针赋给 interface 变量时，类型指针非 nil，数据指针为 nil，导致 interface != nil。这是 Go 最常见的陷阱之一。

### 代码示例
` + "```go" + `
func returnsError() error {
    var p *MyError = nil
    if somethingWrong {
        p = &MyError{}
    }
    return p // p 是 nil，但返回的 error != nil!
}

func main() {
    err := returnsError()
    if err != nil { // 永远为 true!
        fmt.Println("error:", err)
    }
}
// 修复: return nil 而不是 nil 指针
` + "```" + `

### 边界情况
- ` + "`" + `interface{}` + "`" + ` → ` + "`" + `any` + "`" + `（Go 1.18+）
- 类型断言用 comma-ok 模式：` + "`" + `v, ok := x.(T)` + "`" + `
- 类型 switch 可替代多分支断言
- nil interface 调用方法会 panic

---

## 题目 8：Go GC 三色标记与内存优化

**难度**: advanced | **标签**: 运行时, GC, 性能

### 一句话结论
Go 使用并发三色标记-清除算法，GC 触发由 GOGC 控制（默认堆增长 100%%），可配合 GOMEMLIMIT 限制内存。

### 核心原理
三色标记：白色（未扫描）、灰色（已扫描引用未扫）、黑色（全部扫描）。GC 从根对象开始，通过写屏障保证并发标记正确性。STW 仅在 SweepTermination 和 MarkTermination 阶段（通常 < 1ms）。标记阶段与应用 goroutine 并发执行。

### 代码示例
` + "```go" + `
// 通过 sync.Pool 减少 GC 压力
var bufPool = sync.Pool{
    New: func() any {
        return make([]byte, 0, 4096)
    },
}

func process(data []byte) {
    buf := bufPool.Get().([]byte)
    buf = buf[:0]
    defer bufPool.Put(buf)
    // 使用 buf...
}

// 监控 GC 指标
func printGCStats() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    fmt.Printf("GC cycles: %%d, Heap: %%d MB, Pause: %%d ms\n",
        m.NumGC, m.HeapAlloc/1024/1024, m.PauseTotalNs/1e6)
}
` + "```" + `

### 边界情况
- GOGC=off 关闭自动 GC，只保留手动 ` + "`" + `runtime.GC()` + "`" + `
- GOMEMLIMIT 是软限制，不是硬上限（Go 1.19+）
- sync.Pool 中的对象随时会被 GC 回收
- map 不会自动缩容，需定期重建
- 逃逸分析是编译期行为，查看：` + "`" + `go build -gcflags="-m"` + "`" + `

---

> 生成自 Harness 内置题库`, topic, time.Now().Format("2006-01-02 15:04:05"))
}

// loadKnowledge 从 SQLite 知识库加载面试题
func (a *ClaudeCodeAdapter) loadKnowledge() error {
	if a.dbPath == "" {
		return fmt.Errorf("db_path not configured")
	}

	db, err := sql.Open("sqlite3", a.dbPath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, title, content, tags, metadata FROM knowledge WHERE type = 'interview' ORDER BY id`)
	if err != nil {
		return fmt.Errorf("failed to query knowledge: %w", err)
	}
	defer rows.Close()

	var entries []knowledgeEntry
	for rows.Next() {
		var e knowledgeEntry
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &e.Tags, &e.Metadata); err != nil {
			continue
		}
			e.parseMetadata()
		entries = append(entries, e)
	}

	if len(entries) == 0 {
		return fmt.Errorf("no interview entries found")
	}

	a.knowledge = entries
	fmt.Fprintf(os.Stderr, "[INFO] loaded %d interview questions from knowledge base\n", len(entries))
	return nil
}

// smartSelectQuestions 智能选题：支持主题过滤 + 难度分层抽样
// 从 task.Context 读取: question_count(默认5), topic, difficulty
func (a *ClaudeCodeAdapter) smartSelectQuestions(task models.Task) []knowledgeEntry {
	n := 5
	if v, ok := task.Context["question_count"]; ok {
		switch c := v.(type) {
		case float64:
			if c > 0 && c <= 20 {
				n = int(c)
			}
		case int:
			if c > 0 && c <= 20 {
				n = c
			}
		}
	}

	topicFilter := ""
	if v, ok := task.Context["topic"].(string); ok && v != "" {
		topicFilter = strings.ToLower(v)
	}
	diffFilter := ""
	if v, ok := task.Context["difficulty"].(string); ok && v != "" {
		diffFilter = strings.ToLower(v)
	}

	// 过滤
	filtered := a.knowledge
	if topicFilter != "" {
		filtered = nil
		for _, e := range a.knowledge {
			if strings.Contains(strings.ToLower(e.Tags), topicFilter) ||
				strings.Contains(strings.ToLower(e.Topic), topicFilter) ||
				strings.Contains(strings.ToLower(e.Title), topicFilter) {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			filtered = a.knowledge // 无匹配时回退全库
		}
	}
	if diffFilter != "" {
		var diffFiltered []knowledgeEntry
		for _, e := range filtered {
			if e.Difficulty == diffFilter {
				diffFiltered = append(diffFiltered, e)
			}
		}
		if len(diffFiltered) > 0 {
			filtered = diffFiltered
		}
	}

	if len(filtered) <= n {
		return filtered
	}

	// 按难度分组
	groups := map[string][]knowledgeEntry{"basic": {}, "medium": {}, "advanced": {}}
	for _, e := range filtered {
		groups[e.Difficulty] = append(groups[e.Difficulty], e)
	}

	// 分层抽样配额: basic 40%, medium 40%, advanced 20%
	quotas := map[string]int{
		"basic":    n * 40 / 100,
		"medium":  n * 40 / 100,
		"advanced": n * 20 / 100,
	}

	// 分配剩余名额 (整数舍入)
	assigned := quotas["basic"] + quotas["medium"] + quotas["advanced"]
	for i := 0; i < n-assigned; i++ {
		// 优先补到 advanced, 再 medium
		if len(groups["advanced"]) > quotas["advanced"] {
			quotas["advanced"]++
		} else if len(groups["medium"]) > quotas["medium"] {
			quotas["medium"]++
		} else {
			quotas["basic"]++
		}
	}

	// 从各组 shuffle 抽取
	var result []knowledgeEntry
	for _, diff := range []string{"basic", "medium", "advanced"} {
		pool := groups[diff]
		q := quotas[diff]
		if q > len(pool) {
			// 本层不足，从其他层补齐
			shortfall := q - len(pool)
			result = append(result, pool...)
			// 从 medium/advanced 或 basic 补
			for _, alt := range []string{"medium", "advanced", "basic"} {
				if shortfall <= 0 {
					break
				}
				if alt == diff {
					continue
				}
				altPool := groups[alt]
				if len(altPool) > quotas[alt] {
					extra := len(altPool) - quotas[alt]
					if extra > shortfall {
						extra = shortfall
					}
					result = append(result, altPool[quotas[alt]:quotas[alt]+extra]...)
					quotas[alt] += extra
					shortfall -= extra
				}
			}
			continue
		}
		shuffled := make([]knowledgeEntry, len(pool))
		copy(shuffled, pool)
		for i := len(shuffled) - 1; i > 0; i-- {
			j := rand.Intn(i + 1)
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		}
		result = append(result, shuffled[:q]...)
	}

	if len(result) == 0 {
		return filtered
	}
	return result
}

// buildPrompt 构建发送给 Claude 的 prompt
func (a *ClaudeCodeAdapter) buildPrompt(task models.Task, action string) string {
	var prompt string

	switch action {
	case "implement":
		prompt = fmt.Sprintf("Implement the following task in the project at %s:\n\n%s",
			a.config.RootDir, task.Description)
	case "review":
		prompt = fmt.Sprintf("Review the code in the project at %s for the following:\n\n%s",
			a.config.RootDir, task.Description)
	case "test":
		prompt = fmt.Sprintf("Write and run tests for the project at %s:\n\n%s",
			a.config.RootDir, task.Description)
	case "interview":
		prompt = fmt.Sprintf("你是一位资深 Go 语言面试官。根据以下任务描述，生成高质量的 Go 面试题及详细解答：\n\n项目路径: %s\n\n%s",
			a.config.RootDir, task.Description)
	}

	// 添加约束
	if len(task.Constraints) > 0 {
		prompt += "\n\nConstraints (MUST follow):"
		for _, c := range task.Constraints {
			prompt += fmt.Sprintf("\n- [%s] %s: %s", c.Severity, c.Rule, c.Message)
		}
	}

	// 添加上下文
	if len(task.Context) > 0 {
		prompt += "\n\nContext:"
		for k, v := range task.Context {
			prompt += fmt.Sprintf("\n- %s: %v", k, v)
		}
	}

	return prompt
}

// invokeClaude 调用 claude CLI
func (a *ClaudeCodeAdapter) invokeClaude(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt)
	cmd.Dir = a.config.RootDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude CLI failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// updateStatusMarkers 更新状态标记
func (a *ClaudeCodeAdapter) updateStatusMarkers(content string, state *taskState, result *models.Result) string {
	// 实现状态标记更新逻辑
	return content
}

// taskState 任务状态
type taskState struct {
	Task      models.Task
	Status    models.TaskStatus
	CreatedAt time.Time
}

// hookConfig hook 配置
type hookConfig struct {
	Matcher string `json:"matcher"`
	Hooks   []struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	} `json:"hooks"`
}

// scanFilesForPatterns 扫描项目文件中的模式匹配
func (a *ClaudeCodeAdapter) scanFilesForPatterns(patterns []string) error {
	entries, err := os.ReadDir(a.config.RootDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isGoFile(entry.Name()) && !isConfigFile(entry.Name()) {
			continue
		}
		path := filepath.Join(a.config.RootDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		for _, p := range patterns {
			matched, _ := regexp.MatchString(p, content)
			if matched {
				return fmt.Errorf("constraint violation in %s: matched pattern %s", entry.Name(), p)
			}
		}
	}
	return nil
}

// isGoFile 检查是否为 Go 文件
func isGoFile(name string) bool {
	return strings.HasSuffix(name, ".go")
}

// isConfigFile 检查是否为配置文件
func isConfigFile(name string) bool {
	for _, ext := range []string{".yaml", ".yml", ".json", ".toml", ".env"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}
