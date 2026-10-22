package app

import "strings"

// Baseline observations do not replace candidate checks or prove a root cause.
// In particular, missing new APIs can make feature tests fail to compile before
// the implementation exists; that is not a reproduced behavioral regression.
func classifyRegressionBaseline(check *CheckResult) {
	check.Required = false
	if check.State == CheckCanceled {
		check.Outcome = "could_not_execute"
		return
	}
	switch {
	case check.State == CheckPassed && check.ExitCode == 0:
		check.State, check.Outcome = CheckSkipped, "already_passes"
	case strings.Contains(check.Output, "--- FAIL:") && !strings.Contains(check.Output, "[build failed]"):
		check.State, check.Outcome = CheckPassed, "failed_before_change"
	default:
		check.State, check.Outcome = CheckUnavailable, "could_not_execute"
	}
}
