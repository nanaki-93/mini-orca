package app

import (
	"context"

	"github.com/nanaki-93/mini-orca/v2/internal/app/archcheck"
)

func checkChangeSecretAndHygiene(edits []ChangeEdit) CheckResult {
	return archcheck.CheckChangeSecretAndHygiene(edits)
}

func (s *Service) runOptionalGitleaksCheck(ctx context.Context, workspace, revision string) (CheckResult, bool) {
	if !archcheck.HasOptionalGitleaks(workspace) {
		return CheckResult{}, false
	}
	check := s.runCheck(ctx, workspace, revision, "gitleaks", true, []string{"gitleaks", "detect", "--no-git", "--source", ".", "--redact"}, false, true)
	return check, true
}
