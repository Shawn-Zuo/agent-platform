package agents

import (
	"context"
	"fmt"

	"agent-platform/core"
	"agent-platform/tools"
)

type ExecutorAgent struct {
	registry *tools.Registry
}

func NewExecutorAgent(registry *tools.Registry) *ExecutorAgent {
	return &ExecutorAgent{registry: registry}
}

func (a *ExecutorAgent) Name() string        { return "ExecutorAgent" }
func (a *ExecutorAgent) Type() core.AgentType { return core.AgentTypeExecutor }

func (a *ExecutorAgent) Run(ctx context.Context, wfCtx *core.WorkflowContext, step core.Step) (core.StepResult, error) {
	fmt.Printf("[Executor] Running step %s: %s (tool=%s)\n", step.ID, step.Description, step.ToolName)

	// Interpolate previous results into tool input
	input := interpolateInput(step.ToolInput, wfCtx.Results)

	output, err := a.registry.Execute(ctx, step.ToolName, input)
	if err != nil {
		fmt.Printf("[Executor] Tool error: %v\n", err)
		return core.StepResult{StepID: step.ID, Output: err.Error(), IsError: true}, nil
	}

	fmt.Printf("[Executor] Tool result: %s\n", output)
	return core.StepResult{StepID: step.ID, Output: output}, nil
}

// interpolateInput replaces "$step_N.output" placeholders with actual results.
func interpolateInput(input map[string]any, results map[string]core.StepResult) map[string]any {
	out := make(map[string]any, len(input))
	for k, v := range input {
		if s, ok := v.(string); ok {
			if len(s) > 0 && s[0] == '$' {
				ref := s[1:]
				if r, found := results[ref]; found {
					out[k] = r.Output
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
