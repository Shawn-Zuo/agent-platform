// Package mcpserver exposes agent-platform tools through the Model Context
// Protocol (MCP).
package mcpserver

import (
	"context"
	"errors"
	"io"

	"agent-platform/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "agent-platform"
	serverVersion = "0.1.0"
)

// New builds an MCP server backed by the supplied local tool registry.
// Tool errors are returned as MCP tool errors so an LLM can inspect them and
// potentially correct its next call.
func New(registry *tools.Registry) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, &mcp.ServerOptions{
		// The registry is static for the lifetime of this process, so advertising
		// list-changed notifications would be misleading and can produce a
		// notification before initialization when tools are registered quickly.
		Capabilities: &mcp.ServerCapabilities{
			Tools: &mcp.ToolCapabilities{},
		},
	})

	for _, registeredTool := range registry.All() {
		tool := registeredTool
		mcp.AddTool(server, &mcp.Tool{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
			output, err := tool.Execute(ctx, input)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: output}},
			}, nil, nil
		})
	}

	return server
}

// RunStdio serves MCP JSON-RPC messages on stdin/stdout until the client
// disconnects or the context is cancelled.
func RunStdio(ctx context.Context, registry *tools.Registry) error {
	err := New(registry).Run(ctx, &mcp.StdioTransport{})
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
