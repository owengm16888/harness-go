package routine

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
)

// ============================================================
// 智能面试引擎 — 题库 + LLM 混合出题
// ============================================================

// QuestionBank 题库
type QuestionBank struct {
	questions []Question
	categories map[string][]Question
}

// Question 题目
type Question struct {
	ID         string   `json:"id"`
	Category   string   `json:"category"`   // GMP, GC, channel, sync, memory, etc.
	Difficulty int      `json:"difficulty"`  // 1-10
	Question   string   `json:"question"`
	Keywords   []string `json:"keywords"`
	Expected   string   `json:"expected"`    // 期望答案要点
	Followup   []string `json:"followup"`    // 追问列表
}

// NewQuestionBank 创建题库
func NewQuestionBank() *QuestionBank {
	bank := &QuestionBank{
		questions:  make([]Question, 0),
		categories: make(map[string][]Question),
	}
	bank.loadDefaultQuestions()
	return bank
}

// loadDefaultQuestions 加载默认题库
func (qb *QuestionBank) loadDefaultQuestions() {
	questions := []Question{
		// GMP 调度
		{
			ID: "gmp-001", Category: "GMP", Difficulty: 3,
			Question: "请介绍一下 Go 中的 GMP 调度模型",
			Keywords: []string{"goroutine", "machine", "processor", "调度"},
			Expected: "G是goroutine，M是系统线程，P是逻辑处理器。P持有本地运行队列，M绑定P执行G。",
			Followup: []string{"当G阻塞时会发生什么？", "work stealing是什么？", "GOMAXPROCS的作用？"},
		},
		{
			ID: "gmp-002", Category: "GMP", Difficulty: 5,
			Question: "Goroutine 和线程有什么区别？Go 为什么选择 goroutine？",
			Keywords: []string{"栈", "调度", "开销", "用户态"},
			Expected: "goroutine栈2KB可动态增长，线程栈1MB固定。goroutine用户态调度，开销更小。",
			Followup: []string{"goroutine栈如何增长？", "调度时机有哪些？", "抢占式调度如何实现？"},
		},
		{
			ID: "gmp-003", Category: "GMP", Difficulty: 7,
			Question: "Go 的抢占式调度是如何实现的？",
			Keywords: []string{"sysmon", "信号", "抢占", "协作"},
			Expected: "sysmon监控goroutine运行时间，超过10ms发送SIGURG信号触发抢占。",
			Followup: []string{"sysmon多久检查一次？", "为什么需要抢占式调度？", "Go 1.14前后有什么变化？"},
		},

		// GC 垃圾回收
		{
			ID: "gc-001", Category: "GC", Difficulty: 4,
			Question: "请解释一下 Go 的垃圾回收机制",
			Keywords: []string{"三色标记", "写屏障", "STW", "并发"},
			Expected: "三色标记法：白色未标记，灰色已标记子未扫描，黑色已标记子已扫描。写屏障防止丢失。",
			Followup: []string{"STW发生在什么时候？", "如何减少GC压力？", "GOGC参数的作用？"},
		},
		{
			ID: "gc-002", Category: "GC", Difficulty: 6,
			Question: "什么是三色标记法？写屏障的作用是什么？",
			Keywords: []string{"白色", "灰色", "黑色", "屏障"},
			Expected: "三色标记法将对象分为白灰黑三色。写屏障在并发标记时防止已扫描对象引用未扫描对象。",
			Followup: []string{"插入屏障和删除屏障的区别？", "混合写屏障是什么？", "STW的时间如何优化？"},
		},
		{
			ID: "gc-003", Category: "GC", Difficulty: 8,
			Question: "如何调优 Go 的 GC？GOGC 参数如何设置？",
			Keywords: []string{"GOGC", "内存", "调优", "阈值"},
			Expected: "GOGC控制GC触发阈值，默认100表示内存增长100%触发GC。可通过环境变量或debug.SetGCPercent设置。",
			Followup: []string{"GOMEMLIMIT的作用？", "如何监控GC？", "什么时候应该关闭GC？"},
		},

		// Channel
		{
			ID: "ch-001", Category: "Channel", Difficulty: 3,
			Question: "Go 中 channel 的底层实现原理是什么？",
			Keywords: []string{"环形队列", "互斥锁", "goroutine", "阻塞"},
			Expected: "channel底层是环形队列，使用互斥锁保护。发送接收会检查等待的goroutine。",
			Followup: []string{"有缓冲和无缓冲的区别？", "select如何工作？", "channel什么时候会panic？"},
		},
		{
			ID: "ch-002", Category: "Channel", Difficulty: 5,
			Question: "有缓冲 channel 和无缓冲 channel 有什么区别？",
			Keywords: []string{"同步", "异步", "阻塞", "缓冲"},
			Expected: "无缓冲channel是同步的，发送接收同时就绪。有缓冲channel是异步的，缓冲区满才阻塞。",
			Followup: []string{"如何选择使用哪种？", "channel的方向有什么用？", "nil channel会怎样？"},
		},
		{
			ID: "ch-003", Category: "Channel", Difficulty: 7,
			Question: "select 语句的实现原理是什么？多个 case 同时就绪会怎样？",
			Keywords: []string{"随机", "阻塞", "default", "多路复用"},
			Expected: "select实现多路复用，多个case同时就绪时随机选择一个执行。有default则非阻塞。",
			Followup: []string{"select{}会怎样？", "如何实现超时控制？", "select和switch的区别？"},
		},

		// Sync 包
		{
			ID: "sync-001", Category: "Sync", Difficulty: 4,
			Question: "sync.Map 和普通 map+RWMutex 有什么区别？",
			Keywords: []string{"读多写少", "dirty", "read", "原子"},
			Expected: "sync.Map适用于读多写少，双层map结构。RWMutex+map适用于写多场景。",
			Followup: []string{"sync.Map的内部结构？", "什么时候用哪种？", "sync.Map的缺点？"},
		},
		{
			ID: "sync-002", Category: "Sync", Difficulty: 5,
			Question: "sync.WaitGroup 的实现原理是什么？",
			Keywords: []string{"计数器", "信号量", "阻塞", "Done"},
			Expected: "WaitGroup内部是原子计数器+信号量。Add增加计数，Done减少，Wait阻塞直到归零。",
			Followup: []string{"WaitGroup可以复制吗？", "Add必须在goroutine外调用？", "如何实现超时等待？"},
		},
		{
			ID: "sync-003", Category: "Sync", Difficulty: 6,
			Question: "Mutex 和 RWMutex 的实现原理？有什么使用注意事项？",
			Keywords: []string{"互斥", "读写锁", "饥饿", "自旋"},
			Expected: "Mutex使用信号量+自旋，RWMutex允许多读单写。注意不能复制，Lock/Unlock配对。",
			Followup: []string{"什么是锁饥饿？", "如何避免死锁？", "atomic和锁的区别？"},
		},

		// 内存管理
		{
			ID: "mem-001", Category: "Memory", Difficulty: 5,
			Question: "Go 的内存分配是如何工作的？",
			Keywords: []string{"mcache", "mcentral", "mheap", "大小"},
			Expected: "三级分配：mcache(线程缓存)->mcentral(中心缓存)->mheap(堆)。小对象<32KB用mcache。",
			Followup: []string{"逃逸分析是什么？", "栈分配和堆分配的区别？", "如何减少内存分配？"},
		},
		{
			ID: "mem-002", Category: "Memory", Difficulty: 6,
			Question: "什么是逃逸分析？Go 编译器如何决定分配在栈还是堆？",
			Keywords: []string{"栈", "堆", "编译器", "分析"},
			Expected: "逃逸分析在编译期判断变量生命周期，如果超出函数范围则分配到堆，否则栈分配。",
			Followup: []string{"如何查看逃逸分析结果？", "哪些情况会导致逃逸？", "栈分配的优势？"},
		},
		{
			ID: "mem-003", Category: "Memory", Difficulty: 7,
			Question: "sync.Pool 的作用和实现原理是什么？",
			Keywords: []string{"对象池", "GC", "复用", "减少"},
			Expected: "sync.Pool是对象缓存池，减少GC压力。每P有私有和共享池，GC时会被清空。",
			Followup: []string{"Pool什么时候会被清空？", "适合什么场景？", "不适合什么场景？"},
		},

		// 并发模式
		{
			ID: "conc-001", Category: "Concurrency", Difficulty: 4,
			Question: "如何避免 goroutine 泄漏？",
			Keywords: []string{"context", "done", "channel", "超时"},
			Expected: "使用context控制生命周期，done channel通知退出，确保channel关闭，设置超时。",
			Followup: []string{"如何检测goroutine泄漏？", "errgroup怎么用？", "panic的goroutine会怎样？"},
		},
		{
			ID: "conc-002", Category: "Concurrency", Difficulty: 6,
			Question: "Go 的 CSP 并发模型是什么？和传统锁有什么区别？",
			Keywords: []string{"通信", "共享", "channel", "消息"},
			Expected: "CSP通过通信共享内存，而非共享内存通信。Channel传递数据所有权，减少锁竞争。",
			Followup: []string{"什么时候用channel，什么时候用锁？", "channel的开销？", "如何实现生产者消费者？"},
		},
		{
			ID: "conc-003", Category: "Concurrency", Difficulty: 8,
			Question: "如何实现一个高性能的并发安全队列？",
			Keywords: []string{"无锁", "CAS", "ring buffer", "分段"},
			Expected: "可以使用无锁队列(CAS)、ring buffer、分段队列等方案。根据场景选择。",
			Followup: []string{"无锁队列的ABA问题？", "如何处理队列满？", "如何实现优先级队列？"},
		},

		// 项目经验
		{
			ID: "proj-001", Category: "Project", Difficulty: 5,
			Question: "请介绍一下你做过的最有挑战性的 Go 项目",
			Keywords: []string{"架构", "并发", "性能", "问题"},
			Expected: "应该从架构设计、技术选型、遇到的问题、如何解决等方面回答。",
			Followup: []string{"遇到了什么技术难点？", "如何做的性能优化？", "如果重新设计会怎么改进？"},
		},
		{
			ID: "proj-002", Category: "Project", Difficulty: 6,
			Question: "在你的项目中，如何处理高并发场景？",
			Keywords: []string{"限流", "缓存", "队列", "扩容"},
			Expected: "限流(令牌桶/滑动窗口)、缓存(Redis)、消息队列(Kafka)、水平扩容等方案。",
			Followup: []string{"限流算法的区别？", "缓存穿透怎么解决？", "如何保证消息不丢失？"},
		},
	}

	// 添加到题库
	for _, q := range questions {
		qb.questions = append(qb.questions, q)
		qb.categories[q.Category] = append(qb.categories[q.Category], q)
	}
}

