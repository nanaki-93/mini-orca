package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

type APIActionControl struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	Action          string `json:"action"`
	Version         int64  `json:"version"`
	RequestHash     string `json:"request_hash"`
	Confirm         bool   `json:"confirm"`
	ReportedOutcome string `json:"reported_outcome,omitempty"`
	Observation     string `json:"observation,omitempty"`
}

type AgentAPIAction struct {
	agentstate.ExternalAction
	CanApprove   bool `json:"can_approve"`
	CanReconcile bool `json:"can_reconcile"`
	Stale        bool `json:"stale"`
}

func (s *Service) proposeAPIAction(ctx context.Context, record apiIntegrationRecord, scope string, action agentruntime.APIAction, args json.RawMessage, check func(context.Context) error) (string, error) {
	if err := check(ctx); err != nil {
		return "", err
	}
	request, err := json.Marshal(action)
	if err != nil {
		return "", err
	}
	store, err := agentstate.Open(record.root)
	if err != nil {
		return "", err
	}
	s.integrations.mu.Lock()
	for id, authority := range s.integrations.apiActions {
		if !authority.expires.After(time.Now()) {
			delete(s.integrations.apiActions, id)
		}
	}
	full := len(s.integrations.apiActions) >= 128
	s.integrations.mu.Unlock()
	if full {
		return "", fmt.Errorf("external action review limit reached")
	}
	draft := agentstate.ExternalAction{ProjectID: record.query.ProjectID, ProjectRevision: record.query.ProjectRevision, IntegrationID: record.view.Config.ID,
		Inventory: record.view.Inventory.Fingerprint, Scope: scope, OperationKey: action.OperationKey, Request: request, Arguments: args, ExpiresAt: *record.view.ExpiresAt}
	created, err := store.CreateExternalAction(ctx, draft, func() error { return check(ctx) })
	if err != nil {
		return "", err
	}
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	current, exists := s.integrations.apiRecords[record.view.Config.ID]
	if !exists || current.view.Grant != record.view.Grant || context.Cause(ctx) != nil {
		return "", ErrIntegrationConflict
	}
	// Concurrent proposals can fill the admission limit while the durable
	// candidate is being saved. Such a candidate retains no dispatch authority.
	if len(s.integrations.apiActions) >= 128 {
		return "", fmt.Errorf("external action review limit reached")
	}
	s.integrations.apiActions[created.ID] = apiActionAuthority{integrationID: record.view.Config.ID, grant: record.view.Grant, expires: *record.view.ExpiresAt, check: check}
	return created.ID, nil
}

func (s *Service) ListAPIActions(ctx context.Context, query AgentRunQuery) ([]AgentAPIAction, error) {
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, err
	}
	actions, err := store.ExternalActions(ctx, query.ProjectID)
	if err != nil {
		return nil, err
	}
	result := make([]AgentAPIAction, 0, len(actions))
	for _, action := range actions {
		result = append(result, s.apiActionView(ctx, action, query.ProjectRevision))
	}
	return result, s.integrationScope(ctx, query, root)
}

func (s *Service) apiActionView(ctx context.Context, action agentstate.ExternalAction, revision string) AgentAPIAction {
	s.integrations.mu.Lock()
	authority, exists := s.integrations.apiActions[action.ID]
	executing := s.integrations.apiExecuting[action.ID]
	s.integrations.mu.Unlock()
	view := AgentAPIAction{ExternalAction: action, Stale: action.ProjectRevision != revision}
	view.CanApprove = !view.Stale && action.State == "candidate" && exists && authority.expires.After(time.Now()) && authority.check(ctx) == nil
	view.CanReconcile = !executing && (action.State == "unknown" || action.State == "pending" && time.Now().After(action.PendingUntil))
	return view
}

func (s *Service) ControlAPIAction(ctx context.Context, id string, request APIActionControl) (*AgentAPIAction, error) {
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	if !request.Confirm {
		return nil, fmt.Errorf("explicit approval or reconciliation is required")
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, err
	}
	action, err := findExternalAction(ctx, store, query.ProjectID, id)
	if err != nil {
		return nil, err
	}
	if action.Version != request.Version || action.RequestHash != request.RequestHash {
		return nil, agentstate.ErrActionConflict
	}
	var changed *agentstate.ExternalAction
	switch request.Action {
	case "approve":
		if request.ReportedOutcome != "" || request.Observation != "" {
			return nil, fmt.Errorf("approval cannot include a reported outcome")
		}
		changed, err = s.approveAPIAction(ctx, store, action, query)
	case "reconcile":
		if !s.apiActionView(ctx, action, query.ProjectRevision).CanReconcile {
			return nil, agentstate.ErrActionConflict
		}
		changed, err = store.ReconcileExternalAction(ctx, id, request.Version, request.RequestHash, request.ReportedOutcome, request.Observation, func() error { return s.integrationScope(ctx, query, root) })
	default:
		return nil, fmt.Errorf("external action must be approve or reconcile")
	}
	if err != nil {
		return nil, err
	}
	view := s.apiActionView(ctx, *changed, query.ProjectRevision)
	return &view, nil
}

