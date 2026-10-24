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
)

type AgentRemote struct {
	Config        config.RemoteAgent            `json:"config"`
	Configuration string                        `json:"configuration"`
	State         string                        `json:"state"`
	Grant         string                        `json:"grant,omitempty"`
	ExpiresAt     *time.Time                    `json:"expires_at,omitempty"`
	Inventory     *agentruntime.RemoteInventory `json:"inventory,omitempty"`
}

type remoteConnectionRecord struct {
	root, credential string
	query            AgentRunQuery
	view             AgentRemote
}

func remoteDigest(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func remoteConfigView(remote config.RemoteAgent) AgentRemote {
	remote.Skills = slices.Clone(remote.Skills)
	data, _ := json.Marshal(remote) // Only primitive configuration fields.
	return AgentRemote{Config: remote, Configuration: remoteDigest(string(data)), State: "configured"}
}

func cloneRemoteView(view AgentRemote) AgentRemote {
	view.Config.Skills = slices.Clone(view.Config.Skills)
	if view.ExpiresAt != nil {
		expires := *view.ExpiresAt
		view.ExpiresAt = &expires
	}
	if view.Inventory != nil {
		inventory := view.Inventory.Clone()
		view.Inventory = &inventory
	}
	return view
}

func (s *Service) ListRemoteAgents(ctx context.Context, query AgentRunQuery) ([]AgentRemote, error) {
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	result := []AgentRemote{}
	for _, remote := range s.integrations.config.A2A {
		view := remoteConfigView(remote)
		if record, exists := s.integrations.remoteRecords[remote.ID]; exists && record.root == root && record.query == query {
			view = record.view
			if !view.ExpiresAt.After(time.Now()) {
				view.State, view.Grant = "expired", ""
			}
		}
		result = append(result, cloneRemoteView(view))
	}
	return result, nil
}

func (s *Service) remoteConfig(id string) (config.RemoteAgent, bool) {
	for _, remote := range s.integrations.config.A2A {
		if remote.ID == id {
			return remote, true
		}
	}
	return config.RemoteAgent{}, false
}

func remoteConnection(remote config.RemoteAgent) (agentruntime.RemoteConnection, string, error) {
	token, err := integrationCredential(remote.BearerTokenEnv)
	if err != nil {
		return agentruntime.RemoteConnection{}, "", err
	}
	return agentruntime.RemoteConnection{CardURL: remote.CardURL, Endpoint: remote.Endpoint, BearerToken: token, Skills: slices.Clone(remote.Skills), Timeout: time.Duration(remote.TimeoutSeconds) * time.Second}, remoteDigest(token), nil
}

func (s *Service) ControlRemoteAgent(ctx context.Context, id string, request IntegrationControl) (*AgentRemote, error) {
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	remote, exists := s.remoteConfig(id)
	if !exists {
		return nil, ErrIntegrationUnavailable
	}
	view := remoteConfigView(remote)
	if request.Configuration != view.Configuration {
		return nil, ErrIntegrationConflict
	}
	switch request.Action {
	case "revoke":
		done, err := s.revokeRemote(id, request.Grant)
		if err != nil {
			return nil, err
		}
		return &view, waitIntegrationOperations(ctx, done)
	case "discover":
		if !request.ConfirmConnect {
			return nil, fmt.Errorf("explicit remote card discovery confirmation is required")
		}
		return s.discoverRemote(ctx, root, query, view)
	default:
		return nil, fmt.Errorf("remote agent action must be discover or revoke")
	}
}

func (s *Service) revokeRemote(id, grant string) ([]<-chan struct{}, error) {
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	if record, exists := s.integrations.remoteRecords[id]; exists && record.view.Grant != grant && record.view.ExpiresAt.After(time.Now()) {
		return nil, ErrIntegrationConflict
	}
	delete(s.integrations.remoteRecords, id)
	return s.integrations.cancelIDLocked(id), nil
}

func (s *Service) discoverRemote(ctx context.Context, root string, query AgentRunQuery, view AgentRemote) (*AgentRemote, error) {
	connection, credential, err := remoteConnection(view.Config)
	if err != nil {
		return nil, err
	}
	// Replacing a card grant cancels and joins operations admitted under it.
	s.integrations.mu.Lock()
	delete(s.integrations.remoteRecords, view.Config.ID)
	done := s.integrations.cancelIDLocked(view.Config.ID)
	s.integrations.mu.Unlock()
	if err := waitIntegrationOperations(ctx, done); err != nil {
		return nil, err
	}
	owned, finish, err := s.integrations.begin(ctx, view.Config.ID)
	if err != nil {
		return nil, err
	}
	defer finish()
	bounded, cancel, err := s.agentRunContextAt(owned, root)
	if err != nil {
		return nil, err
	}
	defer cancel()
	authorize := func(ctx context.Context) error { return s.integrationScope(ctx, query, root) }
	inventory, err := agentruntime.DiscoverRemote(bounded, connection, authorize)
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(15 * time.Minute)
	view.State, view.Grant, view.ExpiresAt, view.Inventory = "ready", rand.Text(), &expires, &inventory
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	if err := authorize(bounded); err != nil {
		return nil, err
	}
	s.integrations.remoteRecords[view.Config.ID] = remoteConnectionRecord{root: root, query: query, credential: credential, view: view}
	copy := cloneRemoteView(view)
	return &copy, nil
}

func (s *Service) remoteAdmission(ctx context.Context, query AgentRunQuery, use IntegrationUse) (remoteConnectionRecord, agentruntime.RemoteConnection, error) {
	s.integrations.mu.Lock()
	record, exists := s.integrations.remoteRecords[use.ID]
	s.integrations.mu.Unlock()
	if !exists || record.query != query || record.view.Grant != use.Grant || record.view.Inventory.Fingerprint != use.SchemaFingerprint {
		return record, agentruntime.RemoteConnection{}, ErrIntegrationConflict
	}
	if err := s.validateRemoteGrant(ctx, record); err != nil {
		return record, agentruntime.RemoteConnection{}, err
	}
	connection, _, err := remoteConnection(record.view.Config)
	return record, connection, err
}

func (s *Service) validateRemoteGrant(ctx context.Context, record remoteConnectionRecord) error {
	if err := s.integrationScope(ctx, record.query, record.root); err != nil {
		return err
	}
	s.integrations.mu.Lock()
	current, exists := s.integrations.remoteRecords[record.view.Config.ID]
	s.integrations.mu.Unlock()
	if !exists || current.view.Grant != record.view.Grant || !record.view.ExpiresAt.After(time.Now()) {
		return ErrIntegrationConflict
	}
	_, credential, err := remoteConnection(record.view.Config)
	if err != nil || credential != record.credential {
		return ErrIntegrationUnavailable
	}
	return nil
}
