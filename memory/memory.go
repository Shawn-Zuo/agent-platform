package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const DefaultNamespace = "default"

type Entry struct {
	Namespace string     `json:"namespace"`
	Key       string     `json:"key"`
	Value     string     `json:"value"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
}

type WriteOptions struct {
	Namespace string
	TTL       time.Duration
	Tags      []string
}

type Store struct {
	mu              sync.RWMutex
	entries         map[string]Entry
	persistencePath string
	now             func() time.Time
}

func NewStore() *Store {
	return &Store{
		entries: make(map[string]Entry),
		now:     time.Now,
	}
}

// NewPersistentStore loads a JSON-backed store. Mutations are written through
// with an atomic rename, while reads remain in-memory and concurrency-safe.
func NewPersistentStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("memory persistence path is required")
	}
	store := NewStore()
	store.persistencePath = filepath.Clean(path)
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

// Set keeps the original API for callers that do not need persistence errors.
// New code should prefer SetWithOptions.
func (s *Store) Set(key, value string, tags ...string) {
	_ = s.SetWithOptions(key, value, WriteOptions{Tags: tags})
}

func (s *Store) SetWithOptions(key, value string, options WriteOptions) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("memory key is required")
	}
	if options.TTL < 0 {
		return fmt.Errorf("memory TTL must not be negative")
	}
	namespace := normalizeNamespace(options.Namespace)
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(now)

	mapKey := entryKey(namespace, key)
	createdAt := now
	if existing, ok := s.entries[mapKey]; ok {
		createdAt = existing.CreatedAt
	}
	var expiresAt *time.Time
	if options.TTL > 0 {
		expiry := now.Add(options.TTL)
		expiresAt = &expiry
	}
	s.entries[mapKey] = Entry{
		Namespace: namespace,
		Key:       key,
		Value:     value,
		CreatedAt: createdAt,
		UpdatedAt: now,
		ExpiresAt: expiresAt,
		Tags:      normalizeTags(options.Tags),
	}
	return s.persistLocked()
}

func (s *Store) Get(key string) (string, bool) {
	return s.GetInNamespace(DefaultNamespace, key)
}

func (s *Store) GetInNamespace(namespace, key string) (string, bool) {
	namespace = normalizeNamespace(namespace)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(s.now())
	entry, ok := s.entries[entryKey(namespace, key)]
	if !ok {
		return "", false
	}
	return entry.Value, true
}

func (s *Store) Delete(namespace, key string) (bool, error) {
	namespace = normalizeNamespace(namespace)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(s.now())
	mapKey := entryKey(namespace, key)
	if _, ok := s.entries[mapKey]; !ok {
		return false, nil
	}
	delete(s.entries, mapKey)
	if err := s.persistLocked(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) GetByTag(tag string) []Entry {
	return s.GetByTagInNamespace(DefaultNamespace, tag)
}

func (s *Store) GetByTagInNamespace(namespace, tag string) []Entry {
	namespace = normalizeNamespace(namespace)
	tag = strings.TrimSpace(tag)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(s.now())
	var result []Entry
	for _, entry := range s.entries {
		if entry.Namespace != namespace {
			continue
		}
		for _, entryTag := range entry.Tags {
			if entryTag == tag {
				result = append(result, cloneEntry(entry))
				break
			}
		}
	}
	sortEntries(result)
	return result
}

// Search performs deterministic keyword recall over keys, values and tags.
// It intentionally does not claim vector or semantic search.
func (s *Store) Search(namespace, query string, limit int) []Entry {
	namespace = normalizeNamespace(namespace)
	query = strings.ToLower(strings.TrimSpace(query))
	if limit <= 0 {
		limit = 10
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(s.now())

	type rankedEntry struct {
		entry Entry
		score int
	}
	var ranked []rankedEntry
	for _, entry := range s.entries {
		if entry.Namespace != namespace {
			continue
		}
		score := 0
		if query == "" || strings.Contains(strings.ToLower(entry.Key), query) {
			score += 3
		}
		if query != "" && strings.Contains(strings.ToLower(entry.Value), query) {
			score++
		}
		for _, tag := range entry.Tags {
			if query != "" && strings.Contains(strings.ToLower(tag), query) {
				score += 2
			}
		}
		if score > 0 {
			ranked = append(ranked, rankedEntry{entry: cloneEntry(entry), score: score})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if !ranked[i].entry.UpdatedAt.Equal(ranked[j].entry.UpdatedAt) {
			return ranked[i].entry.UpdatedAt.After(ranked[j].entry.UpdatedAt)
		}
		return ranked[i].entry.Key < ranked[j].entry.Key
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	result := make([]Entry, len(ranked))
	for i := range ranked {
		result[i] = ranked[i].entry
	}
	return result
}

func (s *Store) Summary() string {
	return s.SummaryInNamespace(DefaultNamespace)
}

func (s *Store) SummaryInNamespace(namespace string) string {
	entries := s.Entries(namespace)
	if len(entries) == 0 {
		return "(no memories stored)"
	}
	var summary strings.Builder
	fmt.Fprintf(&summary, "Memory namespace %q has %d entries:\n", normalizeNamespace(namespace), len(entries))
	for _, entry := range entries {
		fmt.Fprintf(&summary, "  [%s] = %s\n", entry.Key, entry.Value)
	}
	return summary.String()
}

func (s *Store) Entries(namespace string) []Entry {
	namespace = normalizeNamespace(namespace)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(s.now())
	entries := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.Namespace == namespace {
			entries = append(entries, cloneEntry(entry))
		}
	}
	sortEntries(entries)
	return entries
}

func (s *Store) All() map[string]string {
	return s.AllInNamespace(DefaultNamespace)
}

func (s *Store) AllInNamespace(namespace string) map[string]string {
	entries := s.Entries(namespace)
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		out[entry.Key] = entry.Value
	}
	return out
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.persistencePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read memory store: %w", err)
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("decode memory store: %w", err)
	}
	now := s.now()
	for _, entry := range entries {
		entry.Namespace = normalizeNamespace(entry.Namespace)
		if strings.TrimSpace(entry.Key) == "" || isExpired(entry, now) {
			continue
		}
		entry.Tags = normalizeTags(entry.Tags)
		s.entries[entryKey(entry.Namespace, entry.Key)] = entry
	}
	return nil
}

func (s *Store) persistLocked() error {
	if s.persistencePath == "" {
		return nil
	}
	directory := filepath.Dir(s.persistencePath)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}
	entries := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		entries = append(entries, cloneEntry(entry))
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Namespace != entries[j].Namespace {
			return entries[i].Namespace < entries[j].Namespace
		}
		return entries[i].Key < entries[j].Key
	})
	temporary, err := os.CreateTemp(directory, ".agent-memory-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary memory store: %w", err)
	}
	temporaryPath := temporary.Name()
	cleanup := func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(entries); err != nil {
		cleanup()
		return fmt.Errorf("encode memory store: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync memory store: %w", err)
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("close memory store: %w", err)
	}
	if err := os.Rename(temporaryPath, s.persistencePath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("replace memory store: %w", err)
	}
	return nil
}

func (s *Store) purgeExpiredLocked(now time.Time) {
	for key, entry := range s.entries {
		if isExpired(entry, now) {
			delete(s.entries, key)
		}
	}
}

func normalizeNamespace(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return DefaultNamespace
	}
	return namespace
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]bool, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}

func entryKey(namespace, key string) string {
	return namespace + "\x00" + key
}

func isExpired(entry Entry, now time.Time) bool {
	return entry.ExpiresAt != nil && !now.Before(*entry.ExpiresAt)
}

func cloneEntry(entry Entry) Entry {
	entry.Tags = append([]string(nil), entry.Tags...)
	return entry
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].UpdatedAt.Equal(entries[j].UpdatedAt) {
			return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
		}
		return entries[i].Key < entries[j].Key
	})
}
