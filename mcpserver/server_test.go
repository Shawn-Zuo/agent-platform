package mcpserver

import (
	"context"
	"sort"
	"testing"

	"agent-platform/memory"
	"agent-platform/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connectTestClient(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	server := New(tools.NewDefaultRegistry(memory.NewStore()))
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect MCP server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func TestServerListsDefaultTools(t *testing.T) {
	client := connectTestClient(t)
	result, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	var names []string
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	want := []string{"calculator", "memory_delete", "memory_read", "memory_search", "memory_write", "search_knowledge_base"}
	if len(names) != len(want) {
		t.Fatalf("tool names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("tool names = %v, want %v", names, want)
		}
	}
}

func TestServerCallsCalculator(t *testing.T) {
	client := connectTestClient(t)
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "calculator",
		Arguments: map[string]any{
			"operation": "multiply",
			"a":         123,
			"b":         456,
		},
	})
	if err != nil {
		t.Fatalf("call calculator: %v", err)
	}
	if result.IsError {
		t.Fatalf("calculator returned a tool error: %v", result.Content)
	}
	if got := textContent(t, result); got != "56088" {
		t.Fatalf("calculator output = %q, want %q", got, "56088")
	}
}

func TestServerReturnsToolErrorsToClient(t *testing.T) {
	client := connectTestClient(t)
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "calculator",
		Arguments: map[string]any{
			"operation": "divide",
			"a":         1,
			"b":         0,
		},
	})
	if err != nil {
		t.Fatalf("call calculator: %v", err)
	}
	if !result.IsError {
		t.Fatalf("calculator IsError = false, want true")
	}
	if got := textContent(t, result); got != "division by zero" {
		t.Fatalf("calculator error = %q, want %q", got, "division by zero")
	}
}

func TestServerSharesMemoryAcrossToolCalls(t *testing.T) {
	client := connectTestClient(t)
	ctx := context.Background()

	writeResult, err := client.CallTool(ctx, &mcp.CallToolParams{
		Name:      "memory_write",
		Arguments: map[string]any{"key": "answer", "value": "42"},
	})
	if err != nil || writeResult.IsError {
		t.Fatalf("write memory: result=%v err=%v", writeResult, err)
	}

	readResult, err := client.CallTool(ctx, &mcp.CallToolParams{
		Name:      "memory_read",
		Arguments: map[string]any{"key": "answer"},
	})
	if err != nil {
		t.Fatalf("read memory: %v", err)
	}
	if got := textContent(t, readResult); got != "42" {
		t.Fatalf("memory output = %q, want %q", got, "42")
	}
}

func textContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("content length = %d, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.TextContent", result.Content[0])
	}
	return text.Text
}
