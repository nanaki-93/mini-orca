package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func (s *Service) OpenChangeSession(ctx context.Context, request ChangeCreateRequest) (*ChangeSession, error) {
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if index.ProjectID != request.ProjectID || index.ProjectRevision != request.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	if err := validateChangeRequest(request); err != nil {
		return nil, err
	}
	root := s.manager.Root()
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	session := &ChangeSession{SchemaVersion: 1, ID: "change-" + hex.EncodeToString(nonce[:]), ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: request.Kind, Title: strings.TrimSpace(request.Title), AcceptanceCriteria: request.AcceptanceCriteria, Targets: []ChangeTarget{}, Changes: []ChangeEdit{}, Messages: []ChatSessionMessage{}, Instructions: map[string]string{}, Checks: []DraftCheck{}, State: "draft", Freshness: "current", PolicyFingerprint: policy.Version(), CreatedAt: now, UpdatedAt: now}
	if err := captureChangeScope(root, session, request.Paths); err != nil {
		return nil, err
	}
	_, manifest, err := changeContext(root, session)
	if err != nil {
		return nil, err
	}
	session.ContextManifest = s.contextManifestForRuntime(manifest, s.runtimes.function)
	session.WorkspaceHash, err = benchmarkSourceFingerprint(ctx, root)
	if err != nil {
		return nil, err
	}
	session.Hash = changeProposalHash(session.Changes)
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	history, err := listChangeSessions(root)
	if err != nil {
		return nil, err
	}
	if len(history) >= 200 {
		return nil, fmt.Errorf("change history has reached its 200-session limit")
	}
	if err := s.verifyChangeCurrent(ctx, root, session); err != nil {
		return nil, err
	}
	if err := writeChangeSession(root, session); err != nil {
		return nil, err
	}
	return session, nil
}

func validateChangeRequest(request ChangeCreateRequest) error {
	if len(request.Paths) == 0 || len(request.Paths) > maxChangePaths || len(request.Title) > 200 || strings.TrimSpace(request.Title) == "" || !validChangeKind(request.Kind) {
		return fmt.Errorf("provide a task title, kind and one to eight paths")
	}
	return validateChangeCriteria(request.AcceptanceCriteria)
}

func captureChangeScope(root string, session *ChangeSession, paths []string) error {
	for _, path := range paths {
		if changeTarget(session, path) != nil {
			return fmt.Errorf("duplicate target path")
		}
		target, err := captureChangeTarget(root, path)
		if err != nil {
			return err
		}
		instructions, err := project.ResolveInstructions(root, path)
		if err != nil {
			return err
		}
		session.Targets = append(session.Targets, target)
		session.Instructions[path] = instructions.Fingerprint
	}
	return nil
}

func validChangeKind(kind string) bool {
	return kind == "fix" || kind == "performance" || kind == "feature" || kind == "instructions"
}

func validateChangeCriteria(criteria []string) error {
	if len(criteria) > 16 {
		return fmt.Errorf("at most sixteen acceptance criteria are supported")
	}
	for _, criterion := range criteria {
		if strings.TrimSpace(criterion) == "" || len(criterion) > 1024 {
			return fmt.Errorf("acceptance criteria must be non-empty bounded text")
		}
	}
	return nil
}

func (s *Service) verifyChangeCurrent(ctx context.Context, root string, session *ChangeSession) error {
	index, err := s.manager.Index()
	if err != nil {
		return err
	}
	if s.manager.Root() != root || index.ProjectID != session.ProjectID || index.ProjectRevision != session.ProjectRevision {
		return project.ErrRevisionConflict
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return err
	}
	if policy.Version() != session.PolicyFingerprint {
		return project.ErrRevisionConflict
	}
	for _, expected := range session.Targets {
		current, err := captureChangeTarget(root, expected.Path)
		if err != nil {
			return err
		}
		instructions, err := project.ResolveInstructions(root, expected.Path)
		if err != nil {
			return err
		}
		if current.Exists != expected.Exists || current.Hash != expected.Hash || instructions.Fingerprint != session.Instructions[expected.Path] {
			return project.ErrRevisionConflict
		}
	}
	return (benchmarkFixture{fingerprint: session.WorkspaceHash}).verifyCurrent(ctx, root)
}

