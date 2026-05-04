package routine

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// ============================================================
// 面试官 Agent
// ============================================================

// InterviewerAgent 面试官 Agent
type InterviewerAgent struct {
	name     string
	provider LLMProvider
}

// NewInterviewerAgent 创建面试官 Agent
func NewInterviewerAgent() *InterviewerAgent {
	return &InterviewerAgent{name: "interviewer"}
}

func (a *InterviewerAgent) Name() string {
	return a.name
}

func (a *InterviewerAgent) Role() AgentRole {
	return RoleInterviewer
}

func (a *InterviewerAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	fmt.Fprintf(os.Stderr, "[DEBUG] interviewer.Execute: provider=%v, round=%d\n", a.provider != nil, input.Round)
	if a.provider != nil {
		return a.executeLLM(ctx, input)
	}
	if input.Round == 0 {
		return &AgentOutput{
			Content: a.getOpeningQuestion(input),
			NextAction: "wait_answer",
		}, nil
	}

	// 后续轮次：根据评分决定追问方向
	if input.Score != nil {
		return a.generateFollowupQuestion(input)
	}

	// 默认问题
	return &AgentOutput{
		Content: a.getDefaultQuestion(input),
		NextAction: "wait_answer",
	}, nil
}

func (a *InterviewerAgent) executeLLM(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	system := "你是一位资深Go语言面试官。规则：一次只问一个问题；回答正确提高难度；只输出问题；用中文提问。"
	var messages []LLMMessage
	for _, msg := range input.History {
		role := "user"
		if msg.Role == "interviewer" { role = "assistant" }
		messages = append(messages, LLMMessage{Role: role, Content: msg.Content})
	}
	if input.Round > 0 && input.Answer != "" {
		messages = append(messages, LLMMessage{Role: "user", Content: input.Answer})
	}
	if input.Round == 0 {
		messages = append(messages, LLMMessage{Role: "user", Content: "请开始面试，提出第一个Go技术问题。"})
	}
	resp, err := a.provider.ChatWithSystem(ctx, system, messages)
	if err != nil { return nil, err }
	return &AgentOutput{Content: resp, NextAction: "wait_answer"}, nil
}

// getOpeningQuestion 获取开场问题
func (a *InterviewerAgent) getOpeningQuestion(input AgentInput) string {
	// 从配置中获取面试范围
	focus, _ := input.Context["focus"].(string)
	role, _ := input.Context["role"].(string)

	if focus == "" {
		focus = "Go 后端开发"
	}
	if role == "" {
		role = "后端工程师"
	}

	return fmt.Sprintf(`【面试官】

欢迎参加今天的面试。请先做一个简单的自我介绍，重点介绍：

1. 你的 Go 语言开发经验
2. 做过的最有挑战性的项目
3. 在项目中遇到的技术难点及解决方案

（请开始你的自我介绍）`)
}

// generateFollowupQuestion 生成追问
func (a *InterviewerAgent) generateFollowupQuestion(input AgentInput) (*AgentOutput, error) {
	score := input.Score

	// 根据评分决定追问策略
	if score.Correctness < 5 {
		// 回答有误，纠正并追问
		return &AgentOutput{
			Content: fmt.Sprintf(`【面试官】

你的回答有一些不准确的地方。%s

让我换个角度问你：%s`, getWeaknessFeedback(score), getDeepQuestion(input)),
			NextAction: "wait_answer",
		}, nil
	}

	if score.Depth < 5 {
		// 回答太浅，深挖
		return &AgentOutput{
			Content: fmt.Sprintf(`【面试官】

你的回答方向是对的，但还不够深入。

%s

请更详细地解释：%s`, getMissingPoints(score), getDeepQuestion(input)),
			NextAction: "wait_answer",
		}, nil
	}

	// 回答不错，提升难度
	return &AgentOutput{
		Content: fmt.Sprintf(`【面试官】

回答得不错。%s

接下来我们深入一些：%s`, getStrengthFeedback(score), getHarderQuestion(input)),
		NextAction: "wait_answer",
	}, nil
}

