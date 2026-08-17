package workflow

import (
	"strings"
	"testing"

	"agent-platform/core"
	"agent-platform/memory"
	"agent-platform/tools"
)

func TestValidatePlan(t *testing.T) {
	registry := tools.NewDefaultRegistry(memory.NewStore())
	engine := &Engine{registry: registry}
	valid := core.Plan{Steps: []core.Step{
		{ID: "search", AgentType: core.AgentTypeExecutor, ToolName: "search_knowledge_base"},
		{ID: "save", AgentType: core.AgentTypeMemory, ToolName: "memory_write", DependsOn: []string{"search"}},
	}}

	tests := []struct {
		name string
		plan *core.Plan
		want string
	}{
		{name: "valid DAG", plan: &valid},
		{name: "empty", plan: &core.Plan{}, want: "at least one step"},
		{name: "duplicate ID", plan: &core.Plan{Steps: []core.Step{
			{ID: "same", AgentType: core.AgentTypeExecutor, ToolName: "calculator"},
			{ID: "same", AgentType: core.AgentTypeExecutor, ToolName: "calculator"},
		}}, want: "duplicate step ID"},
		{name: "unknown tool", plan: &core.Plan{Steps: []core.Step{
			{ID: "one", AgentType: core.AgentTypeExecutor, ToolName: "missing"},
		}}, want: "unknown tool"},
		{name: "unknown dependency", plan: &core.Plan{Steps: []core.Step{
			{ID: "one", AgentType: core.AgentTypeExecutor, ToolName: "calculator", DependsOn: []string{"missing"}},
		}}, want: "unknown dependency"},
		{name: "unsupported agent", plan: &core.Plan{Steps: []core.Step{
			{ID: "one", AgentType: "mystery", ToolName: "calculator"},
		}}, want: "unsupported agent type"},
		{name: "cycle", plan: &core.Plan{Steps: []core.Step{
			{ID: "one", AgentType: core.AgentTypeExecutor, ToolName: "calculator", DependsOn: []string{"two"}},
			{ID: "two", AgentType: core.AgentTypeExecutor, ToolName: "calculator", DependsOn: []string{"one"}},
		}}, want: "contain a cycle"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := engine.validatePlan(test.plan)
			if test.want == "" {
				if err != nil {
					t.Fatalf("validatePlan() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validatePlan() error = %v, want substring %q", err, test.want)
			}
		})
	}
}
