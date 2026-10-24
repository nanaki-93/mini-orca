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

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

const serviceFixtureToken = "0123456789abcdef0123456789abcdef"

func remoteServiceFixture(t *testing.T, endpoint string) (*Service, AgentRunQuery, RemoteServiceRequest, RemoteServiceControl) {
	t.Helper()
	t.Setenv("MINI_ORCA_AGENT_FIXTURE_TOKEN", serviceFixtureToken)
	s, _ := newSemanticAnalysisService(t, endpoint, 0)
	base := planInvestigationRequest(t, s)
	query := AgentRunQuery{ProjectID: base.ProjectID, ProjectRevision: base.ProjectRevision}
	registry, err := newIntegrationRegistry(config.IntegrationsConfig{A2AServer: config.AgentServer{Enabled: true, Principals: []config.AgentPrincipal{{ID: "reviewer", BearerTokenEnv: "MINI_ORCA_AGENT_FIXTURE_TOKEN", ProjectID: query.ProjectID, Capabilities: []string{"analysis", "proposal"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	s.integrations = registry
	listener, err := s.NewRemoteService("http://127.0.0.1:9091/a2a")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	views, err := s.ListRemoteServicePrincipals(t.Context(), query)
	if err != nil || len(views) != 1 || views[0].State != "configured" {
		t.Fatalf("passive views %+v %v", views, err)
	}
	request := RemoteServiceRequest{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Operation: "analysis", Question: "Improve Run", Sources: []ExternalSource{{Path: "main.go", Hash: base.Sources[0].Hash}}, AcceptanceCriteria: base.AcceptanceCriteria}
	control := RemoteServiceControl{IntegrationControl: IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Configuration: views[0].Configuration, Action: "grant", ConfirmConnect: true}, ConfirmSourceSharing: true}
	return s, query, request, control
}

func grantRemoteServiceFixture(t *testing.T, s *Service, control RemoteServiceControl) *RemoteServicePrincipal {
	t.Helper()
	view, err := s.ControlRemoteService(t.Context(), "reviewer", control)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestRemoteServiceAdmissionRejectsMissingConsentAndStaleScope(t *testing.T) {
	var calls atomic.Int32
	provider := changeProvider(t, func() string { calls.Add(1); return "unused" })
	s, query, request, control := remoteServiceFixture(t, provider.URL)
	if _, err := s.authenticateRemoteService(t.Context(), serviceFixtureToken); err == nil {
		t.Fatal("configuration granted access")
	}
	denied := control
	denied.ConfirmSourceSharing = false
	if _, err := s.ControlRemoteService(t.Context(), "reviewer", denied); err == nil {
		t.Fatal("source consent omitted")
	}
	s.runtimes.function.effective.RemoteProvider = true
	views, _ := s.ListRemoteServicePrincipals(t.Context(), query)
	control.Configuration = views[0].Configuration
	if _, err := s.ControlRemoteService(t.Context(), "reviewer", control); err == nil {
		t.Fatal("provider consent omitted")
	}
	control.ConfirmRemoteProvider = true
	view := grantRemoteServiceFixture(t, s, control)
	admission, err := s.authenticateRemoteService(t.Context(), serviceFixtureToken)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"provider", "project", "revision", "hash", "capability", "scope", "unknown"} {
		t.Run(name, func(t *testing.T) {
			input := request
			input.ConfirmRemoteProvider = true
			switch name {
			case "provider":
				input.ConfirmRemoteProvider = false
			case "project":
				input.ProjectID = "another-project"
			case "revision":
				input.ProjectRevision = "stale"
			case "hash":
				input.Sources = []ExternalSource{{Path: "main.go", Hash: "stale"}}
			case "capability":
				input.Operation = "apply"
			case "scope":
				input.Sources = make([]ExternalSource, 9)
			}
			data, _ := json.Marshal(input)
			if name == "unknown" {
				data = []byte(strings.TrimSuffix(string(data), "}") + `,"changes":[]}`)
			}
			if _, err := admission.Execute(t.Context(), string(data)); err == nil || calls.Load() != 0 {
				t.Fatal("invalid request dispatched", err)
			}
		})
	}
	view.Config.Capabilities[0] = "apply"
	*view.ExpiresAt = time.Now().Add(-time.Hour)
	if err := admission.Validate(t.Context()); err != nil {
		t.Fatal("returned view mutated stored grant", err)
	}
	t.Setenv("MINI_ORCA_AGENT_FIXTURE_TOKEN", strings.Repeat("r", 32))
	if _, err := s.authenticateRemoteService(t.Context(), strings.Repeat("r", 32)); err == nil {
		t.Fatal("credential rotation reused grant")
	}
	if calls.Load() != 0 {
		t.Fatal("admission made provider calls")
	}
}

func TestRemoteServiceAnalysisAndProposalPreserveLocalReview(t *testing.T) {
	var reply atomic.Value
	var calls atomic.Int32
	reply.Store("")
	provider := changeProvider(t, func() string { calls.Add(1); return reply.Load().(string) })
	s, _, request, control := remoteServiceFixture(t, provider.URL)
	grantRemoteServiceFixture(t, s, control)
	admission, err := s.authenticateRemoteService(t.Context(), serviceFixtureToken)
	if err != nil {
		t.Fatal(err)
	}
	root := s.manager.Root()
	before, _ := os.ReadFile(filepath.Join(root, "main.go"))
	analysis, _ := json.Marshal(planInvestigationFixture(request.Sources[0].Hash))
	reply.Store(string(analysis))
	data, _ := json.Marshal(request)
	result, err := admission.Execute(t.Context(), string(data))
	if err != nil || !strings.Contains(result, `"advisory":true`) || !strings.Contains(result, `"evidence"`) {
		t.Fatalf("analysis %s %v", result, err)
	}
	history, err := s.ChangeHistory(t.Context())
	if err != nil || len(history) != 0 {
		t.Fatal("analysis prepared a proposal", history, err)
	}
	proposal, _ := json.Marshal(map[string]any{"explanation": "Document behavior", "changes": []map[string]string{{"path": "main.go", "content": string(before) + "\n// Reviewed locally before Apply.\n"}}})
	reply.Store(string(proposal))
	request.Operation = "proposal"
	data, _ = json.Marshal(request)
	result, err = admission.Execute(t.Context(), string(data))
	if err != nil {
		t.Fatal("proposal", result, err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "main.go"))
	history, err = s.ChangeHistory(t.Context())
	if err != nil || len(history) != 1 || !strings.Contains(result, `"proposal_id"`) || !strings.Contains(result, "before Apply") || string(before) != string(after) || len(s.changeAuthority) != 0 || calls.Load() != 2 {
		t.Fatalf("proposal bypassed review: %s history=%+v calls=%d err=%v", result, history, calls.Load(), err)
	}
}

func TestRemoteServiceRevocationJoinsProviderAndPreventsPublication(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer provider.Close()
	s, _, request, control := remoteServiceFixture(t, provider.URL)
	view := grantRemoteServiceFixture(t, s, control)
	admission, err := s.authenticateRemoteService(t.Context(), serviceFixtureToken)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(request)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := admission.Execute(ctx, string(data)); result <- err }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("provider did not start")
	}
	control.Action, control.Grant = "revoke", view.Grant
	if _, err := s.ControlRemoteService(ctx, "reviewer", control); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("revoked result published")
		}
	case <-ctx.Done():
		t.Fatal("revoked operation not joined")
	}
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("provider not canceled")
	}
	if err := admission.Validate(t.Context()); !errors.Is(err, ErrIntegrationConflict) {
		t.Fatal("grant survived revoke", err)
	}
}

