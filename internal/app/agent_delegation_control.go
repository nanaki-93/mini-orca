package app

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

type DelegationControl struct {
	ProjectID       string         `json:"project_id"`
	ProjectRevision string         `json:"project_revision"`
	Version         int64          `json:"version"`
	Action          string         `json:"action"`
	Remote          IntegrationUse `json:"remote"`
	Reply           string         `json:"reply,omitempty"`
	Observation     string         `json:"observation,omitempty"`
	Stream          bool           `json:"stream"`
	Confirm         bool           `json:"confirm"`
}

func (s *Service) ControlDelegation(ctx context.Context, id string, request DelegationControl) (*AgentDelegation, error) {
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	if err := validateDelegationControl(request); err != nil {
		return nil, err
	}
	if err := s.claimRemoteExecution(id); err != nil {
		return nil, err
	}
	defer s.finishRemoteExecution(id)
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, err
	}
	receipt, err := findDelegation(ctx, store, query.ProjectID, id)
	if err != nil {
		return nil, err
	}
	if request.Version != receipt.Version {
		return nil, agentstate.ErrDelegationConflict
	}
	if request.Action == "reconcile" {
		changed, err := store.ReconcileDelegation(ctx, id, request.Version, request.Observation, func() error { return s.integrationScope(ctx, query, root) })
		if err != nil {
			return nil, err
		}
		view, err := s.delegationView(ctx, *changed, true)
		view.Active = false
		return &view, err
	}
	return s.controlRemoteTask(ctx, store, receipt, query, request)
}

func validateDelegationControl(request DelegationControl) error {
	if !request.Confirm {
		return fmt.Errorf("explicit remote operation confirmation is required")
	}
	switch request.Action {
	case "input":
		return validateDelegationReply(request)
	case "refresh", "cancel", "reconnect":
		if request.Reply != "" || request.Observation != "" || request.Stream {
			return fmt.Errorf("task controls cannot include input, observations or a streaming override")
		}
	case "reconcile":
		if request.Reply != "" || request.Stream || strings.TrimSpace(request.Observation) == "" {
			return fmt.Errorf("reconciliation requires only a user-reported observation")
		}
	default:
		return fmt.Errorf("action must be input, refresh, reconnect, cancel or reconcile")
	}
	return nil
}

func validateDelegationReply(request DelegationControl) error {
	if !request.Remote.ConfirmSourceSharing || strings.TrimSpace(request.Reply) == "" || len(request.Reply) > 2048 || !utf8.ValidString(request.Reply) || strings.ContainsRune(request.Reply, 0) || request.Observation != "" {
		return fmt.Errorf("provide an explicitly shared reply up to 2048 bytes")
	}
	return nil
}

func (s *Service) controlRemoteTask(ctx context.Context, store *agentstate.Store, receipt agentstate.Delegation, query AgentRunQuery, request DelegationControl) (*AgentDelegation, error) {
	record, connection, err := s.remoteAdmission(ctx, query, request.Remote)
	if err != nil {
		return nil, err
	}
	if receipt.RemoteID != request.Remote.ID || receipt.EndpointHash != remoteDigest(connection.Endpoint) || receipt.CardFingerprint != record.view.Inventory.Fingerprint {
		return nil, agentstate.ErrDelegationConflict
	}
	var captured *externalContext
	if request.Action == "input" {
		value := delegationContext(record.root, receipt)
		captured = &value
	}
	check := s.delegationAuthority(record, captured)
	if err := check(ctx); err != nil {
		return nil, err
	}
	owned, finish, err := s.integrations.begin(ctx, request.Remote.ID)
	if err != nil {
		return nil, err
	}
	defer finish()
	claimed, err := store.ClaimDelegation(owned, receipt.ID, request.Version, request.Action, remoteDigest(request.Reply), time.Now().UTC().Add(connection.Timeout+10*time.Second), func() error { return check(owned) })
	if err != nil {
		return nil, err
	}
	return s.executeDelegation(owned, store, record, connection, *claimed, request.Reply, request.Stream, check)
}