// GetByCategory 按分类获取题目
func (qb *QuestionBank) GetByCategory(category string) []Question {
	return qb.categories[category]
}

// GetByDifficulty 按难度获取题目
func (qb *QuestionBank) GetByDifficulty(min, max int) []Question {
	var result []Question
	for _, q := range qb.questions {
		if q.Difficulty >= min && q.Difficulty <= max {
			result = append(result, q)
		}
	}
	return result
}

// GetRandom 随机获取题目
func (qb *QuestionBank) GetRandom() Question {
	return qb.questions[rand.Intn(len(qb.questions))]
}

// GetRandomByCategory 按分类随机获取
func (qb *QuestionBank) GetRandomByCategory(category string) Question {
	questions := qb.categories[category]
	if len(questions) == 0 {
		return qb.GetRandom()
	}
	return questions[rand.Intn(len(questions))]
}

// Search 搜索题目
func (qb *QuestionBank) Search(keyword string) []Question {
	var result []Question
	keyword = strings.ToLower(keyword)
	for _, q := range qb.questions {
		if strings.Contains(strings.ToLower(q.Question), keyword) {
			result = append(result, q)
			continue
		}
		for _, kw := range q.Keywords {
			if strings.Contains(strings.ToLower(kw), keyword) {
				result = append(result, q)
				break
			}
		}
	}
	return result
}

