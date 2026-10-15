package project

import (
	"fmt"
	"io/fs"
	"strings"
	"unicode"
)

// FindingTarget identifies the original evidence across an isolated checkout.
// Kind identifies its producer; Source distinguishes the two security producers.
type FindingTarget struct {
	ProjectID string `json:"project_id"`
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Path      string `json:"path"`
	Source    string `json:"source,omitempty"`
}

type FindingCompletion struct {
	FindingTarget
	CommitOID string `json:"commit_oid"`
}

func (target FindingTarget) Validate() error {
	for _, value := range []string{target.ProjectID, target.ID, target.Path} {
		if value == "" || len(value) > 1024 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return fmt.Errorf("finding target requires bounded project, finding and path identities")
		}
	}
	if !fs.ValidPath(target.Path) || target.Path == "." || strings.Contains(target.Path, "\\") {
		return fmt.Errorf("finding target path must be project-relative")
	}
	switch target.Kind {
	case "semantic", "performance":
		if target.Source == "" {
			return nil
		}
	case "security":
		if target.Source == SecuritySourceAI || target.Source == SecuritySourceDeterministic {
			return nil
		}
	}
	return fmt.Errorf("finding target producer is invalid")
}

func (s *FindingStore) Completions(projectID string) ([]FindingCompletion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.loadLocked(false)
	if err != nil {
		return nil, err
	}
	result := []FindingCompletion{}
	for _, completion := range document.Completed {
		if completion.ProjectID == projectID {
			result = append(result, completion)
		}
	}
	return result, nil
}

// Complete records merged commits atomically with semantic finding triage. It
// retains producer identities so completion cannot imply verified evidence.
func (s *FindingStore) Complete(completions []FindingCompletion) error {
	if len(completions) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.loadLocked(false)
	if err != nil {
		return err
	}
	seen := make(map[FindingTarget]bool, len(document.Completed))
	for _, completion := range document.Completed {
		seen[completion.FindingTarget] = true
	}
	for _, completion := range completions {
		if err := completion.FindingTarget.Validate(); err != nil {
			return err
		}
		if completion.CommitOID == "" {
			return fmt.Errorf("finding completion requires a commit")
		}
		if !seen[completion.FindingTarget] {
			document.Completed = append(document.Completed, completion)
			seen[completion.FindingTarget] = true
		}
	}
	applyFindingCompletions(document)
	return s.storeLocked(document)
}

func (s *FindingStore) SetTargetStatus(target FindingTarget, status, commitOID string) error {
	if err := target.Validate(); err != nil {
		return err
	}
	if !validFindingStatus(status) || status == FindingStatusRetired {
		return fmt.Errorf("finding status is invalid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.loadLocked(true)
	if err != nil {
		return err
	}
	setDocumentTargetCompletion(document, target, status, commitOID)
	if target.Kind == "semantic" {
		for index := range document.Findings {
			if document.Findings[index].ID == target.ID && document.Findings[index].ProjectID == target.ProjectID {
				document.Findings[index].Status = status
			}
		}
	}
	applyFindingCompletions(document)
	return s.storeLocked(document)
}

func setDocumentTargetCompletion(document *findingDocument, target FindingTarget, status, commitOID string) {
	if status == FindingStatusFixed {
		if strings.TrimSpace(commitOID) == "" {
			commitOID = "manual"
		}
		if !hasFindingCompletion(document.Completed, target) {
			document.Completed = append(document.Completed, FindingCompletion{FindingTarget: target, CommitOID: commitOID})
		}
		return
	}
	retained := document.Completed[:0]
	for _, completion := range document.Completed {
		if completion.FindingTarget != target {
			retained = append(retained, completion)
		}
	}
	document.Completed = retained
}

func updateSemanticCompletion(document *findingDocument, finding UnifiedFinding, status string) {
	if status == FindingStatusFixed && finding.Location.Path != "" {
		target := FindingTarget{ProjectID: finding.ProjectID, Kind: "semantic", ID: finding.ID, Path: finding.Location.Path}
		if target.Validate() == nil {
			setDocumentTargetCompletion(document, target, status, "manual")
		}
		return
	}
	retained := document.Completed[:0]
	for _, completion := range document.Completed {
		if completion.Kind != "semantic" || completion.ID != finding.ID || completion.ProjectID != finding.ProjectID {
			retained = append(retained, completion)
		}
	}
	document.Completed = retained
}

func hasFindingCompletion(completed []FindingCompletion, target FindingTarget) bool {
	for _, completion := range completed {
		if completion.FindingTarget == target {
			return true
		}
	}
	return false
}

func applyFindingCompletions(document *findingDocument) {
	completed := make(map[FindingTarget]bool, len(document.Completed))
	for _, completion := range document.Completed {
		completed[completion.FindingTarget] = true
	}
	for i := range document.Findings {
		finding := &document.Findings[i]
		if completed[FindingTarget{ProjectID: finding.ProjectID, Kind: "semantic", ID: finding.ID, Path: finding.Location.Path}] {
			finding.Status = FindingStatusFixed
		}
	}
}
