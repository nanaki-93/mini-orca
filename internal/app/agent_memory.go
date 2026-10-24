package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

var ErrMemoryNotFound = agentstate.ErrNotFound
var ErrMemoryConflict = agentstate.ErrMemoryConflict
var ErrMemoryUnavailable = errors.New("curated memory unavailable")

// AgentMemory contains private advisory text only on explicit detail/curation
// responses. List responses omit Text; neither operation dispatches a model.
type AgentMemory struct {
	agentstate.MemoryRecord
	Stale        bool   `json:"stale"`
	Expired      bool   `json:"expired"`
	SourceStatus string `json:"source_status"`
}

type MemoryDraft struct {
	Kind          string                    `json:"kind"`
	Title         string                    `json:"title"`
	Text          string                    `json:"text"`
	Sources       []agentstate.MemorySource `json:"sources"`
	RetentionDays int                       `json:"retention_days"`
	SessionID     string                    `json:"session_id,omitempty"`
	Identity      *ChangeIdentity           `json:"identity,omitempty"`
	LessonIndex   *int                      `json:"lesson_index,omitempty"`
}

type MemoryMutation struct {
	ProjectID       string       `json:"project_id"`
	ProjectRevision string       `json:"project_revision"`
	Action          string       `json:"action"`
	Version         int64        `json:"version"`
	Draft           *MemoryDraft `json:"draft,omitempty"`
}

func memoryStore(root string) (*agentstate.Store, error) {
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, memoryStorageError(err)
	}
	return store, nil
}

func memoryStorageError(err error) error {
	return fmt.Errorf("%w: %w: %w", ErrMemoryUnavailable, agentstate.ErrPersistence, err)
}

func readMemories(ctx context.Context, store *agentstate.Store, projectID string) ([]agentstate.MemoryRecord, error) {
	records, err := store.Memories(ctx, projectID)
	if err != nil && context.Cause(ctx) == nil {
		return nil, memoryStorageError(err)
	}
	return records, err
}

func (s *Service) ListMemories(ctx context.Context, query AgentRunQuery) ([]AgentMemory, error) {
	root := s.manager.Root()
	if err := s.agentHistoryScope(query, root); err != nil {
		return nil, err
	}
	store, err := memoryStore(root)
	if err != nil {
		return nil, err
	}
	records, err := readMemories(ctx, store, query.ProjectID)
	if err != nil {
		return nil, err
	}
	result := make([]AgentMemory, 0, len(records))
	for _, record := range records {
		record.Text = ""
		result = append(result, memoryView(record, query.ProjectRevision))
	}
	return result, s.agentHistoryScope(query, root)
}

func memoryView(record agentstate.MemoryRecord, revision string) AgentMemory {
	return AgentMemory{MemoryRecord: record, Stale: record.ProjectRevision != revision, Expired: !record.ExpiresAt.After(time.Now()), SourceStatus: "unchecked"}
}

func findMemory(ctx context.Context, store *agentstate.Store, projectID, id string) (agentstate.MemoryRecord, error) {
	records, err := readMemories(ctx, store, projectID)
	if err != nil {
		return agentstate.MemoryRecord{}, err
	}
	for _, record := range records {
		if record.ID == id {
			return record, nil
		}
	}
	return agentstate.MemoryRecord{}, ErrMemoryNotFound
}

func (s *Service) MemoryDetail(ctx context.Context, query AgentRunQuery, id string) (*AgentMemory, error) {
	root := s.manager.Root()
	if err := s.agentHistoryScope(query, root); err != nil {
		return nil, err
	}
	store, err := memoryStore(root)
	if err != nil {
		return nil, err
	}
	record, err := findMemory(ctx, store, query.ProjectID, id)
	if err != nil {
		return nil, err
	}
	result := memoryView(record, query.ProjectRevision)
	result.SourceStatus, err = memorySourceStatus(ctx, root, record)
	if err != nil {
		return nil, err
	}
	result.Stale = result.Stale || result.SourceStatus == "changed"
	return &result, s.agentHistoryScope(query, root)
}

func memorySourceStatus(ctx context.Context, root string, record agentstate.MemoryRecord) (string, error) {
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return "", err
	}
	if policy.Version() != record.Policy {
		return "changed", nil
	}
	for _, source := range record.Sources {
		if err := validateMemorySource(ctx, policy, source); err != nil {
			if memorySourceChanged(err) {
				return "changed", nil
			}
			return "", err
		}
	}
	return "current", nil
}

