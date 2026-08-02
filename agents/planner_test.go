package agents

import (
	"context"
	"strings"
	"testing"

	"agent-platform/core"
	"agent-platform/llm"
	"agent-platform/tools"
)

type plannerTestLLM struct {
	request llm.ChatRequest
}

func (f *plannerTestLLM) Chat(_ context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
	f.request = request
	return &llm.ChatResponse{Content: `{
  "goal": "echo",
  "steps": [{
    "id": "step_1",
    "description": "echo text",
    "agent_type": "executor",
    "tool_name": "demo__echo",
    "tool_input": {"text": "hello"},
    "depends_on": []
  }]
}`}, nil
}

func (f *plannerTestLLM) ChatAgentLoop(context.Context, string, []llm.Message, []llm.ToolParam) (*llm.ChatResponse, []llm.Message, error) {
	panic("not used")
}

type plannerTestTool struct{}

func (*plannerTestTool) Name() string        { return "demo__echo" }
func (*plannerTestTool) Description() string { return "remote echo tool" }
func (*plannerTestTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"text": map[string]any{"type": "string"}},
		"required":   []string{"text"},
	}
}
func (*plannerTestTool) Execute(context.Context, map[string]any) (string, error) {
	return "", nil
}

func TestPlannerIncludesRegistryToolsInPrompt(t *testing.T) {
	client := &plannerTestLLM{}
	registry := tools.NewRegistry()
	registry.Register(&plannerTestTool{})
	planner := NewPlannerAgent(client, registry)

	plan, err := planner.CreatePlan(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].ToolName != "demo__echo" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	prompt := client.request.Messages[0].Text
	for _, want := range []string{"demo__echo", "remote echo tool", `"required": [`, `"text"`} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("planner prompt does not contain %q:\n%s", want, prompt)
		}
	}
}

var _ llm.Client = (*plannerTestLLM)(nil)
var _ core.Tool = (*plannerTestTool)(nil)
