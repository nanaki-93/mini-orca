package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// DraftState describes where a declaration draft is in its explicit review
// lifecycle. Only a valid draft can start focused checks.
type DraftState string

const (
	DraftGenerated  DraftState = "generated"
	DraftDirty      DraftState = "dirty"
	DraftValidating DraftState = "validating"
	DraftValid      DraftState = "valid"
	DraftInvalid    DraftState = "invalid"
	DraftStale      DraftState = "stale"
)

// Draft is the source-free, project-scoped representation of one editable Go
// declaration. The full file is composed only when a caller needs it.
type Draft struct {
	ID                 string                         `json:"id"`
	ProjectID          string                         `json:"project_id"`
	ProjectRevision    string                         `json:"project_revision"`
	BaseFileHash       string                         `json:"base_file_hash"`
	TargetPath         string                         `json:"target_path"`
	Mode               project.DeclarationEditMode    `json:"mode"`
	TargetSymbol       string                         `json:"target_symbol"`
	Declaration        string                         `json:"declaration"`
	Imports            []string                       `json:"imports,omitempty"`
	Revision           int64                          `json:"revision"`
	Hash               string                         `json:"hash"`
	CompositionHash    string                         `json:"candidate_hash,omitempty"`
	ParentDraftID      string                         `json:"parent_draft_id,omitempty"`
	EffectiveModel     EffectiveModel                 `json:"effective_model,omitempty"`
	PreviousHash       string                         `json:"previous_hash,omitempty"`
	State              DraftState                     `json:"state"`
	Validation         *project.DeclarationValidation `json:"validation,omitempty"`
	TaskSpec           *project.BugTaskSpec           `json:"task_spec,omitempty"`
	EngineeringInsight *project.EngineeringInsight    `json:"engineering_insight,omitempty"`
}

// DraftCreateRequest supplies immutable project identity and the first
// declaration revision. ParentDraftID connects explicit revision proposals.
type DraftCreateRequest struct {
	ID                 string
	ProjectID          string
	ProjectRevision    string
	BaseFileHash       string
	TargetPath         string
	Mode               project.DeclarationEditMode
	TargetSymbol       string
	Declaration        string
	Imports            []string
	ParentDraftID      string
	EffectiveModel     EffectiveModel
	TaskSpec           *project.BugTaskSpec
	EngineeringInsight *project.EngineeringInsight
}

// DraftUpdateRequest changes only the isolated declaration/imports. Base
// identity, target, and edit mode are intentionally immutable.
type DraftUpdateRequest struct {
	ID               string
	ExpectedRevision int64
	Declaration      string
	Imports          []string
}

// DraftCheckRequest pins focused checks to the exact validated declaration
// revision and hash the reviewer saw.
type DraftCheckRequest struct {
	ID               string
	ExpectedRevision int64
	ExpectedHash     string
	Options          DraftCheckOptions
}

type draftCheckEvidence struct {
	Revision        int64
	CompositionHash string
	Report          DraftCheckReport
}

type storedDraft struct {
	draft  Draft
	checks *draftCheckEvidence
}

// CreateDraft records an editable declaration without composing or persisting a
// complete source. It starts generated; validation is a separate operation.
func (s *Service) CreateDraft(request DraftCreateRequest) (*Draft, error) {
	if request.ID == "" {
		request.ID = newDraftID()
	}
	target, err := newTaskTarget(request.ProjectID, request.ProjectRevision, request.TargetPath, request.BaseFileHash, request.Mode, request.TargetSymbol)
	if err != nil {
		return nil, err
	}
	if err := s.ValidateMutableRequest(target.project.id, target.project.revision, target.file.path, target.file.baseHash); err != nil {
		return nil, err
	}
	draft := Draft{
		ID: request.ID, ProjectID: target.project.id, ProjectRevision: target.project.revision,
		BaseFileHash: target.file.baseHash, TargetPath: target.file.path, Mode: target.mode,
		TargetSymbol: target.symbol, Declaration: request.Declaration, Imports: append([]string(nil), request.Imports...),
		Revision: 1, ParentDraftID: request.ParentDraftID, EffectiveModel: request.EffectiveModel, TaskSpec: project.SanitizeBugTaskSpec(request.TaskSpec), EngineeringInsight: project.CloneEngineeringInsight(request.EngineeringInsight), State: DraftGenerated,
	}
	draft.Hash = draftHash(draft.Declaration, draft.Imports)
	return s.drafts.create(draft)
}

