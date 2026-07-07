package llm

import "github.com/anthropics/anthropic-sdk-go"

func UserMessage(text string) []anthropic.MessageParam {
	return []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(text)),
	}
}

func AppendAssistant(history []anthropic.MessageParam, msg *anthropic.Message) []anthropic.MessageParam {
	return append(history, msg.ToParam())
}

func AppendToolResult(history []anthropic.MessageParam, toolID, content string) []anthropic.MessageParam {
	return append(history, anthropic.NewUserMessage(
		anthropic.NewToolResultBlock(toolID, content, false),
	))
}