func (s *Service) ChangeSession(ctx context.Context, id string) (*ChangeSession, error) {
	root := s.manager.Root()
	session, err := readChangeSession(root, id)
	if err != nil {
		return nil, err
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if s.manager.Root() != root || session.ProjectID != index.ProjectID {
		return nil, project.ErrRevisionConflict
	}
	if session.State == "draft" {
		if err := s.verifyChangeCurrent(ctx, root, session); err != nil {
			if !errors.Is(err, project.ErrRevisionConflict) && !errors.Is(err, project.ErrExcludedFile) {
				return nil, err
			}
			session.Freshness = "stale"
		}
	}
	return session, nil
}

// ResumeChange clears historical authority even when the captured source still
// matches. Only a fresh check/review can authorize the restored candidate.
func (s *Service) ResumeChange(ctx context.Context, id string) (*ChangeSession, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	root := s.manager.Root()
	session, err := s.ChangeSession(ctx, id)
	if err != nil {
		return nil, err
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if s.manager.Root() != root || session.ProjectID != index.ProjectID {
		return nil, project.ErrRevisionConflict
	}
	session.Checks, session.ReviewedHash = []DraftCheck{}, ""
	delete(s.changeAuthority, session.ProjectID+"/"+session.ID)
	if err := writeChangeSession(root, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Service) ChangeHistory(ctx context.Context) ([]ChangeHistoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := s.manager.Index(); err != nil {
		return nil, err
	}
	return listChangeSessions(s.manager.Root())
}

func (s *Service) loadChangeForAction(ctx context.Context, id string, identity ChangeIdentity) (*ChangeSession, string, error) {
	root := s.manager.Root()
	session, err := readChangeSession(root, id)
	if err != nil {
		return nil, root, err
	}
	if session.State != "draft" || identity.ProjectID != session.ProjectID || identity.ProjectRevision != session.ProjectRevision || identity.Revision != session.Revision || identity.Hash != session.Hash {
		return nil, root, project.ErrRevisionConflict
	}
	if err := s.verifyChangeCurrent(ctx, root, session); err != nil {
		return nil, root, err
	}
	return session, root, nil
}

func (s *Service) SendChangeMessage(ctx context.Context, id string, request ChangeMessageRequest) (*ChangeSession, error) {
	if strings.TrimSpace(request.Message) == "" || len(request.Message) > 8192 {
		return nil, fmt.Errorf("provide a message of at most 8192 bytes")
	}
	if err := s.RequireRemoteConfirmation(config.FunctionModelScope, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	session, root, err := s.loadChangeForAction(ctx, id, request.ChangeIdentity)
	if err != nil {
		return nil, err
	}
	if len(session.Messages) >= maxChangeMessages {
		return nil, fmt.Errorf("conversation limit reached; start another task")
	}
	if request.Repair {
		return nil, fmt.Errorf("run current proposal checks before requesting repair")
	}
	return s.generateChangeMessage(ctx, root, session, request)
}

func (s *Service) generateChangeMessage(ctx context.Context, root string, session *ChangeSession, request ChangeMessageRequest) (*ChangeSession, error) {
	text, manifest, err := changeContext(root, session)
	if err != nil {
		return nil, err
	}
	messages, err := changeMessages(session, request.Message, text)
	if err != nil {
		return nil, err
	}
	runtime := s.runtimes.function
	if runtime.effective.ContextMaxTokens > 0 && len(messages[0].Content)+len(messages[1].Content) > runtime.effective.ContextMaxTokens*4 {
		return nil, fmt.Errorf("selected context exceeds configured Function token limit")
	}
	timed, cancel := context.WithTimeout(ctx, duration(runtime.effective.Timeout))
	defer cancel()
	schema := changeResponseSchema()
	result, err := s.retryRequestAuthorized(timed, runtime, messages, &schema, func(ctx context.Context) error { return s.verifyChangeCurrent(ctx, root, session) })
	if err != nil {
		return nil, err
	}
	explanation, edits, err := parseChangeResponse(result.Content, session)
	if err != nil {
		return nil, err
	}
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	if _, _, err := s.loadChangeForAction(timed, session.ID, request.ChangeIdentity); err != nil {
		return nil, err
	}
	session.Revision++
	session.Changes, session.Hash = edits, changeProposalHash(edits)
	session.Checks, session.ReviewedHash = []DraftCheck{}, ""
	delete(s.changeAuthority, session.ProjectID+"/"+session.ID)
	session.ContextManifest = s.contextManifestForRuntime(manifest, runtime)
	now := time.Now().UTC()
	session.Messages = append(session.Messages, ChatSessionMessage{Role: "user", Content: request.Message, CreatedAt: now}, ChatSessionMessage{Role: "assistant", Content: explanation, CreatedAt: now})
	session.UpdatedAt = now
	if err := writeChangeSession(root, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Service) ProposeInstructions(ctx context.Context, request InstructionProposalRequest) (*ChangeSession, error) {
	if request.Kind != "instructions" || len(request.Paths) != 1 || filepath.Base(request.Paths[0]) != "AGENTS.md" || len(request.Content) > project.MaxInstructionBytes || strings.TrimSpace(request.Content) == "" || !utf8.ValidString(request.Content) || strings.ContainsRune(request.Content, 0) {
		return nil, fmt.Errorf("provide one AGENTS.md path and non-empty bounded instructions")
	}
	root := s.manager.Root()
	session, err := s.OpenChangeSession(ctx, request.ChangeCreateRequest)
	if err != nil {
		return nil, err
	}
	target := session.Targets[0]
	if target.Exists && target.Content == request.Content {
		return nil, fmt.Errorf("instructions are unchanged")
	}
	session.Changes = []ChangeEdit{{Path: target.Path, Content: request.Content, Hash: contentHash([]byte(request.Content)), Diff: changeDiff(target.Path, target.Content, request.Content)}}
	session.Hash, session.Revision = changeProposalHash(session.Changes), 1
	session.Messages = []ChatSessionMessage{{Role: "assistant", Content: "Review the instruction changes before applying them.", CreatedAt: time.Now().UTC()}}
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	if err := s.verifyChangeCurrent(ctx, root, session); err != nil {
		return nil, err
	}
	if err := writeChangeSession(root, session); err != nil {
		return nil, err
	}
	return session, nil
}
