package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

var ErrIntegrationConflict = errors.New("integration grant changed or expired")
var ErrIntegrationUnavailable = errors.New("integration unavailable")

type IntegrationUse struct {
	ID                   string `json:"id"`
	Grant                string `json:"grant"`
	SchemaFingerprint    string `json:"schema_fingerprint"`
	ConfirmSourceSharing bool   `json:"confirm_source_sharing"`
}

type IntegrationControl struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	Action          string `json:"action"`
	Configuration   string `json:"configuration"`
	Grant           string `json:"grant,omitempty"`
	ConfirmConnect  bool   `json:"confirm_connect"`
}

type AgentIntegration struct {
	Config        config.MCPServer           `json:"config"`
	Configuration string                     `json:"configuration"`
	State         string                     `json:"state"`
	Grant         string                     `json:"grant,omitempty"`
	ExpiresAt     *time.Time                 `json:"expires_at,omitempty"`
	Inventory     *agentruntime.MCPInventory `json:"inventory,omitempty"`
}

type integrationRecord struct {
	root       string
	query      AgentRunQuery
	credential string
	view       AgentIntegration
}

type integrationOperation struct {
	id     string
	cancel context.CancelCauseFunc
	done   chan struct{}
}

type integrationRegistry struct {
	mu              sync.Mutex
	config          config.IntegrationsConfig
	records         map[string]integrationRecord
	active          map[*integrationOperation]bool
	apiRecords      map[string]apiIntegrationRecord
	apiActions      map[string]apiActionAuthority
	apiExecuting    map[string]bool
	remoteRecords   map[string]remoteConnectionRecord
	remoteExecuting map[string]bool
}

func newIntegrationRegistry(input config.IntegrationsConfig) (*integrationRegistry, error) {
	resolved, err := config.ResolveIntegrations(input)
	if err != nil {
		return nil, err
	}
	return &integrationRegistry{config: resolved, records: map[string]integrationRecord{}, active: map[*integrationOperation]bool{}, apiRecords: map[string]apiIntegrationRecord{}, apiActions: map[string]apiActionAuthority{}, apiExecuting: map[string]bool{}, remoteRecords: map[string]remoteConnectionRecord{}, remoteExecuting: map[string]bool{}}, nil
}

func integrationConfigView(server config.MCPServer) AgentIntegration {
	// All fields are JSON-safe primitives; configuration contains references,
	// never resolved credential values.
	data, _ := json.Marshal(server)
	hash := sha256.Sum256(data)
	return AgentIntegration{Config: server, Configuration: hex.EncodeToString(hash[:]), State: "configured"}
}

func (s *Service) ListIntegrations(ctx context.Context, query AgentRunQuery) ([]AgentIntegration, error) {
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	result := make([]AgentIntegration, 0, len(s.integrations.config.MCP))
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	for _, server := range s.integrations.config.MCP {
		view := integrationConfigView(server)
		if record, ok := s.integrations.records[server.ID]; ok && record.root == root && record.query == query {
			view = record.view
			if !view.ExpiresAt.After(time.Now()) {
				view.State, view.Grant = "expired", ""
			}
		}
		result = append(result, cloneIntegrationView(view))
	}
	return result, nil
}

func cloneIntegrationView(view AgentIntegration) AgentIntegration {
	view.Config.Args, view.Config.Tools, view.Config.Environment = slices.Clone(view.Config.Args), slices.Clone(view.Config.Tools), maps.Clone(view.Config.Environment)
	if view.ExpiresAt != nil {
		expires := *view.ExpiresAt
		view.ExpiresAt = &expires
	}
	if view.Inventory != nil {
		inventory := *view.Inventory
		inventory.Tools = slices.Clone(inventory.Tools)
		for i := range inventory.Tools {
			inventory.Tools[i].InputSchema = slices.Clone(inventory.Tools[i].InputSchema)
			inventory.Tools[i].OutputSchema = slices.Clone(inventory.Tools[i].OutputSchema)
		}
		view.Inventory = &inventory
	}
	return view
}

