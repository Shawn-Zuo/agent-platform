package llm

func UserMessage(text string) []Message {
	return []Message{{Role: "user", Text: text}}
}

func AppendToolResult(history []Message, toolID, content string) []Message {
	return append(history, Message{
		Role:        "user",
		ToolResults: []ToolResult{{ToolCallID: toolID, Content: content}},
	})
}
