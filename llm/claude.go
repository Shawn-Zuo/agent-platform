package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type ClaudeClient struct {
	client anthropic.Client
	model  anthropic.Model
}

func NewClaudeClient() *ClaudeClient {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	var client anthropic.Client
	if apiKey != "" {
		client = anthropic.NewClient(option.WithAPIKey(apiKey))
	} else {
		client = anthropic.NewClient()
	}
	return &ClaudeClient{
		client: client,
		model:  "claude-opus-4-8",
	}
}

type ChatRequest struct {
	SystemPrompt string
	Messages     []anthropic.MessageParam
	Tools        []anthropic.ToolUnionParam
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

func (c *ClaudeClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: maxTokens,
		Messages:  req.Messages,
	}
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.SystemPrompt}}
	}
	if len(req.Tools) > 0 {
		params.Tools = req.Tools
	}

	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("claude API error: %w", err)
	}
	return buildResponse(msg), nil
}

// ChatAgentLoop sends a message with optional tools, returning both the parsed response and raw message
// (so callers can append it to history via msg.ToParam()).
func (c *ClaudeClient) ChatAgentLoop(ctx context.Context, systemPrompt string, history []anthropic.MessageParam, tools []anthropic.ToolUnionParam) (*ChatResponse, *anthropic.Message, error) {
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 4096,
		Messages:  history,
	}
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: systemPrompt}}
	}
	if len(tools) > 0 {
		params.Tools = tools
	}

	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	return buildResponse(msg), msg, nil
}

func buildResponse(msg *anthropic.Message) *ChatResponse {
	resp := &ChatResponse{StopReason: string(msg.StopReason)}
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			resp.Content += block.Text
		case "tool_use":
			var input map[string]any
			json.Unmarshal(block.Input, &input) //nolint
			resp.ToolCalls = append(resp.ToolCalls, ToolCallResult{
				ID:    block.ID,
				Name:  block.Name,
				Input: input,
			})
		}
	}
	return resp
}

// BuildToolParam constructs a ToolUnionParam from name, description, and JSON Schema properties.
func BuildToolParam(name, description string, schema map[string]any) anthropic.ToolUnionParam {
	tool := anthropic.ToolParam{
		Name:        name,
		Description: anthropic.String(description),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: schema["properties"],
		},
	}
	return anthropic.ToolUnionParam{OfTool: &tool}
}