// getDefaultQuestion 获取默认问题
func (a *InterviewerAgent) getDefaultQuestion(input AgentInput) string {
	topics := []string{
		"请解释一下 Go 中的 GMP 调度模型",
		"Go 的垃圾回收机制是如何工作的？",
		"请解释一下 Go 的 CSP 并发模型",
		"Go 中 channel 的底层实现原理是什么？",
		"sync.Map 和普通 map+RWMutex 有什么区别？",
	}

	if input.Round < len(topics) {
		return fmt.Sprintf("【面试官】\n\n%s", topics[input.Round])
	}

	return "【面试官】\n\n请介绍一下你对 Go 并发编程的理解。"
}

// ============================================================
// 评估 Agent
// ============================================================

// EvaluatorAgent 评估 Agent
type EvaluatorAgent struct {
	name     string
	provider LLMProvider
}

// NewEvaluatorAgent 创建评估 Agent
func NewEvaluatorAgent() *EvaluatorAgent {
	return &EvaluatorAgent{name: "evaluator"}
}

func (a *EvaluatorAgent) Name() string {
	return a.name
}

func (a *EvaluatorAgent) Role() AgentRole {
	return RoleEvaluator
}

func (a *EvaluatorAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	fmt.Fprintf(os.Stderr, "[DEBUG] evaluator.Execute: provider=%v\n", a.provider != nil)
	if a.provider != nil { return a.executeLLMEval(ctx, input) }
	score := a.evaluateAnswer(input.Question, input.Answer)

	return &AgentOutput{
		Content: formatScore(score),
		Score:   &score,
		NextAction: "next_step",
	}, nil
}

// evaluateAnswer 评估回答
func (a *EvaluatorAgent) evaluateAnswer(question, answer string) Score {
	// 简化的评估逻辑（实际应使用 LLM）
	score := Score{
		Correctness: 7,
		Depth:       6,
		Clarity:     7,
		Practical:   6,
		Strengths:   []string{},
		Weaknesses:  []string{},
		Missing:     []string{},
	}

	// 检查回答长度（简单启发式）
	if len(answer) < 50 {
		score.Depth = 4
		score.Weaknesses = append(score.Weaknesses, "回答过于简短")
	}

	if len(answer) > 500 {
		score.Depth = 8
		score.Strengths = append(score.Strengths, "回答详细")
	}

	// 检查关键词
	lowerAnswer := strings.ToLower(answer)

	keywords := map[string]int{
		"goroutine": 2,
		"channel":   2,
		"mutex":     1,
		"gc":        2,
		"gmp":       3,
		"调度":       2,
		"并发":       1,
		"内存":       1,
	}

	totalBonus := 0
	for keyword, bonus := range keywords {
		if strings.Contains(lowerAnswer, keyword) {
			totalBonus += bonus
		}
	}

	if totalBonus > 0 {
		score.Correctness = min(10, score.Correctness+totalBonus)
		score.Depth = min(10, score.Depth+totalBonus/2)
	}

	// 计算总分
	score.Total = float64(score.Correctness+score.Depth+score.Clarity+score.Practical) / 4 * 10

	// 根据分数设置反馈
	if score.Total >= 80 {
		score.Strengths = append(score.Strengths, "理解深入", "表达清晰")
	} else if score.Total >= 60 {
		score.Strengths = append(score.Strengths, "基础扎实")
		score.Missing = append(score.Missing, "需要更深入的理解")
	} else {
		score.Weaknesses = append(score.Weaknesses, "基础不够扎实")
		score.Missing = append(score.Missing, "需要系统学习")
	}

	return score
}

// ============================================================
// 追问生成器 Agent
// ============================================================

// FollowupAgent 追问生成器 Agent
type FollowupAgent struct {
	name     string
	provider LLMProvider
}

// NewFollowupAgent 创建追问生成器 Agent
func NewFollowupAgent() *FollowupAgent {
	return &FollowupAgent{name: "followup_generator"}
}

func (a *FollowupAgent) Name() string {
	return a.name
}

func (a *FollowupAgent) Role() AgentRole {
	return RoleFollowup
}

func (a *FollowupAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	if a.provider != nil { return a.executeLLMFollowup(ctx, input) }
	question := a.generateFollowup(input)

	return &AgentOutput{
		Content:    question,
		NextAction: "next_step",
	}, nil
}

