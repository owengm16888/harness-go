package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/harness-engineering/harness/pkg/routine"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║           Go 后端工程师面试 - Routine 演示              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 创建引擎
	engine := routine.NewRoutineEngine(routine.EngineConfig{
		EnableScoring:  true,
		EnableFollowup: true,
	})

	// 面试配置
	config := routine.RoutineConfig{
		Name:        "Go后端工程师面试",
		Description: "模拟真实技术面试",
		Type:        routine.TypeInterview,
		Agents: map[string]routine.Agent{
			"interviewer": {
				Role: routine.RoleInterviewer,
				Name: "interviewer",
			},
			"evaluator": {
				Role: routine.RoleEvaluator,
				Name: "evaluator",
			},
			"followup_generator": {
				Role: routine.RoleFollowup,
				Name: "followup_generator",
			},
			"knowledge_gap_analyzer": {
				Role: routine.RoleAnalyzer,
				Name: "knowledge_gap_analyzer",
			},
		},
		Workflow: []routine.WorkflowStep{
			{Name: "start", Agent: "interviewer", Action: "ask_question"},
			{Name: "evaluate", Agent: "evaluator", Action: "evaluate_answer"},
			{Name: "loop", Agent: "interviewer", Action: "ask_question", Until: "max_rounds_reached"},
			{Name: "report", Agent: "knowledge_gap_analyzer", Action: "final_review"},
		},
		Settings: routine.RoutineSettings{
			MaxRounds:    5,
			Timeout:      30 * time.Minute,
			EnableScoring: true,
		},
		Input: map[string]any{
			"focus": "Go 并发编程",
			"level": "mid",
		},
	}

	// 创建实例
	instance, err := engine.Create(nil, config)
	if err != nil {
		fmt.Printf("创建失败: %v\n", err)
		return
	}

	fmt.Printf("面试ID: %s\n", instance.ID)
	fmt.Printf("面试类型: %s\n", config.Type)
	fmt.Printf("最大轮次: %d\n\n", config.Settings.MaxRounds)

	// 模拟面试问答
	qa := []struct {
		question string
		answer   string
	}{
		{
			"请介绍一下 Go 中的 GMP 调度模型",
			"GMP 是 Go 的调度模型。G 代表 Goroutine，是 Go 的轻量级线程；M 代表 Machine，是操作系统线程；P 代表 Processor，是逻辑处理器。P 持有本地运行队列，M 必须绑定 P 才能执行 G。当 G 阻塞时，M 会释放 P，P 被其他 M 获取，从而实现高效并发。",
		},
		{
			"请解释一下 Go 的垃圾回收机制",
			"Go 使用三色标记法进行垃圾回收。白色表示未标记的对象，灰色表示已标记但子对象未扫描的对象，黑色表示已标记且子对象已扫描的对象。GC 从根对象开始扫描，逐步将白色对象标记为灰色，再标记为黑色。使用写屏障防止并发标记时丢失对象。STW 用于标记开始和结束阶段。",
		},
		{
			"Go 中 channel 的底层实现原理是什么？",
			"Channel 底层是一个环形队列，使用互斥锁保护。发送和接收操作都会检查是否有等待的 goroutine。如果有等待的接收方，直接传递数据；否则放入缓冲区。无缓冲 channel 要求发送和接收同时就绪。select 语句可以同时等待多个 channel 操作。",
		},
		{
			"sync.Map 和普通 map+RWMutex 有什么区别？",
			"sync.Map 适用于读多写少的场景，使用 read-only map 和 dirty map 双层结构。读操作先查 read map（无锁），miss 后查 dirty map（加锁）。当 miss 次数超过 dirty 长度时，提升 dirty 为 read。普通 map+RWMutex 适用于写多场景，因为 sync.Map 写操作需要加锁更新 dirty。",
		},
		{
			"如何避免 goroutine 泄漏？",
			"1. 使用 context 控制生命周期，通过 cancel 或 timeout 终止 goroutine；2. 使用 done channel 通知退出；3. 确保 channel 正确关闭，避免接收方永久阻塞；4. 使用 errgroup 管理一组 goroutine；5. 设置合理的超时时间；6. 使用 runtime.NumGoroutine() 监控 goroutine 数量。",
		},
	}

	// 执行面试
	for i, qa := range qa {
		fmt.Printf("━━━ 第 %d 轮 ━━━\n", i+1)
		fmt.Printf("\n【面试官】\n%s\n\n", qa.question)

		// 添加消息
		instance.AddMessage(routine.Message{
			Role:    "interviewer",
			Content: qa.question,
			Round:   i,
		})

		fmt.Printf("【候选人】\n%s\n\n", qa.answer)

		instance.AddMessage(routine.Message{
			Role:    "candidate",
			Content: qa.answer,
			Round:   i,
		})

		// 模拟评分
		score := routine.Score{
			Correctness: 7 + i%3,
			Depth:       6 + i%4,
			Clarity:     8 + i%2,
			Practical:   5 + i%5,
		}
		score.Total = float64(score.Correctness+score.Depth+score.Clarity+score.Practical) / 4 * 10

		instance.AddScore(routine.RoundScore{
			Round:    i + 1,
			Question: qa.question,
			Score:    score,
		})

		fmt.Printf("【评分】\n")
		fmt.Printf("  正确性: %d/10\n", score.Correctness)
		fmt.Printf("  深度:   %d/10\n", score.Depth)
		fmt.Printf("  清晰度: %d/10\n", score.Clarity)
		fmt.Printf("  实用性: %d/10\n", score.Practical)
		fmt.Printf("  综合分: %.1f/100\n\n", score.Total)

		fmt.Println(strings.Repeat("─", 50))
	}

	// 生成报告
	instance.SetStatus(routine.StatusCompleted)

	avg := instance.GetAverageScore()

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║                    最终评估报告                         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  面试名称: %s\n", config.Name)
	fmt.Printf("  面试轮次: %d / %d\n", len(qa), config.Settings.MaxRounds)
	fmt.Printf("  综合评分: %.1f / 100\n\n", avg.Total)

	fmt.Println("  维度评分:")
	fmt.Printf("    正确性: %s %d/10\n", renderBar(avg.Correctness, 10), avg.Correctness)
	fmt.Printf("    深  度: %s %d/10\n", renderBar(avg.Depth, 10), avg.Depth)
	fmt.Printf("    清晰度: %s %d/10\n", renderBar(avg.Clarity, 10), avg.Clarity)
	fmt.Printf("    实用性: %s %d/10\n", renderBar(avg.Practical, 10), avg.Practical)

	fmt.Println()
	fmt.Println("  评估结果:")
	if avg.Total >= 70 {
		fmt.Println("    技术评级: 中级 (Mid)")
		fmt.Println("    面试结果: ✓ 通过")
	} else if avg.Total >= 50 {
		fmt.Println("    技术评级: 初级 (Junior)")
		fmt.Println("    面试结果: ✓ 通过 (需提升)")
	} else {
		fmt.Println("    技术评级: 待提升")
		fmt.Println("    面试结果: ✗ 未通过")
	}

	fmt.Println()
	fmt.Println("  优势领域:")
	fmt.Println("    ✓ 基础知识扎实")
	fmt.Println("    ✓ 理解 GMP 调度模型")
	fmt.Println("    ✓ 掌握 GC 原理")

	fmt.Println()
	fmt.Println("  学习建议:")
	fmt.Println("    1. 深入学习 Go Runtime 源码")
	fmt.Println("    2. 实践高并发项目")
	fmt.Println("    3. 阅读《Go 语言设计与实现》")

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("                    面试结束")
	fmt.Println("═══════════════════════════════════════════════════════════")
}

func renderBar(value, max int) string {
	filled := value * 20 / max
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
	return bar
}
