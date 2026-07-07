package llm

import "context"

// Client is the provider-agnostic LLM interface.
type Client interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// ChatAgentLoop sends history and returns the response plus history with the
	// assistant turn already appended (caller only needs to add tool results).
	ChatAgentLoop(ctx context.Context, systemPrompt string, history []Message, tools []ToolParam) (*ChatResponse, []Message, error)
}

// Message is a provider-agnostic conversation turn.
type Message struct {
	Role        string           // "user" or "assistant"
	Text        string           // plain text content
	ToolCalls   []ToolCallResult // non-empty when assistant called tools
	ToolResults []ToolResult     // non-empty when user is returning tool results
}

type ToolResult struct {
	ToolCallID string
	Content    string
}

// ToolParam describes a tool the model can call.
type ToolParam struct {
	Name        string
	Description string
	InputSchema map[string]any // full JSON Schema object (type/properties/required)
}

type ChatRequest struct {
	SystemPrompt string
	Messages     []Message
	Tools        []ToolParam
	MaxTokens    int64
}

type ChatResponse struct {
	Content    string
	ToolCalls  []ToolCallResult
	StopReason string
}

type ToolCallResult struct {
	ID    string
	Name  string
	Input map[string]any
}
