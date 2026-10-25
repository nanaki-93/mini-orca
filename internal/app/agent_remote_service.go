package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/version"
)

type RemoteServicePrincipal struct {
	Config        config.AgentPrincipal `json:"config"`
	Configuration string                `json:"configuration"`
	State         string                `json:"state"`
	Grant         string                `json:"grant,omitempty"`
	ExpiresAt     *time.Time            `json:"expires_at,omitempty"`
}

type RemoteServiceControl struct {
	IntegrationControl
	ConfirmSourceSharing  bool `json:"confirm_source_sharing"`
	ConfirmRemoteProvider bool `json:"confirm_remote_provider"`
}

type remoteServiceGrant struct {
	root       string
	query      AgentRunQuery
	credential [32]byte
	view       RemoteServicePrincipal
}

// RemoteService keeps access controls tied to the service that actually owns
// the optional listener, rather than an unrelated worktree service instance.
type RemoteService struct {
	*agentruntime.AgentServer
	service  *Service
	identity string
}

func (s *Service) RemoteServiceConfig() config.AgentServer {
	value := s.integrations.config.A2AServer
	value.Principals = slices.Clone(value.Principals)
	for i := range value.Principals {
		value.Principals[i].Capabilities = slices.Clone(value.Principals[i].Capabilities)
	}
	return value
}

func (s *Service) NewRemoteService(endpoint string) (*RemoteService, error) {
	cfg := s.RemoteServiceConfig()
	if !cfg.Enabled {
		return nil, ErrIntegrationUnavailable
	}
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	if s.integrations.serverIdentity != "" {
		return nil, ErrIntegrationConflict
	}
	native, err := agentruntime.NewAgentServer(agentruntime.AgentServerConfig{Endpoint: endpoint, Version: version.Version, Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second, MaxConcurrent: cfg.MaxConcurrent}, s.authenticateRemoteService)
	if err != nil {
		return nil, err
	}
	s.integrations.serverIdentity = rand.Text()
	return &RemoteService{AgentServer: native, service: s, identity: s.integrations.serverIdentity}, nil
}

func (r *RemoteService) Close(ctx context.Context) error {
	registry := r.service.integrations
	registry.mu.Lock()
	var done []<-chan struct{}
	if registry.serverIdentity == r.identity {
		registry.serverIdentity = ""
		clear(registry.serverGrants)
		for operation := range registry.active {
			if strings.HasPrefix(operation.id, "@a2a_server:") {
				operation.cancel(ErrIntegrationConflict)
				done = append(done, operation.done)
			}
		}
	}
	registry.mu.Unlock()
	return errors.Join(r.AgentServer.Close(ctx), waitIntegrationOperations(ctx, done))
}

func (s *Service) remoteServiceView(principal config.AgentPrincipal) RemoteServicePrincipal {
	principal.Capabilities = slices.Clone(principal.Capabilities)
	s.integrations.mu.Lock()
	identity := s.integrations.serverIdentity
	s.integrations.mu.Unlock()
	view := RemoteServicePrincipal{Config: principal, Configuration: s.remoteServiceFingerprint(principal, identity), State: "configured"}
	if !s.integrations.config.A2AServer.Enabled {
		view.State = "disabled"
	} else if identity == "" {
		view.State = "unavailable"
	} else if _, err := remoteServiceCredential(principal); err != nil {
		view.State = "unavailable"
	}
	return view
}

func (s *Service) remoteServiceFingerprint(principal config.AgentPrincipal, listener string) string {
	data, _ := json.Marshal([]any{principal, s.EffectiveModels(), listener})
	return remoteDigest(string(data))
}

