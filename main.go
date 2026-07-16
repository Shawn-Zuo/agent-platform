package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"agent-platform/config"
	"agent-platform/llm"
	"agent-platform/memory"
	"agent-platform/web"
	"agent-platform/workflow"
)

func main() {
	webAddr := flag.String("web", "", "run the web visualization server on this address (e.g. :8080)")
	flag.Parse()

	if err := config.Load(".env"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load .env: %v\n", err)
	}

	var client llm.Client
	switch {
	case os.Getenv("ANTHROPIC_API_KEY") != "":
		client = llm.NewClaudeClient()
		fmt.Println("Using Claude (Anthropic)")
	case os.Getenv("DEEPSEEK_API_KEY") != "":
		client = llm.NewDeepSeekClient()
		fmt.Println("Using DeepSeek")
	default:
		fmt.Fprintln(os.Stderr, "Error: set ANTHROPIC_API_KEY or DEEPSEEK_API_KEY")
		os.Exit(1)
	}

	store := memory.NewStore()

	if *webAddr != "" {
		srv := web.NewServer(client, store)
		fmt.Printf("Web visualization running at http://localhost%s\n", *webAddr)
		if err := http.ListenAndServe(*webAddr, srv.Handler()); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	ctx := context.Background()
	engine := workflow.NewEngine(client, store)

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
