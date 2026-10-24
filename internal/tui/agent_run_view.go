package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

var agentRunSections = []string{"Overview", "Timeline", "Evidence", "Usage", "Recovery"}

func (s *State) stepAgentRunSection(forward bool) {
	delta := 1
	if !forward {
		delta = -1
	}
	s.AgentRunSection = (s.AgentRunSection + delta + len(agentRunSections)) % len(agentRunSections)
	s.DetailScroll = 0
	s.viewport.followSelection = false
}

func (s *State) renderAgentRunPage(th ansiTheme, width int) []string {
	section := agentRunSections[s.AgentRunSection]
	lines := renderPageHeader(th, "TASK · "+strings.ToUpper(section), "←/→ Section · ↑/↓ Stage · Esc Back")
	lines = append(lines, "  "+s.agentStageSelectionLabel())
	detail := s.currentAgentDetail()
	if detail == nil {
		return appendWrappedText(lines, "Native execution detail is unavailable. Recorded stages and guarded retries remain in Overview.", width)
	}
	var body []string
	switch s.AgentRunSection {
	case 1:
		body = renderAgentTimeline(detail)
	case 2:
		body = s.renderAgentEvidence(detail)
	case 3:
		body = renderAgentUsage(detail)
	case 4:
		body = s.renderAgentRecovery(detail)
	}
	for _, line := range body {
		for _, part := range strings.Split(line, "\n") {
			lines = appendWrappedText(lines, theme.SanitizeCodeText(part), width)
		}
	}
	return lines
}

func renderAgentTimeline(detail *AgentRunDetail) []string {
	lines := []string{"Recorded invocations · use :run-output <ID> for private text and tool records."}
	for _, run := range detail.Invocations {
		lines = append(lines, fmt.Sprintf("%s · %s · %s", run.ID, run.Kind, run.Status))
	}
	if detail.HasMore {
		lines = append(lines, "Invocation list truncated to 256 records.")
	}
	lines = append(lines, "", "Timeline · execution events do not grant approval")
	for _, event := range detail.Events {
		lines = append(lines, fmt.Sprintf("#%d %s · %s · %s %s · %s", event.Sequence, event.Time, event.Kind, event.NodeID, event.State, event.InvocationID))
	}
	if len(detail.Events) == 0 {
		lines = append(lines, "No retained events.")
	}
	if detail.EventsTruncated {
		lines = append(lines, "Earlier events are outside retained history.")
	}
	return lines
}

func (s *State) renderAgentEvidence(detail *AgentRunDetail) []string {
	lines := []string{"Evidence references · immutable recorded versions"}
	seen := make(map[string]bool)
	for _, event := range detail.Events {
		for _, artifact := range event.Evidence {
			label := fmt.Sprintf("%s / %s · version %d", artifact.Session.ID, artifact.Name, artifact.Version)
			if !seen[label] {
				lines = append(lines, label)
				seen[label] = true
			}
		}
	}
	if len(seen) == 0 {
		lines = append(lines, "No artifact references in retained progress.")
	}
	lines = append(lines, "Checks, model reviews and proposal approval remain separate. Use [d] to inspect the proposal.")
	disclosure := s.AgentDisclosure
	if disclosure == nil || !s.matchesAgentRequest(disclosure.Request) {
		return append(lines, "Private output is hidden. Select an invocation with :run-output <ID> from Timeline.")
	}
	return append(lines, renderPrivateAgentText(disclosure)...)
}

func renderPrivateAgentText(disclosure *agentDisclosure) []string {
	lines := []string{"", "Private invocation: " + disclosure.Invocation + " · :hide-output closes this disclosure"}
	if disclosure.Status == "unavailable" {
		lines = append(lines, "Output refresh unavailable; last recorded snapshot follows.")
	}
	if disclosure.Detail == nil {
		return append(lines, "Loading private output…")
	}
	text := disclosure.Detail.Text
	if text == nil {
		return append(lines, "Private output unavailable.")
	}
	lines = append(lines, "Model output · advisory · "+text.State)
	if text.Available {
		lines = append(lines, text.Text)
	} else {
		lines = append(lines, "No retained model text for this invocation.")
	}
	if text.Truncated {
		lines = append(lines, "Model text truncated to the retained 64 KiB window.")
	}
	for _, tool := range text.ToolRecords {
		lines = append(lines, "", fmt.Sprintf("Tool %s · %s · call %s · event %s", tool.Name, tool.Kind, tool.CallID, tool.EventID), tool.Value)
		if tool.Truncated {
			lines = append(lines, "Tool record truncated.")
		}
	}
	if len(text.ToolRecords) == 0 {
		lines = append(lines, "No tool argument/result records in the retained event window.")
	}
	if text.ToolsTruncated {
		lines = append(lines, "Tool history truncated; showing up to 16 records from the last 128 events.")
	}
	return lines
}

