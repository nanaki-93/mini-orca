package agentstate

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func triggerFixture() TriggerRecord {
	now := time.Now().UTC()
	return TriggerRecord{ProjectID: "project", ControlRevision: 1, Options: TriggerOptions{Mode: "changes", Paths: []string{"main.go"}}, State: "watching", Observed: map[string]string{"main.go": strings.Repeat("a", 64)}, Handled: map[string]string{"main.go": strings.Repeat("a", 64)}, Grant: &TriggerGrant{ID: "grant-one", ProjectID: "project", Configuration: strings.Repeat("b", 64), SettingsVersion: 1, Policy: "3:" + strings.Repeat("c", 64), ScopeHash: strings.Repeat("d", 64), Model: "test-model", ProviderOrigin: "https://provider.example", CreatedAt: now, ExpiresAt: now.Add(time.Hour), RunLimits: config.AgentLimits{ModelCalls: 4, ToolCalls: 1, InputTokens: 1024, OutputTokens: 1024, ElapsedSeconds: 60, ConcurrentCalls: 1}}, Window: TriggerWindow{StartedAt: now, EndsAt: now.Add(time.Hour), Limits: TriggerAllowance{Runs: 4, ModelCalls: 16, InputTokens: 4096, OutputTokens: 4096, Seconds: 240}, Charged: TriggerAllowance{Runs: 1, ModelCalls: 4, InputTokens: 1024, OutputTokens: 1024, Seconds: 60}}}
}

func TestTriggersCASIsolationPermissionsAndOpenWindowNeverReplenishes(t *testing.T) {
	store, _ := testStore(t)
	empty, err := store.Triggers(t.Context(), "project")
	if err != nil || empty.Version != 0 || empty.Options.Mode != "disabled" || empty.Grant != nil {
		t.Fatal("new project was not disabled", err)
	}
	first, err := store.SaveTriggers(t.Context(), triggerFixture(), 0)
	if err != nil {
		t.Fatal(err)
	}
	location, _ := store.path("triggers.json")
	info, err := os.Stat(location)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("ambient state not private", err)
	}
	for _, mutation := range []func(*TriggerRecord){
		func(r *TriggerRecord) { r.Window.Charged = TriggerAllowance{} },
		func(r *TriggerRecord) { r.Window.Limits.Runs++ },
		func(r *TriggerRecord) { r.Window.StartedAt = r.Window.StartedAt.Add(time.Second) },
		func(r *TriggerRecord) { r.Window.EndsAt = r.Window.EndsAt.Add(time.Second) },
		func(r *TriggerRecord) { r.Grant.ID = "new-without-control" },
	} {
		next := cloneTriggers(first)
		mutation(&next)
		if _, err := store.SaveTriggers(t.Context(), next, first.Version); !errors.Is(err, ErrTriggerConflict) {
			t.Fatal("open window or control identity was replaced", err)
		}
	}
	var wg sync.WaitGroup
	errorsSeen := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			next := cloneTriggers(first)
			next.Grant = nil
			next.ControlRevision++
			_, err := store.SaveTriggers(t.Context(), next, first.Version)
			errorsSeen <- err
		})
	}
	wg.Wait()
	close(errorsSeen)
	succeeded, conflicts := 0, 0
	for err := range errorsSeen {
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrTriggerConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if succeeded != 1 || conflicts != 1 {
		t.Fatal("concurrent controls both won", succeeded, conflicts)
	}
	reopened, err := Open(store.root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Triggers(t.Context(), "project")
	if err != nil || loaded.Grant != nil || loaded.Window.Charged != first.Window.Charged {
		t.Fatal("restart/revocation replenished charges", err)
	}
	if _, err := reopened.Triggers(t.Context(), "other-project"); !errors.Is(err, ErrTriggerConflict) {
		t.Fatal("cross-project state disclosed", err)
	}
}

func TestTriggersRotateOnlyExpiredWindowsAndRetainPriorAttempt(t *testing.T) {
	store, _ := testStore(t)
	record := triggerFixture()
	record.Window.StartedAt, record.Window.EndsAt = time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour)
	record.LastAttempt = &TriggerAttempt{ID: "attempt", ProjectRevision: "revision", Path: "main.go", SourceHash: strings.Repeat("a", 64), State: "interrupted", StartedAt: time.Now().Add(-2 * time.Hour), FinishedAt: time.Now().Add(-time.Hour), Reserved: record.Window.Charged}
	first, err := store.SaveTriggers(t.Context(), record, 0)
	if err != nil {
		t.Fatal(err)
	}
	first.Window.StartedAt, first.Window.EndsAt, first.Window.Charged = time.Now(), time.Now().Add(time.Hour), TriggerAllowance{}
	first.Window.Limits.InputTokens = 512
	next, err := store.SaveTriggers(t.Context(), first, first.Version)
	if err != nil || next.Window.Charged.Runs != 0 || next.LastAttempt.State != "interrupted" {
		t.Fatal("expired window did not rotate with historical receipt intact", err)
	}
}

