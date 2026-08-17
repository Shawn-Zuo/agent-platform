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
	return NewEngineWithRegistry(claude, store, tools.NewDefaultRegistry(store))
}

// NewEngineWithRegistry builds an engine with local tools plus any dynamically
// discovered tools already registered by an MCP client.
func NewEngineWithRegistry(claude llm.Client, store *memory.Store, registry *tools.Registry) *Engine {
	return &Engine{
		planner:  agents.NewPlannerAgent(claude, registry),
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
	if err := e.validatePlan(plan); err != nil {
		e.emit(Event{Kind: "error", Goal: goal, Detail: err.Error()})
		return nil, fmt.Errorf("invalid plan: %w", err)
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

// validatePlan rejects malformed LLM output before any tool is executed. This
// keeps planning failures deterministic and prevents partially executed plans.
func (e *Engine) validatePlan(plan *core.Plan) error {
	if plan == nil || len(plan.Steps) == 0 {
		return fmt.Errorf("plan must contain at least one step")
	}

	steps := make(map[string]core.Step, len(plan.Steps))
	for _, step := range plan.Steps {
		if step.ID == "" {
			return fmt.Errorf("step ID must not be empty")
		}
		if _, exists := steps[step.ID]; exists {
			return fmt.Errorf("duplicate step ID %q", step.ID)
		}
		switch step.AgentType {
		case core.AgentTypeExecutor, core.AgentTypeRAG, core.AgentTypeMemory:
		default:
			return fmt.Errorf("step %q has unsupported agent type %q", step.ID, step.AgentType)
		}
		if _, exists := e.registry.Get(step.ToolName); !exists {
			return fmt.Errorf("step %q references unknown tool %q", step.ID, step.ToolName)
		}
		steps[step.ID] = step
	}

	for _, step := range plan.Steps {
		seenDeps := make(map[string]bool, len(step.DependsOn))
		for _, dependency := range step.DependsOn {
			if dependency == step.ID {
				return fmt.Errorf("step %q depends on itself", step.ID)
			}
			if _, exists := steps[dependency]; !exists {
				return fmt.Errorf("step %q references unknown dependency %q", step.ID, dependency)
			}
			if seenDeps[dependency] {
				return fmt.Errorf("step %q repeats dependency %q", step.ID, dependency)
			}
			seenDeps[dependency] = true
		}
	}

	// Kahn's algorithm proves the dependency graph is acyclic before execution.
	indegree := make(map[string]int, len(plan.Steps))
	dependents := make(map[string][]string, len(plan.Steps))
	for _, step := range plan.Steps {
		indegree[step.ID] = len(step.DependsOn)
		for _, dependency := range step.DependsOn {
			dependents[dependency] = append(dependents[dependency], step.ID)
		}
	}
	queue := make([]string, 0, len(plan.Steps))
	for id, degree := range indegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, dependent := range dependents[id] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}
	if visited != len(plan.Steps) {
		return fmt.Errorf("step dependencies contain a cycle")
	}
	return nil
}
