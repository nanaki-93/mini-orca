package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

var ErrAgentRunNotFound = errors.New("agent invocation not found")
var ErrAgentHistoryUnavailable = errors.New("agent history is unavailable")

func (s *Service) agentHistoryScope(query AgentRunQuery, root string) error {
	index, err := s.manager.Index()
	if err != nil {
		return err
	}
	if query.ProjectID == "" || query.ProjectRevision == "" || index.ProjectID != query.ProjectID || index.ProjectRevision != query.ProjectRevision || s.manager.Root() != root {
		return project.ErrRevisionConflict
	}
	return nil
}

func (s *Service) readAgentHistory(ctx context.Context, query AgentRunQuery) (agentstate.History, *agentstate.Store, error) {
	root := s.manager.Root()
	if err := s.agentHistoryScope(query, root); err != nil {
		return agentstate.History{}, nil, err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return agentstate.History{}, nil, fmt.Errorf("%w: %w", ErrAgentHistoryUnavailable, err)
	}
	history, err := store.History(ctx)
	if err != nil && context.Cause(ctx) == nil {
		return history, store, fmt.Errorf("%w: %w", ErrAgentHistoryUnavailable, err)
	}
	if err == nil {
		err = s.agentHistoryScope(query, root)
	}
	return history, store, err
}

