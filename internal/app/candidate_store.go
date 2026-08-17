package app

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type storedCandidate struct {
	preview GenerationPreview
	checks  *CandidateCheckReport
}

func (s *Service) rememberCandidate(preview *GenerationPreview) {
	s.candidateMu.Lock()
	defer s.candidateMu.Unlock()
	copy := cloneGenerationPreview(preview)
	s.candidates[preview.GenerationID] = &storedCandidate{preview: copy}
}

// Candidate returns a valid, in-memory candidate. Invalid previews are never
// eligible for checks or Apply.
func (s *Service) Candidate(id string) (*GenerationPreview, error) {
	s.candidateMu.Lock()
	defer s.candidateMu.Unlock()
	candidate := s.candidates[id]
	if candidate == nil {
		return nil, fmt.Errorf("validated candidate not found; generate a new preview")
	}
	preview := cloneGenerationPreview(&candidate.preview)
	return &preview, nil
}

// CheckCandidate runs configured checks for a remembered valid candidate.
func (s *Service) CheckCandidate(ctx context.Context, id string, options CandidateCheckOptions) (*CandidateCheckReport, error) {
	preview, err := s.Candidate(id)
	if err != nil {
		return nil, err
	}
	if err := s.ValidateMutableRequest(preview.ProjectID, preview.ProjectRevision, preview.TargetPath, preview.BaseFileHash); err != nil {
		return nil, err
	}
	report, err := s.RunCandidateChecks(ctx, preview.TargetPath, preview.CandidateContent, options)
	if err != nil {
		return nil, err
	}
	s.candidateMu.Lock()
	if candidate := s.candidates[id]; candidate != nil {
		copy := cloneCheckReport(report)
		candidate.checks = &copy
	}
	s.candidateMu.Unlock()
	return &report, nil
}

func (s *Service) checkedCandidate(id string) (GenerationPreview, CandidateCheckReport, error) {
	s.candidateMu.Lock()
	defer s.candidateMu.Unlock()
	candidate := s.candidates[id]
	if candidate == nil {
		return GenerationPreview{}, CandidateCheckReport{}, fmt.Errorf("validated candidate not found; generate a new preview")
	}
	if candidate.checks == nil {
		return GenerationPreview{}, CandidateCheckReport{}, fmt.Errorf("focused candidate checks have not run")
	}
	return cloneGenerationPreview(&candidate.preview), cloneCheckReport(*candidate.checks), nil
}

// SetCandidateTemplate adds source-free action metadata after the HTTP request
// has been validated. It is retained with the candidate for later audit.
func (s *Service) SetCandidateTemplate(id, action, templateID, inputHash string) {
	s.candidateMu.Lock()
	defer s.candidateMu.Unlock()
	if candidate := s.candidates[id]; candidate != nil {
		candidate.preview.Action = action
		candidate.preview.TemplateID = templateID
		candidate.preview.TemplateInputHash = inputHash
	}
}

func cloneGenerationPreview(source *GenerationPreview) GenerationPreview {
	copy := *source
	copy.ContextManifest.Included = append([]project.ContextFile(nil), source.ContextManifest.Included...)
	copy.ContextManifest.Excluded = append([]project.ContextDecision(nil), source.ContextManifest.Excluded...)
	copy.Validation.Diagnostics = append([]project.GenerationFinding(nil), source.Validation.Diagnostics...)
	copy.Validation.Diff.Lines = append([]project.DiffLine(nil), source.Validation.Diff.Lines...)
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
