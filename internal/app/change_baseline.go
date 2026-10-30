package app

import "strings"

// classifyRegressionBaseline records what new tests did on the unmodified
// project, from the raw state of that run. Baseline observations do not
// replace candidate checks or prove a root cause. reproduced reports whether a
// failed run shows the new tests themselves failing.
func classifyRegressionBaseline(check *CheckResult, reproduced func(CheckResult) bool) {
	check.Required = false
	switch {
	case check.State == CheckCanceled:
		check.Outcome = "could_not_execute"
	case check.State == CheckPassed && check.ExitCode == 0:
		check.State, check.Outcome = CheckSkipped, "already_passes"
	case check.State == CheckFailed && reproduced(*check):
		check.State, check.Outcome = CheckPassed, "failed_before_change"
	default:
		check.State, check.Outcome = CheckUnavailable, "could_not_execute"
	}
}

// goTestsFailed separates a failing Go test from a package that did not
// build: missing new APIs make feature tests fail to compile before the
// implementation exists, which is not a reproduced behavioral regression.
func goTestsFailed(check CheckResult) bool {
	return strings.Contains(check.Output, "--- FAIL:") && !strings.Contains(check.Output, "[build failed]")
}

// runnerTestsFailed accepts any failure: a task runner reports only its exit
// status, so a failed run is the only evidence available.
func runnerTestsFailed(CheckResult) bool {
	return true
}
