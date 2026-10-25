package agentstate

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func liveRecordFixture() LiveRecord {
	now := time.Now().UTC()
	return LiveRecord{ID: "session-one", ControlRevision: 1, ProjectID: "project", ProjectRevision: "revision", Policy: "policy", Configuration: "configuration", Model: "gemini-3.8-live", ProviderOrigin: "https://example.com", State: "connecting", Connections: 1, CreatedAt: now, ExpiresAt: now.Add(time.Minute), RunIDs: []string{}, Artifacts: []ArtifactReference{}}
}

func TestLiveReceiptsCASIsolationPermissionsAndImmutableIdentity(t *testing.T) {
	store, _ := testStore(t)
	first, err := store.SaveLiveSession(t.Context(), liveRecordFixture(), 0)
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.path("live-sessions.json")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			copy := first
			copy.ControlRevision++
			copy.State = "connected"
			_, err := store.SaveLiveSession(t.Context(), copy, first.Version)
			results <- err
		})
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrLiveConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("CAS: %d/%d", success, conflicts)
	}
	reopened, err := Open(store.root)
	if err != nil {
		t.Fatal(err)
	}
	records, err := reopened.LiveSessions(t.Context(), "project")
	if err != nil || len(records) != 1 || records[0].State != "connected" {
		t.Fatalf("history: %+v %v", records, err)
	}
	other, err := reopened.LiveSessions(t.Context(), "another")
	if err != nil || len(other) != 0 {
		t.Fatal("cross-project receipts")
	}
	copy := records[0]
	copy.ExpiresAt = copy.ExpiresAt.Add(time.Second)
	if _, err := store.SaveLiveSession(t.Context(), copy, copy.Version); !errors.Is(err, ErrLiveConflict) {
		t.Fatal("lifetime replenished", err)
	}
	copy = records[0]
	copy.Model = "other"
	if _, err := store.SaveLiveSession(t.Context(), copy, copy.Version); !errors.Is(err, ErrLiveConflict) {
		t.Fatal("destination replaced", err)
	}
}

func TestLiveReceiptCorruptionCancellationAndWriteFailureRemainErrors(t *testing.T) {
	store, _ := testStore(t)
	record, err := store.SaveLiveSession(t.Context(), liveRecordFixture(), 0)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.LiveSessions(t.Context(), "project")
	if err != nil {
		t.Fatal(err)
	}
	write := store.write
	store.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("disk full") }
	record.State = "closed"
	record.ControlRevision++
	if _, err := store.SaveLiveSession(t.Context(), record, record.Version); !errors.Is(err, ErrPersistence) {
		t.Fatal(err)
	}
	store.write = write
	after, err := store.LiveSessions(t.Context(), "project")
	if err != nil || after[0].Version != before[0].Version || after[0].State != before[0].State {
		t.Fatal("failed write changed receipt")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.LiveSessions(ctx, "project"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := store.SaveLiveSession(ctx, record, record.Version); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	path, err := store.path("live-sessions.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{`{"version":99}`, `{"version":1,"version":1}`, strings.Repeat("x", 4<<20+1)} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.LiveSessions(t.Context(), "project"); !errors.Is(err, ErrCorrupt) {
			t.Fatal(err)
		}
	}
}
