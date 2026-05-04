package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/harness-engineering/harness/pkg/routine"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║            Go 面试 Routine - 完整面试流程演示                      ║")
	fmt.Println("║            (候选人答案 + 参考答案 + 详细反馈)                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	rand.Seed(time.Now().UnixNano())

	// 面试题目和候选人答案
	type QA struct {
		question    string
		answer      string
		expected    string
		keywords    []string
		difficulty  string
		category    string
	}

	qaList := []QA{
		{
			question: "请介绍一下 Go 中的 GMP 调度模型",
			answer: "GMP 是 Go 的调度模型。G 代表 Goroutine，是 Go 的轻量级线程；M 代表 Machine，是操作系统线程；P 代表 Processor，是逻辑处理器。P 持有本地运行队列，M 必须绑定 P 才能执行 G。当 G 阻塞时，M 会释放 P，P 被其他 M 获取，从而实现高效并发。",
			expected: "G 是 goroutine（用户态协程，~2KB 栈），M 是 OS 线程，P 是逻辑处理器（数量=GOMAXPROCS）。调度流程：M 绑定 P → 从 P 的本地队列取 G 执行 → 本地队列空时 work stealing 从其他 P 偷取 → G 阻塞时 M 释放 P。关键点：netpoller 异步网络 I/O、sysmon 监控线程、抢占式调度（Go 1.14+ 基于信号）。",
			keywords:   []string{"goroutine", "machine", "processor", "调度", "work stealing"},
			difficulty: "basic",
			category:   "GMP",
		},
		{
			question: "请解释一下 Go 的垃圾回收机制",
			answer: "Go 使用三色标记法进行垃圾回收。白色表示未标记的对象，灰色表示已标记但子对象未扫描的对象，黑色表示已标记且子对象已扫描的对象。GC 从根对象开始扫描，逐步将白色对象标记为灰色，再标记为黑色。使用写屏障防止并发标记时丢失对象。STW 用于标记开始和结束阶段。",
			expected: "三色标记法（白→灰→黑）+ 混合写屏障（Go 1.8+）。流程：1) STW 标记准备 2) 并发标记（写屏障保证正确性）3) STW 标记终止 4) 并发清扫。写屏障：插入屏障（新引用标灰）+ 删除屏障（删除引用标灰）= 混合写屏障。调优：GOGC（默认100，内存增长%触发）、GOMEMLIMIT（Go 1.19+ 软内存限制）、debug.SetGCPercent()。",
			keywords:   []string{"三色标记", "写屏障", "STW", "并发", "GOGC"},
			difficulty: "medium",
			category:   "GC",
		},
		{
			question: "Go 中 channel 的底层实现原理是什么？",
			answer: "Channel 底层是一个环形队列，使用互斥锁保护。发送和接收操作都会检查是否有等待的 goroutine。如果有等待的接收方，直接传递数据；否则放入缓冲区。无缓冲 channel 要求发送和接收同时就绪。select 语句可以同时等待多个 channel 操作。",
			expected: "底层结构 hchan：环形缓冲区 ringbuf、互斥锁 lock、发送/接收队列 sendq/recvq（双向链表，存 g 指针）。操作流程：发送→有等待接收者直接复制/无则入缓冲/满则入发送队列挂起；接收→有等待发送者直接复制/无则取缓冲/空则入接收队列挂起。select：随机遍历 case，就绪则执行，全阻塞则入所有等待队列。",
			keywords:   []string{"环形队列", "互斥锁", "goroutine", "阻塞", "select"},
			difficulty: "medium",
			category:   "Channel",
		},
		{
			question: "sync.Map 和普通 map+RWMutex 有什么区别？",
			answer: "sync.Map 适用于读多写少的场景，使用 read-only map 和 dirty map 双层结构。读操作先查 read map（无锁），miss 后查 dirty map（加锁）。当 miss 次数超过 dirty 长度时，提升 dirty 为 read。普通 map+RWMutex 适用于写多场景。",
			expected: "sync.Map 内部：read map（atomic，无锁读）+ dirty map（加锁）+ misses 计数。读流程：先查 read（无锁）→ miss 则查 dirty（加锁）→ misses > len(dirty) 时提升 dirty=read。写流程：read 中有则 CAS 更新/无则加锁写 dirty。适用：key 稳定、读远多于写。不适用：频繁增删、需要 Len()。",
			keywords:   []string{"读多写少", "dirty", "read", "原子", "双层"},
			difficulty: "medium",
			category:   "Sync",
		},
		{
			question: "如何避免 goroutine 泄漏？",
			answer: "1. 使用 context 控制生命周期，通过 cancel 或 timeout 终止 goroutine；2. 使用 done channel 通知退出；3. 确保 channel 正确关闭，避免接收方永久阻塞；4. 使用 errgroup 管理一组 goroutine；5. 设置合理的超时时间；6. 使用 runtime.NumGoroutine() 监控 goroutine 数量。",
			expected: "常见泄漏场景：1) channel 未关闭，接收方永久阻塞 2) 缺少 cancel/timeout，goroutine 永远等待 3) 死锁（A 等 B，B 等 A）4) 无限循环无退出条件。防护措施：context.WithCancel/WithTimeout、done channel（chan struct{}）、errgroup.Group、runtime.NumGoroutine() 监控、go vet 检测。代码模式：select case <-ctx.Done(): return。",
			keywords:   []string{"context", "done", "channel", "超时", "errgroup"},
			difficulty: "medium",
			category:   "Concurrency",
		},
		{
			question: "Go 的内存分配是如何工作的？",
			answer: "Go 使用三级内存分配：mcache（线程缓存）-> mcentral（中心缓存）-> mheap（堆）。小对象 < 32KB 从 mcache 分配，中等对象从 mcentral，大对象直接从 mheap。逃逸分析决定分配在栈还是堆。栈分配更快，无需 GC。",
			expected: "三级分配器：mcache（P 本地，无锁）→ mcentral（全局，加锁）→ mheap（全局，页管理）。对象分类：tiny (<16B, 合并分配)、small (16B-32KB, span 分配)、large (>32KB, 直接 mheap)。逃逸分析：编译期判断，变量逃出函数→堆分配，否则→栈分配。go build -gcflags=-m 查看逃逸。栈：连续栈（Go 1.4+），2KB→可增长→可收缩。",
			keywords:   []string{"mcache", "mcentral", "mheap", "大小", "逃逸"},
			difficulty: "medium",
			category:   "Memory",
		},
	}

	// 执行面试
	totalScore := 0.0
	for i, qa := range qaList {
		fmt.Printf("┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓")
		fmt.Printf("\n┃ 第 %d 轮 | %s | 难度: %s", i+1, qa.category, qa.difficulty)
		fmt.Printf("\n┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛\n\n")

		// 面试官提问
		fmt.Printf("🎓 【面试官提问】\n")
		fmt.Printf("┌──────────────────────────────────────────────────────────────────────┐\n")
		fmt.Printf("│ %s\n", qa.question)
		fmt.Printf("└──────────────────────────────────────────────────────────────────────┘\n\n")

		// 候选人回答
		fmt.Printf("👤 【候选人回答】\n")
		fmt.Printf("┌──────────────────────────────────────────────────────────────────────┐\n")
		// 自动换行
		words := qa.answer
		for len(words) > 0 {
			if len(words) > 66 {
				fmt.Printf("│ %s\n", words[:66])
				words = words[66:]
			} else {
				fmt.Printf("│ %s\n", words)
				break
			}
		}
		fmt.Printf("└──────────────────────────────────────────────────────────────────────┘\n\n")

		// 参考答案
		fmt.Printf("📚 【参考答案】\n")
		fmt.Printf("┌──────────────────────────────────────────────────────────────────────┐\n")
		expected := qa.expected
		for len(expected) > 0 {
			if len(expected) > 66 {
				fmt.Printf("│ %s\n", expected[:66])
				expected = expected[66:]
			} else {
				fmt.Printf("│ %s\n", expected)
				break
			}
		}
		fmt.Printf("└──────────────────────────────────────────────────────────────────────┘\n\n")

		// 评分
		score := evaluateAnswer(qa.answer, qa.expected, qa.keywords)
		totalScore += score.Total

		fmt.Printf("📊 【评分反馈】\n")
		fmt.Printf("┌──────────────────────────────────────────────────────────────────────┐\n")
		fmt.Printf("│ 正确性: %s %d/10\n", renderBar(score.Correctness, 10), score.Correctness)
		fmt.Printf("│ 深  度: %s %d/10\n", renderBar(score.Depth, 10), score.Depth)
		fmt.Printf("│ 清晰度: %s %d/10\n", renderBar(score.Clarity, 10), score.Clarity)
		fmt.Printf("│ 实用性: %s %d/10\n", renderBar(score.Practical, 10), score.Practical)
		fmt.Printf("│ 综合分: %.1f/100\n", score.Total)
		fmt.Printf("├──────────────────────────────────────────────────────────────────────┤\n")

		// 优点
		fmt.Printf("│ ✅ 优点:\n")
		for _, s := range score.Strengths {
			fmt.Printf("│    • %s\n", s)
		}

		// 不足
		fmt.Printf("│ ❌ 不足:\n")
		for _, w := range score.Weaknesses {
			fmt.Printf("│    • %s\n", w)
		}

		// 遗漏
		if len(score.Missing) > 0 {
			fmt.Printf("│ ⚠️  遗漏:\n")
			for _, m := range score.Missing {
				fmt.Printf("│    • %s\n", m)
			}
		}

		fmt.Printf("└──────────────────────────────────────────────────────────────────────┘\n")
		fmt.Println()
		fmt.Println(strings.Repeat("═", 72))
		fmt.Println()
	}

	// 最终报告
	avgScore := totalScore / float64(len(qaList))

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                           最终评估报告                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  面试轮次: %d / %d\n", len(qaList), len(qaList))
	fmt.Printf("  综合评分: %.1f / 100\n\n", avgScore)

	fmt.Println("  维度评分:")
	fmt.Printf("    正确性: %s %.0f/10\n", renderBar(int(avgScore/10), 10), avgScore/10)
	fmt.Println()

	fmt.Println("  评估结果:")
	if avgScore >= 80 {
		fmt.Println("    技术评级: 高级 (Senior)")
		fmt.Println("    面试结果: ✓ 通过")
	} else if avgScore >= 60 {
		fmt.Println("    技术评级: 中级 (Mid)")
		fmt.Println("    面试结果: ✓ 通过")
	} else {
		fmt.Println("    技术评级: 初级 (Junior)")
		fmt.Println("    面试结果: ✗ 未通过")
	}

	fmt.Println()
	fmt.Println("  优势领域:")
	fmt.Println("    ✓ GMP 调度模型理解清晰")
	fmt.Println("    ✓ GC 三色标记法掌握扎实")
	fmt.Println("    ✓ Channel 实现原理理解到位")

	fmt.Println()
	fmt.Println("  薄弱环节:")
	fmt.Println("    ✗ 缺少源码层面的深入分析")
	fmt.Println("    ✗ 实战经验描述不够具体")
	fmt.Println("    ✗ 边界情况考虑不全面")

	fmt.Println()
	fmt.Println("  学习建议:")
	fmt.Println("    1. 阅读 Go Runtime 源码 (src/runtime/)")
	fmt.Println("    2. 实践高并发项目，积累性能调优经验")
	fmt.Println("    3. 学习《Go 语言设计与实现》深入理解原理")
	fmt.Println("    4. 关注 Go 官方博客，了解最新 GC 优化")

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════════════")
	fmt.Println("                            面试结束")
	fmt.Println("════════════════════════════════════════════════════════════════════════")
}

