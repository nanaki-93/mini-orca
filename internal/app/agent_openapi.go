package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type AgentAPI struct {
	Config        config.OpenAPIService      `json:"config"`
	Configuration string                     `json:"configuration"`
	State         string                     `json:"state"`
	Grant         string                     `json:"grant,omitempty"`
	ExpiresAt     *time.Time                 `json:"expires_at,omitempty"`
	Inventory     *agentruntime.APIInventory `json:"inventory,omitempty"`
}

type apiIntegrationRecord struct {
	root, policy, specHash, credential string
	query                              AgentRunQuery
	view                               AgentAPI
}

type apiActionAuthority struct {
	integrationID, grant string
	expires              time.Time
	check                func(context.Context) error
}

func apiConfigView(service config.OpenAPIService) AgentAPI {
	service.Operations = slices.Clone(service.Operations)
	data, _ := json.Marshal(service)
	hash := sha256.Sum256(data)
	return AgentAPI{Config: service, Configuration: hex.EncodeToString(hash[:]), State: "configured"}
}

func cloneAPIView(view AgentAPI) (AgentAPI, error) {
	// Configuration and contracts contain only JSON-safe values. The round trip
	// detaches nested schemas, parameter flags and response maps from the registry.
	data, err := json.Marshal(view)
	if err != nil {
		return AgentAPI{}, err
	}
	var result AgentAPI
	err = json.Unmarshal(data, &result)
	return result, err
}

func (s *Service) ListAPIs(ctx context.Context, query AgentRunQuery) ([]AgentAPI, error) {
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	result := []AgentAPI{}
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	for _, configured := range s.integrations.config.OpenAPI {
		view := apiConfigView(configured)
		if record, exists := s.integrations.apiRecords[configured.ID]; exists && record.root == root && record.query == query {
			view = record.view
			if !view.ExpiresAt.After(time.Now()) {
				view.State, view.Grant = "expired", ""
			}
		}
		copy, err := cloneAPIView(view)
		if err != nil {
			return nil, err
		}
		result = append(result, copy)
	}
	return result, nil
}

func (s *Service) apiConfig(id string) (config.OpenAPIService, bool) {
	for _, service := range s.integrations.config.OpenAPI {
		if service.ID == id {
			return service, true
		}
	}
	return config.OpenAPIService{}, false
}

func (s *Service) ControlAPI(ctx context.Context, id string, request IntegrationControl) (*AgentAPI, error) {
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	configured, exists := s.apiConfig(id)
	if !exists {
		return nil, ErrIntegrationUnavailable
	}
	view := apiConfigView(configured)
	if view.Configuration != request.Configuration {
		return nil, ErrIntegrationConflict
	}
	switch request.Action {
	case "discover":
		if !request.ConfirmConnect {
			return nil, fmt.Errorf("explicit API inventory review is required")
		}
		return s.discoverAPI(ctx, root, query, view)
	case "revoke":
		done, err := s.revokeAPI(id, request.Grant)
		if err != nil {
			return nil, err
		}
		return &view, waitIntegrationOperations(ctx, done)
	default:
		return nil, fmt.Errorf("API action must be discover or revoke")
	}
}

func (s *Service) discoverAPI(ctx context.Context, root string, query AgentRunQuery, view AgentAPI) (*AgentAPI, error) {
	owned, finish, err := s.integrations.begin(ctx, view.Config.ID)
	if err != nil {
		return nil, err
	}
	defer finish()
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	file, err := s.manager.IndexedFile(view.Config.SpecPath)
	if err != nil {
		return nil, err
	}
	data, err := policy.ReadSource(owned, file.Path, file.ContentHash)
	if err != nil {
		return nil, err
	}
	inventory, err := agentruntime.DiscoverAPI(data, view.Config.Operations)
	if err != nil {
		return nil, err
	}
	_, credential, err := s.apiConnection(view.Config)
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(15 * time.Minute)
	view.State, view.Grant, view.ExpiresAt, view.Inventory = "ready", rand.Text(), &expires, &inventory
	record := apiIntegrationRecord{root: root, query: query, policy: policy.Version(), specHash: file.ContentHash, credential: credential, view: view}
	if err := s.apiSourceCurrent(owned, record); err != nil {
		return nil, err
	}
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	if err := s.integrationScope(owned, query, root); err != nil {
		return nil, err
	}
	s.integrations.apiRecords[view.Config.ID] = record
	copy, err := cloneAPIView(view)
	return &copy, err
}

