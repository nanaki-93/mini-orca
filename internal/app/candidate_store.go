package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
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
// declaration. The complete candidate is composed only when a caller needs it.
type Draft struct {
	ID              string                        `json:"id"`
	ProjectID       string                        `json:"project_id"`
	ProjectRevision string                        `json:"project_revision"`
	BaseFileHash    string                        `json:"base_file_hash"`
	TargetPath      string                        `json:"target_path"`
	Mode            project.DeclarationEditMode   `json:"mode"`
	TargetSymbol    string                        `json:"target_symbol"`
	Declaration     string                        `json:"declaration"`
	Imports         []string                      `json:"imports,omitempty"`
	Revision        int64                         `json:"revision"`
	Hash            string                        `json:"hash"`
	CandidateHash   string                        `json:"candidate_hash,omitempty"`
	ParentDraftID   string                        `json:"parent_draft_id,omitempty"`
	EffectiveModel  EffectiveModel                `json:"effective_model,omitempty"`
	PreviousHash    string                        `json:"previous_hash,omitempty"`
	State           DraftState                    `json:"state"`
	Validation      *project.GenerationValidation `json:"validation,omitempty"`
	TaskSpec        *project.BugTaskSpec          `json:"task_spec,omitempty"`
}

// DraftCreateRequest supplies immutable project identity and the first
// declaration revision. ParentDraftID connects explicit revision proposals.
type DraftCreateRequest struct {
	ID              string
	ProjectID       string
	ProjectRevision string
	BaseFileHash    string
	TargetPath      string
	Mode            project.DeclarationEditMode
	TargetSymbol    string
	Declaration     string
	Imports         []string
	ParentDraftID   string
	EffectiveModel  EffectiveModel
	TaskSpec        *project.BugTaskSpec
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
	Options          CandidateCheckOptions
}

// DraftReview exposes only the editable declaration and source-free review
// evidence. Complete candidate source is composed transiently by the daemon.
type DraftReview struct {
	Draft         Draft                 `json:"draft"`
	Checks        *CandidateCheckReport `json:"checks,omitempty"`
	ApplyEligible bool                  `json:"apply_eligible"`
}

// DraftAuditMetadata deliberately omits declaration, imports, candidate text,
// prompts, and diagnostics that could contain source.
type DraftAuditMetadata struct {
	ID              string                      `json:"id"`
	ProjectID       string                      `json:"project_id"`
	ProjectRevision string                      `json:"project_revision"`
	TargetPath      string                      `json:"target_path"`
	TargetSymbol    string                      `json:"target_symbol"`
	Mode            project.DeclarationEditMode `json:"mode"`
	Revision        int64                       `json:"revision"`
	Hash            string                      `json:"hash"`
	State           DraftState                  `json:"state"`
	ClearedAt       time.Time                   `json:"cleared_at"`
}

type draftMetadata struct {
	Version           string
	ScopeMode         workflow.ScopeMode
	Rationale         string
	Action            string
	TemplateID        string
	TemplateInputHash string
	EffectiveModel    EffectiveModel
	ContextManifest   project.ContextManifest
}

type draftCheckEvidence struct {
	Revision      int64
	CandidateHash string
	Report        CandidateCheckReport
}

type storedDraft struct {
	draft  Draft
	meta   draftMetadata
	checks *draftCheckEvidence
}

// CreateDraft records an editable declaration without composing or persisting a
// complete candidate. It starts generated; validation is a separate operation.
func (s *Service) CreateDraft(request DraftCreateRequest) (*Draft, error) {
	if request.ID == "" {
		request.ID = newGenerationID()
	}
	if request.ProjectID == "" || request.ProjectRevision == "" || request.BaseFileHash == "" || request.TargetPath == "" || request.TargetSymbol == "" {
		return nil, fmt.Errorf("draft base project, file, and target identity is required")
	}
	if err := s.ValidateMutableRequest(request.ProjectID, request.ProjectRevision, request.TargetPath, request.BaseFileHash); err != nil {
		return nil, err
	}
	draft := Draft{
		ID: request.ID, ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision,
		BaseFileHash: request.BaseFileHash, TargetPath: request.TargetPath, Mode: request.Mode,
		TargetSymbol: request.TargetSymbol, Declaration: request.Declaration, Imports: append([]string(nil), request.Imports...),
		Revision: 1, ParentDraftID: request.ParentDraftID, EffectiveModel: request.EffectiveModel, TaskSpec: project.SanitizeBugTaskSpec(request.TaskSpec), State: DraftGenerated,
	}
	draft.Hash = draftHash(draft.Declaration, draft.Imports)
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	if _, exists := s.drafts[draft.ID]; exists {
		return nil, fmt.Errorf("draft already exists")
	}
	if draft.ParentDraftID != "" {
		if _, exists := s.drafts[draft.ParentDraftID]; !exists {
			return nil, fmt.Errorf("parent draft not found")
		}
	}
	s.drafts[draft.ID] = &storedDraft{draft: cloneDraft(draft)}
	copy := cloneDraft(draft)
	return &copy, nil
}

