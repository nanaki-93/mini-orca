package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

type DelegationRequest struct {
	ProjectID       string           `json:"project_id"`
	ProjectRevision string           `json:"project_revision"`
	Remote          IntegrationUse   `json:"remote"`
	Kind            string           `json:"kind"`
	Skill           string           `json:"skill"`
	Question        string           `json:"question"`
	Sources         []ExternalSource `json:"sources"`
	Stream          bool             `json:"stream"`
	ConfirmDelegate bool             `json:"confirm_delegate"`
}

type AgentDelegation struct {
	agentstate.Delegation
	Stale    bool `json:"stale"`
	Active   bool `json:"active"`
	Advisory bool `json:"advisory"`
}

func (s *Service) StartDelegation(ctx context.Context, request DelegationRequest) (*AgentDelegation, error) {
	if !request.ConfirmDelegate || !request.Remote.ConfirmSourceSharing {
		return nil, fmt.Errorf("explicit delegation and source-sharing confirmation are required")
	}
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	record, connection, err := s.remoteAdmission(ctx, query, request.Remote)
	if err != nil {
		return nil, err
	}
	if err := validateDelegationSelection(request, *record.view.Inventory); err != nil {
		return nil, err
	}
	captured, err := s.prepareExternalContext(ctx, query, request.Question, request.Sources, s.runtimes.function.effective.ContextMaxTokens)
	if err != nil {
		return nil, err
	}
	check := s.delegationAuthority(record, &captured)
	owned, finish, err := s.integrations.begin(ctx, request.Remote.ID)
	if err != nil {
		return nil, err
	}
	defer finish()
	store, err := agentstate.Open(record.root)
	if err != nil {
		return nil, err
	}
	draft := agentstate.Delegation{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, RemoteID: request.Remote.ID, EndpointHash: remoteDigest(connection.Endpoint), CardFingerprint: record.view.Inventory.Fingerprint, Skill: request.Skill, Kind: request.Kind, Question: request.Question, Policy: captured.policy, InputHash: remoteDigest(captured.prompt)}
	for _, source := range captured.sources {
		draft.Sources = append(draft.Sources, agentstate.MemorySource{Path: source.Path, Hash: source.Hash})
	}
	created, err := store.CreateDelegation(owned, draft, time.Now().UTC().Add(connection.Timeout+10*time.Second), func() error { return check(owned) })
	if err != nil {
		return nil, err
	}
	if err := s.claimRemoteExecution(created.ID); err != nil {
		return nil, err
	}
	defer s.finishRemoteExecution(created.ID)
	return s.executeDelegation(owned, store, record, connection, *created, captured.prompt, request.Stream, check)
}

func (s *Service) delegationAuthority(record remoteConnectionRecord, captured *externalContext) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := s.validateRemoteGrant(ctx, record); err != nil {
			return err
		}
		if captured != nil {
			return s.validateExternalContext(ctx, *captured)
		}
		return nil
	}
}

func (s *Service) executeDelegation(ctx context.Context, store *agentstate.Store, record remoteConnectionRecord, connection agentruntime.RemoteConnection, claimed agentstate.Delegation, prompt string, stream bool, check func(context.Context) error) (*AgentDelegation, error) {
	timed, stop := context.WithTimeout(ctx, connection.Timeout)
	defer stop()
	admitted, cancel, err := s.agentRunContextAt(timed, record.root)
	if err != nil {
		return s.finishDelegation(ctx, store, claimed, agentruntime.RemoteResult{Progress: claimed.Progress}, err)
	}
	defer cancel()
	request := agentruntime.RemoteRequest{DelegationID: claimed.ID, AttemptID: claimed.AttemptID, MessageID: claimed.MessageID, Operation: claimed.Operation, Kind: claimed.Kind, Skill: claimed.Skill, Prompt: prompt, Previous: claimed.Progress, Stream: stream}
	save := func(ctx context.Context, progress agentstate.RemoteProgress) error {
		receipt, release := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer release()
		_, err := store.SaveDelegationProgress(receipt, claimed.ID, claimed.AttemptID, progress, false)
		if err != nil {
			return fmt.Errorf("%w: preserve remote progress: %w", agentstate.ErrPersistence, err)
		}
		return nil
	}
	result, callErr := agentruntime.RunRemote(admitted, connection, *record.view.Inventory, request, check, save)
	if callErr == nil {
		callErr = check(admitted)
	}
	if (stream || claimed.Operation == "reconnect") && callErr == nil && (result.Progress.State == "working" || result.Progress.State == "submitted") {
		callErr = agentruntime.ErrRemoteTransport
	}
	return s.finishDelegation(ctx, store, claimed, result, callErr)
}