func (s *Service) integrationScope(ctx context.Context, query AgentRunQuery, root string) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return s.agentHistoryScope(query, root)
}

func (s *Service) ControlIntegration(ctx context.Context, id string, request IntegrationControl) (*AgentIntegration, error) {
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	server, ok := s.integrationConfig(id)
	if !ok {
		return nil, ErrIntegrationUnavailable
	}
	view := integrationConfigView(server)
	if request.Configuration != view.Configuration {
		return nil, ErrIntegrationConflict
	}
	switch request.Action {
	case "revoke":
		done, err := s.revokeIntegration(id, request.Grant)
		if err != nil {
			return nil, err
		}
		return &view, waitIntegrationOperations(ctx, done)
	case "discover":
		if !request.ConfirmConnect {
			return nil, fmt.Errorf("explicit connection/process approval is required")
		}
		return s.discoverIntegration(ctx, root, query, view)
	default:
		return nil, fmt.Errorf("integration action must be discover or revoke")
	}
}

func (s *Service) integrationConfig(id string) (config.MCPServer, bool) {
	for _, server := range s.integrations.config.MCP {
		if server.ID == id {
			return server, true
		}
	}
	return config.MCPServer{}, false
}

func (s *Service) discoverIntegration(ctx context.Context, root string, query AgentRunQuery, view AgentIntegration) (*AgentIntegration, error) {
	connection, credential, err := s.integrationConnection(view.Config)
	if err != nil {
		return nil, err
	}
	// The outer operation remains registered through publication, closing the
	// race between a successful discovery and concurrent revoke/reindex.
	scoped, finish, err := s.integrations.begin(ctx, view.Config.ID)
	if err != nil {
		return nil, err
	}
	defer finish()
	connection.Begin = nil
	var names []string
	for _, tool := range view.Config.Tools {
		names = append(names, tool.Name)
	}
	authorize := func(ctx context.Context) error { return s.integrationScope(ctx, query, root) }
	inventory, err := agentruntime.DiscoverMCP(scoped, connection, view.Config.ID, names, authorize)
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(15 * time.Minute)
	view.State, view.Grant, view.ExpiresAt, view.Inventory = "ready", rand.Text(), &expires, &inventory
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	if err := authorize(scoped); err != nil {
		return nil, err
	}
	s.integrations.records[view.Config.ID] = integrationRecord{root: root, query: query, credential: credential, view: view}
	result := cloneIntegrationView(view)
	return &result, nil
}

func (r *integrationRegistry) begin(ctx context.Context, id string) (context.Context, func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.active) >= 16 {
		return nil, nil, fmt.Errorf("integration concurrency limit reached")
	}
	scoped, cancel := context.WithCancelCause(ctx)
	op := &integrationOperation{id: id, cancel: cancel, done: make(chan struct{})}
	r.active[op] = true
	return scoped, func() {
		cancel(context.Canceled)
		r.mu.Lock()
		delete(r.active, op)
		close(op.done)
		r.mu.Unlock()
	}, nil
}

func (s *Service) revokeIntegration(id, grant string) ([]<-chan struct{}, error) {
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	if record, ok := s.integrations.records[id]; ok && record.view.Grant != grant && record.view.ExpiresAt.After(time.Now()) {
		return nil, ErrIntegrationConflict
	}
	delete(s.integrations.records, id)
	return s.integrations.cancelIDLocked(id), nil
}

func (r *integrationRegistry) cancelIDLocked(id string) []<-chan struct{} {
	var done []<-chan struct{}
	for op := range r.active {
		if op.id == id {
			op.cancel(ErrIntegrationConflict)
			done = append(done, op.done)
		}
	}
	return done
}

func (s *Service) cancelIntegrations() []<-chan struct{} {
	if s.integrations == nil {
		return nil
	}
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	clear(s.integrations.records)
	clear(s.integrations.apiRecords)
	clear(s.integrations.apiActions)
	clear(s.integrations.remoteRecords)
	var done []<-chan struct{}
	for op := range s.integrations.active {
		op.cancel(ErrIntegrationConflict)
		done = append(done, op.done)
	}
	return done
}

