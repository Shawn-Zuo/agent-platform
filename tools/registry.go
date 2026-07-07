package tools

import (
	"context"
	"fmt"

	"agent-platform/core"
)

type Registry struct {
	tools map[string]core.Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]core.Tool)}
}

func (r *Registry) Register(t core.Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (core.Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Execute(ctx context.Context, name string, input map[string]any) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("tool %q not found", name)
	}
	return t.Execute(ctx, input)
}

func (r *Registry) All() []core.Tool {
	out := make([]core.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}
