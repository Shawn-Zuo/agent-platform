package mcpclient

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Config uses the same top-level shape as common MCP host configuration files.
type Config struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

// ServerConfig describes a local MCP server launched over stdio.
type ServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// LoadConfig reads and validates an MCP client configuration file.
func LoadConfig(filename string) (Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return Config{}, fmt.Errorf("open MCP config: %w", err)
	}
	defer f.Close()

	var cfg Config
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode MCP config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return Config{}, fmt.Errorf("decode MCP config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks server names and process configuration before any subprocess
// is started.
func (c Config) Validate() error {
	if len(c.MCPServers) == 0 {
		return fmt.Errorf("MCP config must contain at least one server")
	}
	for name, server := range c.MCPServers {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("MCP server name cannot be empty")
		}
		if strings.TrimSpace(server.Command) == "" {
			return fmt.Errorf("MCP server %q: command is required", name)
		}
		for key := range server.Env {
			if key == "" || strings.ContainsRune(key, '=') {
				return fmt.Errorf("MCP server %q: invalid environment key %q", name, key)
			}
		}
	}
	return nil
}
