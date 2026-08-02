package agents

import (
	"context"
	"fmt"

	"agent-platform/core"
	"agent-platform/llm"
	"agent-platform/tools"
)

const ragSystem = `You are a RAG Agent. Use the provided retrieval tool when needed, then synthesize a well-reasoned response.`

type RAGAgent struct {
	claude   llm.Client
	registry *tools.Registry
}

func NewRAGAgent(claude llm.Client, registry *tools.Registry) *RAGAgent {
	return &RAGAgent{claude: claude, registry: registry}
}

func (a *RAGAgent) Name() string         { return "RAGAgent" }
func (a *RAGAgent) Type() core.AgentType { return core.AgentTypeRAG }

func (a *RAGAgent) Run(ctx context.Context, wfCtx *core.WorkflowContext, step core.Step) (core.StepResult, error) {
	fmt.Printf("[RAG] Running step %s: %s\n", step.ID, step.Description)

	var toolParams []llm.ToolParam
	for _, t := range a.registry.All() {
		if t.Name() == step.ToolName {
			toolParams = append(toolParams, llm.ToolParam{
				Name:        t.Name(),
				Description: t.Description(),
				InputSchema: t.InputSchema(),
			})
		}
	}

	query := step.Description
	if q, ok := step.ToolInput["query"].(string); ok {
		query = q
	}

	history := llm.UserMessage(fmt.Sprintf("Research and answer: %s", query))

	for i := 0; i < 5; i++ {
		resp, newHistory, err := a.claude.ChatAgentLoop(ctx, ragSystem, history, toolParams)
		if err != nil {
			return core.StepResult{StepID: step.ID, IsError: true, Output: err.Error()}, err
		}

		if len(resp.ToolCalls) == 0 {
			return core.StepResult{StepID: step.ID, Output: resp.Content}, nil
		}

		history = newHistory
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
