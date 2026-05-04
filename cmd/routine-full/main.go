package main

import (
	"context"
	"fmt"
	"time"

	"github.com/harness-engineering/harness/pkg/routine"
)

func main() {
	llm := routine.NewClaudeCLIProvider()
	engine := routine.NewRoutineEngine(routine.EngineConfig{EnableScoring: true})
	engine.SetLLMProvider(llm)

	config := routine.RoutineConfig{
		Name: "Go面试 Routine",
		Type: routine.TypeInterview,
		Settings: routine.RoutineSettings{MaxRounds: 3, Timeout: 10 * time.Minute},
	}

	inst, _ := engine.Create(context.Background(), config)
	engine.Start(context.Background(), inst.ID)
	time.Sleep(15 * time.Second)

	state, _ := engine.GetInstance(context.Background(), inst.ID)
	lastSeen := 0
	printNew(state, &lastSeen)

	answers := []string{
		"slice底层是一个三字段结构体：ptr指向底层数组、len是当前长度、cap是容量。append时先检查cap是否足够，够则直接在底层数组追加并更新len；不够则触发扩容——分配更大的新数组，拷贝旧元素，追加新元素。扩容策略：cap<256时翻倍，>=256时约1.25倍增长。子切片与原slice共享底层数组，可能导致数据意外修改。",
		"Mutex是互斥锁，读写都互斥。RWMutex允许读并行，写独占。Go 1.18引入了饥饿模式防止长时间等待。sync.Map适合读多写少场景，内部用read-only map+dirty map实现，通过原子操作提升读性能。channel底层是hchan结构体，包含环形缓冲区、发送/接收队列和互斥锁。",
		"defer按LIFO顺序执行。关键点：参数在声明时求值而非执行时；defer可以修改命名返回值；循环中的defer可能导致资源泄漏，应用匿名函数包裹。panic时defer仍会执行，recover只在defer中有效。Go 1.21引入了defer的性能优化，小函数内联。",
	}

	for i, answer := range answers {
		fmt.Printf("\n--- Round %d ---\n", i+1)
		fmt.Printf("ANSWER:\n%s\n", answer)
		engine.SubmitAnswer(context.Background(), inst.ID, answer)
		time.Sleep(30 * time.Second)

		state, _ = engine.GetInstance(context.Background(), inst.ID)
		printNew(state, &lastSeen)
	}

	fmt.Printf("\n--- Final Analysis ---\n")
	engine.SubmitAnswer(context.Background(), inst.ID, "__finalize__")
	time.Sleep(20 * time.Second)

	state, _ = engine.GetInstance(context.Background(), inst.ID)
	printNew(state, &lastSeen)

	fmt.Printf("\n=== SCORES ===\n")
	for _, s := range state.Scores {
		fmt.Printf("Round %d: %.1f/100\n", s.Round+1, s.Score.Total)
	}
}

func printNew(inst *routine.RoutineInstance, lastSeen *int) {
	for _, msg := range inst.GetHistory()[*lastSeen:] {
		switch msg.Role {
		case "interviewer":
			fmt.Printf("\nQUESTION:\n%s\n", msg.Content)
		case "evaluator":
			fmt.Printf("\nEVALUATION:\n%s\n", msg.Content)
		case "followup_generator":
			fmt.Printf("\nFOLLOWUP:\n%s\n", msg.Content)
		case "knowledge_gap_analyzer":
			fmt.Printf("\nANALYSIS:\n%s\n", msg.Content)
		}
	}
	*lastSeen = len(inst.GetHistory())
}
