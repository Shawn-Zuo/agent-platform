// Package mcpclient connects agent-platform to external Model Context Protocol
// servers and adapts their tools to core.Tool.
package mcpclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"agent-platform/core"
	"agent-platform/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const clientVersion = "0.1.0"

// Manager owns external MCP sessions and the tools discovered from them.
type Manager struct {
	sessions []*mcp.ClientSession
	tools    []core.Tool
}

// Connect launches every configured stdio server and discovers its tools. It
// is all-or-nothing: if a server fails, already-opened sessions are closed.
func Connect(ctx context.Context, cfg Config) (*Manager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	manager := &Manager{}
	serverNames := make([]string, 0, len(cfg.MCPServers))
	for name := range cfg.MCPServers {
		serverNames = append(serverNames, name)
	}
	sort.Strings(serverNames)

	for _, name := range serverNames {
		if err := manager.connectServer(ctx, name, cfg.MCPServers[name]); err != nil {
			_ = manager.Close()
			return nil, fmt.Errorf("connect MCP server %q: %w", name, err)
		}
	}
	return manager, nil
}

func (m *Manager) connectServer(ctx context.Context, serverName string, cfg ServerConfig) error {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = mergedEnvironment(cfg.Env)
	cmd.Stderr = os.Stderr

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "agent-platform",
		Version: clientVersion,
	}, &mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}})

	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return err
	}
	m.sessions = append(m.sessions, session)

	if session.InitializeResult().Capabilities.Tools == nil {
		return nil
	}
	for remote, err := range session.Tools(ctx, nil) {
		if err != nil {
			return fmt.Errorf("list tools: %w", err)
		}
		tool, err := newRemoteTool(serverName, session, remote)
		if err != nil {
			return err
		}
		m.tools = append(m.tools, tool)
	}
	return nil
}

// RegisterTools adds all discovered remote tools to registry. Name collisions
// are rejected rather than silently replacing local or previously-added tools.
func (m *Manager) RegisterTools(registry *tools.Registry) error {
	seen := make(map[string]bool, len(m.tools))
	for _, tool := range m.tools {
		_, exists := registry.Get(tool.Name())
		if exists || seen[tool.Name()] {
			return fmt.Errorf("MCP tool name %q conflicts with an existing tool", tool.Name())
		}
		seen[tool.Name()] = true
	}
	for _, tool := range m.tools {
		registry.Register(tool)
	}
	return nil
}

// Tools returns a snapshot of the discovered remote tools.
func (m *Manager) Tools() []core.Tool {
	return append([]core.Tool(nil), m.tools...)
}

// Close closes every MCP session and its child process. It is safe to call
// more than once because ClientSession.Close is idempotent.
func (m *Manager) Close() error {
	var errs []error
	for i := len(m.sessions) - 1; i >= 0; i-- {
		if err := m.sessions[i].Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func mergedEnvironment(overrides map[string]string) []string {
	env := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	for key, value := range overrides {
		env[key] = value
	}

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, key+"="+env[key])
	}
	return values
}
