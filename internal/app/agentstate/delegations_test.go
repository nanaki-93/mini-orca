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

func delegationDraft() Delegation {
	hash := strings.Repeat("a", 64)
	return Delegation{ProjectID: "project", ProjectRevision: "revision", RemoteID: "remote", EndpointHash: hash, CardFingerprint: hash, InputHash: hash, Skill: "review", Kind: "review", Question: "Review the selected source", Policy: "policy"}
}
func delegationGuard() error { return nil }

func TestDelegationDurableClaimsRejectReplayAndPinIdentity(t *testing.T) {
	store, _ := testStore(t)
	ctx := t.Context()
	first, err := store.CreateDelegation(ctx, delegationDraft(), time.Now().Add(time.Minute), delegationGuard)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateDelegation(ctx, delegationDraft(), time.Now().Add(time.Minute), delegationGuard); !errors.Is(err, ErrDelegationConflict) {
		t.Fatal("pending replay", err)
	}
	progress := first.Progress
	progress.TaskID, progress.ContextID, progress.State = "task", "context", "input_required"
	ready, err := store.SaveDelegationProgress(ctx, first.ID, first.AttemptID, progress, true)
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	claims := make(chan *Delegation, 2)
	for range 2 {
		wait.Go(func() {
			value, err := store.ClaimDelegation(ctx, ready.ID, ready.Version, "input", strings.Repeat("b", 64), time.Now().Add(time.Minute), delegationGuard)
			if err == nil {
				claims <- value
			} else if !errors.Is(err, ErrDelegationConflict) {
				t.Error(err)
			}
		})
	}
	wait.Wait()
	close(claims)
	if len(claims) != 1 {
		t.Fatal("nonexclusive dispatch", len(claims))
	}
	claimed := <-claims
	if claimed.MessageID == ready.MessageID || claimed.AttemptID == ready.AttemptID {
		t.Fatal("input identity reused")
	}
	wrong := progress
	wrong.TaskID = "wrong"
	if _, err := store.SaveDelegationProgress(ctx, claimed.ID, claimed.AttemptID, wrong, true); !errors.Is(err, ErrDelegationConflict) {
		t.Fatal(err)
	}
	progress.State = "unknown"
	unknown, err := store.SaveDelegationProgress(ctx, claimed.ID, claimed.AttemptID, progress, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimDelegation(ctx, unknown.ID, unknown.Version, "input", unknown.InputHash, time.Now().Add(time.Minute), delegationGuard); !errors.Is(err, ErrDelegationConflict) {
		t.Fatal("uncertain input replayed", err)
	}
	reconciled, err := store.ReconcileDelegation(ctx, unknown.ID, unknown.Version, "Operator verified remote cancellation", delegationGuard)
	if err != nil || reconciled.Progress.TaskID != "task" || reconciled.State != "reconciled" {
		t.Fatalf("recovery=%+v %v", reconciled, err)
	}
	if _, err := store.CreateDelegation(ctx, delegationDraft(), time.Now().Add(time.Minute), delegationGuard); err != nil {
		t.Fatal(err)
	}
}

func TestDelegationFailedPersistenceRetainsPendingReceipt(t *testing.T) {
	store, _ := testStore(t)
	receipt, err := store.CreateDelegation(t.Context(), delegationDraft(), time.Now().Add(time.Minute), delegationGuard)
	if err != nil {
		t.Fatal(err)
	}
	store.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("disk unavailable") }
	progress := receipt.Progress
	progress.State = "completed"
	if _, err := store.SaveDelegationProgress(context.Background(), receipt.ID, receipt.AttemptID, progress, true); !errors.Is(err, ErrPersistence) {
		t.Fatal(err)
	}
	records, err := store.Delegations(t.Context(), "project")
	if err != nil || len(records) != 1 || records[0].State != "pending" {
		t.Fatalf("lost receipt %+v %v", records, err)
	}
}
