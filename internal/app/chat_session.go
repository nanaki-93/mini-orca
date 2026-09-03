package app

import (
	"context"
	"encoding/json"
	"fmt"
	"go/token"
	"io"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const chatSessionResponseVersion = "v1"

// ChatSession is an in-memory conversation pinned to one Go declaration in
// one open file. It never carries a complete source file.
type ChatSession struct {
	ID              string                      `json:"id"`
	ProjectID       string                      `json:"project_id"`
	ProjectRevision string                      `json:"project_revision"`
	BaseFileHash    string                      `json:"base_file_hash"`
	OpenPath        string                      `json:"open_path"`
	Mode            project.DeclarationEditMode `json:"mode"`
	TargetSymbol    string                      `json:"target_symbol"`
	TaskSpec        *project.BugTaskSpec        `json:"task_spec,omitempty"`
	RepairCount     int                         `json:"repair_count,omitempty"`
	State           string                      `json:"state"`
	LatestDraftID   string                      `json:"latest_draft_id,omitempty"`
	Messages        []ChatSessionMessage        `json:"messages"`
	CreatedAt       time.Time                   `json:"created_at"`
	UpdatedAt       time.Time                   `json:"updated_at"`
}

// ChatSessionMessage is visible only in its file-scoped session.
type ChatSessionMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	DraftID   string    `json:"draft_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatSessionCreateRequest struct {
	ProjectID       string
	ProjectRevision string
	BaseFileHash    string
	OpenPath        string
	Mode            project.DeclarationEditMode
	TargetSymbol    string
	TaskSpec        *project.BugTaskSpec
}

type ChatSessionMessageRequest struct {
	SessionID             string
	Message               string
	ParentDraftID         string
	ConfirmRemoteProvider bool
	Repair                bool
}

// DeclarationDraftResponse is the only model contract accepted by a session.
// The absence of a target path and complete file prevents model-directed
// retargeting or writing.
type DeclarationDraftResponse struct {
	Version     string   `json:"version"`
	Declaration string   `json:"declaration"`
	Imports     []string `json:"imports,omitempty"`
	Explanation string   `json:"explanation"`
}

type ChatDraftProposal struct {
	SessionID        string                  `json:"session_id"`
	Draft            Draft                   `json:"draft"`
	AssistantMessage ChatSessionMessage      `json:"assistant_message"`
	ContextManifest  project.ContextManifest `json:"context_manifest"`
}

type chatSession struct {
	session ChatSession
}

// OpenChatSession creates a bounded session only after proving the current
// file, revision, target, and declaration mode are valid.
func (s *Service) OpenChatSession(request ChatSessionCreateRequest) (*ChatSession, error) {
	if err := s.ValidateMutableRequest(request.ProjectID, request.ProjectRevision, request.OpenPath, request.BaseFileHash); err != nil {
		return nil, err
	}
	file, err := s.manager.IndexedFile(request.OpenPath)
	if err != nil {
		return nil, err
	}
	if file.Language != "Go" || file.Binary {
		return nil, fmt.Errorf("file-scoped declaration chat supports Go source files only")
	}
	if err := validateChatTarget(*file, request.Mode, request.TargetSymbol); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	taskSpec, err := s.validateSessionTaskSpec(request.TaskSpec, *file, request)
	if err != nil {
		return nil, err
	}
	session := ChatSession{ID: newChatSessionID(), ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, BaseFileHash: request.BaseFileHash, OpenPath: request.OpenPath, Mode: request.Mode, TargetSymbol: request.TargetSymbol, TaskSpec: taskSpec, State: "active", Messages: []ChatSessionMessage{}, CreatedAt: now, UpdatedAt: now}
	s.chatSessionMu.Lock()
	s.chatSessions[session.ID] = &chatSession{session: cloneChatSession(session)}
	s.chatSessionMu.Unlock()
	return pointerToChatSession(session), nil
}

func validateChatTarget(file project.IndexFile, mode project.DeclarationEditMode, target string) error {
	if target == "" {
		return fmt.Errorf("target symbol is required")
	}
	count := 0
	for _, symbol := range file.Symbols {
		if symbol.Name == target && symbol.AtomicTarget && symbol.Confidence == "exact" {
			count++
		}
	}
	switch mode {
	case project.DeclarationEditReplaceSymbol:
		if count != 1 {
			return fmt.Errorf("replace mode requires one exact selected symbol")
		}
	case project.DeclarationEditCreateSymbol:
		if strings.Contains(target, ".") || !token.IsIdentifier(target) || token.Lookup(target).IsKeyword() || count != 0 {
			return fmt.Errorf("create mode requires one valid absent top-level symbol name")
		}
	default:
		return fmt.Errorf("unsupported declaration edit mode")
	}
	return nil
}

// ChatSession returns the current conversation and marks it stale if the
// project revision, open path, or base file hash no longer matches.
func (s *Service) ChatSession(id string) (*ChatSession, error) {
	s.chatSessionMu.Lock()
	defer s.chatSessionMu.Unlock()
	stored := s.chatSessions[id]
	if stored == nil {
		return nil, fmt.Errorf("chat session not found")
	}
	s.expireChatSessionLocked(stored)
	return pointerToChatSession(cloneChatSession(stored.session)), nil
}

// SendChatSessionMessage produces one declaration draft while retaining the
// session's immutable file and target identity.
func (s *Service) SendChatSessionMessage(ctx context.Context, request ChatSessionMessageRequest) (*ChatDraftProposal, error) {
	if strings.TrimSpace(request.Message) == "" {
		return nil, fmt.Errorf("chat message is required")
	}
	if err := s.RequireRemoteConfirmation(config.FunctionModelScope, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	session, err := s.chatSessionForMessage(request.SessionID, request.ParentDraftID)
	if err != nil {
		return nil, err
	}
	if request.Repair {
		if err := s.reserveTaskRepair(session, request.ParentDraftID); err != nil {
			return nil, err
		}
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	priorDeclaration, err := s.chatConversation(session, request.ParentDraftID)
	if err != nil {
		return nil, err
	}
	projectContext, manifest, err := project.NewContextBuilder().BuildFunctionWithManifest(s.manager.Root(), project.FunctionContextOptions{TargetPath: session.OpenPath, TargetSymbol: session.TargetSymbol, Mode: session.Mode, Index: index, TaskSpec: session.TaskSpec, PriorDeclaration: priorDeclaration, MaxTokens: sessionFunctionContextLimit(s.functionRuntime.effective.ContextMaxTokens)})
	if err != nil {
		return nil, fmt.Errorf("build session context: %w", err)
	}
	manifest = s.contextManifestForRuntime(manifest, s.functionRuntime)
	messages, err := declarationDraftMessages(request.Message, "", projectContext, session.OpenPath, session.TargetSymbol, session.Mode)
	if err != nil {
		return nil, err
	}
	timed, cancel := context.WithTimeout(ctx, duration(s.functionRuntime.effective.Timeout))
	defer cancel()
	result, err := s.retry(timed, s.functionRuntime, messages)
	if timed.Err() != nil {
		return nil, timed.Err()
	}
	if err != nil {
		return nil, err
	}
	response, err := ParseDeclarationDraftResponse(result.Content)
	if err != nil {
		return nil, err
	}
	if err := s.ValidateMutableRequest(session.ProjectID, session.ProjectRevision, session.OpenPath, session.BaseFileHash); err != nil {
		s.markChatSessionStale(session.ID)
		return nil, err
	}
	draft, err := s.CreateDraft(DraftCreateRequest{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, BaseFileHash: session.BaseFileHash, TargetPath: session.OpenPath, Mode: session.Mode, TargetSymbol: session.TargetSymbol, Declaration: response.Declaration, Imports: response.Imports, ParentDraftID: request.ParentDraftID, EffectiveModel: s.EffectiveModel(), TaskSpec: session.TaskSpec})
	if err != nil {
		return nil, err
	}
	assistantMessage := ChatSessionMessage{Role: "assistant", Content: response.Explanation, DraftID: draft.ID, CreatedAt: time.Now().UTC()}
	if err := s.appendChatSessionProposal(session.ID, request.ParentDraftID, request.Message, assistantMessage); err != nil {
		return nil, err
	}
	return &ChatDraftProposal{SessionID: session.ID, Draft: *draft, AssistantMessage: assistantMessage, ContextManifest: manifest}, nil
}

func (s *Service) chatConversation(session ChatSession, parentDraftID string) (string, error) {
	if parentDraftID == "" {
		return "", nil
	}
	draft, err := s.Draft(parentDraftID)
	if err != nil {
		return "", project.ErrRevisionConflict
	}
	if draft.ProjectID != session.ProjectID || draft.ProjectRevision != session.ProjectRevision || draft.BaseFileHash != session.BaseFileHash || draft.TargetPath != session.OpenPath || draft.Mode != session.Mode || draft.TargetSymbol != session.TargetSymbol {
		return "", project.ErrRevisionConflict
	}
	return draft.Declaration, nil
}

func sessionFunctionContextLimit(limit int) int {
	if limit <= 0 {
		return 4000
	}
	return limit
}

func (s *Service) validateSessionTaskSpec(spec *project.BugTaskSpec, file project.IndexFile, request ChatSessionCreateRequest) (*project.BugTaskSpec, error) {
	if spec == nil {
		return nil, nil
	}
	if request.Mode != project.DeclarationEditReplaceSymbol || spec.TargetPath != request.OpenPath || spec.TargetSymbol != request.TargetSymbol {
		return nil, project.ErrRevisionConflict
	}
	info, err := project.GetFileInfo(s.manager.Root(), file.Path)
	if err != nil {
		return nil, err
	}
	validated, err := validateBugTaskSpec(spec, file, info.Content)
	if err != nil {
		return nil, err
	}
	return validated, nil
}

func (s *Service) reserveTaskRepair(session ChatSession, parentDraftID string) error {
	if session.TaskSpec == nil || parentDraftID == "" {
		return fmt.Errorf("check-driven repair requires a task-bound draft")
	}
	if err := s.taskDraftHasFailedChecks(parentDraftID, session); err != nil {
		return err
	}
	s.chatSessionMu.Lock()
	defer s.chatSessionMu.Unlock()
	stored := s.chatSessions[session.ID]
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

func (s *Service) taskDraftHasFailedChecks(id string, session ChatSession) error {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	stored := s.drafts[id]
	if stored == nil || !sameTaskSpec(stored.draft.TaskSpec, session.TaskSpec) || stored.checks == nil || stored.checks.Revision != stored.draft.Revision || stored.checks.CompositionHash != stored.draft.CompositionHash || !checksFailed(stored.checks.Report) {
		return fmt.Errorf("check-driven repair requires failed checks for the current task draft")
	}
	return nil
}

func checksFailed(report DraftCheckReport) bool {
	for _, check := range report.Checks {
		if check.State == CheckFailed || check.State == CheckCanceled {
			return true
		}
	}
	return !report.Applicable
}

func sameTaskSpec(left, right *project.BugTaskSpec) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.SchemaVersion == right.SchemaVersion && left.TargetPath == right.TargetPath && left.TargetSymbol == right.TargetSymbol && left.TargetSignature == right.TargetSignature
}

func (s *Service) chatSessionForMessage(id, parentDraftID string) (ChatSession, error) {
	s.chatSessionMu.Lock()
	defer s.chatSessionMu.Unlock()
	stored := s.chatSessions[id]
	if stored == nil {
		return ChatSession{}, fmt.Errorf("chat session not found")
	}
	s.expireChatSessionLocked(stored)
	if stored.session.State != "active" {
		return ChatSession{}, project.ErrRevisionConflict
	}
	if stored.session.LatestDraftID != parentDraftID {
		return ChatSession{}, project.ErrRevisionConflict
	}
	return cloneChatSession(stored.session), nil
}

func (s *Service) appendChatSessionProposal(id, parentDraftID, userMessage string, assistantMessage ChatSessionMessage) error {
	s.chatSessionMu.Lock()
	defer s.chatSessionMu.Unlock()
	stored := s.chatSessions[id]
	if stored == nil {
		return project.ErrRevisionConflict
	}
	s.expireChatSessionLocked(stored)
	if stored.session.State != "active" || stored.session.LatestDraftID != parentDraftID {
		return project.ErrRevisionConflict
	}
	now := time.Now().UTC()
	stored.session.Messages = append(stored.session.Messages, ChatSessionMessage{Role: "user", Content: userMessage, CreatedAt: now}, assistantMessage)
	stored.session.LatestDraftID = assistantMessage.DraftID
	stored.session.UpdatedAt = now
	return nil
}

func (s *Service) expireChatSessionLocked(stored *chatSession) {
	if stored.session.State != "active" {
		return
	}
	if err := s.ValidateMutableRequest(stored.session.ProjectID, stored.session.ProjectRevision, stored.session.OpenPath, stored.session.BaseFileHash); err != nil {
		stored.session.State = "stale"
		stored.session.UpdatedAt = time.Now().UTC()
	}
}

func (s *Service) markChatSessionStale(id string) {
	s.chatSessionMu.Lock()
	defer s.chatSessionMu.Unlock()
	if stored := s.chatSessions[id]; stored != nil {
		stored.session.State = "stale"
		stored.session.UpdatedAt = time.Now().UTC()
	}
}

func (s *Service) clearChatSessions() {
	s.chatSessionMu.Lock()
	defer s.chatSessionMu.Unlock()
	s.chatSessions = make(map[string]*chatSession)
}

func ParseDeclarationDraftResponse(output string) (DeclarationDraftResponse, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" || len(trimmed) > maxSemanticAnalysisBytes {
		return DeclarationDraftResponse{}, fmt.Errorf("declaration draft response is empty or exceeds the size limit")
	}
	var response DeclarationDraftResponse
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return DeclarationDraftResponse{}, fmt.Errorf("parse declaration draft response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return DeclarationDraftResponse{}, fmt.Errorf("declaration draft response must contain one JSON object")
	}
	if response.Version != chatSessionResponseVersion || strings.TrimSpace(response.Declaration) == "" || strings.TrimSpace(response.Explanation) == "" {
		return DeclarationDraftResponse{}, fmt.Errorf("declaration draft response does not satisfy the required contract")
	}
	return response, nil
}

func cloneChatSession(source ChatSession) ChatSession {
	copy := source
	copy.Messages = append([]ChatSessionMessage(nil), source.Messages...)
	copy.TaskSpec = project.SanitizeBugTaskSpec(source.TaskSpec)
	return copy
}

func pointerToChatSession(session ChatSession) *ChatSession { return &session }
