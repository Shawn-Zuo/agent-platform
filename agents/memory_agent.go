package agents

import (
	"context"
	"fmt"

	"agent-platform/core"
	"agent-platform/memory"
	"agent-platform/tools"
)

type MemoryAgent struct {
	store    *memory.Store
	registry *tools.Registry
}

func NewMemoryAgent(store *memory.Store, registry *tools.Registry) *MemoryAgent {
	return &MemoryAgent{store: store, registry: registry}
}

func (a *MemoryAgent) Name() string        { return "MemoryAgent" }
func (a *MemoryAgent) Type() core.AgentType { return core.AgentTypeMemory }

func (a *MemoryAgent) Run(ctx context.Context, wfCtx *core.WorkflowContext, step core.Step) (core.StepResult, error) {
	fmt.Printf("[Memory] Running step %s: %s (tool=%s)\n", step.ID, step.Description, step.ToolName)

	input := interpolateInput(step.ToolInput, wfCtx.Results)

	output, err := a.registry.Execute(ctx, step.ToolName, input)
	if err != nil {
		return core.StepResult{StepID: step.ID, Output: err.Error(), IsError: true}, nil
	}

	// Also sync to workflow context memory
	if step.ToolName == "memory_write" {
		if key, ok := input["key"].(string); ok {
			if val, ok := input["value"].(string); ok {
				wfCtx.Memory[key] = val
			}
		}
	}

	fmt.Printf("[Memory] Result: %s\n", output)
	return core.StepResult{StepID: step.ID, Output: output}, nil
}