// Draft returns a copy and marks it stale if its pinned project/file identity
// no longer matches the active project.
func (s *Service) Draft(id string) (*Draft, error) {
	draft, err := s.refreshDraft(id)
	if err != nil {
		return nil, err
	}
	return &draft, nil
}

// UpdateDraft creates the next revision only if the caller saw the current
// revision. Any content/import change removes all previous approval evidence.
func (s *Service) UpdateDraft(request DraftUpdateRequest) (*Draft, error) {
	if _, err := s.requireCurrentDraft(request.ID); err != nil {
		return nil, err
	}
	return s.drafts.update(request)
}

// BeginDraftValidation reserves the displayed revision for validation. A
// completion for an older revision is rejected and cannot authorize an edit.
func (s *Service) BeginDraftValidation(id string, expectedRevision int64) (*Draft, error) {
	if _, err := s.requireCurrentDraft(id); err != nil {
		return nil, err
	}
	return s.drafts.beginValidation(id, expectedRevision)
}

// CompleteDraftValidation stores validation only for the revision that began
// it. Invalid drafts retain their editable declaration for correction.
func (s *Service) CompleteDraftValidation(id string, expectedRevision int64, validation project.DeclarationValidation) (*Draft, error) {
	return s.completeDraftValidation(id, expectedRevision, validation, "")
}

func (s *Service) completeDraftValidation(id string, expectedRevision int64, validation project.DeclarationValidation, compositionHash string) (*Draft, error) {
	if _, err := s.requireCurrentDraft(id); err != nil {
		return nil, err
	}
	return s.drafts.completeValidation(id, expectedRevision, validation, compositionHash)
}

// ValidateDraft composes the current declaration in memory and records only
// validation evidence for the exact revision that was requested.
func (s *Service) ValidateDraft(id string, expectedRevision int64) (*Draft, error) {
	draft, err := s.BeginDraftValidation(id, expectedRevision)
	if err != nil {
		return nil, err
	}
	composition, err := s.composeDraft(*draft)
	if err != nil {
		return nil, s.finishDraftValidationAfterError(id, expectedRevision, err)
	}
	return s.completeDraftValidation(id, expectedRevision, composition.Validation, composition.CompositionHash)
}

func (s *Service) finishDraftValidationAfterError(id string, expectedRevision int64, cause error) error {
	if _, err := s.CompleteDraftValidation(id, expectedRevision, project.DeclarationValidation{}); err != nil {
		return err
	}
	return cause
}

func (s *Service) composeDraft(draft Draft) (project.GoDeclarationComposition, error) {
	if err := s.ValidateMutableRequest(draft.ProjectID, draft.ProjectRevision, draft.TargetPath, draft.BaseFileHash); err != nil {
		return project.GoDeclarationComposition{}, err
	}
	path, err := project.ResolveFile(s.manager.Root(), draft.TargetPath)
	if err != nil {
		return project.GoDeclarationComposition{}, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return project.GoDeclarationComposition{}, fmt.Errorf("read draft target: %w", err)
	}
	return project.ComposeGoDeclaration(draft.TargetPath, string(original), project.GoDeclarationEdit{
		Mode: draft.Mode, TargetSymbol: draft.TargetSymbol, Declaration: draft.Declaration, Imports: draft.Imports,
	}), nil
}

// CheckDraft runs isolated checks for exactly one current, validated draft.
// A concurrent manual edit cannot retain its predecessor's check evidence.
func (s *Service) CheckDraft(ctx context.Context, request DraftCheckRequest) (*DraftCheckReport, error) {
	identity, err := newDraftRevisionIdentity(request.ID, request.ExpectedRevision, request.ExpectedHash)
	if err != nil {
		return nil, err
	}
	draft, err := s.loadValidatedDraftForChecks(identity)
	if err != nil {
		return nil, err
	}
	input, compositionHash, err := s.composeDraftCheckInput(draft)
	if err != nil {
		return nil, err
	}
	report, err := s.runDraftChecks(ctx, input, request.Options)
	if err != nil {
		return nil, err
	}
	report = draftCheckReport(draft, compositionHash, report)
	return s.storeDraftCheckReport(identity, draft, compositionHash, report)
}

func (s *Service) loadValidatedDraftForChecks(identity draftRevisionIdentity) (Draft, error) {
	if _, err := s.requireCurrentDraft(identity.id); err != nil {
		return Draft{}, err
	}
	return s.drafts.validated(identity)
}

