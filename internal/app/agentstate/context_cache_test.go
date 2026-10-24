package agentstate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func contextFixture(kind string) ContextKey {
	return ContextKey{Binding: Binding{Project: "project", Revision: "revision", Policy: "policy", Configuration: "configuration"}, Kind: kind, Fingerprint: strings.Repeat("a", 64)}
}

func writeContextFixture(t *testing.T, path string, catalog *contextCatalog) {
	t.Helper()
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestContextClaimsSurviveRestartAndPublishExactlyOnce(t *testing.T) {
	store := memoryStore(t)
	key := contextFixture("summary")
	if _, err := store.Context(t.Context(), key); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	path, _ := store.path("context-cache.json")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("passive read created a cache")
	}
	claimed, owner, err := store.ClaimContext(t.Context(), key, time.Time{}, allowMemoryWrite)
	if err != nil || !owner {
		t.Fatalf("claim=%+v owner=%v error=%v", claimed, owner, err)
	}
	reopened, err := Open(store.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := reopened.ClaimContext(t.Context(), key, time.Time{}, allowMemoryWrite); !errors.Is(err, ErrContextPending) {
		t.Fatalf("pending operation repeated: %v", err)
	}
	if err := reopened.SaveContext(t.Context(), key, "other", []byte(`{"summary":"advice"}`), time.Time{}, allowMemoryWrite); !errors.Is(err, ErrContextPending) {
		t.Fatal(err)
	}
	if err := store.SaveContext(t.Context(), key, claimed.Claim, []byte(`{"summary":"advice"}`), time.Time{}, allowMemoryWrite); err != nil {
		t.Fatal(err)
	}
	reused, owner, err := reopened.ClaimContext(t.Context(), key, time.Time{}, allowMemoryWrite)
	if err != nil || owner || string(reused.Value) != `{"summary":"advice"}` {
		t.Fatalf("reuse=%+v owner=%v %v", reused, owner, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private cache permissions: %v %v", info, err)
	}
	for _, mutate := range []func(*ContextKey){
		func(k *ContextKey) { k.Binding.Project = "other" }, func(k *ContextKey) { k.Binding.Revision = "new" },
		func(k *ContextKey) { k.Binding.Policy = "new" }, func(k *ContextKey) { k.Binding.Configuration = "new" },
		func(k *ContextKey) { k.Fingerprint = strings.Repeat("b", 64) },
	} {
		changed := key
		mutate(&changed)
		if _, err := store.Context(t.Context(), changed); !errors.Is(err, ErrNotFound) {
			t.Fatal("identity crossed", err)
		}
	}
}

func TestContextConcurrentClaimsHaveOneOwner(t *testing.T) {
	store := memoryStore(t)
	var wg sync.WaitGroup
	owners := make(chan bool, 8)
	for range 8 {
		wg.Go(func() {
			_, owner, err := store.ClaimContext(t.Context(), contextFixture("summary"), time.Time{}, allowMemoryWrite)
			if err != nil && !errors.Is(err, ErrContextPending) {
				t.Error(err)
			}
			owners <- owner
		})
	}
	wg.Wait()
	close(owners)
	count := 0
	for owner := range owners {
		if owner {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("paid operation admitted %d times", count)
	}
}

func TestContextFailuresPreserveExistingData(t *testing.T) {
	store := memoryStore(t)
	key := contextFixture("composition")
	expires := time.Now().Add(time.Hour)
	if err := store.SaveContext(t.Context(), key, "", []byte(`{"text":"original"}`), expires, allowMemoryWrite); err != nil {
		t.Fatal(err)
	}
	path, _ := store.path("context-cache.json")
	before, _ := os.ReadFile(path)
	oversized, err := json.Marshal(strings.Repeat("x", maxContextValue))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range [][]byte{[]byte(`null null`), []byte(`{"x":1,"x":2}`), oversized} {
		if err := store.SaveContext(t.Context(), key, "", value, expires, allowMemoryWrite); err == nil {
			t.Fatal("invalid cache value accepted")
		}
	}
	denied := errors.New("source changed")
	if err := store.SaveContext(t.Context(), key, "", []byte(`{}`), expires, func() error { return denied }); !errors.Is(err, denied) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.Context(ctx, key); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	store.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("disk full") }
	if err := store.SaveContext(t.Context(), key, "", []byte(`{}`), expires, allowMemoryWrite); !errors.Is(err, ErrPersistence) {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("failed update replaced prior data")
	}
	if err := os.WriteFile(path, []byte(`{"version":999}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Context(t.Context(), key); !errors.Is(err, ErrCorrupt) {
		t.Fatal(err)
	}
}

func TestContextExpiryNeverReplaysAnUncertainPaidCall(t *testing.T) {
	store := memoryStore(t)
	key := contextFixture("provider")
	claimed, _, err := store.ClaimContext(t.Context(), key, time.Now().Add(time.Hour), allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveContext(t.Context(), key, claimed.Claim, []byte(`{}`), claimed.ExpiresAt.Add(time.Hour), allowMemoryWrite); err == nil {
		t.Fatal("retention extension accepted")
	}
	// Advance only the stored timestamps, without sleeping or changing global time.
	path, _ := store.path("context-cache.json")
	catalog, err := store.loadContext()
	if err != nil {
		t.Fatal(err)
	}
	id, _ := key.identity()
	claimed.CreatedAt, claimed.ExpiresAt = time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour)
	catalog.Records[id] = claimed
	writeContextFixture(t, path, catalog)
	if _, _, err := store.ClaimContext(t.Context(), key, time.Now().Add(time.Hour), allowMemoryWrite); !errors.Is(err, ErrContextPending) {
		t.Fatal("expired uncertain call retried", err)
	}
	claimed.State, claimed.Value = "ready", []byte(`{}`)
	catalog.Records[id] = claimed
	writeContextFixture(t, path, catalog)
	next, owner, err := store.ClaimContext(t.Context(), key, time.Now().Add(time.Hour), allowMemoryWrite)
	if err != nil || !owner || next.Claim == claimed.Claim {
		t.Fatalf("completed expired cache did not renew: %+v %v", next, err)
	}
}
