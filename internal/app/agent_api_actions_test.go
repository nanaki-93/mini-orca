package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

func proposeAPIActionFixture(t *testing.T, service *Service, view *AgentAPI) AgentAPIAction {
	t.Helper()
	service.integrations.mu.Lock()
	record := service.integrations.apiRecords[view.Config.ID]
	service.integrations.mu.Unlock()
	arguments := json.RawMessage(`{"body":{"id":7}}`)
	action, err := view.Inventory.Operations[0].Prepare(view.Config.Endpoint, arguments)
	if err != nil {
		t.Fatal(err)
	}
	check := func(ctx context.Context) error { return service.validateAPIGrant(ctx, record) }
	id, err := service.proposeAPIAction(t.Context(), record, strings.Repeat("a", 64), action, arguments, check)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.ListAPIActions(t.Context(), record.query)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatal("candidate missing")
	return AgentAPIAction{}
}

func approveAPIControl(query AgentRunQuery, action AgentAPIAction) APIActionControl {
	return APIActionControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "approve", Version: action.Version, RequestHash: action.RequestHash, Confirm: true}
}

func TestAPIActionUnknownOutcomeBlocksReplayUntilHumanReconciliation(t *testing.T) {
	var sends atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		io.Copy(io.Discard, r.Body)
		c, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		c.Close()
	}))
	defer remote.Close()
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	query, view := installAPIFixture(t, service, remote.URL)
	action := proposeAPIActionFixture(t, service, view)
	unknown, err := service.ControlAPIAction(t.Context(), action.ID, approveAPIControl(query, action))
	if err != nil || unknown.State != "unknown" || !unknown.CanReconcile || sends.Load() != 1 {
		t.Fatalf("unknown receipt: %+v %v", unknown, err)
	}
	store, err := agentstate.Open(service.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.ExternalActions(t.Context(), query.ProjectID)
	if err != nil || items[0].State != "unknown" {
		t.Fatalf("lost durable outcome: %+v %v", items, err)
	}
	if _, err := store.CreateExternalAction(t.Context(), action.ExternalAction, func() error { return nil }); !errors.Is(err, agentstate.ErrActionUncertain) {
		t.Fatalf("uncertain operation admitted: %v", err)
	}
	control := approveAPIControl(query, *unknown)
	control.Action = "reconcile"
	control.ReportedOutcome = "not_applied"
	control.Observation = "Checked the external service manually; item was unchanged."
	reconciled, err := service.ControlAPIAction(t.Context(), action.ID, control)
	if err != nil || reconciled.State != "reconciled" || reconciled.ReportedOutcome != "not_applied" || sends.Load() != 1 {
		t.Fatalf("reconcile sent a request: %+v %v", reconciled, err)
	}
	if _, err := service.ControlAPIAction(t.Context(), action.ID, approveAPIControl(query, *reconciled)); !errors.Is(err, agentstate.ErrActionConflict) {
		t.Fatal("reconciled receipt became retry permission")
	}
}

func TestAPIActionRevocationCancelsExchangeAndRetainsReceipt(t *testing.T) {
	started, closed := make(chan struct{}), make(chan struct{})
	release := make(chan struct{})
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
			close(closed)
		case <-release:
		}
	}))
	defer remote.Close()
	defer close(release)
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	query, view := installAPIFixture(t, service, remote.URL)
	action := proposeAPIActionFixture(t, service, view)
	done := make(chan error, 1)
	go func() {
		result, err := service.ControlAPIAction(t.Context(), action.ID, approveAPIControl(query, action))
		if err == nil && result.State != "unknown" {
			err = errors.New("canceled write not retained as unknown")
		}
		done <- err
	}()
	<-started
	if _, err := service.ControlAPI(t.Context(), view.Config.ID, IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "revoke", Configuration: view.Configuration, Grant: view.Grant}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("owned exchange did not join")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("remote exchange survived revoke")
	}
	items, err := service.ListAPIActions(t.Context(), query)
	if err != nil || len(items) != 1 || items[0].State != "unknown" {
		t.Fatalf("receipt lost after revoke: %+v %v", items, err)
	}
}

func TestAPIActionSourceChangesAndRestartNeverReuseCandidates(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	query, view := installAPIFixture(t, service, "http://127.0.0.1:1")
	action := proposeAPIActionFixture(t, service, view)
	if err := os.WriteFile(filepath.Join(service.manager.Root(), "api.json"), []byte(applicationAPISpec+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ControlAPIAction(t.Context(), action.ID, approveAPIControl(query, action)); !errors.Is(err, ErrIntegrationConflict) {
		t.Fatalf("changed schema approved: %v", err)
	}
	registry, err := newIntegrationRegistry(service.integrations.config)
	if err != nil {
		t.Fatal(err)
	}
	service.integrations = registry
	items, err := service.ListAPIActions(t.Context(), query)
	if err != nil || len(items) != 1 || items[0].CanApprove {
		t.Fatalf("restart restored authority: %+v %v", items, err)
	}
}
