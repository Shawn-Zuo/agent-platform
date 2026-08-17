package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	return "Read a value from a namespaced agent memory, or list the namespace"
}

func (t *MemoryReadTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{
				"type":        "string",
				"description": "Key to read. If empty, returns all memories.",
			},
			"namespace": map[string]any{
				"type":        "string",
				"description": "Memory namespace, such as a user or session ID. Defaults to default.",
			},
		},
	}
}

func (t *MemoryReadTool) Execute(_ context.Context, input map[string]any) (string, error) {
	key, _ := input["key"].(string)
	namespace, _ := input["namespace"].(string)
	if key == "" {
		return t.store.SummaryInNamespace(namespace), nil
	}
	val, ok := t.store.GetInNamespace(namespace, key)
	if !ok {
		return fmt.Sprintf("No memory found for key %q in namespace %q", key, displayNamespace(namespace)), nil
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
	return "Store a value in namespaced agent memory with optional tags and TTL"
}

func (t *MemoryWriteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key":   map[string]any{"type": "string", "description": "Memory key"},
			"value": map[string]any{"type": "string", "description": "Value to store"},
			"namespace": map[string]any{
				"type":        "string",
				"description": "Memory namespace, such as a user or session ID. Defaults to default.",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional tags used by memory_search.",
			},
			"ttl_seconds": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"description": "Optional lifetime in seconds; 0 means no expiry.",
			},
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
	namespace, _ := input["namespace"].(string)
	tags, err := stringSlice(input["tags"])
	if err != nil {
		return "", err
	}
	ttlSeconds, err := nonNegativeInt(input["ttl_seconds"], 0)
	if err != nil {
		return "", fmt.Errorf("ttl_seconds: %w", err)
	}
	if err := t.store.SetWithOptions(key, value, memory.WriteOptions{
		Namespace: namespace,
		TTL:       time.Duration(ttlSeconds) * time.Second,
		Tags:      tags,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Stored in namespace %q: [%s] = %s", displayNamespace(namespace), key, value), nil
}

type MemorySearchTool struct {
	store *memory.Store
}

func NewMemorySearchTool(store *memory.Store) *MemorySearchTool {
	return &MemorySearchTool{store: store}
}

func (t *MemorySearchTool) Name() string { return "memory_search" }

func (t *MemorySearchTool) Description() string {
	return "Search namespaced agent memory by keyword across keys, values and tags"
}

func (t *MemorySearchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "Keyword to recall"},
			"namespace": map[string]any{
				"type":        "string",
				"description": "Memory namespace. Defaults to default.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     100,
				"description": "Maximum number of results. Defaults to 10.",
			},
		},
		"required": []string{"query"},
	}
}

func (t *MemorySearchTool) Execute(_ context.Context, input map[string]any) (string, error) {
	query, _ := input["query"].(string)
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("query is required")
	}
	namespace, _ := input["namespace"].(string)
	limit, err := nonNegativeInt(input["limit"], 10)
	if err != nil || limit < 1 || limit > 100 {
		return "", fmt.Errorf("limit must be an integer between 1 and 100")
	}
	entries := t.store.Search(namespace, query, limit)
	if len(entries) == 0 {
		return fmt.Sprintf("No memories matched %q in namespace %q", query, displayNamespace(namespace)), nil
	}
	var output strings.Builder
	fmt.Fprintf(&output, "Found %d memories in namespace %q:\n", len(entries), displayNamespace(namespace))
	for _, entry := range entries {
		fmt.Fprintf(&output, "  [%s] = %s", entry.Key, entry.Value)
		if len(entry.Tags) > 0 {
			fmt.Fprintf(&output, " (tags: %s)", strings.Join(entry.Tags, ", "))
		}
		output.WriteByte('\n')
	}
	return output.String(), nil
}

type MemoryDeleteTool struct {
	store *memory.Store
}

func NewMemoryDeleteTool(store *memory.Store) *MemoryDeleteTool {
	return &MemoryDeleteTool{store: store}
}

func (t *MemoryDeleteTool) Name() string { return "memory_delete" }

func (t *MemoryDeleteTool) Description() string {
	return "Delete a value from a namespaced agent memory"
}

func (t *MemoryDeleteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{"type": "string", "description": "Memory key to delete"},
			"namespace": map[string]any{
				"type":        "string",
				"description": "Memory namespace. Defaults to default.",
			},
		},
		"required": []string{"key"},
	}
}

func (t *MemoryDeleteTool) Execute(_ context.Context, input map[string]any) (string, error) {
	key, _ := input["key"].(string)
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("key is required")
	}
	namespace, _ := input["namespace"].(string)
	deleted, err := t.store.Delete(namespace, key)
	if err != nil {
		return "", err
	}
	if !deleted {
		return fmt.Sprintf("No memory found for key %q in namespace %q", key, displayNamespace(namespace)), nil
	}
	return fmt.Sprintf("Deleted key %q from namespace %q", key, displayNamespace(namespace)), nil
}

func displayNamespace(namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		return memory.DefaultNamespace
	}
	return strings.TrimSpace(namespace)
}

func stringSlice(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case []string:
		return typed, nil
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("tags must contain only strings")
			}
			result = append(result, text)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("tags must be an array of strings")
	}
}

func nonNegativeInt(value any, fallback int) (int, error) {
	if value == nil {
		return fallback, nil
	}
	switch typed := value.(type) {
	case int:
		if typed < 0 {
			return 0, fmt.Errorf("must not be negative")
		}
		return typed, nil
	case float64:
		integer := int(typed)
		if typed < 0 || float64(integer) != typed {
			return 0, fmt.Errorf("must be a non-negative integer")
		}
		return integer, nil
	default:
		return 0, fmt.Errorf("must be a non-negative integer")
	}
}
