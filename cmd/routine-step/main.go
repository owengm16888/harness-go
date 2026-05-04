
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/harness-engineering/harness/pkg/routine"
)

func main() {
	llm := routine.NewClaudeCLIProvider()
	engine := routine.NewRoutineEngine(routine.EngineConfig{EnableScoring: true})
	engine.SetLLMProvider(llm)

	config := routine.RoutineConfig{
		Name: "Go面试",
		Type: routine.TypeInterview,
		Settings: routine.RoutineSettings{MaxRounds: 1, Timeout: 10 * time.Minute},
	}

	inst, _ := engine.Create(context.Background(), config)
	engine.Start(context.Background(), inst.ID)
	time.Sleep(30 * time.Second)

	state, _ := engine.GetInstance(context.Background(), inst.ID)
	for _, msg := range state.GetHistory() {
		if msg.Role == "interviewer" {
			fmt.Printf("QUESTION:\n%s\n", msg.Content)
		}
	}

	answer := os.Args[1]
	fmt.Printf("\nANSWER:\n%s\n", answer)

	engine.SubmitAnswer(context.Background(), inst.ID, answer)
	time.Sleep(30 * time.Second)

	state, _ = engine.GetInstance(context.Background(), inst.ID)
	for _, msg := range state.GetHistory() {
		if msg.Role == "evaluator" {
			fmt.Printf("\nEVALUATION:\n%s\n", msg.Content)
		}
		if msg.Role == "followup_generator" {
			fmt.Printf("\nFOLLOWUP:\n%s\n", msg.Content)
		}
	}

	if len(state.Scores) > 0 {
		s := state.Scores[0]
		fmt.Printf("\nSCORE: %.1f/100\n", s.Score.Total)
	}
}
