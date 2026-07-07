package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

type DeepSeekClient struct {
	client *openai.Client
	model  string
}

func NewDeepSeekClient() *DeepSeekClient {
	cfg := openai.DefaultConfig(os.Getenv("DEEPSEEK_API_KEY"))
	cfg.BaseURL = "https://api.deepseek.com/v1"
	return &DeepSeekClient{
		client: openai.NewClientWithConfig(cfg),
		model:  "deepseek-chat",
	}
}

func (c *DeepSeekClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	maxTokens := int(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}
	r := openai.ChatCompletionRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		Messages:  toOpenAIMessages(req.Messages, req.SystemPrompt),
		Tools:     toOpenAITools(req.Tools),
	}
	resp, err := c.client.CreateChatCompletion(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("deepseek API error: %w", err)
	}
	return parseOpenAIResponse(&resp), nil
}

func (c *DeepSeekClient) ChatAgentLoop(ctx context.Context, systemPrompt string, history []Message, tools []ToolParam) (*ChatResponse, []Message, error) {
	r := openai.ChatCompletionRequest{
		Model:     c.model,
		MaxTokens: 4096,
		Messages:  toOpenAIMessages(history, systemPrompt),
		Tools:     toOpenAITools(tools),
	}
	resp, err := c.client.CreateChatCompletion(ctx, r)
	if err != nil {
		return nil, history, fmt.Errorf("deepseek API error: %w", err)
	}
	chatResp := parseOpenAIResponse(&resp)
	updated := append(history, Message{
		Role:      "assistant",
		Text:      chatResp.Content,
		ToolCalls: chatResp.ToolCalls,
	})
	return chatResp, updated, nil
}

func toOpenAIMessages(msgs []Message, systemPrompt string) []openai.ChatCompletionMessage {
	var out []openai.ChatCompletionMessage
	if systemPrompt != "" {
		out = append(out, openai.ChatCompletionMessage{Role: "system", Content: systemPrompt})
	}
	for _, m := range msgs {
		switch m.Role {
		case "user":
			if len(m.ToolResults) > 0 {
				// Each tool result is a separate "tool" role message in OpenAI format.
				for _, tr := range m.ToolResults {
					out = append(out, openai.ChatCompletionMessage{
						Role:       "tool",
						Content:    tr.Content,
						ToolCallID: tr.ToolCallID,
					})
				}
			} else {
				out = append(out, openai.ChatCompletionMessage{Role: "user", Content: m.Text})
			}
		case "assistant":
			msg := openai.ChatCompletionMessage{Role: "assistant", Content: m.Text}
			if len(m.ToolCalls) > 0 {
				tcs := make([]openai.ToolCall, len(m.ToolCalls))
				for i, tc := range m.ToolCalls {
					inputJSON, _ := json.Marshal(tc.Input)
					tcs[i] = openai.ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: openai.FunctionCall{
							Name:      tc.Name,
							Arguments: string(inputJSON),
						},
					}
				}
				msg.ToolCalls = tcs
			}
			out = append(out, msg)
		}
	}
	return out
}

func toOpenAITools(tools []ToolParam) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]openai.Tool, len(tools))
	for i, t := range tools {
		out[i] = openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}
	return out
}

func parseOpenAIResponse(resp *openai.ChatCompletionResponse) *ChatResponse {
	if len(resp.Choices) == 0 {
		return &ChatResponse{}
	}
	choice := resp.Choices[0]
	chatResp := &ChatResponse{
		Content:    choice.Message.Content,
		StopReason: string(choice.FinishReason),
	}
	for _, tc := range choice.Message.ToolCalls {
		var input map[string]any
		json.Unmarshal([]byte(tc.Function.Arguments), &input) //nolint
		chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCallResult{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}
	return chatResp
}
