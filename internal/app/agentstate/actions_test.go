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

func externalActionFixture() ExternalAction {
	return ExternalAction{ProjectID: "project", ProjectRevision: "revision", IntegrationID: "api", Inventory: strings.Repeat("a", 64), Scope: strings.Repeat("b", 64), OperationKey: strings.Repeat("c", 64), Request: json.RawMessage(`{"method":"POST","url":"https://api.example/items","body":{"id":7}}`), Arguments: json.RawMessage(`{"body":{"id":7}}`), ExpiresAt: time.Now().Add(time.Minute)}
}

func TestExternalActionsClaimOnceAndPreserveUncertainOutcomes(t *testing.T) {
	store, _ := testStore(t)
	action, err := store.CreateExternalAction(t.Context(), externalActionFixture(), func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Go(func() {
			_, err := store.ClaimExternalAction(t.Context(), action.ID, action.Version, action.RequestHash, time.Now().Add(time.Second), func() error { return nil })
			results <- err
		})
	}
	group.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrActionConflict) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("claims=%d", success)
	}
	reopened, err := Open(store.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.CreateExternalAction(t.Context(), externalActionFixture(), func() error { return nil }); !errors.Is(err, ErrActionUncertain) {
		t.Fatalf("uncertain operation replayed: %v", err)
	}
	if _, err := reopened.ReconcileExternalAction(t.Context(), action.ID, 2, action.RequestHash, "not_applied", "Manually checked external service", func() error { return nil }); !errors.Is(err, ErrActionConflict) {
		t.Fatalf("active lease reconciled: %v", err)
	}
	unknown, err := reopened.FinishExternalAction(t.Context(), action.ID, 2, "unknown", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.ClaimExternalAction(t.Context(), action.ID, unknown.Version, unknown.RequestHash, time.Now().Add(time.Second), func() error { return nil }); !errors.Is(err, ErrActionConflict) {
		t.Fatal("unknown action retried")
	}
	reconciled, err := reopened.ReconcileExternalAction(t.Context(), action.ID, unknown.Version, unknown.RequestHash, "not_applied", "Manually checked external service", func() error { return nil })
	if err != nil || reconciled.State != "reconciled" || reconciled.ReportedOutcome != "not_applied" {
		t.Fatalf("reconciliation: %+v %v", reconciled, err)
	}
	if _, err := reopened.CreateExternalAction(t.Context(), externalActionFixture(), func() error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestExternalActionsFailClosedOnPersistenceSourceAndCancellation(t *testing.T) {
	store, _ := testStore(t)
	denied := errors.New("source changed")
	if _, err := store.CreateExternalAction(t.Context(), externalActionFixture(), func() error { return denied }); !errors.Is(err, denied) {
		t.Fatal(err)
	}
	action, err := store.CreateExternalAction(t.Context(), externalActionFixture(), func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	original := store.write
	store.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("fixture disk failure") }
	if _, err := store.ClaimExternalAction(t.Context(), action.ID, 1, action.RequestHash, time.Now().Add(time.Second), func() error { return nil }); !errors.Is(err, ErrPersistence) {
		t.Fatalf("failed claim: %v", err)
	}
	store.write = original
	items, err := store.ExternalActions(t.Context(), "project")
	if err != nil || len(items) != 1 || items[0].State != "candidate" {
		t.Fatalf("uncommitted claim became visible: %+v %v", items, err)
	}
	items[0].Request[0] = 'x'
	claimed, err := store.ClaimExternalAction(t.Context(), action.ID, 1, action.RequestHash, time.Now().Add(time.Second), func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.FinishExternalAction(canceled, action.ID, claimed.Version, "completed", 200, json.RawMessage(`{}`)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	store.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("fixture disk failure") }
	if _, err := store.FinishExternalAction(t.Context(), action.ID, claimed.Version, "completed", 200, json.RawMessage(`{}`)); !errors.Is(err, ErrPersistence) {
		t.Fatal(err)
	}
	store.write = original
	items, err = store.ExternalActions(t.Context(), "project")
	if err != nil || items[0].State != "pending" {
		t.Fatalf("lost uncertain receipt: %+v %v", items, err)
	}
	finished, err := store.FinishExternalAction(t.Context(), action.ID, claimed.Version, "completed", 200, json.RawMessage(`{"ok":true}`))
	if err != nil || finished.ResponseHash == "" {
		t.Fatalf("receipt: %+v %v", finished, err)
	}
	if _, err := store.FinishExternalAction(t.Context(), action.ID, claimed.Version, "unknown", 0, nil); !errors.Is(err, ErrActionConflict) {
		t.Fatal("completed receipt overwritten")
	}
}