// ============================================================
// 智能面试官 Agent — 混合出题策略
// ============================================================

// SmartInterviewerAgent 智能面试官
type SmartInterviewerAgent struct {
	name       string
	bank       *QuestionBank
	llm        LLMProvider
	strategy   InterviewStrategy
	asked      map[string]bool // 已问过的题目
	difficulty int             // 当前难度
	categories []string        // 考察分类
}

// InterviewStrategy 出题策略
type InterviewStrategy string

const (
	StrategyBankFirst  InterviewStrategy = "bank_first"  // 题库优先
	StrategyLLMFirst   InterviewStrategy = "llm_first"   // LLM优先
	StrategyHybrid     InterviewStrategy = "hybrid"      // 混合出题
	StrategyAdaptive   InterviewStrategy = "adaptive"    // 自适应出题
)

// NewSmartInterviewerAgent 创建智能面试官
func NewSmartInterviewerAgent(bank *QuestionBank, llm LLMProvider, strategy InterviewStrategy) *SmartInterviewerAgent {
	return &SmartInterviewerAgent{
		name:       "smart_interviewer",
		bank:       bank,
		llm:        llm,
		strategy:   strategy,
		asked:      make(map[string]bool),
		difficulty: 5,
		categories: []string{"GMP", "GC", "Channel", "Sync", "Memory", "Concurrency"},
	}
}

