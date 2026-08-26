package app

import (
	"fmt"
	"regexp"
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
	TargetPath    string `json:"target_path"`
	TargetSymbol  string `json:"target_symbol"`
	Action        string `json:"action"`
	ScopeMode     string `json:"scope_mode"`
	DiffLines     int    `json:"diff_lines"`
	Applicable    bool   `json:"applicable"`
	Checks        string `json:"checks"`
	Model         string `json:"model"`
	UserNote      string `json:"user_note,omitempty"`
}

// ReviewExport is a source-free Markdown artifact. The desktop client writes
// it only after the user explicitly chooses an export destination.
type ReviewExport struct {
	Filename string `json:"filename"`
	Markdown string `json:"markdown"`
}

// CompareCandidates compares exactly two independently validated, preview-only
// candidates. It intentionally does not create, apply, or mutate either one.
func (s *Service) CompareCandidates(leftID, rightID, leftNote, rightNote string) (CandidateComparison, error) {
	if leftID == "" || rightID == "" || leftID == rightID {
		return CandidateComparison{}, fmt.Errorf("two distinct validated candidates are required")
	}
	leftDraft, leftCopy, leftChecks, err := s.candidateReviewState(leftID)
	if err != nil {
		return CandidateComparison{}, fmt.Errorf("both validated candidates are required")
	}
	rightDraft, rightCopy, rightChecks, err := s.candidateReviewState(rightID)
	if err != nil {
		return CandidateComparison{}, fmt.Errorf("both validated candidates are required")
	}
	if leftCopy.ProjectID != rightCopy.ProjectID || leftCopy.ProjectRevision != rightCopy.ProjectRevision || leftCopy.BaseFileHash != rightCopy.BaseFileHash || leftCopy.TargetPath != rightCopy.TargetPath || leftCopy.TargetSymbol != rightCopy.TargetSymbol || leftDraft.Mode != rightDraft.Mode || leftCopy.Action != rightCopy.Action {
		return CandidateComparison{}, fmt.Errorf("candidates must share project revision, base hash, file, symbol, mode, and action")
	}
	if err := s.ValidateMutableRequest(leftCopy.ProjectID, leftCopy.ProjectRevision, leftCopy.TargetPath, leftCopy.BaseFileHash); err != nil {
		return CandidateComparison{}, err
	}
	return CandidateComparison{
		Left:  comparisonItem(&leftCopy, leftChecks, leftNote),
		Right: comparisonItem(&rightCopy, rightChecks, rightNote),
	}, nil
}

func comparisonItem(candidate *GenerationPreview, checksReport *CandidateCheckReport, note string) CandidateComparisonItem {
	checks := "not run"
	if checksReport != nil {
		checks = "failed"
		if checksReport.Applicable {
			checks = "passed"
		}
	}
	return CandidateComparisonItem{
		GenerationID: candidate.GenerationID, CandidateHash: candidate.CandidateHash,
		TargetPath: candidate.TargetPath, TargetSymbol: candidate.TargetSymbol,
		Action: candidate.Action, ScopeMode: string(candidate.ScopeMode),
		DiffLines: len(candidate.Validation.Diff.Lines), Applicable: candidate.Validation.Applicable,
		Checks: checks, Model: candidate.EffectiveModel.Model, UserNote: redactExportText(note),
	}
}

// ExportCandidateReviewMarkdown builds one focused review from the active
// project state. Candidate source, prompt content, and check output stay in
// memory and are never placed in the exported artifact.
func (s *Service) ExportCandidateReviewMarkdown(generationID string) (ReviewExport, error) {
	_, preview, checks, err := s.candidateReviewState(generationID)
	if err != nil {
		return ReviewExport{}, err
	}
	if err := s.ValidateMutableRequest(preview.ProjectID, preview.ProjectRevision, preview.TargetPath, preview.BaseFileHash); err != nil {
		return ReviewExport{}, err
	}

	indexedFile, err := s.manager.IndexedFile(preview.TargetPath)
	if err != nil {
		return ReviewExport{}, err
	}
	var symbol project.SymbolInfo
	for _, item := range indexedFile.Symbols {
		if item.Name == preview.TargetSymbol {
			symbol = item
			break
		}
	}
	if symbol.Name == "" {
		return ReviewExport{}, fmt.Errorf("selected candidate symbol is no longer indexed")
	}
	analysis, err := s.CachedFileAnalysis(preview.TargetPath)
	if err != nil {
		return ReviewExport{}, err
	}
	auditID := ""
	if entries, err := s.AuditHistory(); err == nil {
		for index := len(entries) - 1; index >= 0; index-- {
			if entries[index].GenerationID == generationID {
				auditID = entries[index].ID
				break
			}
		}
	}
	return ReviewExport{
		Filename: reviewExportFilename(preview.TargetPath, preview.TargetSymbol),
		Markdown: ExportReviewMarkdown(analysis, symbol, &preview, checks, auditID),
	}, nil
}

var exportSecret = regexp.MustCompile(`(?i)\b(?:api[_-]?key|access[_-]?token|auth[_-]?token|password|secret|credential)\b\s*[:=]\s*[^\s,;]+`)

// ExportReviewMarkdown is deliberately metadata-only: it omits source, prompts,
// candidate content, and any excluded paths.
func ExportReviewMarkdown(analysis *project.FileAnalysis, symbol project.SymbolInfo, candidate *GenerationPreview, checks *CandidateCheckReport, auditID string) string {
	var out strings.Builder
	out.WriteString("# Mini-Orca focused review\n\n")
	if analysis != nil {
		out.WriteString("## File summary\n\nPath: `" + redactExportText(analysis.Path) + "`\n\nPurpose: " + redactExportText(analysis.Purpose) + "\n\n")
	}
	out.WriteString("## Selected symbol\n\n`" + redactExportText(symbol.Name) + "` · " + redactExportText(symbol.Kind) + "\n\n")
	if analysis != nil && len(analysis.Risks) > 0 {
		out.WriteString("## Findings\n\n")
		for _, risk := range analysis.Risks {
			out.WriteString("- " + redactExportText(risk.Severity) + ": " + redactExportText(risk.Summary) + "\n")
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

func redactExportText(value string) string {
	return exportSecret.ReplaceAllString(value, "[redacted]")
}

func reviewExportFilename(path, symbol string) string {
	name := strings.NewReplacer("/", "-", "\\", "-", ".", "-", " ", "-").Replace(path + "-" + symbol)
	name = strings.Trim(name, "-")
	if name == "" {
		name = "review"
	}
	return "mini-orca-" + name + ".md"
}
