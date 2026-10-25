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

	"github.com/nanaki-93/mini-orca/v2/internal/app/features"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/project/diff"
)

func (s *Service) OpenChangeSession(ctx context.Context, request ChangeCreateRequest) (*ChangeSession, error) {
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if index.ProjectID != request.ProjectID || index.ProjectRevision != request.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	if err := validateChangeCreateRequest(request); err != nil {
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
	session := &ChangeSession{SchemaVersion: 1, ID: "change-" + hex.EncodeToString(nonce[:]), ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: request.Kind, Title: strings.TrimSpace(request.Title), AcceptanceCriteria: request.AcceptanceCriteria, Targets: []ChangeTarget{}, Changes: []ChangeEdit{}, Messages: []ChangeMessage{}, Instructions: map[string]string{}, Checks: []CheckResult{}, State: "draft", Freshness: "current", PolicyFingerprint: policy.Version(), CreatedAt: now, UpdatedAt: now}
	session.MissionOrigin, err = copyMissionChildOrigin(request.missionOrigin)
	if err != nil {
		return nil, err
	}
	session.FindingTargets = append([]project.FindingTarget(nil), request.FindingTargets...)
	if err := captureChangeScope(root, session, request.Paths); err != nil {
		return nil, err
	}
	session.Git, err = captureChangeGit(ctx, root)
	if err != nil {
		return nil, err
	}
	if err := s.captureChangePromptSnapshot(ctx, root, session, request.Plan); err != nil {
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
	if err := storeChangeSession(root, session); err != nil {
		return nil, err
	}
	return session, nil
}

func validateChangeCreateRequest(request ChangeCreateRequest) error {
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
	return validateChangeFindingTargets(session)
}

func validChangeKind(kind string) bool {
	return kind == "fix" || kind == "performance" || kind == "security" || kind == "feature" || kind == "instructions"
}

func validateChangeCriteria(criteria []string) error {
	return features.ValidateCriteria(criteria)
}

func (s *Service) verifyChangeTargetsCurrent(root string, session *ChangeSession) error {
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
	targets := append([]ChangeTarget(nil), session.Targets...)
	if session.Plan != nil {
		targets = append(targets, session.Plan.ReadTargets...)
	}
	for _, expected := range targets {
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
	return nil
}

func (s *Service) verifyChangeCurrent(ctx context.Context, root string, session *ChangeSession) error {
	if err := s.verifyChangeTargetsCurrent(root, session); err != nil {
		return err
	}
	if err := verifySessionGitIdentity(ctx, root, session.Git); err != nil {
		return err
	}
	return (workspaceIdentity{fingerprint: session.WorkspaceHash}).verifyCurrent(ctx, root)
}

func (s *Service) hasActiveWorkflowJob(root string, session *ChangeSession) bool {
	if session == nil || session.Workflow == nil || !isChangePipelineActive(session.Workflow) {
		return false
	}
	s.changeWorkflow.mu.Lock()
	defer s.changeWorkflow.mu.Unlock()
	job := s.changeWorkflow.getJob(session.ID)
	return job != nil && !job.finished && job.root == root && job.id == session.Workflow.ID
}

func (s *Service) ChangeSession(ctx context.Context, id string) (*ChangeSession, error) {
	root := s.manager.Root()
	session, err := loadChangeSession(root, id)
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
		var verifyErr error
		if s.hasActiveWorkflowJob(root, session) {
			verifyErr = s.verifyChangeTargetsCurrent(root, session)
		} else {
			verifyErr = s.verifyChangeCurrent(ctx, root, session)
		}
		if verifyErr != nil {
			if !errors.Is(verifyErr, project.ErrRevisionConflict) && !errors.Is(verifyErr, project.ErrExcludedFile) {
				return nil, verifyErr
			}
			session.Freshness = "stale"
		}
	}
	s.restoreChangeWorkflow(session)
	return session, nil
}

// ResumeChange clears historical authority even when the captured source still
// matches. Only a fresh check/review can authorize the restored candidate.
func (s *Service) ResumeChange(ctx context.Context, id string) (*ChangeSession, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	root := s.manager.Root()
	s.changeWorkflow.mu.Lock()
	job := s.changeWorkflow.getJob(id)
	running := job != nil && !job.finished && job.root == root && job.sessionID == id
	s.changeWorkflow.mu.Unlock()
	if running {
		return nil, fmt.Errorf("%w: cancel the workflow before restoring its conversation", project.ErrRevisionConflict)
	}
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
	if isChangePipelineActive(session.Workflow) {
		return nil, fmt.Errorf("%w: cancel the workflow before restoring its conversation", project.ErrRevisionConflict)
	}
	session.Checks, session.ReviewedHash, session.PinnedTests = []CheckResult{}, "", nil
	invalidateChangeWorkflow(session, "Restored workflow evidence requires a new run.")
	delete(s.changeAuthority, session.ProjectID+"/"+session.ID)
	if err := storeChangeSession(root, session); err != nil {
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
	history, err := listChangeSessions(s.manager.Root())
	if err != nil {
		return nil, err
	}
	s.changeWorkflow.mu.Lock()
	defer s.changeWorkflow.mu.Unlock()
	for i := range history {
		entry := &history[i]
		if entry.WorkflowStatus == "running" || entry.WorkflowStatus == "canceling" {
			job := s.changeWorkflow.getJob(entry.ID)
			if job == nil || job.finished || job.sessionID != entry.ID || job.root != s.manager.Root() {
				entry.WorkflowStatus = "interrupted"
				entry.WorkflowReason = "The workflow stopped before completion. Restore the conversation to run again."
			}
		}
	}
	return history, nil
}

func (s *Service) loadChangeForAction(ctx context.Context, id string, identity ChangeIdentity) (*ChangeSession, string, error) {
	root := s.manager.Root()
	if err := ctx.Err(); err != nil {
		return nil, root, err
	}
	session, err := loadChangeSession(root, id)
	if err != nil {
		return nil, root, err
	}
	if session.State != "draft" || identity.ProjectID != session.ProjectID || identity.ProjectRevision != session.ProjectRevision || identity.Revision != session.Revision || identity.Hash != session.Hash {
		return nil, root, project.ErrRevisionConflict
	}
	if isChangePipelineActive(session.Workflow) && ctx.Value(changeWorkflowContextKey{}) != session.Workflow.ID {
		return nil, root, fmt.Errorf("%w: this proposal belongs to an active workflow", project.ErrRevisionConflict)
	}
	if err := s.verifyChangeCurrent(ctx, root, session); err != nil {
		return nil, root, err
	}
	return session, root, nil
}

func (s *Service) SendChangeMessage(ctx context.Context, id string, request ChangeMessageRequest) (*ChangeSession, error) {
	if len(request.Changes) > 0 {
		return s.updateChangeProposal(ctx, id, request)
	}
	if strings.TrimSpace(request.Message) == "" || len(request.Message) > 8192 {
		return nil, fmt.Errorf("provide a message of at most 8192 bytes")
	}
	runtime, err := s.changeMessageRuntime(ctx, request)
	if err != nil {
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
	return s.generateChangeWithRuntime(ctx, root, session, request, runtime)
}

func (s *Service) updateChangeProposal(ctx context.Context, id string, request ChangeMessageRequest) (*ChangeSession, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, root, err := s.loadChangeForAction(ctx, id, request.ChangeIdentity)
	if err != nil {
		return nil, err
	}
	edits, err := buildChangeEdits(session, request.Changes)
	if err != nil {
		return nil, err
	}
	session.Revision++
	session.Changes, session.Hash = edits, changeProposalHash(edits)
	session.Checks, session.ReviewedHash, session.PinnedTests = []CheckResult{}, "", nil
	invalidateChangeWorkflow(session, "The proposal changed. Run the workflow again before human review.")
	delete(s.changeAuthority, session.ProjectID+"/"+session.ID)
	now := time.Now().UTC()
	note := strings.TrimSpace(request.Message)
	if note != "" && len(session.Messages) < maxChangeMessages {
		session.Messages = append(session.Messages, ChangeMessage{Role: "user", Content: note, CreatedAt: now})
	}
	session.UpdatedAt = now
	if err := storeChangeSession(root, session); err != nil {
		return nil, err
	}
	return session, nil
}

func buildChangeEdits(session *ChangeSession, inputs []ChangeEditInput) ([]ChangeEdit, error) {
	if len(inputs) == 0 || len(inputs) > maxChangePaths {
		return nil, fmt.Errorf("provide one to sixty-four file changes")
	}
	edits := []ChangeEdit{}
	seen := map[string]bool{}
	for _, input := range inputs {
		if seen[input.Path] {
			return nil, fmt.Errorf("proposal repeated a target")
		}
		seen[input.Path] = true
		edit, err := makeChangeEdit(session, input.Path, input.Content)
		if err != nil {
			return nil, err
		}
		if edit != nil {
			edit.Summary = sanitizeEditSummary(input.Summary)
			if len(input.Criteria) > 16 {
				return nil, fmt.Errorf("at most 16 criterion references per file")
			}
			for _, criterion := range input.Criteria {
				if criterion < 1 || criterion > len(session.AcceptanceCriteria) {
					return nil, fmt.Errorf("file criterion must reference an acceptance criterion")
				}
			}
			edit.Criteria = input.Criteria
			edits = append(edits, *edit)
		}
	}
	return edits, nil
}

func (s *Service) changeMessageRuntime(ctx context.Context, request ChangeMessageRequest) (modelRuntime, error) {
	profile := request.Profile
	if profile == "" {
		profile = "code"
	}
	if !validModelSelection(profile) {
		return modelRuntime{}, fmt.Errorf("choose a catalog model ID or configured analyze, bug or code profile")
	}
	if err := s.validateModelSelections(ctx, []string{profile}); err != nil {
		return modelRuntime{}, err
	}
	runtime, _ := s.selectedRuntime(config.CodeModelScope, profile)
	if genTimeout := duration(s.runtimes.function.effective.Timeout); genTimeout > duration(runtime.effective.Timeout) {
		runtime.effective.Timeout = genTimeout.String()
	}
	return runtime, requireModelRuntimeConfirmation(runtime, request.ConfirmRemoteProvider)
}

func (s *Service) generateChangeWithRuntime(ctx context.Context, root string, session *ChangeSession, request ChangeMessageRequest, runtime modelRuntime) (*ChangeSession, error) {
	if err := requireMissionProviderAdmission(ctx, session); err != nil {
		return nil, err
	}
	if err := requireApprovedChangePlan(session); err != nil {
		return nil, err
	}
	stageZeroWorkflow := ctx.Value(changeWorkflowContextKey{}) != nil
	timed, cancel := context.WithTimeout(ctx, duration(runtime.effective.Timeout))
	defer cancel()
	prepared, err := s.prepareChangeContext(timed, root, session, request.Message, runtime, stageZeroWorkflow)
	if err != nil {
		return nil, err
	}
	defer prepared.release()
	schema := changeResponseSchema()
	authorize := func(ctx context.Context) error {
		if err := requireMissionProviderAdmission(ctx, session); err != nil {
			return err
		}
		return s.verifyChangeCurrent(ctx, root, session)
	}
	timed, err = withProjectMemory(prepared.ctx, root, session.ProjectID, session.ProjectRevision, request.Message, changeMemorySources(session, stageZeroWorkflow), authorize)
	if err != nil {
		return nil, err
	}
	result, err := s.retryRequestAuthorized(timed, runtime, prepared.messages, &schema, authorize)
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
	if request.Repair && len(session.Changes) > 0 {
		edits = preservePinnedTests(edits, session.Changes)
	} else if len(session.PinnedTests) > 0 {
		edits = preservePinnedTests(edits, session.PinnedTests)
	} else {
		edits = preservePinnedTests(edits, changeTestEdits(session.Changes))
	}
	session.Revision++
	session.Changes, session.Hash = edits, changeProposalHash(edits)
	session.Checks, session.ReviewedHash, session.PinnedTests = []CheckResult{}, "", nil
	if ctx.Value(changeWorkflowContextKey{}) == nil {
		invalidateChangeWorkflow(session, "The proposal changed. Run the workflow again before human review.")
	}
	delete(s.changeAuthority, session.ProjectID+"/"+session.ID)
	session.ContextManifest = s.contextManifestForRuntime(prepared.manifest, runtime)
	now := time.Now().UTC()
	session.Messages = append(session.Messages, ChangeMessage{Role: "user", Content: request.Message, CreatedAt: now}, ChangeMessage{Role: "assistant", Content: explanation, CreatedAt: now})
	session.UpdatedAt = now
	if err := storeChangeSession(root, session); err != nil {
		return nil, err
	}
	return session, nil
}

func preservePinnedTests(edits, pinned []ChangeEdit) []ChangeEdit {
	for _, test := range pinned {
		found := false
		for _, edit := range edits {
			if edit.Path == test.Path {
				found = true
				break
			}
		}
		if !found {
			edits = append(edits, test)
		}
	}
	return edits
}

func validateDirectProposal(request InstructionProposalRequest) error {
	if len(request.Paths) != 1 || strings.TrimSpace(request.Content) == "" || !utf8.ValidString(request.Content) || strings.ContainsRune(request.Content, 0) {
		return fmt.Errorf("provide one target path and non-empty bounded content")
	}
	if request.Kind == "instructions" && (filepath.Base(request.Paths[0]) != "AGENTS.md" || len(request.Content) > project.MaxInstructionBytes) {
		return fmt.Errorf("provide one AGENTS.md path and non-empty bounded instructions")
	}
	if len(request.Content) > maxChangeBytes {
		return fmt.Errorf("content exceeds 256 KiB")
	}
	return nil
}

type InstructionPreview struct {
	ProjectID       string                        `json:"project_id"`
	ProjectRevision string                        `json:"project_revision"`
	Path            string                        `json:"path"`
	ExistingContent string                        `json:"existing_content"`
	Exists          bool                          `json:"exists"`
	Effective       project.EffectiveInstructions `json:"effective"`
	Presets         []project.InstructionPreset   `json:"presets"`
}

func (s *Service) PreviewInstructions(path string) (*InstructionPreview, error) {
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	root := s.manager.Root()
	effective, err := project.ResolveInstructions(root, path)
	if err != nil {
		return nil, err
	}
	presets, err := project.SuggestInstructionPresets(root, path, index.Files)
	if err != nil {
		return nil, err
	}
	current, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if s.manager.Root() != root || current.ProjectID != index.ProjectID || current.ProjectRevision != index.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	preview := &InstructionPreview{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Path: path, Effective: effective, Presets: presets}
	for _, file := range effective.Files {
		if file.Path == path {
			preview.Exists, preview.ExistingContent = true, file.Content
		}
	}
	return preview, nil
}

func (s *Service) ProposeInstructions(ctx context.Context, request InstructionProposalRequest) (*ChangeSession, error) {
	if err := validateDirectProposal(request); err != nil {
		return nil, err
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
	session.Changes = []ChangeEdit{{Path: target.Path, Content: request.Content, Hash: contentHash([]byte(request.Content)), Diff: diff.BuildUnifiedDiff(target.Path, target.Content, request.Content)}}
	session.Hash, session.Revision = changeProposalHash(session.Changes), 1
	session.Messages = []ChangeMessage{{Role: "assistant", Content: "Review the file changes before applying them.", CreatedAt: time.Now().UTC()}}
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	if err := s.verifyChangeCurrent(ctx, root, session); err != nil {
		return nil, err
	}
	if err := storeChangeSession(root, session); err != nil {
		return nil, err
	}
	return session, nil
}