func (a *SmartInterviewerAgent) Name() string { return a.name }
func (a *SmartInterviewerAgent) Role() AgentRole { return RoleInterviewer }

func (a *SmartInterviewerAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	// 第一轮：开场
	if input.Round == 0 {
		return a.openingQuestion(input), nil
	}

	// 后续轮次：根据策略出题
	switch a.strategy {
	case StrategyBankFirst:
		return a.bankFirstStrategy(ctx, input)
	case StrategyLLMFirst:
		return a.llmFirstStrategy(ctx, input)
	case StrategyHybrid:
		return a.hybridStrategy(ctx, input)
	case StrategyAdaptive:
		return a.adaptiveStrategy(ctx, input)
	default:
		return a.hybridStrategy(ctx, input)
	}
}

// openingQuestion 开场问题
func (a *SmartInterviewerAgent) openingQuestion(input AgentInput) *AgentOutput {
	focus, _ := input.Context["focus"].(string)
	if focus == "" {
		focus = "Go 后端开发"
	}

	return &AgentOutput{
		Content: fmt.Sprintf(`【面试官】

欢迎参加今天的面试。请先做一个简单的自我介绍，重点介绍：

1. 你的 Go 语言开发经验
2. 做过的最有挑战性的项目
3. 在项目中遇到的技术难点及解决方案

（请开始你的自我介绍）`),
		NextAction: "wait_answer",
	}
}

// bankFirstStrategy 题库优先策略
func (a *SmartInterviewerAgent) bankFirstStrategy(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	// 从题库随机抽取未问过的题目
	question := a.pickFromBank()
	if question != nil {
		a.asked[question.ID] = true
		return a.formatBankQuestion(question), nil
	}

	// 题库用完了，用 LLM 生成
	return a.generateLLMQuestion(ctx, input)
}

// llmFirstStrategy LLM优先策略
func (a *SmartInterviewerAgent) llmFirstStrategy(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	// 偶尔从题库抽取
	if rand.Float64() < 0.3 {
		question := a.pickFromBank()
		if question != nil {
			a.asked[question.ID] = true
			return a.formatBankQuestion(question), nil
		}
	}

	// 用 LLM 生成
	return a.generateLLMQuestion(ctx, input)
}

// hybridStrategy 混合出题策略
func (a *SmartInterviewerAgent) hybridStrategy(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	// 根据评分决定出题方式
	if input.Score != nil {
		// 回答好 -> LLM 深挖
		if input.Score.Total >= 80 {
			return a.generateLLMQuestion(ctx, input)
		}
		// 回答一般 -> 题库出题
		question := a.pickFromBank()
		if question != nil {
			a.asked[question.ID] = true
			return a.formatBankQuestion(question), nil
		}
	}

	// 默认：50% 概率选择
	if rand.Float64() < 0.5 {
		question := a.pickFromBank()
		if question != nil {
			a.asked[question.ID] = true
			return a.formatBankQuestion(question), nil
		}
	}

	return a.generateLLMQuestion(ctx, input)
}

// adaptiveStrategy 自适应出题策略
func (a *SmartInterviewerAgent) adaptiveStrategy(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	// 根据评分调整难度
	if input.Score != nil {
		if input.Score.Total >= 80 {
			a.difficulty = min(10, a.difficulty+1)
		} else if input.Score.Total < 50 {
			a.difficulty = max(1, a.difficulty-1)
		}
	}

	// 从题库中找匹配难度的题目
	question := a.pickByDifficulty(a.difficulty)
	if question != nil {
		a.asked[question.ID] = true
		return a.formatBankQuestion(question), nil
	}

	// 用 LLM 生成指定难度的题目
	return a.generateLLMDifficultyQuestion(ctx, input, a.difficulty)
}

// pickFromBank 从题库抽取题目
func (a *SmartInterviewerAgent) pickFromBank() *Question {
	// 优先从未问过的分类中抽取
	for _, cat := range a.categories {
		questions := a.bank.GetByCategory(cat)
		for _, q := range questions {
			if !a.asked[q.ID] {
				return &q
			}
		}
	}

	// 所有分类都问过了，随机抽取
	for _, q := range a.bank.questions {
		if !a.asked[q.ID] {
			return &q
		}
	}

	return nil
}

