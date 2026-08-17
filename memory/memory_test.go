package memory

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreNamespacesAndTTL(t *testing.T) {
	store := NewStore()
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	if err := store.SetWithOptions("preference", "dark mode", WriteOptions{
		Namespace: "user-1",
		TTL:       time.Minute,
		Tags:      []string{"profile", "ui", "profile"},
	}); err != nil {
		t.Fatalf("SetWithOptions: %v", err)
	}
	if _, ok := store.GetInNamespace("user-2", "preference"); ok {
		t.Fatal("memory leaked across namespaces")
	}
	if got, ok := store.GetInNamespace("user-1", "preference"); !ok || got != "dark mode" {
		t.Fatalf("GetInNamespace() = %q, %v", got, ok)
	}
	entries := store.GetByTagInNamespace("user-1", "profile")
	if len(entries) != 1 || len(entries[0].Tags) != 2 {
		t.Fatalf("tag lookup = %+v", entries)
	}

	now = now.Add(time.Minute)
	if _, ok := store.GetInNamespace("user-1", "preference"); ok {
		t.Fatal("expired memory was returned")
	}
}

func TestStoreSearchAndDelete(t *testing.T) {
	store := NewStore()
	if err := store.SetWithOptions("language", "Go", WriteOptions{
		Namespace: "session-a",
		Tags:      []string{"backend"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetWithOptions("editor", "VS Code", WriteOptions{Namespace: "session-a"}); err != nil {
		t.Fatal(err)
	}

	results := store.Search("session-a", "backend", 10)
	if len(results) != 1 || results[0].Key != "language" {
		t.Fatalf("Search() = %+v", results)
	}
	deleted, err := store.Delete("session-a", "language")
	if err != nil || !deleted {
		t.Fatalf("Delete() = %v, %v", deleted, err)
	}
	if _, ok := store.GetInNamespace("session-a", "language"); ok {
		t.Fatal("deleted memory was returned")
	}
}

func TestPersistentStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "memory.json")
	store, err := NewPersistentStore(path)
	if err != nil {
		t.Fatalf("NewPersistentStore: %v", err)
	}
	if err := store.SetWithOptions("answer", "42", WriteOptions{
		Namespace: "conversation-7",
		Tags:      []string{"fact"},
	}); err != nil {
		t.Fatalf("SetWithOptions: %v", err)
	}

	reloaded, err := NewPersistentStore(path)
	if err != nil {
		t.Fatalf("reload persistent store: %v", err)
	}
	if got, ok := reloaded.GetInNamespace("conversation-7", "answer"); !ok || got != "42" {
		t.Fatalf("reloaded memory = %q, %v", got, ok)
	}
	if entries := reloaded.GetByTagInNamespace("conversation-7", "fact"); len(entries) != 1 {
		t.Fatalf("reloaded tags = %+v", entries)
	}
}