func TestTriggersMalformedPathsAndOverBudgetReceiptsAreRejected(t *testing.T) {
	for _, options := range []TriggerOptions{
		{Mode: "changes", Paths: []string{"../secret"}}, {Mode: "changes", Paths: []string{"a.go", "a.go"}},
		{Mode: "changes", Paths: []string{"a\tb.go"}}, {Mode: "changes", Paths: []string{"/a.go"}},
		{Mode: "interval", IntervalSeconds: 1, Paths: []string{"a.go"}}, {Mode: "disabled", Paths: []string{"a.go"}},
	} {
		if options.Validate() == nil {
			t.Fatal("unsafe watch accepted", options)
		}
	}
	store, _ := testStore(t)
	for _, change := range []func(*TriggerRecord){
		func(r *TriggerRecord) { r.Observed["secret"] = strings.Repeat("a", 64) },
		func(r *TriggerRecord) { r.Window.Charged.InputTokens = r.Window.Limits.InputTokens + 1 },
		func(r *TriggerRecord) { r.Grant.ExpiresAt = r.Grant.CreatedAt.Add(25 * time.Hour) },
		func(r *TriggerRecord) { r.Grant.RunLimits.ConcurrentCalls = 2 },
	} {
		record := triggerFixture()
		change(&record)
		if _, err := store.SaveTriggers(t.Context(), record, 0); !errors.Is(err, ErrCorrupt) {
			t.Fatal("invalid receipt accepted", err)
		}
	}
}

func TestTriggersCorruptionCancellationAndWriteFailureStayVisible(t *testing.T) {
	store, _ := testStore(t)
	record, err := store.SaveTriggers(t.Context(), triggerFixture(), 0)
	if err != nil {
		t.Fatal(err)
	}
	write := store.write
	store.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("disk full") }
	record.State = "failed"
	if _, err := store.SaveTriggers(t.Context(), record, record.Version); !errors.Is(err, ErrPersistence) {
		t.Fatal("write failure lost", err)
	}
	store.write = write
	loaded, err := store.Triggers(t.Context(), "project")
	if err != nil || loaded.State != "watching" || loaded.Version != record.Version {
		t.Fatal("failed write changed durable state", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.Triggers(ctx, "project"); !errors.Is(err, context.Canceled) {
		t.Fatal("read ignored cancellation", err)
	}
	if _, err := store.SaveTriggers(ctx, record, record.Version); !errors.Is(err, context.Canceled) {
		t.Fatal("write ignored cancellation", err)
	}
	location, _ := store.path("triggers.json")
	if err := os.WriteFile(location, []byte(`{"schema":1,"unexpected":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Triggers(t.Context(), "project"); !errors.Is(err, ErrCorrupt) {
		t.Fatal("corrupt state became disabled", err)
	}
}
