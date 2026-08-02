package mcpclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxToolNameLength = 64

type remoteTool struct {
	name        string
	remoteName  string
	description string
	inputSchema map[string]any
	session     *mcp.ClientSession
}

func newRemoteTool(serverName string, session *mcp.ClientSession, tool *mcp.Tool) (*remoteTool, error) {
	if tool == nil || strings.TrimSpace(tool.Name) == "" {
		return nil, fmt.Errorf("MCP server %q returned a tool without a name", serverName)
	}
	schema, err := schemaMap(tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("MCP tool %q input schema: %w", tool.Name, err)
	}
	description := strings.TrimSpace(tool.Description)
	if description == "" {
		description = "Tool provided by an external MCP server"
	}
	return &remoteTool{
		name:        publicToolName(serverName, tool.Name),
		remoteName:  tool.Name,
		description: fmt.Sprintf("[MCP server %s] %s", serverName, description),
		inputSchema: schema,
		session:     session,
	}, nil
}

func (t *remoteTool) Name() string                { return t.name }
func (t *remoteTool) Description() string         { return t.description }
func (t *remoteTool) InputSchema() map[string]any { return t.inputSchema }
func (t *remoteTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	result, err := t.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      t.remoteName,
		Arguments: input,
	})
	if err != nil {
		return "", fmt.Errorf("call remote MCP tool %q: %w", t.remoteName, err)
	}
	output, err := resultText(result)
	if err != nil {
		return "", fmt.Errorf("decode remote MCP tool %q result: %w", t.remoteName, err)
	}
	if result.IsError {
		return "", fmt.Errorf("remote MCP tool %q failed: %s", t.remoteName, output)
	}
	return output, nil
}

func schemaMap(schema any) (map[string]any, error) {
	if schema == nil {
		return map[string]any{"type": "object"}, nil
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	if result["type"] == nil {
		result["type"] = "object"
	}
	if result["type"] != "object" {
		return nil, fmt.Errorf("schema type is %q, want object", result["type"])
	}
	return result, nil
}

func resultText(result *mcp.CallToolResult) (string, error) {
	if result == nil {
		return "", fmt.Errorf("empty result")
	}
	parts := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
			continue
		}
		b, err := content.MarshalJSON()
		if err != nil {
			return "", err
		}
		parts = append(parts, string(b))
	}
	if len(parts) == 0 && result.StructuredContent != nil {
		b, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return "", err
		}
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n"), nil
}

// publicToolName produces names accepted by the LLM providers used by this
// project while preserving a readable MCP server namespace.
func publicToolName(serverName, toolName string) string {
	raw := serverName + "__" + toolName
	var normalized strings.Builder
	for _, r := range raw {
		if isASCIILetterOrDigit(r) || r == '_' || r == '-' {
			normalized.WriteRune(r)
		} else {
			normalized.WriteByte('_')
		}
	}
	name := strings.Trim(normalized.String(), "_-")
	if name == "" {
		name = "mcp_tool"
	}
	if len(name) <= maxToolNameLength {
		return name
	}
	sum := sha256.Sum256([]byte(raw))
	suffix := "_" + hex.EncodeToString(sum[:4])
	return name[:maxToolNameLength-len(suffix)] + suffix
}

func isASCIILetterOrDigit(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}
