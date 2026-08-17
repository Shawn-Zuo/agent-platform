package tools

import (
	"context"
	"strings"
	"testing"

	"agent-platform/memory"
)

func TestMemoryToolsNamespacedLifecycle(t *testing.T) {
	store := memory.NewStore()
	write := NewMemoryWriteTool(store)
	read := NewMemoryReadTool(store)
	search := NewMemorySearchTool(store)
	deleteTool := NewMemoryDeleteTool(store)
	ctx := context.Background()

	_, err := write.Execute(ctx, map[string]any{
		"namespace":   "user-9",
		"key":         "preferred_language",
		"value":       "Go",
		"tags":        []any{"profile", "backend"},
		"ttl_seconds": float64(60),
	})
	if err != nil {
		t.Fatalf("memory_write: %v", err)
	}
	got, err := read.Execute(ctx, map[string]any{
		"namespace": "user-9",
		"key":       "preferred_language",
	})
	if err != nil || got != "Go" {
		t.Fatalf("memory_read = %q, %v", got, err)
	}
	searchResult, err := search.Execute(ctx, map[string]any{
		"namespace": "user-9",
		"query":     "backend",
		"limit":     float64(5),
	})
	if err != nil || !strings.Contains(searchResult, "preferred_language") {
		t.Fatalf("memory_search = %q, %v", searchResult, err)
	}
	if _, err := deleteTool.Execute(ctx, map[string]any{
		"namespace": "user-9",
		"key":       "preferred_language",
	}); err != nil {
		t.Fatalf("memory_delete: %v", err)
	}
	if _, ok := store.GetInNamespace("user-9", "preferred_language"); ok {
		t.Fatal("memory_delete did not remove the value")
	}
}

func TestMemoryWriteRejectsInvalidTTL(t *testing.T) {
	_, err := NewMemoryWriteTool(memory.NewStore()).Execute(context.Background(), map[string]any{
		"key":         "x",
		"value":       "y",
		"ttl_seconds": 1.5,
	})
	if err == nil {
		t.Fatal("memory_write accepted a fractional TTL")
	}
}
