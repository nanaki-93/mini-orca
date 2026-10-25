package tui

import (
	"fmt"
	"strings"
	"time"
)

func workflowElapsed(start, end string) string {
	a, err := time.Parse(time.RFC3339Nano, start)
	if err != nil || a.IsZero() {
		return "not timed"
	}
	b, err := time.Parse(time.RFC3339Nano, end)
	if err != nil || b.IsZero() {
		b = time.Now()
	}
	return max(time.Duration(0), b.Sub(a)).Round(time.Second).String()
}

func workflowTimingSummary(chg *ChangeSession) string {
	if chg == nil || chg.Workflow == nil {
		return ""
	}
	w := chg.Workflow
	end := w.UpdatedAt
	if w.Status == "running" || w.Status == "canceling" {
		end = ""
	}
	mode := "manual repair"
	if w.AutoRepair {
		mode = "up to 2 automatic repairs"
	}
	recovery := "resume: " + firstWorkflowStage(w.ResumeFrom)
	if w.Status == "interrupted" || w.Status == "outdated" || w.Status == "stale" {
		recovery = "restart requires fresh consent and checks"
	}
	return fmt.Sprintf("Attempt %d · elapsed %s · %s · %s", max(1, w.Attempt)+w.AutoRepairAttempts, workflowElapsed(w.StartedAt, end), mode, recovery)
}

func firstWorkflowStage(value string) string {
	if value == "" {
		return "create"
	}
	return value
}

func baselineOutcomeLabel(check Check) string {
	switch check.Outcome {
	case "failed_before_change":
		return "New tests failed before the patch; inspect whether this matches the reported issue."
	case "already_passes":
		return "New tests already passed before the patch; bug reproduction is not established."
	case "could_not_execute":
		return "Behavioral baseline unavailable (for example, compilation failed); reproduction is not established."
	default:
		return ""
	}
}

func (s *State) execRetryCommand(arg string) {
	if s.Change == nil {
		s.Error = "Select a proposal first."
		return
	}
	from := strings.TrimSpace(arg)
	if from == "" {
		from = resumableWorkflowStage(s.Change)
	}
	if from != "create" && from != "test" && from != "review" {
		s.Error = "Use :retry create, :retry test or :retry review."
		return
	}
	s.queueWorkflowRetry(ActionRequest{Kind: "repair-change", Target: s.Change.ID, ResumeFrom: from}, nil)
}
