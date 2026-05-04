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

	write("# Go 模拟面试 — LLM 完整记录\n\n")
	write("> 时间: %s\n> 引擎: Harness Routine + Claude CLI\n\n---\n\n")

	llm := routine.NewClaudeCLIProvider()
	engine := routine.NewRoutineEngine(routine.EngineConfig{EnableScoring: true})
	engine.SetLLMProvider(llm)

	config := routine.RoutineConfig{
		Name: "Go面试", Type: routine.TypeInterview,
		Settings: routine.RoutineSettings{MaxRounds: 3, Timeout: 10 * time.Minute},
	}

	inst, _ := engine.Create(context.Background(), config)
	fmt.Printf("面试: %s | 文件: %s\n\n", inst.ID, outPath)

	// 启动面试 → 第一题
	engine.Start(context.Background(), inst.ID)
	time.Sleep(15 * time.Second)
	saved := dumpAll(engine, inst.ID, 0)

	for round := 1; round <= 3; round++ {
		fmt.Printf("\n--- 第 %d 轮 ---\n", round)

		// 生成详细答案
		write("\n---\n\n## 📝 第 %d 轮 — LLM 生成答案\n\n", round)
		fmt.Printf("📝 生成答案中...\n")
		genAnswer(llm, inst, round)

		// 提交答案 → 触发评估 + 追问 + 下一题
		fmt.Printf("📤 提交答案，等待评估+追问+下一题...\n")
		engine.SubmitAnswer(context.Background(), inst.ID, fmt.Sprintf("第%d轮完成", round))

		// 固定等待: 评估(~15s) + 追问(~15s) + 下一题(~15s) = ~45s
		time.Sleep(60 * time.Second)
		saved = dumpAll(engine, inst.ID, saved)
	}

	// 最终分析
	fmt.Printf("\n📋 最终分析...\n")
	engine.SubmitAnswer(context.Background(), inst.ID, "__finalize__")
	time.Sleep(30 * time.Second)
	dumpAll(engine, inst.ID, saved)

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

func genAnswer(llm routine.LLMProvider, inst *routine.RoutineInstance, round int) {
	history := inst.GetHistory()
	var question string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "interviewer" {
			question = history[i].Content
			break
		}
	}

	system := `你是 Go 语言面试专家。生成三个版本的答案：
1. **详细完整答案**：核心原理+代码示例+边界情况+最佳实践
2. **一句话总结**：一句话概括
3. **口头简答**：2-3句口语化

格式：
### 详细完整答案
(内容)
### 一句话总结
(一句话)
### 口头简答
(2-3句)`

	messages := []routine.LLMMessage{
		{Role: "user", Content: fmt.Sprintf("题目：\n%s\n\n生成三个版本。", question)},
	}
	resp, err := llm.ChatWithSystem(context.Background(), system, messages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成失败: %v\n", err)
		return
	}
	write("%s\n\n", resp)
	fmt.Printf("✅ 答案已生成\n")
}

func dumpAll(engine *routine.DefaultRoutineEngine, id string, lastSeen int) int {
	inst, _ := engine.GetInstance(context.Background(), id)
	history := inst.GetHistory()
	for i := lastSeen; i < len(history); i++ {
		msg := history[i]
		switch msg.Role {
		case "interviewer":
			write("## 🎤 面试官提问\n\n%s\n\n", msg.Content)
			fmt.Printf("🎤 %s\n", trunc(msg.Content, 100))
		case "evaluator":
			write("## 📊 LLM 评估\n\n%s\n\n", msg.Content)
			fmt.Printf("📊 %s\n", trunc(msg.Content, 100))
		case "followup_generator":
			write("## 🔄 LLM 追问\n\n%s\n\n", msg.Content)
			fmt.Printf("🔄 %s\n", trunc(msg.Content, 100))
		case "knowledge_gap_analyzer":
			write("## 📋 最终分析\n\n%s\n\n", msg.Content)
			fmt.Printf("📋 %s\n", trunc(msg.Content, 100))
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
