package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/harness-engineering/harness/pkg/routine"
)

func main() {
	fmt.Println("=== Go 模拟面试 Routine (LLM 模式) ===")
	fmt.Println()

	// 创建 LLM 提供者
	llm := routine.NewClaudeCLIProvider()
	fmt.Printf("LLM: %s\n", llm.Name())

	// 创建引擎并注入 LLM
	engine := routine.NewRoutineEngine(routine.EngineConfig{
		EnableScoring: true,
	})
	engine.SetLLMProvider(llm)

	// 面试配置
	config := routine.RoutineConfig{
		Name: "Go面试模拟",
		Type: routine.TypeInterview,
		Settings: routine.RoutineSettings{
			MaxRounds: 3,
			Timeout:   10 * time.Minute,
		},
	}

	instance, _ := engine.Create(context.Background(), config)
	fmt.Printf("面试实例: %s | 最大轮次: %d\n", instance.ID, config.Settings.MaxRounds)
	fmt.Println(strings.Repeat("=", 50))

	// 启动面试
	engine.Start(context.Background(), instance.ID)
	time.Sleep(8 * time.Second)

	inst, _ := engine.GetInstance(context.Background(), instance.ID)
	lastSeen := printNewMessages(inst, 0)

	// 模拟回答 (实际场景由用户输入)
	answers := []string{
		"GMP 是 Go 的调度模型。G 是 goroutine，M 是系统线程，P 是逻辑处理器。P 持有本地运行队列，M 绑定 P 才能执行 G。当 G 阻塞时 M 释放 P，其他 M 获取 P 继续执行。全局队列和本地队列通过 work stealing 平衡。Go 1.14 引入了基于信号的异步抢占。",
		"Mutex 是互斥锁，读写都互斥。RWMutex 允许读并行，写独占。Go 1.18 引入了饥饿模式，防止长时间等待。sync.Map 适合读多写少场景，内部用 read-only map + dirty map 实现。",
		"defer 按 LIFO 顺序执行。关键点：参数在声明时求值，可修改命名返回值，循环中 defer 可能导致资源泄漏。panic 时 defer 仍执行，recover 只在 defer 中有效。Go 1.21 引入了 defer 的性能优化。",
	}

	for i, answer := range answers {
		fmt.Printf("\n%s\n", strings.Repeat("-", 50))
		fmt.Printf("👤 第 %d 轮回答\n", i+1)

		engine.SubmitAnswer(context.Background(), instance.ID, answer)
		time.Sleep(10 * time.Second)

		inst, _ = engine.GetInstance(context.Background(), instance.ID)
		lastSeen = printNewMessages(inst, lastSeen)

		if inst.Status == routine.StatusCompleted {
			break
		}
	}

	// 最终分析
	fmt.Printf("\n%s\n", strings.Repeat("-", 50))
	engine.SubmitAnswer(context.Background(), instance.ID, "__finalize__")
	time.Sleep(10 * time.Second)

	inst, _ = engine.GetInstance(context.Background(), instance.ID)
	printNewMessages(inst, lastSeen)

	// 评分统计
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("📊 面试统计")
	fmt.Println(strings.Repeat("=", 50))

	scores := inst.Scores
	if len(scores) > 0 {
		var total float64
		for _, s := range scores {
			fmt.Printf("  第 %d 轮: 正确性%.0f 深度%.0f 清晰度%.0f 实用性%.0f | 综合 %.1f\n",
				s.Round+1, s.Score.Correctness, s.Score.Depth,
				s.Score.Clarity, s.Score.Practical, s.Score.Total)
			total += s.Score.Total
		}
		fmt.Printf("\n  平均分: %.1f/100\n", total/float64(len(scores)))
	}
	fmt.Printf("  总轮次: %d\n", inst.Round)
}

func printNewMessages(inst *routine.RoutineInstance, lastSeen int) int {
	history := inst.GetHistory()
	for i := lastSeen; i < len(history); i++ {
		msg := history[i]
		switch msg.Role {
		case "interviewer":
			fmt.Printf("\n🎤 面试官: %s\n", msg.Content)
		case "evaluator":
			fmt.Printf("\n📊 评估:\n%s\n", msg.Content)
		case "followup_generator":
			fmt.Printf("\n🔄 追问: %s\n", msg.Content)
		case "knowledge_gap_analyzer":
			fmt.Printf("\n📋 分析:\n%s\n", msg.Content)
		case "candidate":
			preview := msg.Content
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			fmt.Printf("   💬 %s\n", preview)
		}
	}
	return len(history)
}