// MutateMemory is an explicit local user action. It never edits project source,
// executes checks, or promotes a conversation/model answer automatically.
func (s *Service) MutateMemory(ctx context.Context, id string, request MemoryMutation) (*AgentMemory, error) {
	root := s.manager.Root()
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	authorize := func() error {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		return s.agentHistoryScope(query, root)
	}
	if err := authorize(); err != nil {
		return nil, err
	}
	if err := validateMemoryMutation(id, request); err != nil {
		return nil, err
	}
	store, err := memoryStore(root)
	if err != nil {
		return nil, err
	}
	if request.Action == "forget" {
		err := store.ForgetMemory(ctx, request.ProjectID, id, request.Version, authorize)
		return nil, memoryMutationError(err)
	}
	var record agentstate.MemoryRecord
	if request.Action == "accept" {
		record, err = findMemory(ctx, store, request.ProjectID, id)
	} else {
		record, err = s.prepareMemoryDraft(ctx, root, id, request)
	}
	if err != nil {
		return nil, err
	}
	// Match the workflow's lock order: proposal authority before journal writes.
	// The store rechecks guard under its file lock, so guard must not acquire it.
	if record.Provenance.Origin != "user" {
		s.changesMu.Lock()
		defer s.changesMu.Unlock()
	}
	guard := func() error {
		if err := authorize(); err != nil {
			return err
		}
		return s.validateMemoryCuration(ctx, root, query, record)
	}
	if request.Action == "accept" {
		record, err = store.AcceptMemory(ctx, request.ProjectID, id, request.Version, guard)
	} else {
		record, err = store.SaveMemory(ctx, record, request.Version, guard)
	}
	if err != nil {
		return nil, memoryMutationError(err)
	}
	result := memoryView(record, request.ProjectRevision)
	result.SourceStatus = "current"
	return &result, nil
}

func memoryMutationError(err error) error {
	if errors.Is(err, agentstate.ErrPersistence) || errors.Is(err, agentstate.ErrCorrupt) {
		return memoryStorageError(err)
	}
	return err
}

func validateMemoryMutation(id string, request MemoryMutation) error {
	if !slices.Contains([]string{"create", "edit", "accept", "forget"}, request.Action) {
		return fmt.Errorf("unknown memory action")
	}
	if request.Action == "create" {
		if id != "" || request.Version != 0 {
			return fmt.Errorf("create requires no memory identity")
		}
	} else if id == "" || request.Version < 1 {
		return fmt.Errorf("current memory identity and version are required")
	}
	needsDraft := request.Action == "create" || request.Action == "edit"
	if needsDraft != (request.Draft != nil) {
		return fmt.Errorf("create/edit require a draft; accept/forget require only identity")
	}
	return nil
}

func (s *Service) prepareMemoryDraft(ctx context.Context, root, id string, request MemoryMutation) (agentstate.MemoryRecord, error) {
	draft := *request.Draft
	if draft.RetentionDays == 0 {
		draft.RetentionDays = 30
	}
	if draft.RetentionDays < 1 || draft.RetentionDays > 365 {
		return agentstate.MemoryRecord{}, fmt.Errorf("memory retention must be 1–365 days")
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return agentstate.MemoryRecord{}, err
	}
	if id == "" {
		id = rand.Text()
	}
	record := agentstate.MemoryRecord{ID: id, ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Policy: policy.Version(), Kind: draft.Kind, Title: draft.Title, Text: draft.Text, Sources: slices.Clone(draft.Sources), ExpiresAt: time.Now().UTC().Add(time.Duration(draft.RetentionDays) * 24 * time.Hour), Provenance: agentstate.MemoryProvenance{Origin: "user"}}
	if draft.SessionID != "" {
		err = s.retainProposalMemory(ctx, root, &record, draft)
	} else if draft.Identity != nil || draft.LessonIndex != nil || draft.Kind == "verified_outcome" {
		err = fmt.Errorf("proposal identity is required for retained lessons and verified outcomes")
	}
	return record, err
}

