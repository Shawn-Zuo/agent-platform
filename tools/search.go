package tools

import (
	"context"
	"fmt"
	"strings"
)

// MockSearchTool simulates a knowledge base search (no real API needed for demo)
type MockSearchTool struct {
	kb map[string]string
}

func NewMockSearchTool() *MockSearchTool {
	return &MockSearchTool{
		kb: map[string]string{
			"go":         "Go is a statically typed, compiled language designed at Google. Known for simplicity, concurrency (goroutines/channels), and fast compilation.",
			"anthropic":  "Anthropic is an AI safety company founded in 2021, creator of Claude AI models. Focused on safe and beneficial AI systems.",
			"agent":      "An AI agent is a system that perceives its environment and takes actions to achieve goals. Modern agents use LLMs as the reasoning core.",
			"workflow":   "A workflow engine orchestrates multi-step processes, managing task dependencies, state, and execution order across multiple components.",
			"rag":        "Retrieval-Augmented Generation (RAG) enhances LLM responses by retrieving relevant documents from a knowledge base before generation.",
			"mcp":        "Model Context Protocol (MCP) is an open standard for connecting AI models to external data sources and tools in a unified way.",
			"llm":        "Large Language Models (LLMs) are neural networks trained on vast text data, capable of understanding and generating human language.",
		},
	}
}

func (t *MockSearchTool) Name() string { return "search_knowledge_base" }

func (t *MockSearchTool) Description() string {
	return "Search the knowledge base for information on a topic"
}

func (t *MockSearchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query or topic to look up",
			},
		},
		"required": []string{"query"},
	}
}

func (t *MockSearchTool) Execute(_ context.Context, input map[string]any) (string, error) {
	query, _ := input["query"].(string)
	query = strings.ToLower(strings.TrimSpace(query))

	var results []string
	for key, val := range t.kb {
		if strings.Contains(query, key) || strings.Contains(key, query) {
			results = append(results, fmt.Sprintf("[%s]: %s", key, val))
		}
	}

	if len(results) == 0 {
		return fmt.Sprintf("No results found for %q in knowledge base.", query), nil
	}
	return strings.Join(results, "\n"), nil
}
