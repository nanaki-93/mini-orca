package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func (s *State) agentFleetBreakdown() (total, running, review, failed, toMerge int) {
	items := s.AgentItems()
	total = len(items)
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.Status)) {
		case "running", "queued", "working", "in_progress", "canceling":
			running++
		case "ready", "checked", "reviewed", "awaiting_human_review":
			review++
		case "failed", "changes_requested", "interrupted", "stale":
			failed++
		}
	}
	if s.GitWorkspace != nil {
		for _, wt := range s.GitWorkspace.Worktrees {
			if worktreeStatus(wt) == "TO_MERGE" {
				toMerge++
			}
		}
	}
	return total, running, review, failed, toMerge
}

func appendWrappedText(lines []string, text string, width int) []string {
	for _, row := range theme.WrapVisibleLine(text, max(1, width-4)) {
		lines = append(lines, "  "+row)
	}
	return lines
}

func (s *State) selectedCheck() *Check {
	if s.Scan == nil || len(s.Scan.Phases) == 0 {
		return nil
	}
	return &s.Scan.Phases[s.ClampedIdx()]
}

func (s *State) renderCheckDetails(th ansiTheme, width int) []string {
	check := s.selectedCheck()
	if check == nil {
		return nil
	}
	rows := []string{th.statusBadge(check.State) + " " + client.FirstNonEmpty(check.Name, "Check")}
	rows = append(rows, "Command: "+strings.Join(check.Command, " "))
	if check.DurationMS > 0 {
		rows = append(rows, fmt.Sprintf("Duration: %dms", check.DurationMS))
	}
	if check.Output == "" {
		rows = append(rows, "No diagnostic output recorded.")
	} else {
		for _, line := range strings.Split(check.Output, "\n") {
			rows = append(rows, theme.SanitizeCodeText(line))
		}
	}
	return th.renderPanelBox("SELECTED CHECK", "PgUp/PgDn: details", adaptivePanelWidth(width), rows)
}
