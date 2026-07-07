package memory

import (
	"fmt"
	"sync"
	"time"
)

type Entry struct {
	Key       string
	Value     string
	Timestamp time.Time
	Tags      []string
}

type Store struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

func NewStore() *Store {
	return &Store{entries: make(map[string]Entry)}
}

func (s *Store) Set(key, value string, tags ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = Entry{
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
		Tags:      tags,
	}
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[key]
	if !ok {
		return "", false
	}
	return e.Value, true
}

func (s *Store) GetByTag(tag string) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Entry
	for _, e := range s.entries {
		for _, t := range e.Tags {
			if t == tag {
				result = append(result, e)
				break
			}
		}
	}
	return result
}

func (s *Store) Summary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.entries) == 0 {
		return "(no memories stored)"
	}
	summary := fmt.Sprintf("Memory store has %d entries:\n", len(s.entries))
	for k, e := range s.entries {
		summary += fmt.Sprintf("  [%s] = %s\n", k, e.Value)
	}
	return summary
}

func (s *Store) All() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.entries))
	for k, e := range s.entries {
		out[k] = e.Value
	}
	return out
}
