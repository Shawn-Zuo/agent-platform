package tools

import (
	"context"
	"fmt"

	"agent-platform/memory"
)

type MemoryReadTool struct {
	store *memory.Store
}

func NewMemoryReadTool(store *memory.Store) *MemoryReadTool {
	return &MemoryReadTool{store: store}
}

func (t *MemoryReadTool) Name() string { return "memory_read" }

func (t *MemoryReadTool) Description() string {
	return "Read a value from agent memory by key, or list all memories"
}

func (t *MemoryReadTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{
				"type":        "string",
				"description": "Key to read. If empty, returns all memories.",
			},
		},
	}
}

func (t *MemoryReadTool) Execute(_ context.Context, input map[string]any) (string, error) {
	key, _ := input["key"].(string)
	if key == "" {
		return t.store.Summary(), nil
	}
	val, ok := t.store.Get(key)
	if !ok {
		return fmt.Sprintf("No memory found for key %q", key), nil
	}
	return val, nil
}

type MemoryWriteTool struct {
	store *memory.Store
}

func NewMemoryWriteTool(store *memory.Store) *MemoryWriteTool {
	return &MemoryWriteTool{store: store}
}

func (t *MemoryWriteTool) Name() string { return "memory_write" }

func (t *MemoryWriteTool) Description() string {
	return "Store a value in agent memory under a given key"
}

func (t *MemoryWriteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key":   map[string]any{"type": "string", "description": "Memory key"},
			"value": map[string]any{"type": "string", "description": "Value to store"},
		},
		"required": []string{"key", "value"},
	}
}

func (t *MemoryWriteTool) Execute(_ context.Context, input map[string]any) (string, error) {
	key, _ := input["key"].(string)
	value, _ := input["value"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}
	t.store.Set(key, value)
	return fmt.Sprintf("Stored: [%s] = %s", key, value), nil
}
