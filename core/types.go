package core

import "context"

type AgentType string

const (
	AgentTypePlanner  AgentType = "planner"
	AgentTypeExecutor AgentType = "executor"
	AgentTypeMemory   AgentType = "memory"
	AgentTypeRAG      AgentType = "rag"
)

type Message struct {
	Role    string
	Content string
}

type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

type ToolResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

type Plan struct {
	Goal  string
	Steps []Step
}

type Step struct {
	ID          string
	Description string
	AgentType   AgentType
	ToolName    string
	ToolInput   map[string]any
	DependsOn   []string
}

type StepResult struct {
	StepID  string
	Output  string
	IsError bool
}

type WorkflowContext struct {
	Goal    string
	Results map[string]StepResult
	Memory  map[string]string
}

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Execute(ctx context.Context, input map[string]any) (string, error)
}

type Agent interface {
	Name() string
	Type() AgentType
	Run(ctx context.Context, wfCtx *WorkflowContext, step Step) (StepResult, error)
}
