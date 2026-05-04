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

	write("# Go 模拟面试 — LLM 生成完整答案\n\n")
	write("> 时间: %s\n> 引擎: Harness Routine + Claude CLI\n\n---\n\n")

	llm := routine.NewClaudeCLIProvider()

	config := routine.RoutineConfig{
		Name: "Go面试", Type: routine.TypeInterview,
		Settings: routine.RoutineSettings{MaxRounds: 5, Timeout: 10 * time.Minute},
	}

	engine := routine.NewRoutineEngine(routine.EngineConfig{EnableScoring: true})
	engine.SetLLMProvider(llm)

	inst, _ := engine.Create(context.Background(), config)
	fmt.Printf("面试: %s | 文件: %s\n\n", inst.ID, outPath)

	engine.Start(context.Background(), inst.ID)
	waitStable(engine, inst.ID, 0)
	saved := dumpNew(engine, inst.ID, 0)

	// 每轮: LLM 提问 → LLM 生成完整答案 + 总结 + 口头版
	for round := 1; round <= 5; round++ {
		fmt.Printf("\n--- 第 %d 轮 ---\n", round)

		// LLM 生成完整答案
		write("\n---\n\n## 📝 第 %d 轮 — LLM 生成答案\n\n", round)
		fmt.Printf("📝 生成答案中...\n")

		genAnswer(llm, inst, round)
		waitStable(engine, inst.ID, saved)
		saved = dumpNew(engine, inst.ID, saved)

		// 提交触发下一题
		engine.SubmitAnswer(context.Background(), inst.ID, fmt.Sprintf("第%d轮完成，请继续", round))
		waitStable(engine, inst.ID, saved)
		saved = dumpNew(engine, inst.ID, saved)
	}

	// 最终总结
	write("\n---\n\n## 📊 面试结束\n\n")
	state, _ := engine.GetInstance(context.Background(), inst.ID)
	write("总轮次: %d\n", state.Round)

	fmt.Printf("\n✅ 已保存: %s\n", outPath)
}

// genAnswer 让 LLM 生成详细答案 + 总结 + 口头版
func genAnswer(llm routine.LLMProvider, inst *routine.RoutineInstance, round int) {
	// 获取最新问题
	history := inst.GetHistory()
	var question string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "interviewer" {
			question = history[i].Content
			break
		}
	}

	system := `你是 Go 语言面试专家。请根据面试题目，生成三个版本的答案：

1. **详细完整答案**：包含核心原理、代码示例、边界情况、最佳实践。结构清晰，适合学习。
2. **一句话总结**：用一句话概括核心要点，便于记忆。
3. **口头简答**：模拟候选人面试时的口头回答，2-3 句话，自然口语化，突出关键点。

请严格按以下格式输出：

### 详细完整答案
（完整内容，含代码）

### 一句话总结
（一句话）

### 口头简答
（2-3 句口语化回答）`

	messages := []routine.LLMMessage{
		{Role: "user", Content: fmt.Sprintf("面试题目：\n%s\n\n请生成三个版本的答案。", question)},
	}

	resp, err := llm.ChatWithSystem(context.Background(), system, messages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成失败: %v\n", err)
		return
	}

	write("%s\n\n", resp)
	fmt.Printf("✅ 答案已生成\n")
}

func waitStable(engine *routine.DefaultRoutineEngine, id string, lastSeen int) {
	prevCount := lastSeen
	idle := 0
	for i := 0; i < 120; i++ {
		time.Sleep(5 * time.Second)
		inst, _ := engine.GetInstance(context.Background(), id)
		current := len(inst.GetHistory())
		if current == prevCount {
			idle++
			if idle >= 2 {
				return
			}
		} else {
			idle = 0
			prevCount = current
			fmt.Printf("  ... %d 条消息\n", current)
		}
		if inst.Status == routine.StatusCompleted {
			time.Sleep(3 * time.Second)
			return
		}
	}
}

func dumpNew(engine *routine.DefaultRoutineEngine, id string, lastSeen int) int {
	inst, _ := engine.GetInstance(context.Background(), id)
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
			write("## 🔄 追问/下一题\n\n%s\n\n", msg.Content)
			fmt.Printf("🔄 %s\n", trunc(msg.Content, 120))
		case "knowledge_gap_analyzer":
			write("## 📋 分析\n\n%s\n\n", msg.Content)
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