// generateFollowup 生成追问
func (a *FollowupAgent) executeLLMFollowup(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	system := "你负责生成下一轮面试问题。基于上轮回答质量决定难度。只输出一个问题，用中文。"
	var messages []LLMMessage
	for _, msg := range input.History {
		role := "user"
		if msg.Role == "interviewer" || msg.Role == "followup_generator" { role = "assistant" }
		messages = append(messages, LLMMessage{Role: role, Content: msg.Content})
	}
	messages = append(messages, LLMMessage{Role: "user", Content: "请生成下一个面试问题。"})
	resp, err := a.provider.ChatWithSystem(ctx, system, messages)
	if err != nil { return nil, err }
	return &AgentOutput{Content: resp, NextAction: "next_step"}, nil
}

func (a *FollowupAgent) generateFollowup(input AgentInput) string {
	if input.Score == nil {
		return "请继续深入解释一下。"
	}

	// 优先追问薄弱点
	if len(input.Score.Missing) > 0 {
		return fmt.Sprintf("你提到了一些概念，但似乎遗漏了 %s。请详细解释一下这部分。", input.Score.Missing[0])
	}

	if len(input.Score.Weaknesses) > 0 {
		return fmt.Sprintf("关于 %s，能否举一个具体的例子来说明？", input.Score.Weaknesses[0])
	}

	// 深入追问
	return "你刚才的回答不错。能否从源码层面解释一下底层实现？"
}

// ============================================================
// 知识盲区分析器 Agent
// ============================================================

// AnalyzerAgent 知识盲区分析器 Agent
type AnalyzerAgent struct {
	name     string
	provider LLMProvider
}

// NewAnalyzerAgent 创建知识盲区分析器 Agent
func NewAnalyzerAgent() *AnalyzerAgent {
	return &AnalyzerAgent{name: "knowledge_gap_analyzer"}
}

func (a *AnalyzerAgent) Name() string {
	return a.name
}

func (a *AnalyzerAgent) Role() AgentRole {
	return RoleAnalyzer
}

func (a *AnalyzerAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	if a.provider != nil { return a.executeLLMAnalyze(ctx, input) }
	analysis := a.analyze(input)

	return &AgentOutput{
		Content:    formatAnalysis(analysis),
		Analysis:   &analysis,
		Done:       true,
		NextAction: "end",
	}, nil
}

// analyze 分析
func (a *AnalyzerAgent) executeLLMAnalyze(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	system := "你是面试分析官。根据面试历史生成最终评估。输出：技术评级(junior/mid/senior)、是否通过、优势领域、薄弱环节、学习计划、总结。用中文。"
	var messages []LLMMessage
	for _, msg := range input.History {
		role := "user"
		if msg.Role == "interviewer" || msg.Role == "evaluator" { role = "assistant" }
		messages = append(messages, LLMMessage{Role: role, Content: msg.Content})
	}
	messages = append(messages, LLMMessage{Role: "user", Content: "请生成面试最终评估报告。"})
	resp, err := a.provider.ChatWithSystem(ctx, system, messages)
	if err != nil { return nil, err }
	analysis := Analysis{Level: "mid", Pass: true, StrongAreas: []string{}, WeakAreas: []string{}, StudyPlan: []StudyItem{}, Summary: resp}
	return &AgentOutput{Content: resp, Analysis: &analysis, Done: true, NextAction: "end"}, nil
}

