package app

import "github.com/nanaki-93/mini-orca/v2/internal/app/archcheck"

func changeArchitectureAndComplexityChecks(root string, edits []ChangeEdit) []CheckResult {
	return archcheck.ChangeArchitectureAndComplexityChecks(root, edits)
}
