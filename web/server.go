package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"agent-platform/llm"
	"agent-platform/memory"
	"agent-platform/tools"
	"agent-platform/workflow"
)

// Server exposes a small web UI to run workflows and visualize execution live.
type Server struct {
	client llm.Client
	store  *memory.Store
	tools  *tools.Registry
}

func NewServer(client llm.Client, store *memory.Store) *Server {
	return NewServerWithRegistry(client, store, tools.NewDefaultRegistry(store))
}

func NewServerWithRegistry(client llm.Client, store *memory.Store, registry *tools.Registry) *Server {
	return &Server{client: client, store: store, tools: registry}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/tools", s.handleTools)
	mux.HandleFunc("/api/memory", s.handleMemory)
	mux.HandleFunc("/api/run", s.handleRun)
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	// Build a throwaway engine just to enumerate registered tools.
	eng := workflow.NewEngineWithRegistry(s.client, s.store, s.tools)
	writeJSON(w, eng.ToolInfo())
}

func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.store.AllInNamespace(r.URL.Query().Get("namespace")))
}

// handleRun streams workflow events over Server-Sent Events (SSE).
func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	goal := r.URL.Query().Get("goal")
	if goal == "" {
		http.Error(w, "missing goal", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	eng := workflow.NewEngineWithRegistry(s.client, s.store, s.tools)

	var mu sync.Mutex
	send := func(ev workflow.Event) {
		mu.Lock()
		defer mu.Unlock()
		b, err := json.Marshal(ev)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}
	eng.OnEvent = send

	ctx := r.Context()
	if _, err := eng.Run(ctx, goal); err != nil {
		send(workflow.Event{Kind: "error", Goal: goal, Detail: err.Error()})
	}
	// Signal stream end.
	mu.Lock()
	fmt.Fprint(w, "event: end\ndata: {}\n\n")
	flusher.Flush()
	mu.Unlock()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