func (s *Service) retainProposalMemory(ctx context.Context, root string, record *agentstate.MemoryRecord, draft MemoryDraft) error {
	if draft.Identity == nil || draft.Text != "" {
		return fmt.Errorf("proposal memory requires exact identity and derives text from its receipt")
	}
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, capturedRoot, err := s.loadChangeForAction(ctx, draft.SessionID, *draft.Identity)
	if err != nil {
		return err
	}
	if root != capturedRoot {
		return project.ErrRevisionConflict
	}
	record.Provenance = agentstate.MemoryProvenance{SessionID: session.ID, ProposalHash: session.Hash}
	switch draft.Kind {
	case "verified_outcome":
		if draft.LessonIndex != nil || !s.hasVerifiedMemoryChecks(session) {
			return fmt.Errorf("verified memory requires current passing proposal check receipts")
		}
		record.Provenance.Origin = "proposal_checks"
		record.Text = proposalMemoryText(session)
	case "lesson":
		if session.Workflow == nil || draft.LessonIndex == nil || *draft.LessonIndex < 0 || *draft.LessonIndex >= len(session.Workflow.HarnessLessons) {
			return fmt.Errorf("select a retained harness lesson")
		}
		record.Provenance.Origin, record.Provenance.WorkflowID = "harness_lesson", session.Workflow.ID
		record.Text = session.Workflow.HarnessLessons[*draft.LessonIndex]
	default:
		return fmt.Errorf("proposal memory must be a verified outcome or retained lesson")
	}
	return validateProposalMemorySources(record, session)
}

func proposalMemoryText(session *ChangeSession) string {
	var text strings.Builder
	text.WriteString("The isolated proposal checks passed for proposal " + session.Hash + ". This receipt describes that proposal, not current source behavior.")
	for _, check := range session.Checks {
		if check.State == "passed" {
			text.WriteString("\nPassed: " + check.Name)
		}
	}
	return text.String()
}

func (s *Service) hasVerifiedMemoryChecks(session *ChangeSession) bool {
	return s.hasLivePassingChecks(session) && slices.ContainsFunc(session.Checks, func(check CheckResult) bool { return check.Required && check.State == CheckPassed })
}

func validateProposalMemorySources(record *agentstate.MemoryRecord, session *ChangeSession) error {
	for _, source := range record.Sources {
		if !slices.ContainsFunc(session.Targets, func(target ChangeTarget) bool { return target.Path == source.Path && target.Hash == source.Hash }) {
			return fmt.Errorf("proposal memory must cite a captured proposal source")
		}
	}
	return nil
}

func (s *Service) validateMemoryCuration(ctx context.Context, root string, query AgentRunQuery, record agentstate.MemoryRecord) error {
	if record.ProjectID != query.ProjectID || record.ProjectRevision != query.ProjectRevision {
		return project.ErrRevisionConflict
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return err
	}
	if policy.Version() != record.Policy {
		return project.ErrRevisionConflict
	}
	if len(record.Sources) == 0 || len(record.Sources) > agentstate.MaxMemorySources {
		return fmt.Errorf("memory requires 1–64 source scopes")
	}
	for _, source := range record.Sources {
		if err := validateMemorySource(ctx, policy, source); err != nil {
			return err
		}
	}
	if record.Provenance.Origin == "user" {
		return nil
	}
	return s.validateProposalMemory(ctx, root, record)
}

func validateMemorySource(ctx context.Context, policy *project.ContextPolicy, source agentstate.MemorySource) error {
	if err := source.Validate(); err != nil {
		return err
	}
	data, err := policy.ReadSource(ctx, source.Path, source.Hash)
	if err != nil {
		return err
	}
	if source.EndLine > strings.Count(string(data), "\n")+1 {
		return fmt.Errorf("memory span exceeds source")
	}
	return nil
}

func (s *Service) validateProposalMemory(ctx context.Context, root string, record agentstate.MemoryRecord) error {
	// The curation owner holds changesMu across validation and publication.
	session, err := loadChangeSession(root, record.Provenance.SessionID)
	if err != nil {
		return err
	}
	if session.Hash != record.Provenance.ProposalHash {
		return project.ErrRevisionConflict
	}
	if err := s.verifyChangeCurrent(ctx, root, session); err != nil {
		return err
	}
	if record.Provenance.Origin == "proposal_checks" {
		if !s.hasVerifiedMemoryChecks(session) || record.Text != proposalMemoryText(session) {
			return fmt.Errorf("proposal check authority is no longer current")
		}
	} else if session.Workflow == nil || session.Workflow.ID != record.Provenance.WorkflowID || !slices.Contains(session.Workflow.HarnessLessons, record.Text) {
		return fmt.Errorf("retained lesson no longer matches its workflow")
	}
	return validateProposalMemorySources(&record, session)
}
