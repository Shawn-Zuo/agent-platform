package main

import (
	"context"
	"fmt"
	"os"

	"agent-platform/llm"
	"agent-platform/memory"
	"agent-platform/workflow"
)

func main() {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "Error: ANTHROPIC_API_KEY environment variable is not set")
		os.Exit(1)
	}

	ctx := context.Background()
	claude := llm.NewClaudeClient()
	store := memory.NewStore()
	engine := workflow.NewEngine(claude, store)

	// Demo goals — each exercises different agent types
	goals := []string{
		"Calculate 123 multiplied by 456, then store the result in memory under key 'product'",
		"Search the knowledge base to learn about RAG and agents, then summarize what you found",
	}

	for i, goal := range goals {
		fmt.Printf("\n\n### Demo %d ###\n", i+1)
		results, err := engine.Run(ctx, goal)
		if err != nil {
			fmt.Printf("Workflow error: %v\n", err)
			continue
		}

		fmt.Println("\n--- Final Results ---")
		for stepID, r := range results {
			status := "OK"
			if r.IsError {
				status = "ERROR"
			}
			fmt.Printf("[%s] %s: %s\n", status, stepID, r.Output)
		}
	}

	fmt.Printf("\n\n--- Memory Store ---\n%s\n", store.Summary())
}
