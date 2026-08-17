package app

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type CandidateComparison struct {
	Left  CandidateComparisonItem `json:"left"`
	Right CandidateComparisonItem `json:"right"`
}

type CandidateComparisonItem struct {
	GenerationID  string `json:"generation_id"`
	CandidateHash string `json:"candidate_hash"`
	DiffLines     int    `json:"diff_lines"`
	Applicable    bool   `json:"applicable"`
	Checks        string `json:"checks"`
	Model         string `json:"model"`
}

func (s *Service) CompareCandidates(leftID, rightID string) (CandidateComparison, error) {
	s.candidateMu.Lock()
	defer s.candidateMu.Unlock()
	leftStored, rightStored := s.candidates[leftID], s.candidates[rightID]
	if leftStored == nil || rightStored == nil {
		return CandidateComparison{}, fmt.Errorf("both validated candidates are required")
	}
	left, right := &leftStored.preview, &rightStored.preview
	if left.ProjectID != right.ProjectID || left.ProjectRevision != right.ProjectRevision || left.BaseFileHash != right.BaseFileHash || left.TargetPath != right.TargetPath || left.TargetSymbol != right.TargetSymbol || left.Action != right.Action {
		return CandidateComparison{}, fmt.Errorf("candidates must share project revision, base hash, file, symbol, and action")
	}
	return CandidateComparison{Left: comparisonItem(left, leftStored.checks), Right: comparisonItem(right, rightStored.checks)}, nil
}

func comparisonItem(candidate *GenerationPreview, checksReport *CandidateCheckReport) CandidateComparisonItem {
	checks := "not run"
	if checksReport != nil {
		checks = "failed"
		if checksReport.Applicable {
			checks = "passed"
		}
	}
	return CandidateComparisonItem{GenerationID: candidate.GenerationID, CandidateHash: candidate.CandidateHash, DiffLines: len(candidate.Validation.Diff.Lines), Applicable: candidate.Validation.Applicable, Checks: checks, Model: candidate.EffectiveModel.Model}
}

// ExportReviewMarkdown is deliberately metadata-only: it omits source, prompts,
// candidate content, and any excluded paths.
func ExportReviewMarkdown(analysis *project.FileAnalysis, symbol project.SymbolInfo, candidate *GenerationPreview, checks *CandidateCheckReport, auditID string) string {
	var out strings.Builder
	out.WriteString("# Mini-Orca focused review\n\n")
	if analysis != nil {
		out.WriteString("## File summary\n\nPath: `" + analysis.Path + "`\n\nPurpose: " + analysis.Purpose + "\n\n")
	}
	out.WriteString("## Selected symbol\n\n`" + symbol.Name + "` · " + symbol.Kind + "\n\n")
	if analysis != nil && len(analysis.Risks) > 0 {
		out.WriteString("## Findings\n\n")
		for _, risk := range analysis.Risks {
			out.WriteString("- " + risk.Severity + ": " + risk.Summary + "\n")
		}
		out.WriteString("\n")
	}
	if candidate != nil {
		out.WriteString("## Candidate metadata\n\n- Hash: `" + candidate.CandidateHash + "`\n- Scope valid: " + fmt.Sprint(candidate.Validation.Applicable) + "\n- Diff lines: " + fmt.Sprint(len(candidate.Validation.Diff.Lines)) + "\n\n")
	}
	if checks != nil {
		out.WriteString("## Checks\n\n")
		for _, check := range checks.Checks {
			out.WriteString("- " + check.Name + ": " + check.State + "\n")
		}
		out.WriteString("\n")
	}
	if auditID != "" {
		out.WriteString("Audit reference: `" + auditID + "`\n")
	}
	return out.String()
}
