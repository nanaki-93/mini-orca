package app

import (
	"context"

	"github.com/nanaki-93/mini-orca/v2/internal/app/checkexec"
)

type BootstrapExecStep = checkexec.BootstrapStep
type BootstrapExecResult = checkexec.BootstrapResult

// ExecuteBootstrapToolchain runs `mise trust && mise install` and `lefthook install`
// in the active project root when the corresponding configuration files and binaries exist.
func (s *Service) ExecuteBootstrapToolchain(ctx context.Context, revision string, confirm bool) (*BootstrapExecResult, error) {
	if confirm {
		if _, err := s.TrustProjectExecution(revision, true); err != nil {
			return nil, err
		}
	} else if err := s.requireProjectExecutionTrust(revision); err != nil {
		return nil, err
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	return &BootstrapExecResult{
		ProjectID:       analysis.ProjectID,
		ProjectRevision: analysis.ProjectRevision,
		Steps:           checkexec.RunBootstrapSteps(ctx, s.manager.Root()),
	}, nil
}
