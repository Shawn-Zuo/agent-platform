package workflow

import (
	"context"
	"fmt"

	"agent-platform/agents"
	"agent-platform/core"
	"agent-platform/llm"
	"agent-platform/memory"
	"agent-platform/tools"
)

type Engine struct {
	planner  *agents.PlannerAgent
	executor *agents.ExecutorAgent
	rag      *agents.RAGAgent
	memAgent *agents.MemoryAgent
	registry *tools.Registry
	store    *memory.Store
}

func NewEngine(claude *llm.ClaudeClient, store *memory.Store) *Engine {
	registry := tools.NewRegistry()
	registry.Register(&tools.CalculatorTool{})
	registry.Register(tools.NewMockSearchTool())
	registry.Register(tools.NewMemoryReadTool(store))
	registry.Register(tools.NewMemoryWriteTool(store))

	return &Engine{
		planner:  agents.NewPlannerAgent(claude),
		executor: agents.NewExecutorAgent(registry),
		rag:      agents.NewRAGAgent(claude, registry),
		memAgent: agents.NewMemoryAgent(store, registry),
		registry: registry,
		store:    store,
	}
}

func (e *Engine) Run(ctx context.Context, goal string) (map[string]core.StepResult, error) {
	fmt.Printf("\n========================================\n")
	fmt.Printf("Workflow Engine starting\nGoal: %s\n", goal)
	fmt.Printf("========================================\n\n")

	plan, err := e.planner.CreatePlan(ctx, goal)
	if err != nil {
		return nil, fmt.Errorf("planning failed: %w", err)
	}

	wfCtx := &core.WorkflowContext{
		Goal:    goal,
		Results: make(map[string]core.StepResult),
		Memory:  make(map[string]string),
	}

	// Topological execution respecting dependencies
	completed := make(map[string]bool)
	for len(completed) < len(plan.Steps) {
		progress := false
		for _, step := range plan.Steps {
			if completed[step.ID] {
				continue
			}
			if !depsReady(step.DependsOn, completed) {
				continue
			}

			result, err := e.runStep(ctx, wfCtx, step)
			if err != nil {
				return wfCtx.Results, fmt.Errorf("step %s failed: %w", step.ID, err)
			}
			wfCtx.Results[step.ID] = result
			completed[step.ID] = true
			progress = true
		}
		if !progress {
			return wfCtx.Results, fmt.Errorf("workflow deadlock: circular or unsatisfiable dependencies")
		}
	}

	fmt.Printf("\n========================================\n")
	fmt.Printf("Workflow completed: %d steps executed\n", len(completed))
	fmt.Printf("========================================\n")
	return wfCtx.Results, nil
}

func (e *Engine) runStep(ctx context.Context, wfCtx *core.WorkflowContext, step core.Step) (core.StepResult, error) {
	var agent core.Agent
	switch step.AgentType {
	case core.AgentTypeRAG:
		agent = e.rag
	case core.AgentTypeMemory:
		agent = e.memAgent
	default:
		agent = e.executor
	}
	return agent.Run(ctx, wfCtx, step)
}

func depsReady(deps []string, completed map[string]bool) bool {
	for _, d := range deps {
		if !completed[d] {
			return false
		}
	}
	return true
}
