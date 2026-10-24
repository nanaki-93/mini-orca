package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

func TestDelegationConsentInputRecoveryAndPassiveHistory(t *testing.T) {
	f := newDelegationFixture(t)
	s, query, request := delegationService(t, f)
	request.ConfirmDelegate = false
	if _, err := s.StartDelegation(t.Context(), request); err == nil || f.calls.Load() != 0 {
		t.Fatal("missing consent dispatched", err)
	}
	request.ConfirmDelegate = true
	ready, err := s.StartDelegation(t.Context(), request)
	if err != nil || ready.State != "input_required" || ready.Active || ready.Progress.Text != "Private advisory" {
		t.Fatalf("start=%+v %v", ready, err)
	}
	if !strings.Contains(f.payload.Load().(string), "main.go") || strings.Contains(f.payload.Load().(string), "mini_orca.remote_delegation") {
		t.Fatal("wrong captured prompt", f.payload.Load())
	}
	cards, calls := f.cards.Load(), f.calls.Load()
	list, err := s.ListDelegations(t.Context(), query)
	if err != nil || len(list) != 1 || list[0].Progress.Text != "" || list[0].Question != "" {
		t.Fatalf("private list=%+v %v", list, err)
	}
	detail, err := s.DelegationDetail(t.Context(), query, ready.ID)
	if err != nil || detail.Progress.Text == "" || f.cards.Load() != cards || f.calls.Load() != calls {
		t.Fatal("history dispatched", err)
	}
	control := DelegationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Version: ready.Version, Action: "input", Remote: request.Remote, Reply: "Review complexity", Confirm: true}
	f.status.Store("TASK_STATE_COMPLETED")
	completed, err := s.ControlDelegation(t.Context(), ready.ID, control)
	if err != nil || completed.State != "completed" || completed.MessageID == ready.MessageID || f.calls.Load() != calls+1 {
		t.Fatalf("input=%+v %v", completed, err)
	}
	if _, err := s.ControlDelegation(t.Context(), ready.ID, control); !errors.Is(err, agentstate.ErrDelegationConflict) || f.calls.Load() != calls+1 {
		t.Fatal("input replay", err)
	}
}

func TestDelegationStaleSourceBlocksInputButAllowsCancel(t *testing.T) {
	f := newDelegationFixture(t)
	s, query, request := delegationService(t, f)
	ready, err := s.StartDelegation(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.manager.Root(), "main.go"), []byte("package main\n// changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	control := DelegationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Version: ready.Version, Action: "input", Remote: request.Remote, Reply: "Continue", Confirm: true}
	if _, err := s.ControlDelegation(t.Context(), ready.ID, control); err == nil || f.calls.Load() != 1 {
		t.Fatal("stale input sent", err)
	}
	control.Action, control.Reply = "cancel", ""
	canceled, err := s.ControlDelegation(t.Context(), ready.ID, control)
	if err != nil || canceled.State != "canceled" || !canceled.Stale || f.method.Load() != "CancelTask" {
		t.Fatalf("cancel=%+v %v", canceled, err)
	}
}

func TestDelegationUnknownBlocksRestartReplayAndReconcilesLocally(t *testing.T) {
	f := newDelegationFixture(t)
	s, query, request := delegationService(t, f)
	f.fail.Store(true)
	unknown, err := s.StartDelegation(t.Context(), request)
	if err != nil || unknown.State != "unknown" {
		t.Fatalf("unknown=%+v %v", unknown, err)
	}
	if _, err := s.StartDelegation(t.Context(), request); !errors.Is(err, agentstate.ErrDelegationConflict) || f.calls.Load() != 1 {
		t.Fatal("uncertain replay", err)
	}
	// Forget all process-local grants, as after restart; durable records remain.
	s.cancelIntegrations()
	cards := f.cards.Load()
	records, err := s.ListDelegations(t.Context(), query)
	if err != nil || records[0].State != "unknown" || f.cards.Load() != cards {
		t.Fatal("restart resumed", err)
	}
	control := DelegationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Version: unknown.Version, Action: "reconcile", Observation: "Operator verified the remote task never started", Confirm: true}
	reconciled, err := s.ControlDelegation(t.Context(), unknown.ID, control)
	if err != nil || reconciled.State != "reconciled" || f.calls.Load() != 1 || f.cards.Load() != cards {
		t.Fatalf("reconciliation=%+v %v", reconciled, err)
	}
}

func TestDelegationReconnectSubscribesWithoutResend(t *testing.T) {
	f := newDelegationFixture(t)
	f.status.Store("TASK_STATE_WORKING")
	s, query, request := delegationService(t, f)
	ready, err := s.StartDelegation(t.Context(), request)
	if err != nil || ready.State != "working" {
		t.Fatalf("async=%+v %v", ready, err)
	}
	f.status.Store("TASK_STATE_COMPLETED")
	control := DelegationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Version: ready.Version, Action: "reconnect", Remote: request.Remote, Confirm: true}
	complete, err := s.ControlDelegation(t.Context(), ready.ID, control)
	if err != nil || complete.State != "completed" || f.method.Load() != "SubscribeToTask" || f.calls.Load() != 2 {
		t.Fatalf("reconnect=%+v %v", complete, err)
	}
}