func (s *Service) ListAgentRuns(ctx context.Context, query AgentRunQuery) (*AgentRunList, error) {
	history, _, err := s.readAgentHistory(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.listAgentHistory(history, query)
}

type agentListCursor struct {
	Project, Revision, ID string
	Started               time.Time
}

func decodeAgentListCursor(query AgentRunQuery) (agentListCursor, error) {
	var cursor agentListCursor
	if query.Cursor == "" {
		return cursor, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(query.Cursor)
	if err != nil || len(data) > 1024 {
		return cursor, fmt.Errorf("invalid run history cursor")
	}
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.Project != query.ProjectID || cursor.Revision != query.ProjectRevision || cursor.ID == "" || cursor.Started.IsZero() {
		return cursor, fmt.Errorf("run history cursor belongs to another snapshot")
	}
	return cursor, nil
}

func (s *Service) listAgentHistory(history agentstate.History, query AgentRunQuery) (*AgentRunList, error) {
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 {
		return nil, fmt.Errorf("run history limit must be between 1 and 100")
	}
	cursor, err := decodeAgentListCursor(query)
	if err != nil {
		return nil, err
	}
	result := &AgentRunList{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Runs: []AgentRun{}}
	parents := agentParents(history)
	for _, run := range history.Runs {
		if run.ParentID != "" || run.Binding.Project != query.ProjectID || beforeAgentCursor(run, cursor) {
			continue
		}
		if len(result.Runs) == query.Limit {
			last := result.Runs[len(result.Runs)-1]
			data, err := json.Marshal(agentListCursor{Project: query.ProjectID, Revision: query.ProjectRevision, ID: last.ID, Started: last.StartedAt})
			if err != nil {
				return nil, err
			}
			result.NextCursor = base64.RawURLEncoding.EncodeToString(data)
			break
		}
		value := s.publicAgentRun(run, query, parents)
		summarizeAgentInput(&value, history, parents)
		result.Runs = append(result.Runs, value)
	}
	return result, nil
}

func beforeAgentCursor(run agentstate.RunSnapshot, cursor agentListCursor) bool {
	return cursor.ID != "" && (run.Started.After(cursor.Started) || run.Started.Equal(cursor.Started) && run.ID <= cursor.ID)
}

func agentParents(history agentstate.History) map[string]string {
	parents := make(map[string]string, len(history.Runs))
	for _, run := range history.Runs {
		parents[run.ID] = run.ParentID
	}
	return parents
}

func agentRoot(id string, parents map[string]string) string {
	for parents[id] != "" {
		id = parents[id]
	} // The store validates bounded, acyclic ancestry.
	return id
}

func (s *Service) publicAgentRun(run agentstate.RunSnapshot, query AgentRunQuery, parents map[string]string) AgentRun {
	active, canceling := s.agentRuns.state(run.ID)
	active = active && run.Status == "running"
	canceling = canceling && active
	value := AgentRun{ID: run.ID, RootID: agentRoot(run.ID, parents), ParentID: run.ParentID, ProjectID: run.Binding.Project, ProjectRevision: run.Binding.Revision, Kind: strings.TrimSuffix(run.SchemaVersion, "-v1"), Status: run.Status, Active: active, Stale: run.Binding.Revision != query.ProjectRevision, StartedAt: run.Started, UpdatedAt: run.Updated, Usage: run.Usage, Limits: run.Limits, Input: run.Input}
	value.ChangeSessionID, value.WorkflowID = run.ChangeSessionID, run.WorkflowID
	if run.Status == "running" && !active {
		value.Status = "interrupted"
	}
	if canceling {
		value.Status = "canceling"
	}
	value.CanCancel = active && !value.Stale && !canceling && run.ParentID == ""
	return value
}

func summarizeAgentInput(value *AgentRun, history agentstate.History, parents map[string]string) {
	if value.ParentID != "" || value.Status != "completed" {
		return
	}
	for _, child := range history.Runs {
		if child.Input == nil || agentRoot(child.ID, parents) != value.ID {
			continue
		}
		value.Input = child.Input
		if child.Updated.After(value.UpdatedAt) {
			value.UpdatedAt = child.Updated
		}
		switch child.Status {
		case "waiting_input", "canceled", "expired":
			value.Status = child.Status
		case "failed":
			value.Status = "changes_requested"
		}
		return
	}
}

func findAgentRun(history agentstate.History, projectID, id string) (agentstate.RunSnapshot, error) {
	for _, run := range history.Runs {
		if run.ID == id && run.Binding.Project == projectID {
			return run, nil
		}
	}
	return agentstate.RunSnapshot{}, ErrAgentRunNotFound
}

func (s *Service) AgentRunDetail(ctx context.Context, query AgentRunQuery, id string, includeText bool) (*AgentRunDetail, error) {
	root := s.manager.Root()
	history, store, err := s.readAgentHistory(ctx, query)
	if err != nil {
		return nil, err
	}
	run, err := findAgentRun(history, query.ProjectID, id)
	if err != nil {
		return nil, err
	}
	parents := agentParents(history)
	result := &AgentRunDetail{Run: s.publicAgentRun(run, query, parents), Invocations: []AgentRun{}, Events: []AgentRunEvent{}}
	summarizeAgentInput(&result.Run, history, parents)
	for _, child := range history.Runs {
		if agentRoot(child.ID, parents) != result.Run.RootID {
			continue
		}
		if len(result.Invocations) == 256 {
			result.HasMore = true
			break
		}
		result.Invocations = append(result.Invocations, s.publicAgentRun(child, query, parents))
	}
	for _, event := range history.Events {
		if event.RootID == result.Run.RootID {
			result.Events = append(result.Events, publicProgress(event))
		}
	}
	if len(result.Events) > 256 {
		result.Events = result.Events[len(result.Events)-256:]
		result.EventsTruncated = true
	}
	if len(history.Events) == agentstate.MaxProgressEvents {
		result.EventsTruncated = true
	}
	if run.Input != nil {
		result.Input, _, err = s.agentRunInput(ctx, store, run, root)
	}
	if err == nil && includeText {
		result.Text, err = s.readAgentPrivate(ctx, store, run)
	}
	if err == nil {
		err = s.agentHistoryScope(query, root)
	}
	return result, err
}

func (s *Service) readAgentText(ctx context.Context, store *agentstate.Store, run agentstate.RunSnapshot) (*AgentPrivateText, error) {
	if run.SchemaVersion == "grounding-v1" {
		return &AgentPrivateText{State: "research_report_required"}, nil
	}
	value := s.agentRuns.privateText(run.ID)
	artifact, err := store.ExportArtifact(ctx, agentstate.ArtifactKey{Session: run.Session, Name: "completion.json"}, 0)
	if errors.Is(err, agentstate.ErrNotFound) {
		return &value, nil
	}
	if err != nil {
		return nil, err
	}
	var output struct{ Content string }
	if err := json.Unmarshal(artifact.Data, &output); err != nil {
		return nil, fmt.Errorf("decode saved model output: %w", err)
	}
	value = AgentPrivateText{Available: true, State: "final_model_output", Artifact: &agentstate.ArtifactReference{Session: run.Session, Name: "completion.json", Version: artifact.Version}}
	value.Text, value.Truncated = boundAgentText(output.Content)
	return &value, nil
}

func (s *Service) agentRunInput(ctx context.Context, store *agentstate.Store, run agentstate.RunSnapshot, root string) (*ChangeInput, string, error) {
	if run.Input == nil {
		return nil, "", fmt.Errorf("invocation has no human input")
	}
	record, err := store.Session(ctx, run.Session)
	if err != nil {
		return nil, "", err
	}
	var input agentruntime.ConfirmationInput
	if err := json.Unmarshal(record.State["mini_orca.confirmation"], &input); err != nil {
		return nil, "", fmt.Errorf("read confirmation identity: %w", err)
	}
	session, err := loadChangeSession(root, input.Snapshot.SessionID)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	if session.Workflow == nil || session.Workflow.Input == nil || session.Workflow.Input.RunID != run.ID || session.Workflow.Input.RequestID != run.Input.RequestID {
		return nil, "", nil // The proposal was superseded; history grants no input control.
	}
	return session.Workflow.Input, session.ID, nil
}

func (s *Service) ControlAgentRun(ctx context.Context, id string, request AgentRunControl) (*AgentRunDetail, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	if request.Action != "cancel" {
		return nil, fmt.Errorf("only cancel is supported; start work through an admitted domain action")
	}
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	detail, err := s.AgentRunDetail(ctx, query, id, false)
	if err != nil {
		return nil, err
	}
	if !detail.Run.CanCancel {
		return nil, fmt.Errorf("%w: run is not an active current root", project.ErrRevisionConflict)
	}
	if err := s.agentRuns.cancel(id); err != nil {
		return nil, err
	}
	s.notifyEvent("agent_run")
	return s.AgentRunDetail(ctx, query, id, false)
}

func (s *Service) RespondAgentRunInput(ctx context.Context, id string, request ChangeInputResponse) (*ChangeSession, error) {
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	root := s.manager.Root()
	history, store, err := s.readAgentHistory(ctx, query)
	if err != nil {
		return nil, err
	}
	run, err := findAgentRun(history, query.ProjectID, id)
	if err != nil {
		return nil, err
	}
	input, sessionID, err := s.agentRunInput(ctx, store, run, root)
	if err != nil {
		return nil, err
	}
	if input == nil || input.ChangeIdentity != request.ChangeIdentity || input.RequestID != request.RequestID || input.WorkflowID != request.WorkflowID {
		return nil, project.ErrRevisionConflict
	}
	// The domain owner rechecks source, current project, proposal, checks and receipt.
	return s.RespondChangeInput(ctx, sessionID, request)
}
