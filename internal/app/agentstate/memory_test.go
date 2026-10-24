package agentstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func memoryFixture() MemoryRecord {
	return MemoryRecord{ID: "memory", ProjectID: "project", ProjectRevision: "revision", Policy: "policy", Kind: "lesson",
		Title: "Cancellation", Text: "Cancellation must stop owned child processes.",
		Sources:    []MemorySource{{Path: "main.go", Hash: "sha256:" + strings.Repeat("a", 64), StartLine: 2, EndLine: 8}},
		Provenance: MemoryProvenance{Origin: "user"}, ExpiresAt: time.Now().UTC().Add(24 * time.Hour)}
}

func memoryStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func allowMemoryWrite() error { return nil }

func acceptedMemory(t *testing.T, store *Store, record MemoryRecord) MemoryRecord {
	t.Helper()
	saved, err := store.SaveMemory(t.Context(), record, 0, allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := store.AcceptMemory(t.Context(), saved.ProjectID, saved.ID, saved.Version, allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	return accepted
}

func TestMemoryRequiresExplicitAcceptanceAndVersionedEdits(t *testing.T) {
	store := memoryStore(t)
	record := memoryFixture()
	saved, err := store.SaveMemory(t.Context(), record, 0, allowMemoryWrite)
	if err != nil || saved.State != "candidate" || saved.Version != 1 {
		t.Fatalf("candidate=%+v %v", saved, err)
	}
	query := func() []MemoryMatch {
		t.Helper()
		matches, err := store.SearchMemory(t.Context(), record.ProjectID, record.ProjectRevision, record.Policy, "cancellation", record.Sources, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return matches
	}
	if len(query()) != 0 {
		t.Fatal("candidate became retrievable")
	}
	accepted, err := store.AcceptMemory(t.Context(), record.ProjectID, record.ID, 1, allowMemoryWrite)
	if err != nil || accepted.Version != 2 || len(query()) != 1 {
		t.Fatalf("acceptance=%+v %v", accepted, err)
	}
	accepted.Text = "Cancellation must join all child processes."
	edited, err := store.SaveMemory(t.Context(), accepted, 2, allowMemoryWrite)
	if err != nil || edited.Version != 3 || edited.State != "candidate" || len(query()) != 0 {
		t.Fatalf("edit retained acceptance: %+v %v", edited, err)
	}
	if _, err := store.AcceptMemory(t.Context(), record.ProjectID, record.ID, 2, allowMemoryWrite); !errors.Is(err, ErrMemoryConflict) {
		t.Fatalf("stale acceptance=%v", err)
	}
	accepted, err = store.AcceptMemory(t.Context(), record.ProjectID, record.ID, 3, allowMemoryWrite)
	if err != nil || accepted.Version != 4 || len(query()) != 1 {
		t.Fatalf("fresh acceptance=%+v %v", accepted, err)
	}
	if err := store.ForgetMemory(t.Context(), record.ProjectID, record.ID, 3, allowMemoryWrite); !errors.Is(err, ErrMemoryConflict) {
		t.Fatalf("stale forget=%v", err)
	}
	if err := store.ForgetMemory(t.Context(), record.ProjectID, record.ID, 4, allowMemoryWrite); err != nil || len(query()) != 0 {
		t.Fatalf("forgotten memory remains retrievable: %v", err)
	}
	reopened, err := Open(store.root)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := reopened.Memories(t.Context(), record.ProjectID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("forget did not persist: %+v %v", rows, err)
	}
}

func TestMemorySearchEnforcesProjectPolicySourceSpanAndExpiry(t *testing.T) {
	store := memoryStore(t)
	record := acceptedMemory(t, store, memoryFixture())
	for _, test := range []struct {
		name, projectID, revision, policy, hash string
		start, end                              int
		now                                     time.Time
		count                                   int
		stale                                   bool
	}{
		{"current", "project", "revision", "policy", record.Sources[0].Hash, 2, 8, time.Now(), 1, false},
		{"other project", "foreign", "revision", "policy", record.Sources[0].Hash, 2, 8, time.Now(), 0, false},
		{"historical revision", "project", "new", "policy", record.Sources[0].Hash, 2, 8, time.Now(), 1, true},
		{"changed policy", "project", "revision", "new", record.Sources[0].Hash, 2, 8, time.Now(), 0, false},
		{"changed source", "project", "revision", "policy", "sha256:" + strings.Repeat("b", 64), 2, 8, time.Now(), 0, false},
		{"narrower declaration", "project", "revision", "policy", record.Sources[0].Hash, 2, 7, time.Now(), 0, false},
		{"whole file grant", "project", "revision", "policy", record.Sources[0].Hash, 0, 0, time.Now(), 1, false},
		{"expired", "project", "revision", "policy", record.Sources[0].Hash, 2, 8, record.ExpiresAt, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			scope := []MemorySource{{Path: "main.go", Hash: test.hash, StartLine: test.start, EndLine: test.end}}
			matches, err := store.SearchMemory(t.Context(), test.projectID, test.revision, test.policy, "cancellation", scope, test.now)
			if err != nil || len(matches) != test.count || len(matches) > 0 && matches[0].Stale != test.stale {
				t.Fatalf("scope leaked or lost provenance: %+v %v", matches, err)
			}
		})
	}
	for _, query := range []string{"unknown", ""} {
		matches, err := store.SearchMemory(t.Context(), "project", "revision", "policy", query, record.Sources, time.Now())
		if err != nil || len(matches) != 0 {
			t.Fatalf("irrelevant query matched: %+v %v", matches, err)
		}
	}
}

func TestMemoryWritesPreservePriorDataOnFailureAndCorruption(t *testing.T) {
	store := memoryStore(t)
	record := acceptedMemory(t, store, memoryFixture())
	location, err := store.path("memory.json")
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(location)
	if err != nil {
		t.Fatal(err)
	}
	original := store.write
	store.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("disk full") }
	if _, err := store.SaveMemory(t.Context(), record, record.Version, allowMemoryWrite); !errors.Is(err, ErrPersistence) {
		t.Fatalf("write failure=%v", err)
	}
	after, err := os.ReadFile(location)
	if err != nil || string(before) != string(after) {
		t.Fatalf("failed write damaged previous memory: %v", err)
	}
	store.write = original
	if err := os.WriteFile(location, []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Memories(t.Context(), "project"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("unknown memory version became empty: %v", err)
	}
	if _, err := store.SaveMemory(t.Context(), record, record.Version, allowMemoryWrite); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("corrupt memory was overwritten: %v", err)
	}
	after, err = os.ReadFile(location)
	if err != nil || string(after) != `{"version":99}` {
		t.Fatalf("historical corrupt bytes were erased: %q %v", after, err)
	}
}

func TestMemoryConcurrentEditsUseCompareAndSwap(t *testing.T) {
	store := memoryStore(t)
	record := acceptedMemory(t, store, memoryFixture())
	other, err := Open(store.root)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, writer := range []*Store{store, other} {
		workers.Go(func() {
			<-start
			_, err := writer.SaveMemory(t.Context(), record, record.Version, allowMemoryWrite)
			results <- err
		})
	}
	close(start)
	workers.Wait()
	close(results)
	written, conflicts := 0, 0
	for err := range results {
		if err == nil {
			written++
		} else if errors.Is(err, ErrMemoryConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if written != 1 || conflicts != 1 {
		t.Fatalf("concurrent edits lost updates: written=%d conflicts=%d", written, conflicts)
	}
}

func TestMemoryPrunesExpiredRecordsOnExplicitEditAndBoundsSearch(t *testing.T) {
	store := memoryStore(t)
	base := acceptedMemory(t, store, memoryFixture())
	catalog, err := store.loadMemory()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		copy := base
		copy.ID = fmt.Sprintf("record-%02d", i)
		catalog.Records[copy.ID] = copy
	}
	expired := base
	expired.ID, expired.CreatedAt, expired.UpdatedAt, expired.ExpiresAt = "expired", time.Now().Add(-48*time.Hour), time.Now().Add(-47*time.Hour), time.Now().Add(-24*time.Hour)
	catalog.Records[expired.ID] = expired
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	location, err := store.path("memory.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(location, data, 0600); err != nil {
		t.Fatal(err)
	}
	matches, err := store.SearchMemory(t.Context(), "project", "revision", "policy", "cancellation", base.Sources, time.Now())
	if err != nil || len(matches) != 8 {
		t.Fatalf("search was unbounded: %d %v", len(matches), err)
	}
	if _, err := store.SaveMemory(t.Context(), base, base.Version, allowMemoryWrite); err != nil {
		t.Fatal(err)
	}
	current, err := store.loadMemory()
	if err != nil || len(current.Records) != 11 {
		t.Fatalf("retention cleanup failed: %+v %v", current, err)
	}
	if _, retained := current.Records[expired.ID]; retained {
		t.Fatal("expired content was retained after curation")
	}
}

func TestMemoryRejectsInvalidScopeAndCanceledAuthority(t *testing.T) {
	for _, alter := range []func(*MemoryRecord){
		func(r *MemoryRecord) { r.Sources[0].Path = "../outside.go" },
		func(r *MemoryRecord) { r.Sources[0].Hash = "invented" },
		func(r *MemoryRecord) { r.Sources[0].StartLine = 0 },
		func(r *MemoryRecord) { r.Title = "unsafe\x1b]52" },
		func(r *MemoryRecord) { r.Text = strings.Repeat("x", MaxMemoryText+1) },
		func(r *MemoryRecord) { r.Kind = "verified_outcome" },
		func(r *MemoryRecord) { r.ExpiresAt = time.Now().Add(366 * 24 * time.Hour) },
	} {
		store, record := memoryStore(t), memoryFixture()
		alter(&record)
		if _, err := store.SaveMemory(t.Context(), record, 0, allowMemoryWrite); err == nil {
			t.Fatal("invalid memory accepted")
		}
		rows, err := store.Memories(t.Context(), "project")
		if err != nil || len(rows) != 0 {
			t.Fatalf("rejected memory persisted: %+v %v", rows, err)
		}
	}
	store := memoryStore(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.SaveMemory(ctx, memoryFixture(), 0, allowMemoryWrite); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	if _, err := store.SaveMemory(t.Context(), memoryFixture(), 0, nil); err == nil {
		t.Fatal("missing publication authority accepted")
	}
}