func (s *Service) waitIntegrations(ctx context.Context) error {
	return waitIntegrationOperations(ctx, s.cancelIntegrations())
}

func waitIntegrationOperations(ctx context.Context, operations []<-chan struct{}) error {
	for _, done := range operations {
		select {
		case <-done:
		case <-ctx.Done():
			return context.Cause(ctx)
		}
	}
	return nil
}

func (s *Service) integrationInvestigation(ctx context.Context, root, scope string, specs []agentruntime.ToolSpec, uses []IntegrationUse, authorize func(context.Context) error) (*agentruntime.Investigation, error) {
	if len(uses) > 2 {
		return nil, fmt.Errorf("an investigation permits at most two integrations")
	}
	checks := []func(context.Context) error{authorize}
	seen := map[string]bool{}
	for _, use := range slices.Clone(uses) {
		if seen[use.ID] || !use.ConfirmSourceSharing {
			return nil, fmt.Errorf("each selected integration requires explicit source-sharing approval")
		}
		seen[use.ID] = true
		tools, check, err := s.admitSelectedIntegration(ctx, root, scope, use, authorize)
		if err != nil {
			return nil, err
		}
		specs, checks = append(specs, tools...), append(checks, check)
	}
	combined := func(ctx context.Context) error {
		for _, check := range checks {
			if err := check(ctx); err != nil {
				return err
			}
		}
		return nil
	}
	if len(uses) > 0 {
		data, _ := json.Marshal([]any{scope, uses})
		hash := sha256.Sum256(data)
		scope = hex.EncodeToString(hash[:])
	}
	return s.skillInvestigation(ctx, root, scope, specs, combined)
}

func (s *Service) admitSelectedIntegration(ctx context.Context, root, scope string, use IntegrationUse, authorize func(context.Context) error) ([]agentruntime.ToolSpec, func(context.Context) error, error) {
	if _, exists := s.apiConfig(use.ID); exists {
		return s.admitAPI(ctx, root, scope, use, authorize)
	}
	return s.admitIntegration(ctx, root, use, authorize)
}

func (s *Service) admitIntegration(ctx context.Context, root string, use IntegrationUse, source func(context.Context) error) ([]agentruntime.ToolSpec, func(context.Context) error, error) {
	s.integrations.mu.Lock()
	record, ok := s.integrations.records[use.ID]
	s.integrations.mu.Unlock()
	if !ok || record.root != root || record.view.Grant != use.Grant || record.view.Inventory.Fingerprint != use.SchemaFingerprint || !record.view.ExpiresAt.After(time.Now()) {
		return nil, nil, ErrIntegrationConflict
	}
	connection, credential, err := s.integrationConnection(record.view.Config)
	if err != nil || credential != record.credential {
		return nil, nil, ErrIntegrationUnavailable
	}
	check := func(ctx context.Context) error {
		if err := source(ctx); err != nil {
			return err
		}
		return s.validateIntegrationGrant(ctx, record, use)
	}
	if err := check(ctx); err != nil {
		return nil, nil, err
	}
	return agentruntime.MCPTools(connection, use.ID, *record.view.Inventory, check), check, nil
}

func (s *Service) validateIntegrationGrant(ctx context.Context, record integrationRecord, use IntegrationUse) error {
	if err := s.integrationScope(ctx, record.query, record.root); err != nil {
		return err
	}
	s.integrations.mu.Lock()
	current, ok := s.integrations.records[use.ID]
	s.integrations.mu.Unlock()
	if !ok || current.view.Grant != use.Grant || !record.view.ExpiresAt.After(time.Now()) {
		return ErrIntegrationConflict
	}
	_, credential, err := s.integrationConnection(record.view.Config)
	if err != nil || credential != record.credential {
		return ErrIntegrationUnavailable
	}
	return nil
}
