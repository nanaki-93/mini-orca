package project

import "github.com/nanaki-93/mini-orca/v2/internal/project/insight"

func normalizeEmptyFindingsOutput(output string) string {
	return insight.NormalizeEmptyFindingsOutput(output)
}
