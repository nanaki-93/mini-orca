package project

import "strings"

const (
	BugTaskSpecSchemaVersion = "1"
	MaxBugTaskItems          = 8
	MaxBugTaskItemBytes      = 512
	MaxBugTaskCandidateBytes = 16 * 1024
)

// BugTaskSpec is a bounded model proposal for one exact declaration edit. Its
// target fields are always validated against the deterministic project index.
type BugTaskSpec struct {
	SchemaVersion      string               `json:"schema_version"`
	TargetPath         string               `json:"target_path"`
	TargetSymbol       string               `json:"target_symbol"`
	TargetSignature    string               `json:"target_signature"`
	AcceptanceCriteria []string             `json:"acceptance_criteria"`
	NonGoals           []string             `json:"non_goals"`
	GoTestCandidate    *GoTestCandidateSpec `json:"go_test_candidate,omitempty"`
}

// GoTestCandidateSpec is advisory text only. Mini-Orca never writes or runs it
// until a user has independently reviewed a generated candidate.
type GoTestCandidateSpec struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// SanitizeBugTaskSpec applies the same secret redaction and text normalization
// used by persisted findings while preserving no shared mutable slices.
func SanitizeBugTaskSpec(spec *BugTaskSpec) *BugTaskSpec {
	if spec == nil {
		return nil
	}
	result := &BugTaskSpec{
		SchemaVersion:      strings.TrimSpace(spec.SchemaVersion),
		TargetPath:         strings.TrimSpace(spec.TargetPath),
		TargetSymbol:       strings.TrimSpace(spec.TargetSymbol),
		TargetSignature:    strings.TrimSpace(spec.TargetSignature),
		AcceptanceCriteria: sanitizeBugTaskItems(spec.AcceptanceCriteria),
		NonGoals:           sanitizeBugTaskItems(spec.NonGoals),
	}
	if spec.GoTestCandidate != nil {
		result.GoTestCandidate = &GoTestCandidateSpec{
			Name:    sanitizeFindingText(spec.GoTestCandidate.Name, MaxBugTaskItemBytes),
			Content: sanitizeFindingText(spec.GoTestCandidate.Content, MaxBugTaskCandidateBytes),
		}
	}
	return result
}

func sanitizeBugTaskItems(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, sanitizeFindingText(item, MaxBugTaskItemBytes))
	}
	return result
}

func cloneBugTaskSpec(spec *BugTaskSpec) *BugTaskSpec {
	if spec == nil {
		return nil
	}
	copy := *spec
	copy.AcceptanceCriteria = append([]string(nil), spec.AcceptanceCriteria...)
	copy.NonGoals = append([]string(nil), spec.NonGoals...)
	if spec.GoTestCandidate != nil {
		candidate := *spec.GoTestCandidate
		copy.GoTestCandidate = &candidate
	}
	return &copy
}
