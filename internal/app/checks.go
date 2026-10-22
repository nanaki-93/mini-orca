package app

import (
	"context"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/archgate"
	"github.com/nanaki-93/mini-orca/v2/internal/app/sandbox"
)

const (
	CheckNotRun      = archgate.CheckNotRun
	CheckPassed      = archgate.CheckPassed
	CheckFailed      = archgate.CheckFailed
	CheckSkipped     = archgate.CheckSkipped
	CheckCanceled    = archgate.CheckCanceled
	CheckUnavailable = archgate.CheckUnavailable
)

type CheckResult = archgate.CheckResult
type CheckOptions = archgate.CheckOptions

func (s *Service) runTaskTestCheck(ctx context.Context, workspace, revision, name string, command []string, expectPass bool) CheckResult {
	check, err := s.executeCheck(ctx, workspace, revision, CheckResult{Name: name, Required: true, Command: append([]string(nil), command...)}, true)
	if check.State == CheckCanceled {
		return check
	}
	passed := err == nil
	if passed == expectPass {
		check.State = CheckPassed
	} else {
		check.State = CheckFailed
	}
	return check
}

func parseGoSource(path, source string) CheckResult {
	return sandbox.ParseGoSource(path, source)
}

func (s *Service) runCheck(ctx context.Context, workspace, revision, name string, required bool, command []string, failOnOutput, executesProjectCode bool) CheckResult {
	check, err := s.executeCheck(ctx, workspace, revision, CheckResult{Name: name, Required: required, Command: append([]string(nil), command...)}, executesProjectCode)
	if check.State == CheckCanceled {
		return check
	}
	if err != nil || failOnOutput && check.Output != "" {
		check.State = CheckFailed
		return check
	}
	check.State = CheckPassed
	return check
}

func (s *Service) executeCheck(ctx context.Context, workspace, revision string, check CheckResult, executesProjectCode bool) (CheckResult, error) {
	if err := ctx.Err(); err != nil {
		check.State = CheckCanceled
		check.Output = err.Error()
		return check, err
	}
	started := time.Now()
	timed, cancel := context.WithTimeout(ctx, s.focusedCheckTimeout)
	defer cancel()
	result, err := s.runCheckCommand(timed, workspace, check.Command, revision, executesProjectCode)
	check.Output = sanitizeCheckOutput(result.Output, result.Truncated, workspace, s.manager.Root())
	check.ExitCode = result.ExitCode
	check.DurationMS = time.Since(started).Milliseconds()
	if timed.Err() != nil {
		check.State = CheckCanceled
		check.Output = timed.Err().Error()
		return check, timed.Err()
	}
	return check, err
}

func (s *Service) runCheckCommand(ctx context.Context, workspace string, command []string, revision string, executesProjectCode bool) (sandbox.CommandResult, error) {
	if !executesProjectCode {
		return sandbox.RunCommand(ctx, workspace, command)
	}
	return sandbox.RunCommandWithStart(ctx, workspace, command, true, func() error {
		return s.requireProjectExecutionTrust(revision)
	})
}

func requiredChecksPassed(checks []CheckResult) bool {
	return sandbox.RequiredChecksPassed(checks)
}

func sanitizeCheckOutput(output string, truncated bool, workspace, root string) string {
	return sandbox.SanitizeOutput(output, truncated, workspace, root)
}

func copyCheckWorkspace(ctx context.Context, source, destination string) error {
	return sandbox.CopyWorkspace(ctx, source, destination)
}
