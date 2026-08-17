package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"agent-platform/config"
	"agent-platform/llm"
	"agent-platform/mcpclient"
	"agent-platform/mcpserver"
	"agent-platform/memory"
	"agent-platform/tools"
	"agent-platform/web"
	"agent-platform/workflow"
)

func main() {
	webAddr := flag.String("web", "", "run the web visualization server on this address (e.g. :8080)")
	mcpStdio := flag.Bool("mcp", false, "run the MCP server over stdio")
	mcpConfigPath := flag.String("mcp-config", "", "connect to external stdio MCP servers defined in this JSON file")
	memoryFile := flag.String("memory-file", "", "persist shared agent memory to this JSON file")
	goal := flag.String("goal", "", "run one CLI workflow goal instead of the built-in demos")
	flag.Parse()
	if *mcpStdio && *webAddr != "" {
		fmt.Fprintln(os.Stderr, "Error: -mcp and -web cannot be used together")
		os.Exit(2)
	}
	if *goal != "" && (*mcpStdio || *webAddr != "") {
		fmt.Fprintln(os.Stderr, "Error: -goal is only available in CLI workflow mode")
		os.Exit(2)
	}

	if err := config.Load(".env"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load .env: %v\n", err)
	}

	store := memory.NewStore()
	if *memoryFile != "" {
		persistentStore, err := memory.NewPersistentStore(*memoryFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Memory store error: %v\n", err)
			os.Exit(1)
		}
		store = persistentStore
		fmt.Fprintf(os.Stderr, "Persistent memory enabled: %s\n", *memoryFile)
	}
	registry := tools.NewDefaultRegistry(store)
	if *mcpConfigPath != "" {
		cfg, err := mcpclient.LoadConfig(*mcpConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "MCP client configuration error: %v\n", err)
			os.Exit(1)
		}
		connectCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		manager, err := mcpclient.Connect(connectCtx, cfg)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "MCP client connection error: %v\n", err)
			os.Exit(1)
		}
		if err := manager.RegisterTools(registry); err != nil {
			_ = manager.Close()
			fmt.Fprintf(os.Stderr, "MCP tool registration error: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			if err := manager.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "MCP client close error: %v\n", err)
			}
		}()
		fmt.Fprintf(os.Stderr, "Connected to %d MCP server(s); registered %d remote tool(s)\n", len(cfg.MCPServers), len(manager.Tools()))
	}

	if *mcpStdio {
		if err := mcpserver.RunStdio(context.Background(), registry); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
		return
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

	if *webAddr != "" {
		srv := web.NewServerWithRegistry(client, store, registry)
		fmt.Printf("Web visualization running at http://localhost%s\n", *webAddr)
		if err := http.ListenAndServe(*webAddr, srv.Handler()); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	ctx := context.Background()
	engine := workflow.NewEngineWithRegistry(client, store, registry)

	goals := []string{
		"Calculate 123 multiplied by 456, then store the result in memory under key 'product'",
		"Search the knowledge base to learn about RAG and agents, then summarize what you found",
	}
	if *goal != "" {
		goals = []string{*goal}
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