func (s *Service) ListRemoteServicePrincipals(ctx context.Context, query AgentRunQuery) ([]RemoteServicePrincipal, error) {
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	views := []RemoteServicePrincipal{}
	for _, principal := range s.RemoteServiceConfig().Principals {
		if principal.ProjectID != query.ProjectID {
			continue
		}
		view := s.remoteServiceView(principal)
		s.integrations.mu.Lock()
		record, exists := s.integrations.serverGrants[principal.ID]
		s.integrations.mu.Unlock()
		if exists && record.root == root && record.query == query && record.view.Configuration == view.Configuration && view.State == "configured" {
			if s.validateRemoteServiceGrant(ctx, record) == nil {
				view = cloneRemoteServicePrincipal(record.view)
			} else {
				view.State = "expired"
			}
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) ControlRemoteService(ctx context.Context, id string, request RemoteServiceControl) (*RemoteServicePrincipal, error) {
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	principal, ok := s.remoteServicePrincipal(id)
	if !ok || principal.ProjectID != query.ProjectID {
		return nil, ErrIntegrationUnavailable
	}
	view := s.remoteServiceView(principal)
	if view.Configuration != request.Configuration {
		return nil, ErrIntegrationConflict
	}
	if request.Action == "revoke" {
		return &view, s.revokeRemoteService(ctx, id, request.Grant)
	}
	if request.Action != "grant" || !request.ConfirmConnect || !request.ConfirmSourceSharing || view.State != "configured" {
		return nil, fmt.Errorf("explicit project access and source-sharing consent are required for a configured principal")
	}
	if err := s.confirmRemoteServiceProviders(principal, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	return s.grantRemoteService(ctx, root, query, view, request)
}

func (s *Service) grantRemoteService(ctx context.Context, root string, query AgentRunQuery, view RemoteServicePrincipal, request RemoteServiceControl) (*RemoteServicePrincipal, error) {
	id := view.Config.ID
	credential, err := remoteServiceCredential(view.Config)
	if err != nil {
		return nil, err
	}
	if err := s.revokeRemoteService(ctx, id, request.Grant); err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(15 * time.Minute)
	view.State, view.Grant, view.ExpiresAt = "authorized", rand.Text(), &expires
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	if _, exists := s.integrations.serverGrants[id]; exists {
		return nil, ErrIntegrationConflict
	}
	if s.integrations.serverIdentity == "" || view.Configuration != s.remoteServiceFingerprint(view.Config, s.integrations.serverIdentity) {
		return nil, ErrIntegrationConflict
	}
	s.integrations.serverGrants[id] = remoteServiceGrant{root: root, query: query, credential: credential, view: cloneRemoteServicePrincipal(view)}
	return &view, nil
}

func cloneRemoteServicePrincipal(view RemoteServicePrincipal) RemoteServicePrincipal {
	view.Config.Capabilities = slices.Clone(view.Config.Capabilities)
	if view.ExpiresAt != nil {
		expires := *view.ExpiresAt
		view.ExpiresAt = &expires
	}
	return view
}

func (s *Service) remoteServicePrincipal(id string) (config.AgentPrincipal, bool) {
	for _, principal := range s.integrations.config.A2AServer.Principals {
		if principal.ID == id {
			return principal, true
		}
	}
	return config.AgentPrincipal{}, false
}

func remoteServiceCredential(principal config.AgentPrincipal) ([32]byte, error) {
	token := os.Getenv(principal.BearerTokenEnv)
	if len(token) < 32 || len(token) > 4096 || strings.ContainsFunc(token, func(char rune) bool { return char < 33 || char > 126 }) {
		return [32]byte{}, ErrIntegrationUnavailable
	}
	return sha256.Sum256([]byte(token)), nil
}

func (s *Service) confirmRemoteServiceProviders(principal config.AgentPrincipal, confirmed bool) error {
	for _, capability := range principal.Capabilities {
		scope := config.CodeModelScope
		if capability == "proposal" {
			scope = config.BugModelScope
		}
		if err := s.RequireRemoteConfirmation(scope, confirmed); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) revokeRemoteService(ctx context.Context, id, grant string) error {
	s.integrations.mu.Lock()
	if record, exists := s.integrations.serverGrants[id]; exists && record.view.Grant != grant && record.view.ExpiresAt.After(time.Now()) {
		s.integrations.mu.Unlock()
		return ErrIntegrationConflict
	}
	delete(s.integrations.serverGrants, id)
	done := s.integrations.cancelIDLocked("@a2a_server:" + id)
	s.integrations.mu.Unlock()
	return waitIntegrationOperations(ctx, done)
}

func (s *Service) validateRemoteServiceGrant(ctx context.Context, record remoteServiceGrant) error {
	if err := s.integrationScope(ctx, record.query, record.root); err != nil {
		return err
	}
	s.integrations.mu.Lock()
	current, exists := s.integrations.serverGrants[record.view.Config.ID]
	s.integrations.mu.Unlock()
	credential, err := remoteServiceCredential(record.view.Config)
	if err != nil || credential != record.credential || !exists || current.view.Grant != record.view.Grant || !record.view.ExpiresAt.After(time.Now()) {
		return ErrIntegrationConflict
	}
	view := s.remoteServiceView(record.view.Config)
	if view.State != "configured" || view.Configuration != record.view.Configuration {
		return ErrIntegrationConflict
	}
	return nil
}

func (s *Service) authenticateRemoteService(ctx context.Context, token string) (agentruntime.ServerAdmission, error) {
	var principal config.AgentPrincipal
	matched := 0
	digest := sha256.Sum256([]byte(token))
	for _, candidate := range s.integrations.config.A2AServer.Principals {
		credential, err := remoteServiceCredential(candidate)
		if err == nil && subtle.ConstantTimeCompare(digest[:], credential[:]) == 1 {
			principal, matched = candidate, matched+1
		}
	}
	if matched != 1 {
		return agentruntime.ServerAdmission{}, ErrIntegrationUnavailable
	}
	s.integrations.mu.Lock()
	record, exists := s.integrations.serverGrants[principal.ID]
	s.integrations.mu.Unlock()
	if !exists {
		return agentruntime.ServerAdmission{}, ErrIntegrationConflict
	}
	check := func(ctx context.Context) error { return s.validateRemoteServiceGrant(ctx, record) }
	if err := check(ctx); err != nil {
		return agentruntime.ServerAdmission{}, err
	}
	return agentruntime.ServerAdmission{Owner: remoteDigest(principal.ID + record.view.Grant), ExpiresAt: *record.view.ExpiresAt, Validate: check,
		Execute: func(ctx context.Context, text string) (string, error) {
			return s.executeRemoteService(ctx, record, text)
		}}, nil
}
