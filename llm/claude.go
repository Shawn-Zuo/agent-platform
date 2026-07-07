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

func (c *ClaudeClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: maxTokens,
		Messages:  toAnthropicMessages(req.Messages),
	}
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.SystemPrompt}}
	}
	if len(req.Tools) > 0 {
		params.Tools = toAnthropicTools(req.Tools)
	}
	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("claude API error: %w", err)
	}
	return parseAnthropicResponse(msg), nil
}

func (c *ClaudeClient) ChatAgentLoop(ctx context.Context, systemPrompt string, history []Message, tools []ToolParam) (*ChatResponse, []Message, error) {
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 4096,
		Messages:  toAnthropicMessages(history),
	}
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: systemPrompt}}
	}
	if len(tools) > 0 {
		params.Tools = toAnthropicTools(tools)
	}
	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, history, fmt.Errorf("claude API error: %w", err)
	}
	resp := parseAnthropicResponse(msg)
	updated := append(history, Message{
		Role:      "assistant",
		Text:      resp.Content,
		ToolCalls: resp.ToolCalls,
	})
	return resp, updated, nil
}

// toAnthropicMessages converts abstract Messages to Anthropic MessageParams.
func toAnthropicMessages(msgs []Message) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "user":
			if len(m.ToolResults) > 0 {
				blocks := make([]anthropic.ContentBlockParamUnion, len(m.ToolResults))
				for i, tr := range m.ToolResults {
					blocks[i] = anthropic.NewToolResultBlock(tr.ToolCallID, tr.Content, false)
				}
				out = append(out, anthropic.MessageParam{Role: "user", Content: blocks})
			} else {
				out = append(out, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Text)))
			}
		case "assistant":
			var blocks []anthropic.ContentBlockParamUnion
			if m.Text != "" {
				blocks = append(blocks, anthropic.NewTextBlock(m.Text))
			}
			for _, tc := range m.ToolCalls {
				inputJSON, _ := json.Marshal(tc.Input)
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    tc.ID,
						Name:  tc.Name,
						Input: json.RawMessage(inputJSON),
					},
				})
			}
			out = append(out, anthropic.MessageParam{Role: "assistant", Content: blocks})
		}
	}
	return out
}

func toAnthropicTools(tools []ToolParam) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, len(tools))
	for i, t := range tools {
		tool := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: t.InputSchema["properties"],
			},
		}
		out[i] = anthropic.ToolUnionParam{OfTool: &tool}
	}
	return out
}

func parseAnthropicResponse(msg *anthropic.Message) *ChatResponse {
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
