package app

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"slices"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

// RemoteServiceRequest is the JSON text payload of the analysis/proposal skills.
// It contains selections and intent, never replacement source or permissions.
type RemoteServiceRequest struct {
	ProjectID             string           `json:"project_id"`
	ProjectRevision       string           `json:"project_revision"`
	Operation             string           `json:"operation"`
	Question              string           `json:"question"`
	Sources               []ExternalSource `json:"sources"`
	AcceptanceCriteria    []string         `json:"acceptance_criteria"`
	ConfirmRemoteProvider bool             `json:"confirm_remote_provider"`
}

func (s *Service) executeRemoteService(ctx context.Context, grant remoteServiceGrant, text string) (string, error) {
	request, err := decodeRemoteServiceRequest(text, grant)
	if err != nil {
		return "", err
	}
	scoped, finish, err := s.integrations.begin(ctx, "@a2a_server:"+grant.view.Config.ID)
	if err != nil {
		return "", err
	}
	defer finish()
	if err := s.validateRemoteServiceGrant(scoped, grant); err != nil {
		return "", err
	}
	scope := config.FunctionModelScope
	if request.Operation == "proposal" {
		scope = config.BugModelScope
	}
	if err := s.RequireRemoteConfirmation(scope, request.ConfirmRemoteProvider); err != nil {
		return "", err
	}
	runtime, _ := s.runtimes.forScope(string(scope))
	captured, err := s.prepareExternalContext(scoped, grant.query, request.Question, request.Sources, runtime.effective.ContextMaxTokens)
	if err != nil {
		return "", err
	}
	result, err := s.runRemoteServiceWork(scoped, request, captured)
	if err != nil {
		return "", err
	}
	if err := s.validateRemoteServiceGrant(scoped, grant); err != nil {
		return "", err
	}
	if err := s.validateExternalContext(scoped, captured); err != nil {
		return "", err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode remote work receipt: %w", err)
	}
	return string(data), nil
}

func decodeRemoteServiceRequest(text string, grant remoteServiceGrant) (RemoteServiceRequest, error) {
	var request RemoteServiceRequest
	if len(text) > 8<<10 || !jsontext.Value(text).IsValid() {
		return request, fmt.Errorf("invalid remote work request")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, fmt.Errorf("decode remote work request: %w", err)
	}
	if request.ProjectID != grant.query.ProjectID || request.ProjectRevision != grant.query.ProjectRevision || !slices.Contains(grant.view.Config.Capabilities, request.Operation) {
		return request, ErrIntegrationConflict
	}
	if len(request.Sources) == 0 {
		return request, fmt.Errorf("select one to eight current source hashes")
	}
	if err := validateExternalQuestion(request.Question, request.Sources); err != nil {
		return request, err
	}
	if err := validateChangeCriteria(request.AcceptanceCriteria); err != nil {
		return request, err
	}
	return request, nil
}

func (s *Service) runRemoteServiceWork(ctx context.Context, request RemoteServiceRequest, captured externalContext) (any, error) {
	paths := make([]string, 0, len(request.Sources))
	sources := make([]PlanInvestigationSource, 0, len(request.Sources))
	for _, source := range request.Sources {
		paths = append(paths, source.Path)
		sources = append(sources, PlanInvestigationSource{Path: source.Path, Hash: source.Hash, Exists: true})
	}
	if request.Operation == "analysis" {
		result, err := s.InvestigatePlan(ctx, PlanInvestigationRequest{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision,
			Plan:  ChangePlanInput{Problem: request.Question, Approach: "Investigate the admitted source and report evidence-backed suggestions."},
			Paths: paths, Sources: sources, AcceptanceCriteria: request.AcceptanceCriteria, ConfirmRemoteProvider: request.ConfirmRemoteProvider})
		return struct {
			Advisory bool               `json:"advisory"`
			Analysis *PlanInvestigation `json:"analysis"`
		}{true, result}, err
	}
	draft, err := s.OpenChangeSession(ctx, ChangeCreateRequest{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision,
		Kind: "feature", Title: "Remote proposal for local review", Paths: paths, AcceptanceCriteria: request.AcceptanceCriteria})
	if err != nil {
		return nil, err
	}
	if err := s.validateExternalContext(ctx, captured); err != nil {
		return nil, err
	}
	proposal, err := s.SendChangeMessage(ctx, draft.ID, ChangeMessageRequest{ChangeIdentity: ChangeIdentity{ProjectID: draft.ProjectID, ProjectRevision: draft.ProjectRevision, Revision: draft.Revision, Hash: draft.Hash},
		Profile: "bug", Message: request.Question, ConfirmRemoteProvider: request.ConfirmRemoteProvider})
	if err != nil {
		return nil, err
	}
	return struct {
		Advisory   bool   `json:"advisory"`
		ProposalID string `json:"proposal_id"`
		ChangeIdentity
		State      string `json:"state"`
		NextAction string `json:"next_action"`
	}{true, proposal.ID, ChangeIdentity{ProjectID: proposal.ProjectID, ProjectRevision: proposal.ProjectRevision, Revision: proposal.Revision, Hash: proposal.Hash}, proposal.State, "Open the proposal locally, run trusted checks and explicitly review before Apply."}, nil
}
