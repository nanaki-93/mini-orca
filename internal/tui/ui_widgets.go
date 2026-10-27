package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

// taskTone is how a task reads in lists: a glyph, a short label and a color.
type taskTone struct {
	glyph, label, color string
	group               string
}

// Task groups, in the order lists show them.
const (
	groupNeedsYou = "Needs you"
	groupWorking  = "Agents working"
	groupFailed   = "Failed or stale"
	groupDrafts   = "Drafts"
	groupDone     = "Done"
)

func (s *State) toneForTask(th ansiTheme, status string) taskTone {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "queued", "working", "in_progress":
		return taskTone{s.spinner(), "Agents working", th.amber, groupWorking}
	case "canceling":
		return taskTone{s.spinner(), "Canceling", th.amber, groupWorking}
	case "ready", "checked", "reviewed", "awaiting_human_review":
		return taskTone{"◉", "Needs review", th.accent, groupNeedsYou}
	case "changes_requested":
		return taskTone{"◉", "Changes requested", th.amber, groupNeedsYou}
	case "failed", "interrupted", "error":
		return taskTone{"✖", "Failed", th.red, groupFailed}
	case "stale", "outdated":
		return taskTone{"◌", "Stale", th.amber, groupFailed}
	case "applied", "done", "committed", "merged":
		return taskTone{"✔", "Applied", th.green, groupDone}
	case "canceled", "reverted", "undone":
		return taskTone{"⊘", strings.ToUpper(status[:1]) + status[1:], th.muted, groupDone}
	default:
		return taskTone{"○", "Draft", th.muted, groupDrafts}
	}
}

// stageGlyph marks a workflow stage's state.
func (s *State) stageGlyph(th ansiTheme, state domain.StageState) string {
	switch state {
	case domain.StageCompleted:
		return th.green + "✔" + th.reset
	case domain.StageRunning:
		return th.amber + s.spinner() + th.reset
	case domain.StageWaiting:
		return th.accent + "◉" + th.reset
	case domain.StageFailed, domain.StageInterrupted:
		return th.red + "✖" + th.reset
	case domain.StageStale, domain.StageBlocked:
		return th.amber + "◌" + th.reset
	case domain.StageCanceled, domain.StageSkipped:
		return th.muted + "⊘" + th.reset
	default:
		return th.muted + "○" + th.reset
	}
}

// checkGlyph marks a check's state.
func (th ansiTheme) checkGlyph(state string) string {
	switch phaseTone(state) {
	case "passed":
		return th.green + "✔" + th.reset
	case "failed":
		return th.red + "✖" + th.reset
	case "running":
		return th.amber + "●" + th.reset
	default:
		return th.muted + "○" + th.reset
	}
}

// phaseTone normalizes check, stage and verdict states into four tones.
func phaseTone(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "passed", "completed", "completed_empty", "fresh", "approve", "approved", "checked", "reviewed", "applied", "committed", "merged", "verified":
		return "passed"
	case "failed", "error", "rejected", "changes_requested", "stale", "outdated", "interrupted", "conflicts", "blocked":
		return "failed"
	case "running", "in_progress", "working", "queued", "canceling", "pausing", "pending_review", "waiting":
		return "running"
	default:
		return "idle"
	}
}

// gateRow is one condition Apply depends on.
func (th ansiTheme) gateRow(done bool, active bool, label, detail string, width int) string {
	mark := th.muted + "○" + th.reset
	if done {
		mark = th.green + "✔" + th.reset
	} else if active {
		mark = th.accent + "◉" + th.reset
	}
	labelStyle := th.fg
	if active {
		labelStyle = th.bold + th.fg
	}
	return mark + " " + labelStyle + theme.PadVisibleRight(label, 18) + th.reset + th.muted + theme.TruncatePlain(detail, max(8, width-22)) + th.reset
}

// diffStat renders "+12 −3" with colors.
func (th ansiTheme) diffStat(added, removed int) string {
	if added == 0 && removed == 0 {
		return ""
	}
	return th.green + fmt.Sprintf("+%d", added) + th.reset + " " + th.red + fmt.Sprintf("−%d", removed) + th.reset
}

// rightAlign places right at the end of a row of width columns after left.
func rightAlign(left, right string, width int) string {
	rw := theme.VisibleRuneCount(right)
	if right == "" {
		return theme.TruncateVisible(left, width)
	}
	lw := max(0, width-rw-1)
	return theme.PadVisibleRight(theme.TruncateVisible(left, lw), lw) + " " + right
}

// emptyPane is the body of a pane with nothing to show: what is missing and
// the keys that would fill it.
func emptyPane(th ansiTheme, title string, hints ...string) []string {
	out := []string{"", th.bold + th.fg + title + th.reset}
	for _, hint := range hints {
		out = append(out, th.muted+hint+th.reset)
	}
	return out
}

// keyHint renders "key action" pairs for inline guidance.
func (th ansiTheme) keyHint(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, th.cyan+pairs[i]+th.reset+" "+th.muted+pairs[i+1]+th.reset)
	}
	return strings.Join(parts, "   ")
}