func (a *AnalyzerAgent) analyze(input AgentInput) Analysis {
	analysis := Analysis{
		StrongAreas: []string{},
		WeakAreas:   []string{},
		StudyPlan:   []StudyItem{},
	}

	// 统计各维度平均分
	var totalCorrectness, totalDepth, totalClarity, totalPractical int
	var count int

	for _, score := range input.Scores {
		totalCorrectness += score.Score.Correctness
		totalDepth += score.Score.Depth
		totalClarity += score.Score.Clarity
		totalPractical += score.Score.Practical
		count++
	}

	if count == 0 {
		analysis.Level = "junior"
		analysis.Pass = false
		analysis.Summary = "没有足够的数据进行评估"
		return analysis
	}

	avgCorrectness := totalCorrectness / count
	avgDepth := totalDepth / count
	avgClarity := totalClarity / count
	avgPractical := totalPractical / count

	// 判断水平
	avg := (avgCorrectness + avgDepth + avgClarity + avgPractical) / 4

	switch {
	case avg >= 8:
		analysis.Level = "senior"
		analysis.Pass = true
	case avg >= 6:
		analysis.Level = "mid"
		analysis.Pass = true
	default:
		analysis.Level = "junior"
		analysis.Pass = false
	}

	// 分析强项和弱项
	if avgCorrectness >= 7 {
		analysis.StrongAreas = append(analysis.StrongAreas, "基础知识扎实")
	} else {
		analysis.WeakAreas = append(analysis.WeakAreas, "基础知识需要加强")
		analysis.StudyPlan = append(analysis.StudyPlan, StudyItem{
			Topic:    "Go 基础语法和核心概念",
			Why:      "基础不够扎实，需要系统学习",
			Resource: "Go 官方文档 + 《Go 程序设计语言》",
			Priority: 1,
		})
	}

	if avgDepth >= 7 {
		analysis.StrongAreas = append(analysis.StrongAreas, "理解深入，能讲清原理")
	} else {
		analysis.WeakAreas = append(analysis.WeakAreas, "理解不够深入")
		analysis.StudyPlan = append(analysis.StudyPlan, StudyItem{
			Topic:    "Go Runtime 源码分析",
			Why:      "需要深入理解底层实现",
			Resource: "Go Runtime 源码 + 《Go 语言设计与实现》",
			Priority: 2,
		})
	}

	if avgClarity >= 7 {
		analysis.StrongAreas = append(analysis.StrongAreas, "表达清晰，逻辑清楚")
	} else {
		analysis.WeakAreas = append(analysis.WeakAreas, "表达需要改进")
	}

	if avgPractical >= 7 {
		analysis.StrongAreas = append(analysis.StrongAreas, "有实际项目经验")
	} else {
		analysis.WeakAreas = append(analysis.WeakAreas, "缺少实践经验")
		analysis.StudyPlan = append(analysis.StudyPlan, StudyItem{
			Topic:    "Go 项目实战",
			Why:      "需要通过项目积累经验",
			Resource: "开源项目贡献 + 个人项目",
			Priority: 3,
		})
	}

	// 生成总结
	analysis.Summary = fmt.Sprintf("候选人整体水平为 %s，", analysis.Level)
	if analysis.Pass {
		analysis.Summary += "通过面试。"
	} else {
		analysis.Summary += "未通过面试。"
	}

	if len(analysis.StrongAreas) > 0 {
		analysis.Summary += "优势：" + strings.Join(analysis.StrongAreas, "、") + "。"
	}
	if len(analysis.WeakAreas) > 0 {
		analysis.Summary += "不足：" + strings.Join(analysis.WeakAreas, "、") + "。"
	}

	return analysis
}

// ============================================================
// 辅助函数
// ============================================================

func (a *EvaluatorAgent) executeLLMEval(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	system := "你是技术评估官。对候选人回答评分。输出格式：正确性:X/10 深度:X/10 清晰度:X/10 实用性:X/10 综合分:XX/100 优点:... 不足:... 遗漏:..."
	question := input.Question
	if question == "" {
		for i := len(input.History)-1; i >= 0; i-- {
			if input.History[i].Role == "interviewer" { question = input.History[i].Content; break }
		}
	}
	messages := []LLMMessage{{Role: "user", Content: fmt.Sprintf("问题:%s\n\n回答:%s", question, input.Answer)}}
	resp, err := a.provider.ChatWithSystem(ctx, system, messages)
	if err != nil { return nil, err }
	score := Score{Strengths: []string{}, Weaknesses: []string{}, Missing: []string{}}
	var c, d, cl, p float64
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "正确性:") { fmt.Sscanf(line, "正确性: %f/10", &c)
		} else if strings.HasPrefix(line, "深度:") { fmt.Sscanf(line, "深度: %f/10", &d)
		} else if strings.HasPrefix(line, "清晰度:") { fmt.Sscanf(line, "清晰度: %f/10", &cl)
		} else if strings.HasPrefix(line, "实用性:") { fmt.Sscanf(line, "实用性: %f/10", &p)
		} else if strings.HasPrefix(line, "综合分:") { fmt.Sscanf(line, "综合分: %f/100", &score.Total) }
	}
	score.Correctness, score.Depth, score.Clarity, score.Practical = int(c), int(d), int(cl), int(p)
	if score.Total == 0 { score.Total = (c+d+cl+p) * 2.5 }
	return &AgentOutput{Content: resp, Score: &score, NextAction: "next_step"}, nil
}