// pickByDifficulty 按难度抽取
func (a *SmartInterviewerAgent) pickByDifficulty(difficulty int) *Question {
	questions := a.bank.GetByDifficulty(difficulty-1, difficulty+1)
	for _, q := range questions {
		if !a.asked[q.ID] {
			return &q
		}
	}
	return nil
}

// formatBankQuestion 格式化题库题目
func (a *SmartInterviewerAgent) formatBankQuestion(q *Question) *AgentOutput {
	content := fmt.Sprintf("【面试官】\n\n%s", q.Question)

	// 添加追问提示
	if len(q.Followup) > 0 {
		content += "\n\n（如果回答得好，我会追问：）"
		for _, f := range q.Followup {
			content += "\n- " + f
		}
	}

	return &AgentOutput{
		Content:    content,
		Question:   q.Question,
		NextAction: "wait_answer",
		Context: map[string]any{
			"question_id": q.ID,
			"category":    q.Category,
			"difficulty":  q.Difficulty,
			"expected":    q.Expected,
		},
	}
}

// generateLLMQuestion 用 LLM 生成题目
func (a *SmartInterviewerAgent) generateLLMQuestion(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	if a.llm == nil {
		return a.fallbackQuestion(input), nil
	}

	// 构建 prompt
	prompt := a.buildLLMPrompt(input)

	// 调用 LLM
	response, err := a.llm.ChatWithSystem(ctx, prompt, []LLMMessage{})
	if err != nil {
		slog.Warn("LLM call failed, using fallback", "error", err)
		return a.fallbackQuestion(input), nil
	}

	return &AgentOutput{
		Content:    fmt.Sprintf("【面试官】\n\n%s", response),
		Question:   response,
		NextAction: "wait_answer",
	}, nil
}

// generateLLMDifficultyQuestion 用 LLM 生成指定难度题目
func (a *SmartInterviewerAgent) generateLLMDifficultyQuestion(ctx context.Context, input AgentInput, difficulty int) (*AgentOutput, error) {
	if a.llm == nil {
		return a.fallbackQuestion(input), nil
	}

	prompt := fmt.Sprintf(`你是一位资深Go面试官。请生成一道难度为 %d/10 的Go面试题。

要求：
1. 难度 %d: %s
2. 考察范围：%s
3. 只问一个问题
4. 不要自问自答

输出格式：
问题内容`, difficulty, difficulty, difficultyDesc(difficulty), strings.Join(a.categories, "、"))

	response, err := a.llm.ChatWithSystem(ctx, prompt, []LLMMessage{})
	if err != nil {
		return a.fallbackQuestion(input), nil
	}

	return &AgentOutput{
		Content:    fmt.Sprintf("【面试官】\n\n%s", response),
		Question:   response,
		NextAction: "wait_answer",
	}, nil
}

// buildLLMPrompt 构建 LLM prompt
func (a *SmartInterviewerAgent) buildLLMPrompt(input AgentInput) string {
	// 收集已问过的题目
	var askedList []string
	for id := range a.asked {
		askedList = append(askedList, id)
	}

	return fmt.Sprintf(`你是一位资深Go面试官。

当前面试状态：
- 已问 %d 题
- 当前难度: %d/10
- 考察范围: %s

请生成下一道面试题。要求：
1. 不要重复已问过的题目
2. 根据候选人之前的回答调整难度
3. 只问一个问题
4. 不要自问自答

输出格式：
问题内容`, len(a.asked), a.difficulty, strings.Join(a.categories, "、"))
}

// fallbackQuestion 兜底问题
func (a *SmartInterviewerAgent) fallbackQuestion(input AgentInput) *AgentOutput {
	fallbacks := []string{
		"请介绍一下 Go 中的接口是如何实现的？",
		"Go 的 error 处理有什么最佳实践？",
		"请解释一下 Go 的 defer 语句的执行顺序",
		"Go 中如何实现一个线程安全的单例模式？",
		"请介绍一下 Go 的反射机制及其使用场景",
	}

	question := fallbacks[rand.Intn(len(fallbacks))]

	return &AgentOutput{
		Content:    fmt.Sprintf("【面试官】\n\n%s", question),
		Question:   question,
		NextAction: "wait_answer",
	}
}

// ============================================================
// 智能评估 Agent — 结合题库期望答案
// ============================================================

// SmartEvaluatorAgent 智能评估官
type SmartEvaluatorAgent struct {
	name string
	llm  LLMProvider
	bank *QuestionBank
}

