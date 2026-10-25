package agentstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func legacyMigrationFixture(t *testing.T) (*Store, Key, *database, string, []byte) {
	t.Helper()
	s, key, _ := inputFixture(t, false)
	for _, id := range []string{"finished", "abandoned"} {
		now := time.Now().UTC()
		run := Run{ID: id, Session: key, BudgetID: "budget", RequestHash: strings.Repeat("c", 64), SchemaHash: strings.Repeat("d", 64), SchemaVersion: "completion-v1", Status: "running", Started: now, Updated: now}
		if _, err := s.BeginRun(t.Context(), run, nil); err != nil {
			t.Fatal(err)
		}
		if id == "finished" {
			if err := s.FinishRun(t.Context(), id, "completed", nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := s.AppendEvent(t.Context(), key, Event{ID: "private", Data: json.RawMessage(`{"source":"PRIVATE_SOURCE"}`)}, State{"provenance": json.RawMessage(`"ORIGINAL_PROVENANCE"`)}); err != nil {
		t.Fatal(err)
	}
	db, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	db.Version, db.RuntimeFamily = 1, ""
	data, err := json.Marshal(db)
	if err != nil {
		t.Fatal(err)
	}
	path, err := s.path("journal.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return s, key, db, path, data
}

func TestJournalMigrationPreservesReadOnlyHistoryAndIndependentCheckpoints(t *testing.T) {
	s, key, before, path, original := legacyMigrationFixture(t)
	reopened, err := Open(s.root)
	if err != nil {
		t.Fatal(err)
	}
	history, err := reopened.History(t.Context())
	if err != nil || len(history.Runs) != 3 {
		t.Fatal(history, err)
	}
	read, err := os.ReadFile(path)
	if err != nil || string(read) != string(original) {
		t.Fatal("passive legacy read rewrote data", err)
	}
	if _, err := reopened.CreateSession(t.Context(), Key{App: key.App, User: key.User, ID: "new-admitted-session"}, State{}); err != nil {
		t.Fatal(err)
	}
	after, err := reopened.load()
	if err != nil {
		t.Fatal(err)
	}
	if after.Version != journalVersion || after.RuntimeFamily != journalRuntimeFamily || after.Runs["abandoned"].Status != "interrupted" {
		t.Fatal("migration did not fence legacy execution")
	}
	for _, id := range []string{"finished", "run"} {
		if !reflect.DeepEqual(before.Runs[id], after.Runs[id]) {
			t.Fatal("completed or waiting checkpoint changed", id)
		}
	}
	if !reflect.DeepEqual(before.Budgets, after.Budgets) || !reflect.DeepEqual(before.Traces, after.Traces) || !reflect.DeepEqual(before.Sessions[key.key()], after.Sessions[key.key()]) {
		t.Fatal("migration erased usage, traces, private content or provenance")
	}
	if after.ProgressSequence != before.ProgressSequence+1 || after.Progress[len(after.Progress)-1].State != "interrupted" {
		t.Fatal("interruption event missing")
	}
	if err := reopened.FinishRun(t.Context(), "abandoned", "completed", nil); err == nil {
		t.Fatal("interrupted SDK invocation resumed")
	}
	if _, err := reopened.AppendEvent(t.Context(), key, Event{ID: "later", Data: json.RawMessage(`{}`)}, nil); err != nil {
		t.Fatal(err)
	}
	latest, err := reopened.load()
	if err != nil || latest.ProgressSequence != after.ProgressSequence {
		t.Fatal("migration repeated", err)
	}
	checkpoint := before.Runs["run"]
	admission := InputAdmission{RunID: checkpoint.ID, RequestID: checkpoint.Input.RequestID, RequestHash: checkpoint.RequestHash, Binding: before.Budgets[checkpoint.BudgetID].Binding}
	if _, err := reopened.InputRun(t.Context(), admission); err != nil {
		t.Fatal("valid host checkpoint lost", err)
	}
}

func TestJournalMigrationFailureCancellationAndUnknownCommitOutcome(t *testing.T) {
	for _, kind := range []string{"write-failure", "canceled-before-commit", "committed-but-acknowledgment-failed"} {
		t.Run(kind, func(t *testing.T) {
			s, _, before, path, original := legacyMigrationFixture(t)
			write := s.write
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			s.write = func(path string, data []byte, mode os.FileMode, authorize func() error) error {
				switch kind {
				case "write-failure":
					return errors.New("disk unavailable")
				case "canceled-before-commit":
					cancel()
					return write(path, data, mode, authorize)
				default:
					if err := write(path, data, mode, authorize); err != nil {
						return err
					}
					return errors.New("commit acknowledgment unavailable")
				}
			}
			err := s.mutate(ctx, func(*database) error { return nil })
			if kind == "canceled-before-commit" {
				if !errors.Is(err, context.Canceled) {
					t.Fatal("cancellation was lost", err)
				}
			} else if !errors.Is(err, ErrPersistence) {
				t.Fatal("failed persistence became success", err)
			}
			read, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if kind != "committed-but-acknowledgment-failed" && string(read) != string(original) {
				t.Fatal("failed migration partially replaced history")
			}
			reopened, err := Open(s.root)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.mutate(t.Context(), func(*database) error { return nil }); err != nil {
				t.Fatal(err)
			}
			after, err := reopened.load()
			if err != nil || after.Version != journalVersion || after.ProgressSequence != before.ProgressSequence+1 || after.Runs["abandoned"].Status != "interrupted" {
				t.Fatal("recovery duplicated migration or lost interruption", err)
			}
		})
	}
}

func TestJournalMigrationRejectsFutureDowngradedAndCorruptHistory(t *testing.T) {
	for _, kind := range []string{"future-version", "unknown-runtime", "downgrade", "missing-runtime", "corrupt-run", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			s, _, db, path, _ := legacyMigrationFixture(t)
			switch kind {
			case "future-version":
				db.Version = journalVersion + 1
			case "unknown-runtime":
				db.Version, db.RuntimeFamily = journalVersion, "future-runtime"
			case "downgrade":
				db.Version, db.RuntimeFamily = 1, journalRuntimeFamily
			case "missing-runtime":
				db.Version = journalVersion
			case "corrupt-run":
				run := db.Runs["abandoned"]
				run.SchemaVersion = "future-graph"
				db.Runs[run.ID] = run
			}
			data, err := json.Marshal(db)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "duplicate" {
				data = append([]byte(`{"version":1,`), data[1:]...)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(s.root); !errors.Is(err, ErrCorrupt) {
				t.Fatal("incompatible history admitted", err)
			}
			if err := s.mutate(t.Context(), func(*database) error { t.Fatal("corrupt state reached mutation"); return nil }); !errors.Is(err, ErrCorrupt) {
				t.Fatal(err)
			}
			preserved, err := os.ReadFile(path)
			if err != nil || string(preserved) != string(data) {
				t.Fatal("corrupt history overwritten", err)
			}
		})
	}
}

func TestJournalMigrationConcurrentAdmissionsCommitOneInterruption(t *testing.T) {
	s, key, before, _, _ := legacyMigrationFixture(t)
	var group sync.WaitGroup
	errorsFound := make(chan error, 2)
	for _, id := range []string{"first-admission", "second-admission"} {
		group.Go(func() {
			store, err := Open(s.root)
			if err == nil {
				_, err = store.CreateSession(t.Context(), Key{App: key.App, User: key.User, ID: id}, State{})
			}
			errorsFound <- err
		})
	}
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	after, err := s.load()
	if err != nil || after.ProgressSequence != before.ProgressSequence+1 || len(after.Sessions) != len(before.Sessions)+2 {
		t.Fatal("concurrent migration lost work or duplicated interruption", err)
	}
}

func TestJournalMigrationFullJournalRetainsBoundedContiguousEvents(t *testing.T) {
	s, _, db, path, _ := legacyMigrationFixture(t)
	for i := len(db.Runs); i < 8192; i++ {
		run := db.Runs["abandoned"]
		run.ID = fmt.Sprintf("orphan-%04d", i)
		db.Runs[run.ID] = run
	}
	before := db.ProgressSequence
	data, err := json.Marshal(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.mutate(t.Context(), func(*database) error { return nil }); err != nil {
		t.Fatal("full journal could not migrate within its bounded transaction", err)
	}
	after, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Runs) != 8192 || len(after.Progress) != MaxProgressEvents || after.ProgressSequence != before+8190 {
		t.Fatal("migration lost records or sequence")
	}
	for i, event := range after.Progress {
		if event.State != "interrupted" || event.Sequence != after.ProgressSequence-uint64(MaxProgressEvents-1-i) {
			t.Fatal("noncontiguous migration window")
		}
	}
}
