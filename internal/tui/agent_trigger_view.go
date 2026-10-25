package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func triggerTime(value time.Time) string {
	if value.IsZero() {
		return "not scheduled"
	}
	return value.Local().Format("Jan 02 15:04:05 MST")
}

func triggerWindowSummary(w client.TriggerWindow) string {
	if w.StartedAt.IsZero() {
		return "Window: no attempts reserved."
	}
	l, c := w.Limits, w.Charged
	return fmt.Sprintf("Remaining: %d/%d attempts, %d model calls, %d input tokens, %d output tokens, %ds. Window ends %s.", max(0, l.Runs-c.Runs), l.Runs, max(0, l.ModelCalls-c.ModelCalls), max(0, l.InputTokens-c.InputTokens), max(0, l.OutputTokens-c.OutputTokens), max(0, l.Seconds-c.Seconds), triggerTime(w.EndsAt))
}

func (s *State) renderTriggers(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "AMBIENT ANALYSIS", "[w] Watch · [g] Grant · [v] Revoke · [d] Disable")
	if !s.triggersCurrent() {
		return append(lines, "  Main checkout only. Press [r] to inspect state.", "  Watching and provider permission are separate.")
	}
	v := s.AgentTriggers
	if v.Data == nil {
		lines = append(lines, "  State: "+settingsText(client.FirstNonEmpty(v.Status, "not loaded")))
		return appendWrappedText(lines, client.FirstNonEmpty(v.Failure, "Press [r] to read local state."), width-4)
	}
	d := v.Data
	label := strings.ReplaceAll(d.State, "_", " ")
	if d.State == "granted" && d.Record.Grant != nil && !d.Record.Grant.ExpiresAt.After(time.Now()) {
		label = "grant expired"
	}
	if v.Status != "current" {
		label += " · last known"
	}
	lines = append(lines, "  Permission: "+th.statusBadge(label))
	if v.Pending != "" {
		lines = append(lines, "  "+th.cyan+"Request pending · [x] Cancel local request"+th.reset)
	}
	for _, notice := range []string{v.Recovery, v.Failure} {
		if notice != "" {
			lines = appendWrappedText(lines, externalSafeText(notice), width-4)
		}
	}
	lines = append(lines, "  "+theme.TruncatePlain("Main: "+settingsText(v.Path), max(1, width-4)))
	o := d.Record.Options
	watch := fmt.Sprintf("Watch: %s · %d selected paths", o.Mode, len(o.Paths))
	if o.Mode == "interval" {
		watch += fmt.Sprintf(" · every %ds", o.IntervalSeconds)
	}
	lines = appendWrappedText(lines, watch, width-4)
	if d.Record.Grant != nil {
		lines = appendWrappedText(lines, "Grant ends: "+triggerTime(d.Record.Grant.ExpiresAt), width-4)
	}
	lines = appendWrappedText(lines, "Next eligible: "+triggerTime(d.Record.NextEligibleAt), width-4)
	last := "Last attempt: none"
	if a := d.Record.LastAttempt; a != nil {
		last = "Last attempt: " + a.State + " · " + a.Path
	}
	lines = appendWrappedText(lines, externalSafeText(last), width-4)
	lines = appendWrappedText(lines, triggerWindowSummary(d.Record.Window), width-4)
	lines = appendWrappedText(lines, externalSafeText(d.Record.Reason), width-4)
	if !v.Details {
		return append(lines, "  [Enter] Scope, provider, budget and receipt details")
	}
	return append(lines, s.renderTriggerDetails(th, width)...)
}

func (s *State) renderTriggerDetails(th ansiTheme, width int) []string {
	v := s.AgentTriggers
	d := v.Data
	contentW := max(1, width-6)
	lines := []string{""}
	for _, item := range [][2]string{{"Checkout", v.Path}, {"Revision", d.ProjectRevision}, {"Model", d.Model}, {"Destination", d.ProviderOrigin}, {"Configuration", d.Configuration}, {"Policy", d.Policy}} {
		lines = append(lines, renderDiagnosticRow(th, th.cyan, item[0]+":", th.fg, settingsText(item[1]), contentW)...)
	}
	lines = append(lines, renderDiagnosticBulletRows(th, th.cyan, "Selected source paths:", th.cyan, th.fg, triggerDisplayPaths(d.Record.Options.Paths), contentW)...)
	l := d.RunLimits
	lines = appendWrappedText(lines, fmt.Sprintf("Per attempt: %d model calls · %d input / %d output tokens · %ds", l.ModelCalls, l.InputTokens, l.OutputTokens, l.ElapsedSeconds), width-4)
	lines = appendWrappedText(lines, fmt.Sprintf("Configured window: %d attempts · %d model calls · %d input / %d output tokens · %ds", d.Ceilings.MaxRuns, d.Ceilings.MaxModelCalls, d.Ceilings.MaxInputTokens, d.Ceilings.MaxOutputTokens, d.Ceilings.WindowSeconds), width-4)
	lines = appendWrappedText(lines, fmt.Sprintf("Detection: poll %ds · debounce %ds · quiet %ds", d.Ceilings.PollSeconds, d.Ceilings.DebounceSeconds, d.Ceilings.QuietSeconds), width-4)
	if a := d.Record.LastAttempt; a != nil {
		lines = appendWrappedText(lines, "Attempt: "+settingsText(a.ID)+" · "+a.State, width-4)
		lines = appendWrappedText(lines, "Source: "+settingsText(a.Path)+" · SHA-256 "+settingsText(a.SourceHash), width-4)
		lines = appendWrappedText(lines, "Started: "+triggerTime(a.StartedAt)+" · finished: "+triggerTime(a.FinishedAt), width-4)
		lines = appendWrappedText(lines, fmt.Sprintf("Reserved: %d calls · %d input / %d output tokens · %ds", a.Reserved.ModelCalls, a.Reserved.InputTokens, a.Reserved.OutputTokens, a.Reserved.Seconds), width-4)
		lines = appendWrappedText(lines, fmt.Sprintf("Accounted usage: %d model calls · %d input / %d output tokens · %d calls without measured usage. Reservations are not refunded.", a.Usage.ModelCalls, a.Usage.InputTokens, a.Usage.OutputTokens, a.Usage.UnknownUsageCalls), width-4)
		for _, row := range renderAgentAccounting(a.Usage) {
			lines = appendWrappedText(lines, externalSafeText(row), width-4)
		}
	}
	lines = appendWrappedText(lines, "Semantic advice only. Provider sharing includes filtered project context. Source changes still require proposal Review and Apply.", width-4)
	return append(lines, "  [Enter] Hide details · PgUp/PgDn scroll · [r] Refresh")
}

func triggerDisplayPaths(paths []string) []string {
	rows := make([]string, len(paths))
	for i, path := range paths {
		rows[i] = settingsText(path)
	}
	return rows
}