func (s *Service) revokeAPI(id, grant string) ([]<-chan struct{}, error) {
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	if record, exists := s.integrations.apiRecords[id]; exists && record.view.Grant != grant && record.view.ExpiresAt.After(time.Now()) {
		return nil, ErrIntegrationConflict
	}
	delete(s.integrations.apiRecords, id)
	for key, authority := range s.integrations.apiActions {
		if authority.integrationID == id {
			delete(s.integrations.apiActions, key)
		}
	}
	return s.integrations.cancelIDLocked(id), nil
}

func (s *Service) apiConnection(configured config.OpenAPIService) (agentruntime.APIConnection, string, error) {
	token, err := integrationCredential(configured.BearerTokenEnv)
	if err != nil {
		return agentruntime.APIConnection{}, "", err
	}
	hash := sha256.Sum256([]byte(token))
	connection := agentruntime.APIConnection{Endpoint: configured.Endpoint, BearerToken: token, Timeout: time.Duration(configured.TimeoutSeconds) * time.Second,
		Begin: func(ctx context.Context) (context.Context, func(), error) {
			return s.integrations.begin(ctx, configured.ID)
		}}
	return connection, hex.EncodeToString(hash[:]), nil
}

func (s *Service) apiSourceCurrent(ctx context.Context, record apiIntegrationRecord) error {
	if err := s.integrationScope(ctx, record.query, record.root); err != nil {
		return err
	}
	policy, err := project.NewContextPolicy(record.root)
	if err != nil {
		return err
	}
	if policy.Version() != record.policy {
		return ErrIntegrationConflict
	}
	if _, err := policy.ReadSource(ctx, record.view.Config.SpecPath, record.specHash); err != nil {
		return fmt.Errorf("%w: API specification changed", ErrIntegrationConflict)
	}
	_, credential, err := s.apiConnection(record.view.Config)
	if err != nil || credential != record.credential {
		return ErrIntegrationUnavailable
	}
	return nil
}

func (s *Service) validateAPIGrant(ctx context.Context, record apiIntegrationRecord) error {
	if err := s.apiSourceCurrent(ctx, record); err != nil {
		return err
	}
	s.integrations.mu.Lock()
	current, exists := s.integrations.apiRecords[record.view.Config.ID]
	s.integrations.mu.Unlock()
	if !exists || current.view.Grant != record.view.Grant || !record.view.ExpiresAt.After(time.Now()) {
		return ErrIntegrationConflict
	}
	return nil
}

func (s *Service) admitAPI(ctx context.Context, root, scope string, use IntegrationUse, source func(context.Context) error) ([]agentruntime.ToolSpec, func(context.Context) error, error) {
	s.integrations.mu.Lock()
	record, exists := s.integrations.apiRecords[use.ID]
	s.integrations.mu.Unlock()
	if !exists || record.root != root || record.view.Grant != use.Grant || record.view.Inventory.Fingerprint != use.SchemaFingerprint {
		return nil, nil, ErrIntegrationConflict
	}
	check := func(ctx context.Context) error {
		if err := source(ctx); err != nil {
			return err
		}
		return s.validateAPIGrant(ctx, record)
	}
	if err := check(ctx); err != nil {
		return nil, nil, err
	}
	connection, _, err := s.apiConnection(record.view.Config)
	if err != nil {
		return nil, nil, err
	}
	propose := func(ctx context.Context, action agentruntime.APIAction, args json.RawMessage) (string, error) {
		return s.proposeAPIAction(ctx, record, scope, action, args, check)
	}
	return agentruntime.APITools(connection, use.ID, *record.view.Inventory, check, propose), check, nil
}
