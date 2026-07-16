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

// Event is a workflow lifecycle event emitted for visualization/observability.
type Event struct {
	Kind   string           `json:"kind"` // "plan" | "step_start" | "step_done" | "workflow_done" | "error"
	Goal   string           `json:"goal,omitempty"`
	Plan   *core.Plan       `json:"plan,omitempty"`
	Step   *core.Step       `json:"step,omitempty"`
	Result *core.StepResult `json:"result,omitempty"`
	Detail string           `json:"detail,omitempty"`
}

type Engine struct {
	planner  *agents.PlannerAgent
	executor *agents.ExecutorAgent
	rag      *agents.RAGAgent
	memAgent *agents.MemoryAgent
	registry *tools.Registry
	store    *memory.Store

	// OnEvent, if set, is called synchronously for each lifecycle event.
	OnEvent func(Event)
}

func (e *Engine) emit(ev Event) {
	if e.OnEvent != nil {
		e.OnEvent(ev)
	}
}

// ToolInfo returns name/description for each registered tool (for the UI).
func (e *Engine) ToolInfo() []map[string]string {
	var out []map[string]string
	for _, t := range e.registry.All() {
		out = append(out, map[string]string{
			"name":        t.Name(),
			"description": t.Description(),
		})
	}
	return out
}

func NewEngine(claude llm.Client, store *memory.Store) *Engine {
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
		e.emit(Event{Kind: "error", Goal: goal, Detail: err.Error()})
		return nil, fmt.Errorf("planning failed: %w", err)
	}
	e.emit(Event{Kind: "plan", Goal: goal, Plan: plan})

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

			step := step
			e.emit(Event{Kind: "step_start", Step: &step})
			result, err := e.runStep(ctx, wfCtx, step)
			if err != nil {
				e.emit(Event{Kind: "error", Step: &step, Detail: err.Error()})
				return wfCtx.Results, fmt.Errorf("step %s failed: %w", step.ID, err)
			}
			wfCtx.Results[step.ID] = result
			completed[step.ID] = true
			progress = true
			r := result
			e.emit(Event{Kind: "step_done", Step: &step, Result: &r})
		}
		if !progress {
			return wfCtx.Results, fmt.Errorf("workflow deadlock: circular or unsatisfiable dependencies")
		}
	}

	fmt.Printf("\n========================================\n")
	fmt.Printf("Workflow completed: %d steps executed\n", len(completed))
	fmt.Printf("========================================\n")
	e.emit(Event{Kind: "workflow_done", Goal: goal, Detail: fmt.Sprintf("%d steps executed", len(completed))})
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