// NewSmartEvaluatorAgent 创建智能评估官
func NewSmartEvaluatorAgent(llm LLMProvider, bank *QuestionBank) *SmartEvaluatorAgent {
	return &SmartEvaluatorAgent{
		name: "smart_evaluator",
		llm:  llm,
		bank: bank,
	}
}

func (a *SmartEvaluatorAgent) Name() string { return a.name }
func (a *SmartEvaluatorAgent) Role() AgentRole { return RoleEvaluator }

func (a *SmartEvaluatorAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	// 获取期望答案
	expected := ""
	if qID, ok := input.Context["question_id"].(string); ok {
		for _, q := range a.bank.questions {
			if q.ID == qID {
				expected = q.Expected
				break
			}
		}
	}

	// 使用 LLM 评估
	if a.llm != nil {
		return a.llmEvaluate(ctx, input, expected)
	}

	// 基础评估
	return a.basicEvaluate(input, expected), nil
}

// llmEvaluate LLM 评估
func (a *SmartEvaluatorAgent) llmEvaluate(ctx context.Context, input AgentInput, expected string) (*AgentOutput, error) {
	prompt := fmt.Sprintf(`你是技术评估官。请评估候选人的回答。

问题: %s
候选人回答: %s
期望答案要点: %s

请输出评分（JSON格式）：
{
  "correctness": 1-10,
  "depth": 1-10,
  "clarity": 1-10,
  "practical": 1-10,
  "strengths": ["优点1", "优点2"],
  "weaknesses": ["不足1"],
  "missing": ["遗漏点1"]
}`, input.Question, input.Answer, expected)

	response, err := a.llm.ChatWithSystem(ctx, prompt, []LLMMessage{})
	if err != nil {
		return a.basicEvaluate(input, expected), nil
	}

	// 解析 LLM 响应
	score := a.parseScore(response)

	return &AgentOutput{
		Content: formatScore(score),
		Score:   &score,
		NextAction: "next_step",
	}, nil
}

// basicEvaluate 基础评估
func (a *SmartEvaluatorAgent) basicEvaluate(input AgentInput, expected string) *AgentOutput {
	score := Score{
		Correctness: 7,
		Depth:       6,
		Clarity:     7,
		Practical:   6,
	}

	// 基于关键词匹配评分
	lowerAnswer := strings.ToLower(input.Answer)
	lowerExpected := strings.ToLower(expected)

	matchedKeywords := 0
	if expected != "" {
		keywords := strings.Split(lowerExpected, "，")
		for _, kw := range keywords {
			if strings.Contains(lowerAnswer, strings.TrimSpace(kw)) {
				matchedKeywords++
			}
		}
		if len(keywords) > 0 {
			matchRate := float64(matchedKeywords) / float64(len(keywords))
			score.Correctness = int(5 + matchRate*5)
			score.Depth = int(4 + matchRate*6)
		}
	}

	// 回答长度影响深度分
	if len(input.Answer) > 200 {
		score.Depth = min(10, score.Depth+1)
	}
	if len(input.Answer) > 500 {
		score.Practical = min(10, score.Practical+1)
	}

	score.Total = float64(score.Correctness+score.Depth+score.Clarity+score.Practical) / 4 * 10

	return &AgentOutput{
		Content:    formatScore(score),
		Score:      &score,
		NextAction: "next_step",
	}
}

// parseScore 解析 LLM 评分
func (a *SmartEvaluatorAgent) parseScore(response string) Score {
	// 简化解析
	score := Score{
		Correctness: 7,
		Depth:       6,
		Clarity:     7,
		Practical:   6,
		Total:       65,
		Strengths:   []string{"回答了问题"},
		Weaknesses:  []string{},
		Missing:     []string{},
	}

	// 检查关键词
	lower := strings.ToLower(response)
	if strings.Contains(lower, "correct") || strings.Contains(lower, "正确") {
		score.Correctness = 8
	}
	if strings.Contains(lower, "deep") || strings.Contains(lower, "深入") {
		score.Depth = 8
	}

	score.Total = float64(score.Correctness+score.Depth+score.Clarity+score.Practical) / 4 * 10
	return score
}

// ============================================================
// 辅助函数
// ============================================================

func difficultyDesc(d int) string {
	switch {
	case d <= 3:
		return "基础概念，适合初级工程师"
	case d <= 5:
		return "原理理解，适合中级工程师"
	case d <= 7:
		return "深入源码，适合高级工程师"
	default:
		return "架构设计，适合资深工程师"
	}
}


