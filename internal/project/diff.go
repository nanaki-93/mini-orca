package project

import "github.com/nanaki-93/mini-orca/v2/internal/project/diff"

type UnifiedDiff = diff.UnifiedDiff
type DiffLine = diff.DiffLine

// BuildUnifiedDiff aligns unchanged lines between edits and preserves both source line numbers.
func BuildUnifiedDiff(path, original, composed string) UnifiedDiff {
	return diff.BuildUnifiedDiff(path, original, composed)
}
