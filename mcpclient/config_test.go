package mcpclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	data := []byte(`{
  "mcpServers": {
    "demo": {
      "command": "demo-server",
      "args": ["--stdio"],
      "env": {"TOKEN": "secret"}
    }
  }
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	server := cfg.MCPServers["demo"]
	if server.Command != "demo-server" || len(server.Args) != 1 || server.Env["TOKEN"] != "secret" {
		t.Fatalf("unexpected config: %+v", server)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "no servers", cfg: Config{}},
		{name: "empty name", cfg: Config{MCPServers: map[string]ServerConfig{"": {Command: "server"}}}},
		{name: "missing command", cfg: Config{MCPServers: map[string]ServerConfig{"demo": {}}}},
		{name: "invalid env", cfg: Config{MCPServers: map[string]ServerConfig{"demo": {Command: "server", Env: map[string]string{"A=B": "x"}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.cfg.Validate(); err == nil {
				t.Fatal("Validate succeeded, want error")
			}
		})
	}
}