// evaluateAnswer 评估答案
func evaluateAnswer(answer, expected string, keywords []string) routine.Score {
	score := routine.Score{
		Correctness: 6,
		Depth:       5,
		Clarity:     7,
		Practical:   5,
		Strengths:   []string{},
		Weaknesses:  []string{},
		Missing:     []string{},
	}

	lowerAnswer := strings.ToLower(answer)

	// 关键词匹配
	matched := 0
	for _, kw := range keywords {
		if strings.Contains(lowerAnswer, strings.ToLower(kw)) {
			matched++
		}
	}

	// 根据关键词匹配率调整正确性
	if len(keywords) > 0 {
		matchRate := float64(matched) / float64(len(keywords))
		score.Correctness = int(5 + matchRate*5)
		score.Depth = int(4 + matchRate*6)
	}

	// 回答长度影响深度分
	if len(answer) > 200 {
		score.Depth = min(10, score.Depth+1)
		score.Strengths = append(score.Strengths, "回答详细")
	}
	if len(answer) > 400 {
		score.Practical = min(10, score.Practical+1)
	}

	// 检查是否包含代码示例
	if strings.Contains(answer, "func ") || strings.Contains(answer, "go ") {
		score.Practical = min(10, score.Practical+1)
		score.Strengths = append(score.Strengths, "包含代码示例")
	}

	// 设置优点
	if score.Correctness >= 8 {
		score.Strengths = append(score.Strengths, "答案准确")
	}
	if score.Depth >= 7 {
		score.Strengths = append(score.Strengths, "理解深入")
	}
	if score.Clarity >= 8 {
		score.Strengths = append(score.Strengths, "表达清晰")
	}

	// 设置不足
	if score.Correctness < 7 {
		score.Weaknesses = append(score.Weaknesses, "部分概念不够准确")
	}
	if score.Depth < 6 {
		score.Weaknesses = append(score.Weaknesses, "理解不够深入")
	}

	// 设置遗漏点
	expectedLower := strings.ToLower(expected)
	importantKeywords := []string{"源码", "底层", "原理", "实现", "优化"}
	for _, kw := range importantKeywords {
		if strings.Contains(expectedLower, kw) && !strings.Contains(lowerAnswer, kw) {
			score.Missing = append(score.Missing, fmt.Sprintf("缺少%s层面的分析", kw))
		}
	}

	// 计算总分
	score.Total = float64(score.Correctness+score.Depth+score.Clarity+score.Practical) / 4 * 10

	return score
}

func renderBar(value, max int) string {
	if value > max {
		value = max
	}
	filled := value * 20 / max
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
	return bar
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
