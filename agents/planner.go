package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-platform/core"
	"agent-platform/llm"
)

const plannerSystem = `You are a Planner Agent. Given a user goal, decompose it into a structured plan with ordered steps.

Each step must have:
- id: unique step identifier (e.g. "step_1")
- description: what this step accomplishes
- agent_type: one of "executor", "rag", "memory"
- tool_name: the tool to use (calculator, search_knowledge_base, memory_read, memory_write)
- tool_input: JSON object with the tool's required inputs
- depends_on: list of step IDs this step depends on (empty for first steps)

Return ONLY valid JSON in this format:
{
  "goal": "...",
  "steps": [
    {
      "id": "step_1",
      "description": "...",
      "agent_type": "executor",
      "tool_name": "...",
      "tool_input": {...},
      "depends_on": []
    }
  ]
}`

type PlannerAgent struct {
	claude *llm.ClaudeClient
}

func NewPlannerAgent(claude *llm.ClaudeClient) *PlannerAgent {
	return &PlannerAgent{claude: claude}
}

func (a *PlannerAgent) Name() string        { return "PlannerAgent" }
func (a *PlannerAgent) Type() core.AgentType { return core.AgentTypePlanner }

func (a *PlannerAgent) CreatePlan(ctx context.Context, goal string) (*core.Plan, error) {
	fmt.Printf("\n[Planner] Creating plan for goal: %s\n", goal)

	messages := llm.UserMessage(fmt.Sprintf("Goal: %s\n\nAvailable tools: calculator, search_knowledge_base, memory_read, memory_write\n\nCreate a step-by-step plan.", goal))

	resp, err := a.claude.Chat(ctx, llm.ChatRequest{
		SystemPrompt: plannerSystem,
		Messages:     messages,
		MaxTokens:    2048,
	})
	if err != nil {
		return nil, fmt.Errorf("planner LLM error: %w", err)
	}

	// Strip markdown code fences if present
	raw := resp.Content
	if idx := indexJSON(raw); idx >= 0 {
		raw = raw[idx:]
	}
	if end := lastBrace(raw); end >= 0 {
		raw = raw[:end+1]
	}

	var planJSON struct {
		Goal  string `json:"goal"`
		Steps []struct {
			ID          string         `json:"id"`
			Description string         `json:"description"`
			AgentType   string         `json:"agent_type"`
			ToolName    string         `json:"tool_name"`
			ToolInput   map[string]any `json:"tool_input"`
			DependsOn   []string       `json:"depends_on"`
		} `json:"steps"`
	}

	if err := json.Unmarshal([]byte(raw), &planJSON); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w\nRaw: %s", err, raw)
	}

	plan := &core.Plan{Goal: planJSON.Goal}
	for _, s := range planJSON.Steps {
		plan.Steps = append(plan.Steps, core.Step{
			ID:          s.ID,
			Description: s.Description,
			AgentType:   core.AgentType(s.AgentType),
			ToolName:    s.ToolName,
			ToolInput:   s.ToolInput,
			DependsOn:   s.DependsOn,
		})
	}

	fmt.Printf("[Planner] Plan created with %d steps\n", len(plan.Steps))
	return plan, nil
}

func (a *PlannerAgent) Run(ctx context.Context, wfCtx *core.WorkflowContext, step core.Step) (core.StepResult, error) {
	plan, err := a.CreatePlan(ctx, wfCtx.Goal)
	if err != nil {
		return core.StepResult{StepID: step.ID, IsError: true, Output: err.Error()}, err
	}
	b, _ := json.MarshalIndent(plan, "", "  ")
	return core.StepResult{StepID: step.ID, Output: string(b)}, nil
}

func indexJSON(s string) int {
	for i, c := range s {
		if c == '{' {
			return i
		}
	}
	return -1
}

func lastBrace(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '}' {
			return i
		}
	}
	return -1
}