func (s *Service) composeDraftCheckInput(draft Draft) (draftCheckInput, string, error) {
	composition, err := s.composeDraft(draft)
	if err != nil {
		return draftCheckInput{}, "", err
	}
	if !composition.Validation.Applicable || composition.CompositionHash != draft.CompositionHash {
		return draftCheckInput{}, "", project.ErrRevisionConflict
	}
	file, err := s.manager.IndexedFile(draft.TargetPath)
	if err != nil {
		return draftCheckInput{}, "", err
	}
	if file.ContentHash != draft.BaseFileHash {
		return draftCheckInput{}, "", project.ErrRevisionConflict
	}
	input := draftCheckInput{file: *file, source: composition.Source}
	if draft.TaskSpec != nil {
		input.taskTest = draft.TaskSpec.GoTestCandidate
	}
	return input, composition.CompositionHash, nil
}

func draftCheckReport(draft Draft, compositionHash string, report DraftCheckReport) DraftCheckReport {
	report.DraftID = draft.ID
	report.DraftRevision = draft.Revision
	report.DraftHash = draft.Hash
	report.CompositionHash = compositionHash
	report.ProjectID = draft.ProjectID
	report.ProjectRevision = draft.ProjectRevision
	report.BaseFileHash = draft.BaseFileHash
	return report
}

func (s *Service) storeDraftCheckReport(identity draftRevisionIdentity, draft Draft, compositionHash string, report DraftCheckReport) (*DraftCheckReport, error) {
	if _, err := s.requireCurrentDraft(identity.id); err != nil {
		return nil, err
	}
	return s.drafts.storeCheckReport(identity, draft, compositionHash, report)
}

// ExpireDraftsForOpenFile marks drafts stale when a UI changes the open file or
// its base hash. Passing the active project identity makes the boundary clear
// to the file-scoped chat session added in the next phase.
func (s *Service) ExpireDraftsForOpenFile(projectID, revision, openPath, baseFileHash string) {
	s.drafts.expireForOpenFile(projectID, revision, openPath, baseFileHash)
}

func (s *Service) refreshDraft(id string) (Draft, error) {
	draft, ok := s.drafts.snapshot(id)
	if !ok {
		return Draft{}, fmt.Errorf("draft not found")
	}
	if draft.State == DraftStale {
		return draft, nil
	}
	if err := s.ValidateMutableRequest(draft.ProjectID, draft.ProjectRevision, draft.TargetPath, draft.BaseFileHash); err != nil {
		stale, exists := s.drafts.markStale(id)
		if !exists {
			return Draft{}, fmt.Errorf("draft not found")
		}
		return stale, nil
	}
	return draft, nil
}

func (s *Service) requireCurrentDraft(id string) (Draft, error) {
	draft, err := s.refreshDraft(id)
	if err != nil {
		return Draft{}, err
	}
	if draft.State == DraftStale {
		return Draft{}, project.ErrRevisionConflict
	}
	return draft, nil
}

func staleDraft(stored *storedDraft) {
	stored.draft.State = DraftStale
	stored.draft.Validation = nil
	stored.draft.CompositionHash = ""
	stored.checks = nil
}

func (s *Service) clearDraftsForProjectChange() {
	s.drafts.clear()
}

func draftHash(declaration string, imports []string) string {
	copy := append([]string(nil), imports...)
	sort.Strings(copy)
	sum := sha256.Sum256([]byte(declaration + "\x00" + strings.Join(copy, "\x00")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneDraft(source Draft) Draft {
	copy := source
	copy.Imports = append([]string(nil), source.Imports...)
	copy.TaskSpec = project.SanitizeBugTaskSpec(source.TaskSpec)
	copy.EngineeringInsight = project.CloneEngineeringInsight(source.EngineeringInsight)
	if source.Validation != nil {
		validation := cloneValidation(*source.Validation)
		copy.Validation = &validation
	}
	return copy
}

func cloneValidation(source project.DeclarationValidation) project.DeclarationValidation {
	copy := source
	copy.Diagnostics = append([]project.DeclarationFinding(nil), source.Diagnostics...)
	copy.Diff.Lines = append([]project.DiffLine(nil), source.Diff.Lines...)
	return copy
}

func cloneCheckReport(source DraftCheckReport) DraftCheckReport {
	copy := source
	copy.Checks = make([]DraftCheck, len(source.Checks))
	for i, check := range source.Checks {
		copy.Checks[i] = check
		copy.Checks[i].Command = append([]string(nil), check.Command...)
	}
	return copy
}
