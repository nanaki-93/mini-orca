package app

import (
	"fmt"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// scopedRuntimes owns the immutable runtime selected for each fixed workflow
// scope. Runtime profiles are established during service construction.
type scopedRuntimes struct {
	analyze  modelRuntime
	bug      modelRuntime
	function modelRuntime
}

func (r scopedRuntimes) forScope(scope string) (modelRuntime, bool) {
	switch scope {
	case r.analyze.effective.Scope:
		return r.analyze, true
	case r.bug.effective.Scope:
		return r.bug, true
	case r.function.effective.Scope:
		return r.function, true
	default:
		return modelRuntime{}, false
	}
}

// draftStore owns all mutable declaration drafts and their check evidence.
// Callers hold its mutex only while reading or changing in-memory records.
type draftStore struct {
	mu      sync.Mutex
	records map[string]*storedDraft
}

func newDraftStore() *draftStore {
	return &draftStore{records: make(map[string]*storedDraft)}
}

func (s *draftStore) snapshot(id string) (Draft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[id]
	if stored == nil {
		return Draft{}, false
	}
	return cloneDraft(stored.draft), true
}

func (s *draftStore) markStale(id string) (Draft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[id]
	if stored == nil {
		return Draft{}, false
	}
	staleDraft(stored)
	return cloneDraft(stored.draft), true
}

func (s *draftStore) create(draft Draft) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[draft.ID]; exists {
		return nil, fmt.Errorf("draft already exists")
	}
	if draft.ParentDraftID != "" {
		if _, exists := s.records[draft.ParentDraftID]; !exists {
			return nil, fmt.Errorf("parent draft not found")
		}
	}
	s.records[draft.ID] = &storedDraft{draft: cloneDraft(draft)}
	published := cloneDraft(draft)
	return &published, nil
}

func (s *draftStore) update(request DraftUpdateRequest) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[request.ID]
	if stored == nil {
		return nil, fmt.Errorf("draft not found")
	}
	if stored.draft.State == DraftStale || request.ExpectedRevision != stored.draft.Revision {
		return nil, project.ErrRevisionConflict
	}
	stored.draft.PreviousHash = stored.draft.Hash
	stored.draft.Declaration = request.Declaration
	stored.draft.Imports = append([]string(nil), request.Imports...)
	stored.draft.Revision++
	stored.draft.Hash = draftHash(stored.draft.Declaration, stored.draft.Imports)
	stored.draft.Validation = nil
	stored.draft.CompositionHash = ""
	stored.draft.State = DraftDirty
	stored.checks = nil
	published := cloneDraft(stored.draft)
	return &published, nil
}

func (s *draftStore) beginValidation(id string, expectedRevision int64) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[id]
	if stored == nil {
		return nil, fmt.Errorf("draft not found")
	}
	if stored.draft.State == DraftStale || stored.draft.Revision != expectedRevision {
		return nil, project.ErrRevisionConflict
	}
	stored.draft.State = DraftValidating
	stored.draft.Validation = nil
	stored.draft.CompositionHash = ""
	stored.checks = nil
	published := cloneDraft(stored.draft)
	return &published, nil
}

func (s *draftStore) completeValidation(id string, expectedRevision int64, validation project.DeclarationValidation, compositionHash string) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[id]
	if stored == nil {
		return nil, fmt.Errorf("draft not found")
	}
	if stored.draft.State == DraftStale || stored.draft.Revision != expectedRevision || stored.draft.State != DraftValidating {
		return nil, project.ErrRevisionConflict
	}
	copyValidation := cloneValidation(validation)
	stored.draft.Validation = &copyValidation
	if validation.Applicable {
		stored.draft.State = DraftValid
		stored.draft.CompositionHash = compositionHash
	} else {
		stored.draft.State = DraftInvalid
		stored.draft.CompositionHash = ""
	}
	published := cloneDraft(stored.draft)
	return &published, nil
}

func (s *draftStore) validated(identity draftRevisionIdentity) (Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[identity.id]
	if stored == nil {
		return Draft{}, fmt.Errorf("draft not found")
	}
	if stored.draft.State == DraftStale || !identity.matches(stored.draft) {
		return Draft{}, project.ErrRevisionConflict
	}
	if stored.draft.State != DraftValid || stored.draft.Validation == nil || !stored.draft.Validation.Applicable || stored.draft.CompositionHash == "" {
		return Draft{}, fmt.Errorf("draft is not valid; validate the latest revision")
	}
	return cloneDraft(stored.draft), nil
}