func (s *Service) finishDelegation(ctx context.Context, store *agentstate.Store, claimed agentstate.Delegation, result agentruntime.RemoteResult, callErr error) (*AgentDelegation, error) {
	progress := result.Progress
	if !agentstate.RemoteTerminal(progress.State) && callErr != nil {
		progress.State = remoteFailureState(claimed, result, callErr)
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	completed, err := store.SaveDelegationProgress(cleanup, claimed.ID, claimed.AttemptID, progress, true)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("%w: preserve delegation outcome", agentstate.ErrPersistence), err, callErr)
	}
	if errors.Is(callErr, agentstate.ErrPersistence) {
		return nil, callErr
	}
	view, err := s.delegationView(cleanup, *completed, true)
	view.Active = false
	return &view, err
}

func validateDelegationSelection(request DelegationRequest, inventory agentruntime.RemoteInventory) error {
	if request.Kind != "analysis" && request.Kind != "review" {
		return fmt.Errorf("delegation kind must be analysis or review")
	}
	if !slices.ContainsFunc(inventory.Skills, func(skill agentruntime.RemoteSkill) bool { return skill.ID == request.Skill }) {
		return fmt.Errorf("select an allowlisted advertised skill")
	}
	if request.Stream && !inventory.Streaming {
		return fmt.Errorf("this remote agent does not support streaming")
	}
	return nil
}

func remoteFailureState(claimed agentstate.Delegation, result agentruntime.RemoteResult, err error) string {
	if errors.Is(err, agentruntime.ErrRemoteAuth) {
		return "auth_required"
	}
	if !result.Attempted && claimed.Operation == "start" {
		return "not_sent"
	}
	return "unknown"
}

func (s *Service) delegationView(ctx context.Context, receipt agentstate.Delegation, detail bool) (AgentDelegation, error) {
	view := AgentDelegation{Delegation: receipt, Advisory: true}
	index, err := s.manager.Index()
	if err != nil {
		return view, err
	}
	view.Stale = index.ProjectID != receipt.ProjectID || index.ProjectRevision != receipt.ProjectRevision
	if detail && !view.Stale {
		captured := delegationContext(s.manager.Root(), receipt)
		if err := s.validateExternalContext(ctx, captured); err != nil {
			if !memorySourceChanged(err) {
				return view, err
			}
			view.Stale = true
		}
	}
	s.integrations.mu.Lock()
	view.Active = s.integrations.remoteExecuting[receipt.ID]
	s.integrations.mu.Unlock()
	if !detail {
		view.Progress.Text, view.Progress.Artifacts = "", nil
		view.Question, view.Observation = "", ""
	}
	return view, nil
}

func delegationContext(root string, receipt agentstate.Delegation) externalContext {
	captured := externalContext{root: root, policy: receipt.Policy, query: AgentRunQuery{ProjectID: receipt.ProjectID, ProjectRevision: receipt.ProjectRevision}}
	for _, source := range receipt.Sources {
		captured.sources = append(captured.sources, ExternalSource{Path: source.Path, Hash: source.Hash})
	}
	return captured
}

func (s *Service) ListDelegations(ctx context.Context, query AgentRunQuery) ([]AgentDelegation, error) {
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, err
	}
	records, err := store.Delegations(ctx, query.ProjectID)
	if err != nil {
		return nil, err
	}
	result := []AgentDelegation{}
	for _, receipt := range records {
		view, err := s.delegationView(ctx, receipt, false)
		if err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, s.integrationScope(ctx, query, root)
}

func (s *Service) DelegationDetail(ctx context.Context, query AgentRunQuery, id string) (*AgentDelegation, error) {
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, err
	}
	receipt, err := findDelegation(ctx, store, query.ProjectID, id)
	if err != nil {
		return nil, err
	}
	view, err := s.delegationView(ctx, receipt, true)
	if err != nil {
		return nil, err
	}
	return &view, s.integrationScope(ctx, query, root)
}

func findDelegation(ctx context.Context, store *agentstate.Store, projectID, id string) (agentstate.Delegation, error) {
	records, err := store.Delegations(ctx, projectID)
	if err != nil {
		return agentstate.Delegation{}, err
	}
	for _, receipt := range records {
		if receipt.ID == id {
			return receipt, nil
		}
	}
	return agentstate.Delegation{}, agentstate.ErrNotFound
}

func (s *Service) claimRemoteExecution(id string) error {
	s.integrations.mu.Lock()
	defer s.integrations.mu.Unlock()
	if s.integrations.remoteExecuting[id] {
		return agentstate.ErrDelegationConflict
	}
	s.integrations.remoteExecuting[id] = true
	return nil
}

func (s *Service) finishRemoteExecution(id string) {
	s.integrations.mu.Lock()
	delete(s.integrations.remoteExecuting, id)
	s.integrations.mu.Unlock()
}