// Draft returns a copy and marks it stale if its pinned project/file identity
// no longer matches the active project.
func (s *Service) Draft(id string) (*Draft, error) {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	stored := s.drafts[id]
	if stored == nil {
		return nil, fmt.Errorf("draft not found")
	}
	s.expireDraftLocked(stored)
	copy := cloneDraft(stored.draft)
	return &copy, nil
}

// UpdateDraft creates the next revision only if the caller saw the current
// revision. Any content/import change removes all previous approval evidence.
func (s *Service) UpdateDraft(request DraftUpdateRequest) (*Draft, error) {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	stored := s.drafts[request.ID]
	if stored == nil {
		return nil, fmt.Errorf("draft not found")
	}
	s.expireDraftLocked(stored)
	if stored.draft.State == DraftStale {
		return nil, project.ErrRevisionConflict
	}
	if request.ExpectedRevision != stored.draft.Revision {
		return nil, project.ErrRevisionConflict
	}
	stored.draft.PreviousHash = stored.draft.Hash
	stored.draft.Declaration = request.Declaration
	stored.draft.Imports = append([]string(nil), request.Imports...)
	stored.draft.Revision++
	stored.draft.Hash = draftHash(stored.draft.Declaration, stored.draft.Imports)
	stored.draft.Validation = nil
	stored.draft.CandidateHash = ""
	stored.draft.State = DraftDirty
	stored.checks = nil
	copy := cloneDraft(stored.draft)
	return &copy, nil
}

// BeginDraftValidation reserves the displayed revision for validation. A
// completion for an older revision is rejected and cannot authorize an edit.
func (s *Service) BeginDraftValidation(id string, expectedRevision int64) (*Draft, error) {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	stored := s.drafts[id]
	if stored == nil {
		return nil, fmt.Errorf("draft not found")
	}
	s.expireDraftLocked(stored)
	if stored.draft.State == DraftStale || stored.draft.Revision != expectedRevision {
		return nil, project.ErrRevisionConflict
	}
	stored.draft.State = DraftValidating
	stored.draft.Validation = nil
	stored.draft.CandidateHash = ""
	stored.checks = nil
	copy := cloneDraft(stored.draft)
	return &copy, nil
}

// CompleteDraftValidation stores validation only for the revision that began
// it. Invalid drafts retain their editable declaration for correction.
func (s *Service) CompleteDraftValidation(id string, expectedRevision int64, validation project.GenerationValidation) (*Draft, error) {
	return s.completeDraftValidation(id, expectedRevision, validation, "")
}

func (s *Service) completeDraftValidation(id string, expectedRevision int64, validation project.GenerationValidation, candidateHash string) (*Draft, error) {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	stored := s.drafts[id]
	if stored == nil {
		return nil, fmt.Errorf("draft not found")
	}
	s.expireDraftLocked(stored)
	if stored.draft.State == DraftStale || stored.draft.Revision != expectedRevision || stored.draft.State != DraftValidating {
		return nil, project.ErrRevisionConflict
	}
	copyValidation := cloneValidation(validation)
	stored.draft.Validation = &copyValidation
	if validation.Applicable {
		stored.draft.State = DraftValid
		stored.draft.CandidateHash = candidateHash
	} else {
		stored.draft.State = DraftInvalid
		stored.draft.CandidateHash = ""
	}
	copy := cloneDraft(stored.draft)
	return &copy, nil
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
	return s.completeDraftValidation(id, expectedRevision, composition.Validation, composition.CandidateHash)
}