func (s *draftStore) storeCheckReport(identity draftRevisionIdentity, draft Draft, compositionHash string, report DraftCheckReport) (*DraftCheckReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[identity.id]
	if stored == nil || stored.draft.State != DraftValid || !identity.matches(stored.draft) || stored.draft.CompositionHash != compositionHash {
		return nil, project.ErrRevisionConflict
	}
	stored.checks = &draftCheckEvidence{Revision: draft.Revision, CompositionHash: compositionHash, Report: cloneCheckReport(report)}
	published := cloneCheckReport(report)
	return &published, nil
}

func (s *draftStore) applyState(identity applyIdentity) (Draft, DraftCheckReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[identity.draft.id]
	if stored == nil {
		return Draft{}, DraftCheckReport{}, fmt.Errorf("draft not found")
	}
	if stored.draft.State == DraftStale || !identity.matchesDraft(stored.draft) {
		return Draft{}, DraftCheckReport{}, project.ErrRevisionConflict
	}
	if stored.draft.State != DraftValid || stored.draft.Validation == nil || !stored.draft.Validation.Applicable || stored.draft.CompositionHash == "" {
		return Draft{}, DraftCheckReport{}, fmt.Errorf("draft is not valid; validate the latest revision")
	}
	if stored.checks == nil || stored.checks.Revision != stored.draft.Revision || stored.checks.CompositionHash != stored.draft.CompositionHash || !stored.checks.Report.Applicable {
		return Draft{}, DraftCheckReport{}, fmt.Errorf("focused draft checks have not passed for the latest draft revision")
	}
	return cloneDraft(stored.draft), cloneCheckReport(stored.checks.Report), nil
}

func (s *draftStore) hasFailedChecks(id string, taskSpec *project.BugTaskSpec) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[id]
	return stored != nil && sameTaskSpec(stored.draft.TaskSpec, taskSpec) && stored.checks != nil && stored.checks.Revision == stored.draft.Revision && stored.checks.CompositionHash == stored.draft.CompositionHash && checksFailed(stored.checks.Report)
}

func (s *draftStore) expireForOpenFile(projectID, revision, openPath, baseFileHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stored := range s.records {
		if stored.draft.ProjectID != projectID || stored.draft.ProjectRevision != revision || (openPath != "" && stored.draft.TargetPath != openPath) || (baseFileHash != "" && stored.draft.BaseFileHash != baseFileHash) {
			staleDraft(stored)
		}
	}
}

func (s *draftStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[string]*storedDraft)
}

// chatSessionStore owns in-memory conversations pinned to one declaration.
type chatSessionStore struct {
	mu      sync.Mutex
	records map[string]*chatSession
}

func newChatSessionStore() *chatSessionStore {
	return &chatSessionStore{records: make(map[string]*chatSession)}
}

func (s *chatSessionStore) snapshot(id string) (ChatSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[id]
	if stored == nil {
		return ChatSession{}, false
	}
	return cloneChatSession(stored.session), true
}

func (s *chatSessionStore) markStale(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if stored := s.records[id]; stored != nil {
		stored.session.State = "stale"
		stored.session.UpdatedAt = time.Now().UTC()
	}
}

func (s *chatSessionStore) create(session ChatSession) *ChatSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[session.ID] = &chatSession{session: cloneChatSession(session)}
	published := cloneChatSession(session)
	return &published
}

func (s *chatSessionStore) reserveRepair(id, parentDraftID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[id]
	if stored == nil || stored.session.State != "active" || stored.session.LatestDraftID != parentDraftID {
		return project.ErrRevisionConflict
	}
	if stored.session.RepairCount >= 3 {
		return fmt.Errorf("check-driven repair limit reached")
	}
	stored.session.RepairCount++
	stored.session.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *chatSessionStore) forMessage(id, parentDraftID string) (ChatSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[id]
	if stored == nil {
		return ChatSession{}, fmt.Errorf("chat session not found")
	}
	if stored.session.State != "active" || stored.session.LatestDraftID != parentDraftID {
		return ChatSession{}, project.ErrRevisionConflict
	}
	return cloneChatSession(stored.session), nil
}

func (s *chatSessionStore) appendProposal(id, parentDraftID, userMessage string, assistantMessage ChatSessionMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[id]
	if stored == nil || stored.session.State != "active" || stored.session.LatestDraftID != parentDraftID {
		return project.ErrRevisionConflict
	}
	now := time.Now().UTC()
	stored.session.Messages = append(stored.session.Messages, ChatSessionMessage{Role: "user", Content: userMessage, CreatedAt: now}, assistantMessage)
	stored.session.LatestDraftID = assistantMessage.DraftID
	stored.session.UpdatedAt = now
	return nil
}

func (s *chatSessionStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[string]*chatSession)
}