func renderAgentUsage(detail *AgentRunDetail) []string {
	u, l := detail.Run.Usage, detail.Run.Limits
	lines := []string{
		"Shared run budget · includes child work; do not add invocation counters together.",
		fmt.Sprintf("Model calls: %d / %d · active %d", u.ModelCalls, l.ModelCalls, u.ActiveCalls),
		fmt.Sprintf("Tool calls: %d / %d · repairs %d / %d", u.ToolCalls, l.ToolCalls, u.Repairs, l.Repairs),
		fmt.Sprintf("Input budget charged: %d / %d · reserved %d", u.InputTokens, l.InputTokens, u.ReservedInputTokens),
		fmt.Sprintf("Output budget charged: %d / %d · reserved %d", u.OutputTokens, l.OutputTokens, u.ReservedOutputTokens),
		fmt.Sprintf("Unknown total usage: %d calls · conservative charges remain in the budget", u.UnknownUsageCalls),
		fmt.Sprintf("Elapsed: %.1fs / %ds · concurrency limit %d", float64(u.ElapsedMillis)/1000, l.ElapsedSeconds, l.ConcurrentCalls),
	}
	lines = append(lines, renderAgentAccounting(u)...)
	return append(lines, "Remote agent usage is separate in External Work; internal remote consumption remains unknown unless reported.", "Cost: unknown · a versioned price table is required.", "Usage is recorded accounting, not a measured performance improvement.")
}

func renderAgentAccounting(usage client.AgentUsage) []string {
	if usage.Accounting == nil || usage.Accounting.Version != 1 {
		return []string{"Legacy accounting · reported tokens, cache usage and estimates cannot be separated."}
	}
	var input, output, cached int64
	var failed, reported, partial, cacheKnown int
	for _, group := range usage.Accounting.Providers {
		input, output, cached = input+group.ReportedInputTokens, output+group.ReportedOutputTokens, cached+group.CachedInputTokens
		failed, reported, partial, cacheKnown = failed+group.FailedCalls, reported+group.ReportedCalls, partial+group.PartialUsageCalls, cacheKnown+group.CacheKnownCalls
	}
	return []string{
		fmt.Sprintf("Provider-reported tokens: input %d · output %d", input, output),
		fmt.Sprintf("Conservative estimates: input %d · output %d", usage.InputTokens-input, usage.OutputTokens-output),
		fmt.Sprintf("Cached input reported: %d · cache known on %d / %d reported receipts", cached, cacheKnown, reported+partial),
		fmt.Sprintf("Failed provider attempts: %d · complete usage %d · partial usage %d", failed, reported, partial),
		"Partial receipts are lower bounds; estimated charges are not provider measurements.",
	}
}

func (s *State) renderAgentRecovery(detail *AgentRunDetail) []string {
	lines := []string{"Run: " + detail.Run.ID + " · " + detail.Run.Status}
	if s.AgentStreamStatus != "" {
		lines = append(lines, "Progress connection: "+s.AgentStreamStatus)
	}
	if detail.Run.Stale {
		lines = append(lines, "Stale history grants no current execution authority.")
	}
	if detail.Run.CanCancel {
		lines = append(lines, "Cancel: :run-cancel · the worker owns cleanup.")
	}
	if !s.hasRunningWorkflow() {
		lines = append(lines, "Retry: :retry test reuses the saved proposal; :retry review requires current passing checks.")
	}
	lines = append(lines, "Task pause/resume is unavailable. Analysis has separate pause/resume controls.")
	input := currentAgentInput(s.Change)
	if input == nil {
		return append(lines, "No pending approval for this exact proposal. Apply and Undo use their existing guards.")
	}
	destination := s.Change.WorktreePath
	if destination == "" && s.Project != nil {
		destination = s.Project.Path
	}
	lines = append(lines, "", "Pending input: "+input.RequestID, "Action: "+input.Action+" · expires "+input.ExpiresAt, "Destination: "+destination, fmt.Sprintf("Proposal revision %d · hash %s", input.Revision, input.Hash), "Captured scope:")
	for _, scope := range input.Scope {
		lines = append(lines, fmt.Sprintf("%s · source %s · proposed %s", scope.Path, scope.SourceHash, scope.ProposalHash))
	}
	return append(lines, "Review every changed file using [d], then :run-input approve, reject or cancel.", "Recording a decision does not apply files. Apply remains separate.")
}