func getWeaknessFeedback(score *Score) string {
	if len(score.Weaknesses) > 0 {
		return "不足之处：" + strings.Join(score.Weaknesses, "、")
	}
	return "回答有一些不准确的地方"
}

func getStrengthFeedback(score *Score) string {
	if len(score.Strengths) > 0 {
		return "优点：" + strings.Join(score.Strengths, "、")
	}
	return "回答不错"
}

func getMissingPoints(score *Score) string {
	if len(score.Missing) > 0 {
		return "你似乎遗漏了：" + strings.Join(score.Missing, "、")
	}
	return "可以再详细一些"
}

func getDeepQuestion(input AgentInput) string {
	questions := map[string]string{
		"GMP":     "请详细解释 G、M、P 三者之间的关系，以及调度流程",
		"GC":      "请解释三色标记法的工作原理，以及写屏障的作用",
		"channel": "请从源码层面解释 channel 的实现原理",
		"goroutine": "请解释 goroutine 的栈是如何增长的",
		"并发":     "请解释 Go 的内存模型和 happens-before 关系",
	}

	lowerQ := strings.ToLower(input.Question)
	for keyword, question := range questions {
		if strings.Contains(lowerQ, strings.ToLower(keyword)) {
			return question
		}
	}

	return "请从更深层次解释一下这个问题。"
}

func getHarderQuestion(input AgentInput) string {
	questions := []string{
		"如果让你设计一个高并发的消息队列，你会如何使用 Go 的并发原语？",
		"在什么情况下你会选择使用 sync.Map 而不是 RWMutex+map？",
		"Go 的 GC 在什么场景下会出现性能问题？如何优化？",
		"请解释一下 Go 的栈增长和收缩机制。",
		"如何避免 goroutine 泄漏？请给出具体的方法和代码示例。",
	}

	if input.Round < len(questions) {
		return questions[input.Round]
	}

	return "请介绍一下你在项目中遇到的最复杂的并发问题，以及你是如何解决的。"
}

func formatScore(score Score) string {
	return fmt.Sprintf(`【评分】
正确性: %d/10
深度:   %d/10
清晰度: %d/10
实用性: %d/10
综合分: %.1f/100

优点: %s
不足: %s
遗漏: %s`,
		score.Correctness, score.Depth, score.Clarity, score.Practical,
		score.Total,
		strings.Join(score.Strengths, "、"),
		strings.Join(score.Weaknesses, "、"),
		strings.Join(score.Missing, "、"),
	)
}

func formatAnalysis(analysis Analysis) string {
	report := fmt.Sprintf(`【最终评估】

技术评级: %s
是否通过: %s

优势领域:
%s

薄弱环节:
%s

学习计划:
%s

总结: %s`,
		analysis.Level,
		formatBool(analysis.Pass),
		formatList(analysis.StrongAreas),
		formatList(analysis.WeakAreas),
		formatStudyPlan(analysis.StudyPlan),
		analysis.Summary,
	)

	return report
}

func formatBool(b bool) string {
	if b {
		return "通过 ✓"
	}
	return "未通过 ✗"
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "  无"
	}
	var result []string
	for _, item := range items {
		result = append(result, "  - "+item)
	}
	return strings.Join(result, "\n")
}

func formatStudyPlan(items []StudyItem) string {
	if len(items) == 0 {
		return "  无"
	}
	var result []string
	for _, item := range items {
		result = append(result, fmt.Sprintf("  - %s\n    原因: %s\n    资源: %s", item.Topic, item.Why, item.Resource))
	}
	return strings.Join(result, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Scores 辅助类型
type Scores []RoundScore
