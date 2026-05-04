package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/harness-engineering/harness/pkg/routine"
)

var file *os.File

func main() {
	ts := time.Now().Format("20060102-150405")
	outPath := fmt.Sprintf("docs/interview/routine-session-%s.md", ts)
	f, _ := os.Create(outPath)
	file = f
	defer file.Close()

	write("# Go 模拟面试记录 (LLM Routine)\n\n")
	write("> 生成时间: '%s'\n> 引擎: Harness Routine + Claude CLI LLM\n\n---\n\n", time.Now().Format("2006-01-02 15:04:05"))

	llm := routine.NewClaudeCLIProvider()
	engine := routine.NewRoutineEngine(routine.EngineConfig{EnableScoring: true})
	engine.SetLLMProvider(llm)

	config := routine.RoutineConfig{
		Name: "Go面试 Routine", Type: routine.TypeInterview,
		Settings: routine.RoutineSettings{MaxRounds: 3, Timeout: 10 * time.Minute},
	}

	inst, _ := engine.Create(context.Background(), config)
	fmt.Printf("面试: %s | 文件: %s\n\n", inst.ID, outPath)

	engine.Start(context.Background(), inst.ID)
	saved := waitAndDump(engine, inst.ID, 0)

	answers := []string{
		"slice底层是一个三字段结构体：ptr指向底层数组、len是当前长度、cap是容量。append时先检查cap是否足够，够则直接在底层数组追加并更新len；不够则触发扩容——分配更大的新数组，拷贝旧元素，追加新元素。扩容策略：cap<256时翻倍，>=256时约1.25倍增长。子切片与原slice共享底层数组，可能导致数据意外修改。",
		"Mutex是互斥锁，读写都互斥。RWMutex允许读并行，写独占。Go 1.18引入了饥饿模式防止长时间等待。sync.Map适合读多写少场景，内部用read-only map+dirty map实现，通过原子操作提升读性能。channel底层是hchan结构体，包含环形缓冲区、发送/接收队列和互斥锁。",
		"defer按LIFO顺序执行。关键点：参数在声明时求值而非执行时；defer可以修改命名返回值；循环中的defer可能导致资源泄漏，应用匿名函数包裹。panic时defer仍会执行，recover只在defer中有效。Go 1.21引入了defer的性能优化，小函数内联。",
	}

	for i, answer := range answers {
		write("\n---\n\n## 👤 第 %d 轮候选人回答\n\n%s\n\n", i+1, answer)
		fmt.Printf("👤 第 %d 轮回答已写入\n", i+1)
		engine.SubmitAnswer(context.Background(), inst.ID, answer)
		saved = waitAndDump(engine, inst.ID, saved)
	}

	engine.SubmitAnswer(context.Background(), inst.ID, "__finalize__")
	saved = waitAndDump(engine, inst.ID, saved)

	// 评分统计
	state, _ := engine.GetInstance(context.Background(), inst.ID)
	write("\n---\n\n## 📊 面试统计\n\n| 轮次 | 得分 |\n|------|------|\n")
	var total float64
	for _, s := range state.Scores {
		write("| %d | %.1f/100 |\n", s.Round+1, s.Score.Total)
		total += s.Score.Total
	}
	if len(state.Scores) > 0 {
		write("| **平均** | **%.1f/100** |\n", total/float64(len(state.Scores)))
	}

	fmt.Printf("\n✅ 已保存: %s\n", outPath)
}

// waitAndDump 轮询等待消息数稳定，然后输出
func waitAndDump(engine *routine.DefaultRoutineEngine, id string, lastSeen int) int {
	prevCount := lastSeen
	idle := 0
	for i := 0; i < 120; i++ {
		time.Sleep(5 * time.Second) // 每 5 秒检查一次
		inst, _ := engine.GetInstance(context.Background(), id)
		current := len(inst.GetHistory())

		if current == prevCount {
			idle++
			if idle >= 2 { // 连续 10 秒无新消息 = 稳定
				return dumpNew(engine, inst, lastSeen)
			}
		} else {
			idle = 0
			prevCount = current
			fmt.Printf("  ... %d 条消息\n", current)
		}

		if inst.Status == routine.StatusCompleted {
			time.Sleep(3 * time.Second)
			return dumpNew(engine, inst, lastSeen)
		}
	}
	return dumpNew(engine, nil, lastSeen)
}

func dumpNew(engine *routine.DefaultRoutineEngine, inst *routine.RoutineInstance, lastSeen int) int {
	if inst == nil {
		id := ""
		for _, i := range func() []*routine.RoutineInstance {
			list, _ := engine.ListInstances(context.Background())
			return list
		}() {
			id = i.ID
			inst = i
			break
		}
		if inst == nil {
			return lastSeen
		}
		_ = id
	}

	history := inst.GetHistory()
	for i := lastSeen; i < len(history); i++ {
		msg := history[i]
		switch msg.Role {
		case "interviewer":
			write("## 🎤 面试官提问\n\n%s\n\n", msg.Content)
			fmt.Printf("🎤 %s\n", trunc(msg.Content, 120))
		case "evaluator":
			write("## 📊 LLM 评估\n\n%s\n\n", msg.Content)
			fmt.Printf("📊 %s\n", trunc(msg.Content, 120))
		case "followup_generator":
			write("## 🔄 LLM 追问\n\n%s\n\n", msg.Content)
			fmt.Printf("🔄 %s\n", trunc(msg.Content, 120))
		case "knowledge_gap_analyzer":
			write("## 📋 最终分析\n\n%s\n\n", msg.Content)
			fmt.Printf("📋 %s\n", trunc(msg.Content, 120))
		}
	}
	return len(history)
}

func write(format string, args ...any) {
	file.WriteString(fmt.Sprintf(format, args...))
	file.Sync()
}

func trunc(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