func findExternalAction(ctx context.Context, store *agentstate.Store, projectID, id string) (agentstate.ExternalAction, error) {
	items, err := store.ExternalActions(ctx, projectID)
	if err != nil {
		return agentstate.ExternalAction{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return agentstate.ExternalAction{}, agentstate.ErrNotFound
}

func (s *Service) actionAdmission(ctx context.Context, action agentstate.ExternalAction, query AgentRunQuery) (apiIntegrationRecord, apiActionAuthority, agentruntime.APIOperation, error) {
	s.integrations.mu.Lock()
	authority, exists := s.integrations.apiActions[action.ID]
	record, ready := s.integrations.apiRecords[action.IntegrationID]
	s.integrations.mu.Unlock()
	if !exists || !ready || action.State != "candidate" || action.ProjectRevision != query.ProjectRevision || authority.grant != record.view.Grant || action.Inventory != record.view.Inventory.Fingerprint {
		return record, authority, agentruntime.APIOperation{}, agentstate.ErrActionConflict
	}
	if err := authority.check(ctx); err != nil {
		return record, authority, agentruntime.APIOperation{}, err
	}
	var exact agentruntime.APIAction
	if err := json.Unmarshal(action.Request, &exact); err != nil {
		return record, authority, agentruntime.APIOperation{}, agentstate.ErrCorrupt
	}
	for _, operation := range record.view.Inventory.Operations {
		if operation.ID == exact.OperationID && operation.Effect == "mutate_external" {
			return record, authority, operation, nil
		}
	}
	return record, authority, agentruntime.APIOperation{}, agentstate.ErrActionConflict
}

func (s *Service) approveAPIAction(ctx context.Context, store *agentstate.Store, action agentstate.ExternalAction, query AgentRunQuery) (*agentstate.ExternalAction, error) {
	record, authority, operation, err := s.actionAdmission(ctx, action, query)
	if err != nil {
		return nil, err
	}
	owned, finish, err := s.integrations.begin(ctx, record.view.Config.ID)
	if err != nil {
		return nil, err
	}
	defer finish()
	bounded, cancel, err := s.agentRunContextAt(owned, record.root)
	if err != nil {
		return nil, err
	}
	defer cancel()
	if err := agentruntime.BudgetFromContext(bounded).AdmitTool(bounded); err != nil {
		return nil, err
	}
	connection, _, err := s.apiConnection(record.view.Config)
	if err != nil {
		return nil, err
	}
	connection.Begin = nil // The outer owner spans the durable outcome receipt.
	if err := s.claimActionExecution(action.ID); err != nil {
		return nil, err
	}
	defer s.finishActionExecution(action.ID)
	claimed, err := store.ClaimExternalAction(bounded, action.ID, action.Version, action.RequestHash, time.Now().UTC().Add(connection.Timeout+10*time.Second), func() error { return authority.check(bounded) })
	if err != nil {
		return nil, err
	}
	response, callErr := agentruntime.ExecuteAPI(bounded, connection, operation, claimed.Arguments, claimed.RequestHash, authority.check)
	state, payload := apiActionOutcome(response, callErr)
	cleanup, release := context.WithTimeout(context.WithoutCancel(bounded), 5*time.Second)
	defer release()
	completed, saveErr := store.FinishExternalAction(cleanup, claimed.ID, claimed.Version, state, response.Status, payload)
	if saveErr != nil {
		return nil, errors.Join(fmt.Errorf("%w: preserve external action outcome", agentstate.ErrPersistence), saveErr)
	}
	// The receipt reports completed/not_sent/unknown explicitly. An unknown
	// outcome is never converted into a successful external write or retried.
	return completed, nil
}

func apiActionOutcome(response agentruntime.APIResponse, err error) (string, json.RawMessage) {
	if !response.Attempted {
		return "not_sent", nil
	}
	if err != nil {
		return "unknown", nil
	}
	return "completed", response.Data
}

func (s *Service) claimActionExecution(id string) error {
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	if s.integrations.apiExecuting[id] {
		return agentstate.ErrActionConflict
	}
	s.integrations.apiExecuting[id] = true
	return nil
}

func (s *Service) finishActionExecution(id string) {
	s.integrations.mu.Lock()
	delete(s.integrations.apiExecuting, id)
	delete(s.integrations.apiActions, id)
	s.integrations.mu.Unlock()
}
