package agentstate

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

var ErrActionConflict = errors.New("external action changed or is no longer approvable")
var ErrActionUncertain = errors.New("an external operation requires outcome reconciliation")

// ExternalAction is private source-bearing review and recovery data. Request
// contains the exact method/URL/body, never resolved authentication values.
type ExternalAction struct {
	ID              string          `json:"id"`
	Version         int64           `json:"version"`
	ProjectID       string          `json:"project_id"`
	ProjectRevision string          `json:"project_revision"`
	IntegrationID   string          `json:"integration_id"`
	Inventory       string          `json:"inventory"`
	Scope           string          `json:"scope"`
	OperationKey    string          `json:"operation_key"`
	Request         json.RawMessage `json:"request"`
	Arguments       json.RawMessage `json:"arguments"`
	RequestHash     string          `json:"request_hash"`
	State           string          `json:"state"`
	HTTPStatus      int             `json:"http_status,omitempty"`
	Response        json.RawMessage `json:"response,omitempty"`
	ResponseHash    string          `json:"response_hash,omitempty"`
	ReportedOutcome string          `json:"reported_outcome,omitempty"`
	Observation     string          `json:"observation,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	ExpiresAt       time.Time       `json:"expires_at"`
	PendingUntil    time.Time       `json:"pending_until,omitempty"`
}

func (action ExternalAction) validate() error {
	for _, id := range []string{action.ID, action.ProjectID, action.ProjectRevision, action.IntegrationID} {
		if !validID(id) {
			return ErrCorrupt
		}
	}
	for _, hash := range []string{action.Inventory, action.Scope, action.OperationKey, action.RequestHash} {
		if !validHash(hash) {
			return ErrCorrupt
		}
	}
	if action.Version < 1 || action.Version > 1<<53 || action.CreatedAt.IsZero() || action.UpdatedAt.Before(action.CreatedAt) || !action.ExpiresAt.After(action.CreatedAt) {
		return ErrCorrupt
	}
	if len(action.Request) > 24<<10 || !validJSON(action.Request) || artifactHash(action.Request) != action.RequestHash || len(action.Arguments) > 16<<10 || !validJSON(action.Arguments) {
		return ErrCorrupt
	}
	return validateActionOutcome(action)
}

func validateActionOutcome(action ExternalAction) error {
	if err := validateActionResponse(action); err != nil {
		return err
	}
	if action.State != "reconciled" && (action.ReportedOutcome != "" || action.Observation != "") {
		return ErrCorrupt
	}
	switch action.State {
	case "candidate", "pending", "not_sent":
		return validateUndispatchedAction(action)
	case "completed":
		if action.HTTPStatus < 200 || action.HTTPStatus >= 300 || len(action.Response) == 0 {
			return ErrCorrupt
		}
	case "unknown":
		if len(action.Response) != 0 {
			return ErrCorrupt
		}
	case "reconciled":
		return validateReconciledAction(action)
	default:
		return ErrCorrupt
	}
	return nil
}

func validateActionResponse(action ExternalAction) error {
	if action.HTTPStatus != 0 && (action.HTTPStatus < 100 || action.HTTPStatus > 599) {
		return ErrCorrupt
	}
	if len(action.Response) > 16<<10 || len(action.Response) > 0 && (!validJSON(action.Response) || artifactHash(action.Response) != action.ResponseHash) || len(action.Response) == 0 && action.ResponseHash != "" {
		return ErrCorrupt
	}
	return nil
}

func validateUndispatchedAction(action ExternalAction) error {
	if action.HTTPStatus != 0 || len(action.Response) != 0 {
		return ErrCorrupt
	}
	switch action.State {
	case "candidate":
		if !action.PendingUntil.IsZero() {
			return ErrCorrupt
		}
	case "pending":
		if !action.PendingUntil.After(action.UpdatedAt) {
			return ErrCorrupt
		}
	}
	return nil
}

func validateReconciledAction(action ExternalAction) error {
	if len(action.Response) != 0 || !slices.Contains([]string{"applied", "not_applied", "other"}, action.ReportedOutcome) || !validMemoryText(action.Observation, 2048, true) {
		return ErrCorrupt
	}
	return nil
}

func validateActions(db *database) error {
	if len(db.Actions) > 512 {
		return fmt.Errorf("external action receipt limit reached")
	}
	for id, action := range db.Actions {
		if action.ID != id || action.validate() != nil {
			return ErrCorrupt
		}
	}
	return nil
}

func cloneExternalAction(action ExternalAction) ExternalAction {
	action.Request, action.Arguments, action.Response = slices.Clone(action.Request), slices.Clone(action.Arguments), slices.Clone(action.Response)
	return action
}

func actionGuard(ctx context.Context, authorize func() error) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if authorize == nil {
		return ErrActionConflict
	}
	return authorize()
}

func uncertainOperation(db *database, key, except string) bool {
	for id, action := range db.Actions {
		if id != except && action.OperationKey == key && (action.State == "pending" || action.State == "unknown") {
			return true
		}
	}
	return false
}

func (s *Store) CreateExternalAction(ctx context.Context, draft ExternalAction, authorize func() error) (*ExternalAction, error) {
	now := time.Now().UTC()
	draft.ID, draft.Version, draft.State = rand.Text(), 1, "candidate"
	draft.CreatedAt, draft.UpdatedAt = now, now
	draft.RequestHash = artifactHash(draft.Request)
	draft = cloneExternalAction(draft)
	if err := draft.validate(); err != nil {
		return nil, err
	}
	err := s.mutate(ctx, func(db *database) error {
		if err := actionGuard(ctx, authorize); err != nil {
			return err
		}
		if uncertainOperation(db, draft.OperationKey, "") {
			return ErrActionUncertain
		}
		if db.Actions == nil {
			db.Actions = map[string]ExternalAction{}
		}
		db.Actions[draft.ID] = draft
		return nil
	})
	return &draft, err
}

func (s *Store) ExternalActions(ctx context.Context, projectID string) ([]ExternalAction, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	db, err := s.load()
	if err != nil {
		return nil, err
	}
	result := []ExternalAction{}
	for _, action := range db.Actions {
		if action.ProjectID == projectID {
			result = append(result, cloneExternalAction(action))
		}
	}
	slices.SortFunc(result, func(a, b ExternalAction) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return result, nil
}

func expectedAction(db *database, id string, version int64, hash string) (ExternalAction, error) {
	action, exists := db.Actions[id]
	if !exists {
		return action, ErrNotFound
	}
	if action.Version != version || action.RequestHash != hash {
		return action, ErrActionConflict
	}
	return action, nil
}

func (s *Store) ClaimExternalAction(ctx context.Context, id string, version int64, hash string, deadline time.Time, authorize func() error) (*ExternalAction, error) {
	var result ExternalAction
	err := s.mutate(ctx, func(db *database) error {
		if err := actionGuard(ctx, authorize); err != nil {
			return err
		}
		action, err := expectedAction(db, id, version, hash)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if action.State != "candidate" || !action.ExpiresAt.After(now) || !deadline.After(now) || deadline.After(now.Add(75*time.Second)) {
			return ErrActionConflict
		}
		if uncertainOperation(db, action.OperationKey, id) {
			return ErrActionUncertain
		}
		action.State, action.Version, action.UpdatedAt, action.PendingUntil = "pending", action.Version+1, now, deadline
		db.Actions[id], result = action, cloneExternalAction(action)
		return nil
	})
	return &result, err
}

// FinishExternalAction is independent of current project/source authority: a
// receipt must survive cancellation or a checkout switch after an external effect.
func (s *Store) FinishExternalAction(ctx context.Context, id string, version int64, state string, status int, response json.RawMessage) (*ExternalAction, error) {
	if !slices.Contains([]string{"completed", "not_sent", "unknown"}, state) {
		return nil, ErrActionConflict
	}
	var result ExternalAction
	err := s.mutate(ctx, func(db *database) error {
		action, exists := db.Actions[id]
		if !exists || action.Version != version || action.State != "pending" {
			return ErrActionConflict
		}
		action.State, action.HTTPStatus = state, status
		action.Response = slices.Clone(response)
		if len(response) > 0 {
			action.ResponseHash = artifactHash(response)
		}
		action.Version++
		action.UpdatedAt = time.Now().UTC()
		db.Actions[id], result = action, cloneExternalAction(action)
		return nil
	})
	return &result, err
}

func (s *Store) ReconcileExternalAction(ctx context.Context, id string, version int64, hash, outcome, observation string, authorize func() error) (*ExternalAction, error) {
	var result ExternalAction
	err := s.mutate(ctx, func(db *database) error {
		if err := actionGuard(ctx, authorize); err != nil {
			return err
		}
		action, err := expectedAction(db, id, version, hash)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if action.State != "unknown" && !(action.State == "pending" && now.After(action.PendingUntil)) {
			return ErrActionConflict
		}
		action.State, action.ReportedOutcome, action.Observation = "reconciled", outcome, observation
		action.Version++
		action.UpdatedAt = now
		db.Actions[id], result = action, cloneExternalAction(action)
		return nil
	})
	return &result, err
}