func TestRemoteServiceInvalidationAndPrincipalIsolation(t *testing.T) {
	s, query, _, control := remoteServiceFixture(t, "http://127.0.0.1:1")
	grantRemoteServiceFixture(t, s, control)
	admission, err := s.authenticateRemoteService(t.Context(), serviceFixtureToken)
	if err != nil {
		t.Fatal(err)
	}
	s.cancelIntegrations()
	if err := admission.Validate(t.Context()); err == nil {
		t.Fatal("grant survived project invalidation")
	}
	views, err := s.ListRemoteServicePrincipals(t.Context(), query)
	if err != nil || len(views) != 1 || views[0].Grant != "" {
		t.Fatalf("invalidated view %+v %v", views, err)
	}
	s.integrations.config.A2AServer.Principals[0].ProjectID = "other-project"
	views, err = s.ListRemoteServicePrincipals(t.Context(), query)
	if err != nil || len(views) != 0 {
		t.Fatal("another project's principal disclosed", views, err)
	}
}

func TestRemoteServiceListenerOwnershipAndCloseRevokeGrants(t *testing.T) {
	s, _, _, _ := remoteServiceFixture(t, "http://127.0.0.1:1")
	if _, err := s.NewRemoteService("http://127.0.0.1:9091/a2a"); !errors.Is(err, ErrIntegrationConflict) {
		t.Fatal("duplicate listener owner", err)
	}
	// A second service can share configuration without owning the listener.
	child, err := New(s.config, s.manager)
	if err != nil {
		t.Fatal(err)
	}
	child.integrations, err = newIntegrationRegistry(config.IntegrationsConfig{A2AServer: s.RemoteServiceConfig()})
	if err != nil {
		t.Fatal(err)
	}
	index, _ := child.manager.Index()
	query := AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}
	views, err := child.ListRemoteServicePrincipals(t.Context(), query)
	if err != nil || len(views) != 1 || views[0].State != "unavailable" {
		t.Fatal("inactive service advertised access", views, err)
	}
	listener, err := child.NewRemoteService("http://127.0.0.1:9091/a2a")
	if err != nil {
		t.Fatal(err)
	}
	views, _ = child.ListRemoteServicePrincipals(t.Context(), query)
	control := RemoteServiceControl{IntegrationControl: IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Configuration: views[0].Configuration, Action: "grant", ConfirmConnect: true}, ConfirmSourceSharing: true}
	grantRemoteServiceFixture(t, child, control)
	admission, err := child.authenticateRemoteService(t.Context(), serviceFixtureToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := admission.Validate(t.Context()); err == nil {
		t.Fatal("closed listener retained access")
	}
}
