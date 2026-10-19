package project

import "github.com/nanaki-93/mini-orca/v2/internal/project/insight"

const (
	BugTaskSpecSchemaVersion = insight.BugTaskSpecSchemaVersion
	MaxBugTaskItems          = insight.MaxBugTaskItems
	MaxBugTaskItemBytes      = insight.MaxBugTaskItemBytes
	MaxBugTaskCandidateBytes = insight.MaxBugTaskCandidateBytes
)

type BugTaskSpec = insight.BugTaskSpec
type GoTestCandidateSpec = insight.GoTestCandidateSpec

// SanitizeBugTaskSpec applies the same secret redaction and text normalization
// used by persisted findings while preserving no shared mutable slices.
func SanitizeBugTaskSpec(spec *BugTaskSpec) *BugTaskSpec {
	return insight.SanitizeBugTaskSpec(spec)
}

func cloneBugTaskSpec(spec *BugTaskSpec) *BugTaskSpec {
	return insight.CloneBugTaskSpec(spec)
}