func (s *Service) finishDraftValidationAfterError(id string, expectedRevision int64, cause error) error {
	if _, err := s.CompleteDraftValidation(id, expectedRevision, project.GenerationValidation{}); err != nil {
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
func (s *Service) CheckDraft(ctx context.Context, request DraftCheckRequest) (*CandidateCheckReport, error) {
	s.draftMu.Lock()
	stored := s.drafts[request.ID]
	if stored == nil {
		s.draftMu.Unlock()
		return nil, fmt.Errorf("draft not found")
	}
	s.expireDraftLocked(stored)
	if stored.draft.State == DraftStale || stored.draft.Revision != request.ExpectedRevision || stored.draft.Hash != request.ExpectedHash {
		s.draftMu.Unlock()
		return nil, project.ErrRevisionConflict
	}
	if stored.draft.State != DraftValid || stored.draft.Validation == nil || !stored.draft.Validation.Applicable || stored.draft.CandidateHash == "" {
		s.draftMu.Unlock()
		return nil, fmt.Errorf("draft is not valid; validate the latest revision")
	}
	draft := cloneDraft(stored.draft)
	s.draftMu.Unlock()

	composition, err := s.composeDraft(draft)
	if err != nil {
		return nil, err
	}
	if !composition.Validation.Applicable || composition.CandidateHash != draft.CandidateHash {
		return nil, project.ErrRevisionConflict
	}
	var taskTest *project.GoTestCandidateSpec
	if draft.TaskSpec != nil {
		taskTest = draft.TaskSpec.GoTestCandidate
	}
	report, err := s.RunCandidateChecksForTask(ctx, draft.TargetPath, composition.CandidateContent, request.Options, taskTest)
	if err != nil {
		return nil, err
	}
	report.DraftID = draft.ID
	report.DraftRevision = draft.Revision
	report.DraftHash = draft.Hash
	report.CandidateHash = composition.CandidateHash
	report.ProjectID = draft.ProjectID
	report.ProjectRevision = draft.ProjectRevision
	report.BaseFileHash = draft.BaseFileHash

	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	stored = s.drafts[request.ID]
	if stored == nil {
		return nil, project.ErrRevisionConflict
	}
	s.expireDraftLocked(stored)
	if stored.draft.State != DraftValid || stored.draft.Revision != draft.Revision || stored.draft.Hash != draft.Hash || stored.draft.CandidateHash != composition.CandidateHash {
		return nil, project.ErrRevisionConflict
	}
	stored.checks = &draftCheckEvidence{Revision: draft.Revision, CandidateHash: composition.CandidateHash, Report: cloneCheckReport(report)}
	copy := cloneCheckReport(report)
	return &copy, nil
}

// ReviewDraft returns the latest source-free state, including whether the
// exact validated and checked revision can be explicitly applied.
func (s *Service) ReviewDraft(id string) (*DraftReview, error) {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	stored := s.drafts[id]
	if stored == nil {
		return nil, fmt.Errorf("draft not found")
	}
	s.expireDraftLocked(stored)
	review := &DraftReview{Draft: cloneDraft(stored.draft)}
	if stored.checks != nil && stored.checks.Revision == stored.draft.Revision && stored.checks.CandidateHash == stored.draft.CandidateHash {
		checks := cloneCheckReport(stored.checks.Report)
		review.Checks = &checks
		review.ApplyEligible = stored.draft.State == DraftValid && stored.draft.Validation != nil && stored.draft.Validation.Applicable && checks.Applicable
	}
	return review, nil
}

// ExpireDraftsForOpenFile marks drafts stale when a UI changes the open file or
// its base hash. Passing the active project identity makes the boundary clear
// to the file-scoped chat session added in the next phase.
func (s *Service) ExpireDraftsForOpenFile(projectID, revision, openPath, baseFileHash string) {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	for _, stored := range s.drafts {
		if stored.draft.ProjectID != projectID || stored.draft.ProjectRevision != revision || (openPath != "" && stored.draft.TargetPath != openPath) || (baseFileHash != "" && stored.draft.BaseFileHash != baseFileHash) {
			staleDraft(stored)
		}
	}
}

// DraftAuditMetadata returns only source-free records left after a project
// change. The draft itself is intentionally unavailable after the switch.
func (s *Service) DraftAuditMetadata() []DraftAuditMetadata {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	return append([]DraftAuditMetadata(nil), s.draftAudit...)
}

func (s *Service) expireDraftLocked(stored *storedDraft) {
	if stored.draft.State == DraftStale {
		return
	}
	if err := s.ValidateMutableRequest(stored.draft.ProjectID, stored.draft.ProjectRevision, stored.draft.TargetPath, stored.draft.BaseFileHash); err != nil {
		staleDraft(stored)
	}
}

func staleDraft(stored *storedDraft) {
	stored.draft.State = DraftStale
	stored.draft.Validation = nil
	stored.draft.CandidateHash = ""
	stored.checks = nil
}

func (s *Service) clearDraftsForProjectChange() {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	now := time.Now().UTC()
	for _, stored := range s.drafts {
		draft := stored.draft
		s.draftAudit = append(s.draftAudit, DraftAuditMetadata{ID: draft.ID, ProjectID: draft.ProjectID, ProjectRevision: draft.ProjectRevision, TargetPath: draft.TargetPath, TargetSymbol: draft.TargetSymbol, Mode: draft.Mode, Revision: draft.Revision, Hash: draft.Hash, State: draft.State, ClearedAt: now})
	}
	s.drafts = make(map[string]*storedDraft)
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
	if source.Validation != nil {
		validation := cloneValidation(*source.Validation)
		copy.Validation = &validation
	}
	return copy
}

func cloneValidation(source project.GenerationValidation) project.GenerationValidation {
	copy := source
	copy.Diagnostics = append([]project.GenerationFinding(nil), source.Diagnostics...)
	copy.Diff.Lines = append([]project.DiffLine(nil), source.Diff.Lines...)
	return copy
}

// The candidate methods below preserve the existing API until Task 40 moves
// its routes to revisioned draft operations. They read candidate source only
// transiently from a declaration draft; no complete candidate is stored.
func (s *Service) rememberCandidate(preview *GenerationPreview) {
	request, err := s.draftRequestFromPreview(preview)
	if err != nil {
		return
	}
	request.ID = preview.GenerationID
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	validation := cloneValidation(preview.Validation)
	draft := Draft{ID: request.ID, ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, BaseFileHash: request.BaseFileHash, TargetPath: request.TargetPath, Mode: request.Mode, TargetSymbol: request.TargetSymbol, Declaration: request.Declaration, Imports: append([]string(nil), request.Imports...), Revision: 1, ParentDraftID: request.ParentDraftID, EffectiveModel: request.EffectiveModel, State: DraftValid, Validation: &validation}
	draft.Hash = draftHash(draft.Declaration, draft.Imports)
	s.drafts[draft.ID] = &storedDraft{draft: draft, meta: draftMetadata{Version: preview.Version, ScopeMode: preview.ScopeMode, Rationale: preview.Rationale, Action: preview.Action, TemplateID: preview.TemplateID, TemplateInputHash: preview.TemplateInputHash, EffectiveModel: preview.EffectiveModel, ContextManifest: cloneManifest(preview.ContextManifest)}}
}

func (s *Service) Candidate(id string) (*GenerationPreview, error) {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	stored := s.drafts[id]
	if stored == nil {
		return nil, fmt.Errorf("validated candidate not found; generate a new preview")
	}
	s.expireDraftLocked(stored)
	if stored.draft.State == DraftStale {
		return nil, project.ErrRevisionConflict
	}
	if stored.draft.State != DraftValid || stored.draft.Validation == nil || !stored.draft.Validation.Applicable {
		return nil, fmt.Errorf("draft is not valid; validate the latest revision")
	}
	preview, err := s.composePreviewLocked(stored)
	if err != nil {
		staleDraft(stored)
		return nil, err
	}
	return preview, nil
}

func (s *Service) CheckCandidate(ctx context.Context, id string, options CandidateCheckOptions) (*CandidateCheckReport, error) {
	draft, err := s.Draft(id)
	if err != nil {
		return nil, err
	}
	if draft.CandidateHash == "" {
		preview, err := s.Candidate(id)
		if err != nil {
			return nil, err
		}
		draft.CandidateHash = preview.CandidateHash
		s.draftMu.Lock()
		if stored := s.drafts[id]; stored != nil && stored.draft.Revision == draft.Revision && stored.draft.Hash == draft.Hash {
			stored.draft.CandidateHash = draft.CandidateHash
		}
		s.draftMu.Unlock()
	}
	return s.CheckDraft(ctx, DraftCheckRequest{ID: id, ExpectedRevision: draft.Revision, ExpectedHash: draft.Hash, Options: options})
}

func (s *Service) checkedCandidate(id string) (GenerationPreview, CandidateCheckReport, error) {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	stored := s.drafts[id]
	if stored == nil {
		return GenerationPreview{}, CandidateCheckReport{}, fmt.Errorf("validated candidate not found; generate a new preview")
	}
	s.expireDraftLocked(stored)
	if stored.draft.State == DraftStale {
		return GenerationPreview{}, CandidateCheckReport{}, project.ErrRevisionConflict
	}
	if stored.draft.State != DraftValid || stored.draft.Validation == nil || !stored.draft.Validation.Applicable {
		return GenerationPreview{}, CandidateCheckReport{}, fmt.Errorf("draft is not valid; validate the latest revision")
	}
	preview, err := s.composePreviewLocked(stored)
	if err != nil {
		return GenerationPreview{}, CandidateCheckReport{}, err
	}
	if stored.checks == nil || stored.checks.Revision != stored.draft.Revision || stored.checks.CandidateHash != preview.CandidateHash {
		return GenerationPreview{}, CandidateCheckReport{}, fmt.Errorf("focused candidate checks have not run for the latest draft revision")
	}
	return *preview, cloneCheckReport(stored.checks.Report), nil
}

func (s *Service) candidateReviewState(id string) (Draft, GenerationPreview, *CandidateCheckReport, error) {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	stored := s.drafts[id]
	if stored == nil {
		return Draft{}, GenerationPreview{}, nil, fmt.Errorf("validated candidate not found; generate a new preview")
	}
	s.expireDraftLocked(stored)
	if stored.draft.State == DraftStale {
		return Draft{}, GenerationPreview{}, nil, project.ErrRevisionConflict
	}
	if stored.draft.State != DraftValid || stored.draft.Validation == nil || !stored.draft.Validation.Applicable {
		return Draft{}, GenerationPreview{}, nil, fmt.Errorf("draft is not valid; validate the latest revision")
	}
	preview, err := s.composePreviewLocked(stored)
	if err != nil {
		return Draft{}, GenerationPreview{}, nil, err
	}
	var checks *CandidateCheckReport
	if stored.checks != nil && stored.checks.Revision == stored.draft.Revision && stored.checks.CandidateHash == preview.CandidateHash {
		copy := cloneCheckReport(stored.checks.Report)
		checks = &copy
	}
	return cloneDraft(stored.draft), *preview, checks, nil
}

func (s *Service) SetCandidateTemplate(id, action, templateID, inputHash string) {
	s.draftMu.Lock()
	defer s.draftMu.Unlock()
	if stored := s.drafts[id]; stored != nil {
		stored.meta.Action = action
		stored.meta.TemplateID = templateID
		stored.meta.TemplateInputHash = inputHash
	}
}

func (s *Service) composePreviewLocked(stored *storedDraft) (*GenerationPreview, error) {
	path, err := project.ResolveFile(s.manager.Root(), stored.draft.TargetPath)
	if err != nil {
		return nil, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	composition := project.ComposeGoDeclaration(stored.draft.TargetPath, string(original), project.GoDeclarationEdit{Mode: stored.draft.Mode, TargetSymbol: stored.draft.TargetSymbol, Declaration: stored.draft.Declaration, Imports: stored.draft.Imports})
	if !composition.Validation.Applicable {
		return nil, fmt.Errorf("draft no longer composes as a focused declaration")
	}
	validation := cloneValidation(*stored.draft.Validation)
	return &GenerationPreview{GenerationID: stored.draft.ID, Version: stored.meta.Version, ProjectID: stored.draft.ProjectID, ProjectRevision: stored.draft.ProjectRevision, BaseFileHash: stored.draft.BaseFileHash, TargetPath: stored.draft.TargetPath, TargetSymbol: stored.draft.TargetSymbol, ScopeMode: stored.meta.ScopeMode, CandidateContent: composition.CandidateContent, CandidateHash: composition.CandidateHash, Rationale: stored.meta.Rationale, Action: stored.meta.Action, TemplateID: stored.meta.TemplateID, TemplateInputHash: stored.meta.TemplateInputHash, EffectiveModel: stored.meta.EffectiveModel, ContextManifest: cloneManifest(stored.meta.ContextManifest), Validation: validation}, nil
}

func (s *Service) draftRequestFromPreview(preview *GenerationPreview) (DraftCreateRequest, error) {
	declaration, imports, err := extractDeclarationEdit(preview.CandidateContent, preview.TargetSymbol)
	if err != nil {
		return DraftCreateRequest{}, err
	}
	return DraftCreateRequest{ProjectID: preview.ProjectID, ProjectRevision: preview.ProjectRevision, BaseFileHash: preview.BaseFileHash, TargetPath: preview.TargetPath, Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: preview.TargetSymbol, Declaration: declaration, Imports: imports, EffectiveModel: preview.EffectiveModel}, nil
}

func extractDeclarationEdit(source, target string) (string, []string, error) {
	if declaration, ok := standaloneDeclaration(source, target); ok {
		return declaration, nil, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "candidate.go", source, parser.ParseComments)
	if err != nil {
		return "", nil, err
	}
	for _, declaration := range file.Decls {
		if declarationTargetName(declaration) != target {
			continue
		}
		var out bytes.Buffer
		if err := format.Node(&out, fset, declaration); err != nil {
			return "", nil, err
		}
		return strings.TrimSpace(out.String()), renderedImports(fset, file), nil
	}
	return "", nil, fmt.Errorf("candidate does not contain the requested declaration")
}

func standaloneDeclaration(source, target string) (string, bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "declaration.go", "package declaration\n\n"+source, parser.ParseComments)
	if err != nil || len(file.Decls) != 1 || declarationTargetName(file.Decls[0]) != target {
		return "", false
	}
	var out bytes.Buffer
	if format.Node(&out, fset, file.Decls[0]) != nil {
		return "", false
	}
	return strings.TrimSpace(out.String()), true
}

func declarationTargetName(declaration ast.Decl) string {
	function, ok := declaration.(*ast.FuncDecl)
	if ok {
		if function.Recv == nil || len(function.Recv.List) == 0 {
			return function.Name.Name
		}
		return receiverTypeName(function.Recv.List[0].Type) + "." + function.Name.Name
	}
	group, ok := declaration.(*ast.GenDecl)
	if ok && len(group.Specs) == 1 {
		if group.Tok == token.TYPE {
			if typeSpec, ok := group.Specs[0].(*ast.TypeSpec); ok {
				return typeSpec.Name.Name
			}
		}
		if group.Tok == token.VAR && !group.Lparen.IsValid() {
			if variable, ok := group.Specs[0].(*ast.ValueSpec); ok && len(variable.Names) == 1 {
				return variable.Names[0].Name
			}
		}
	}
	return ""
}

func receiverTypeName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverTypeName(value.X)
	case *ast.ParenExpr:
		return receiverTypeName(value.X)
	default:
		return ""
	}
}

func renderedImports(fset *token.FileSet, file *ast.File) []string {
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		var out bytes.Buffer
		if format.Node(&out, fset, spec) == nil {
			imports = append(imports, out.String())
		}
	}
	return imports
}

func cloneManifest(source project.ContextManifest) project.ContextManifest {
	copy := source
	copy.Included = append([]project.ContextFile(nil), source.Included...)
	copy.Excluded = append([]project.ContextDecision(nil), source.Excluded...)
	return copy
}

func cloneGenerationPreview(source *GenerationPreview) GenerationPreview {
	copy := *source
	copy.ContextManifest = cloneManifest(source.ContextManifest)
	copy.Validation = cloneValidation(source.Validation)
	return copy
}

func cloneCheckReport(source CandidateCheckReport) CandidateCheckReport {
	copy := source
	copy.Checks = make([]CandidateCheck, len(source.Checks))
	for i, check := range source.Checks {
		copy.Checks[i] = check
		copy.Checks[i].Command = append([]string(nil), check.Command...)
	}
	return copy
}
