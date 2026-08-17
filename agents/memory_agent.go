package agents

import (
	"context"
	"fmt"
	"strings"

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

func (a *MemoryAgent) Name() string         { return "MemoryAgent" }
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
				wfCtx.Memory[workflowMemoryKey(input, key)] = val
			}
		}
	} else if step.ToolName == "memory_delete" {
		if key, ok := input["key"].(string); ok {
			delete(wfCtx.Memory, workflowMemoryKey(input, key))
		}
	}

	fmt.Printf("[Memory] Result: %s\n", output)
	return core.StepResult{StepID: step.ID, Output: output}, nil
}

func workflowMemoryKey(input map[string]any, key string) string {
	namespace, _ := input["namespace"].(string)
	namespace = strings.TrimSpace(namespace)
	if namespace == "" || namespace == memory.DefaultNamespace {
		return key
	}
	return namespace + "/" + key
}
