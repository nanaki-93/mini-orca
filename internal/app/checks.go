package app

import (
	"context"

	"github.com/nanaki-93/mini-orca/v2/internal/app/archcheck"
	"github.com/nanaki-93/mini-orca/v2/internal/app/checkexec"
)

const (
	CheckNotRun      = archcheck.CheckNotRun
	CheckPassed      = archcheck.CheckPassed
	CheckFailed      = archcheck.CheckFailed
	CheckSkipped     = archcheck.CheckSkipped
	CheckCanceled    = archcheck.CheckCanceled
	CheckUnavailable = archcheck.CheckUnavailable
)

type CheckResult = archcheck.CheckResult
type CheckOptions = archcheck.CheckOptions

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
	return checkexec.ParseGoSource(path, source)
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
	timed, cancel := context.WithTimeout(ctx, s.focusedCheckTimeout)
	defer cancel()
	result, err := s.runCheckCommand(timed, workspace, check.Command, revision, executesProjectCode)
	check.Output = sanitizeCheckOutput(result.Output, result.Truncated, workspace, s.manager.Root())
	check.ExitCode = result.ExitCode
	if timed.Err() != nil {
		check.State = CheckCanceled
		check.Output = timed.Err().Error()
		return check, timed.Err()
	}
	return check, err
}

func (s *Service) runCheckCommand(ctx context.Context, workspace string, command []string, revision string, executesProjectCode bool) (checkexec.CommandResult, error) {
	if !executesProjectCode {
		return checkexec.RunCommand(ctx, workspace, command)
	}
	return checkexec.RunCommandWithStart(ctx, workspace, command, true, func() error {
		return s.requireProjectExecutionTrust(revision)
	})
}

func checkChildEnvironment(parent []string) []string {
	return checkexec.ChildEnvironment(parent)
}

func requiredChecksPassed(checks []CheckResult) bool {
	return checkexec.RequiredChecksPassed(checks)
}

func sanitizeCheckOutput(output string, truncated bool, workspace, root string) string {
	return checkexec.SanitizeOutput(output, truncated, workspace, root)
}

func copyCheckWorkspace(ctx context.Context, source, destination string) error {
	return checkexec.CopyWorkspace(ctx, source, destination)
}
