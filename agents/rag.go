package agents

import (
	"context"
	"fmt"

	"agent-platform/core"
	"agent-platform/llm"
	"agent-platform/tools"

	"github.com/anthropics/anthropic-sdk-go"
)

const ragSystem = `You are a RAG Agent. You retrieve relevant information and synthesize a comprehensive answer.
Use the search_knowledge_base tool to find information, then provide a well-reasoned response.`

type RAGAgent struct {
	claude   *llm.ClaudeClient
	registry *tools.Registry
}

func NewRAGAgent(claude *llm.ClaudeClient, registry *tools.Registry) *RAGAgent {
	return &RAGAgent{claude: claude, registry: registry}
}

func (a *RAGAgent) Name() string        { return "RAGAgent" }
func (a *RAGAgent) Type() core.AgentType { return core.AgentTypeRAG }

func (a *RAGAgent) Run(ctx context.Context, wfCtx *core.WorkflowContext, step core.Step) (core.StepResult, error) {
	fmt.Printf("[RAG] Running step %s: %s\n", step.ID, step.Description)

	var toolParams []anthropic.ToolUnionParam
	for _, t := range a.registry.All() {
		if t.Name() == "search_knowledge_base" {
			toolParams = append(toolParams, llm.BuildToolParam(t.Name(), t.Description(), t.InputSchema()))
		}
	}

	query := step.Description
	if q, ok := step.ToolInput["query"].(string); ok {
		query = q
	}

	history := llm.UserMessage(fmt.Sprintf("Research and answer: %s", query))

	for i := 0; i < 5; i++ {
		resp, msg, err := a.claude.ChatAgentLoop(ctx, ragSystem, history, toolParams)
		if err != nil {
			return core.StepResult{StepID: step.ID, IsError: true, Output: err.Error()}, err
		}

		if len(resp.ToolCalls) == 0 {
			return core.StepResult{StepID: step.ID, Output: resp.Content}, nil
		}

		history = llm.AppendAssistant(history, msg)

		for _, tc := range resp.ToolCalls {
			result, err := a.registry.Execute(ctx, tc.Name, tc.Input)
			if err != nil {
				result = "Error: " + err.Error()
			}
			fmt.Printf("[RAG] Tool %s result: %s\n", tc.Name, truncate(result, 100))
			history = llm.AppendToolResult(history, tc.ID, result)
		}
	}

	return core.StepResult{StepID: step.ID, Output: "RAG agent completed retrieval loop"}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
