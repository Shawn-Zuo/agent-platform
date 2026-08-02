package mcpclient

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"agent-platform/core"
	"agent-platform/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConnectDiscoversAndCallsStdioTool(t *testing.T) {
	cfg := Config{MCPServers: map[string]ServerConfig{
		"demo": {
			Command: os.Args[0],
			Args:    []string{"-test.run=^TestMCPHelperProcess$"},
			Env: map[string]string{
				"GO_WANT_MCP_HELPER_PROCESS": "1",
				"MCP_HELPER_PREFIX":          "remote:",
			},
		},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	manager, err := Connect(ctx, cfg)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	registry := tools.NewRegistry()
	if err := manager.RegisterTools(registry); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}
	tool, ok := registry.Get("demo__echo")
	if !ok {
		t.Fatalf("remote tool demo__echo was not registered")
	}
	got, err := tool.Execute(context.Background(), map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "remote:hello" {
		t.Fatalf("Execute output = %q, want %q", got, "remote:hello")
	}
	failTool, ok := registry.Get("demo__fail")
	if !ok {
		t.Fatalf("remote tool demo__fail was not registered")
	}
	if _, err := failTool.Execute(context.Background(), map[string]any{}); err == nil {
		t.Fatal("remote tool error was not propagated")
	}
}

func TestRegisterToolsRejectsCollision(t *testing.T) {
	// Collision behavior is covered through the concrete remote tool without a
	// live session because registration does not execute it.
	tool := &remoteTool{name: "demo__echo"}
	manager := &Manager{tools: []core.Tool{tool}}
	registry := tools.NewRegistry()
	registry.Register(tool)
	if err := manager.RegisterTools(registry); err == nil {
		t.Fatal("RegisterTools succeeded, want a name collision error")
	}
}

func TestMCPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_HELPER_PROCESS") != "1" {
		return
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "Echo text",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string"},
			},
			"required": []string{"text"},
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.TextContent{Text: os.Getenv("MCP_HELPER_PREFIX") + input["text"].(string)},
		}}, nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "fail",
		Description: "Return a tool error",
		InputSchema: map[string]any{"type": "object"},
	}, func(context.Context, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, any, error) {
		return nil, nil, errors.New("expected remote failure")
	})

	err := server.Run(context.Background(), &mcp.StdioTransport{})
	if err != nil && !errors.Is(err, io.EOF) {
		os.Exit(2)
	}
	os.Exit(0)
}
