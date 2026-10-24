package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func (s *State) requestTraceExport(arg string) {
	if strings.TrimSpace(arg) != "" {
		s.Error = "Use :run-export to review the current trace snapshot."
		return
	}
	s.confirmAgentTraceExport()
}

func (s *State) confirmAgentTraceExport() {
	detail := s.currentAgentDetail()
	if detail == nil || detail.Trace == nil || detail.Trace.Export == nil || !detail.Trace.Export.Enabled {
		s.Error = "Load a retained trace with finished spans and configure a telemetry destination first."
		return
	}
	if s.AgentRunPending != "" {
		s.Error = "Finish the pending execution request before exporting."
		return
	}
	trace, preview := detail.Trace, detail.Trace.Export
	request := *s.agentProgressRequest()
	action := agentRunAction{ID: nextTUIReqID("trace-export"), Operation: "export", Invocation: detail.Run.ID, Request: request, Change: cloneChangeSessionSnapshot(s.Change), ExportCount: preview.Finished,
		Export: client.AgentTraceExportRequest{ProjectIdentity: request.Project, Snapshot: trace.Snapshot, DestinationFingerprint: preview.Fingerprint, Confirm: true}}
	path := request.WorktreePath
	if path == "" {
		path = s.Project.Path
	}
	message := fmt.Sprintf("Checkout: %s\nProject: %s · revision %s\nRun: %s · recorded revision %s\nDestination: %s\nDestination identity: %s\nSnapshot: %s\nSend %d finished spans. %d unfinished spans are excluded; %d spans are outside retained history.\nHistory incomplete: %t · truncated: %t\nOnly opaque identities, fixed outcomes and timing are sent. No prompts, source, tool values or diagnostics. Send once; an unavailable acknowledgement may mean delivery occurred. No automatic retry.", path, request.Project.ProjectID, request.Project.ProjectRevision, action.Invocation, detail.Run.ProjectRevision, preview.Destination, preview.Fingerprint, trace.Snapshot, preview.Finished, preview.Unfinished, trace.Omitted, trace.Incomplete, trace.Truncated)
	s.Confirmation = &Confirmation{Title: "Export trace snapshot", Message: externalSafeText(message), AcceptLabel: "Send once", OnConfirm: func(state *State) {
		if state.AgentRunPending != "" || !state.matchesTraceExport(action) {
			state.Error = "Trace, checkout or destination changed. Inspect and confirm again."
			return
		}
		state.AgentRunPending = action.ID
		state.enqueueAction(ActionRequest{Kind: "agent-run", AgentRun: &action})
	}}
}

func (s *State) matchesTraceExport(action agentRunAction) bool {
	if !s.matchesAgentRequest(action.Request) {
		return false
	}
	detail := s.currentAgentDetail()
	if detail == nil || detail.Run.ID != action.Invocation || detail.Trace == nil || detail.Trace.Export == nil {
		return false
	}
	preview := detail.Trace.Export
	return preview.Enabled && preview.Finished == action.ExportCount && preview.Fingerprint == action.Export.DestinationFingerprint && detail.Trace.Snapshot == action.Export.Snapshot
}

func (s *State) applyTraceExportReceipt(action agentRunAction, receipt client.AgentTraceExportReceipt) {
	if receipt.RunID != action.Invocation || receipt.ProjectIdentity != action.Export.ProjectIdentity || receipt.Snapshot != action.Export.Snapshot || receipt.DestinationFingerprint != action.Export.DestinationFingerprint || receipt.Sent != action.ExportCount || !validTraceExportReceipt(receipt) {
		s.Error = "Export acknowledgement does not match the confirmed snapshot; delivery is unknown."
		return
	}
	if receipt.State == "unknown" {
		s.CmdOutput = "Trace delivery is unknown; the collector did not provide a valid acknowledgement. No automatic retry."
		return
	}
	s.CmdOutput = fmt.Sprintf("Trace export %s · accepted %d / %d · rejected %d · snapshot %s", receipt.State, *receipt.Accepted, receipt.Sent, *receipt.Rejected, receipt.Snapshot)
	if receipt.State != "exported" {
		s.CmdOutput += " · collector warning or rejection; no automatic retry"
	}
}

func (s *State) applyTraceExportAction(result agentRunActionResult) {
	if s.AgentRunPending != result.Action.ID {
		return
	}
	s.AgentRunPending = ""
	if !s.matchesAgentRequest(result.Action.Request) {
		return
	}
	if result.Err != nil {
		s.Error = "Export acknowledgement unavailable; delivery may have occurred. " + result.Err.Error()
		return
	}
	s.applyTraceExportReceipt(result.Action, result.Export)
}

func validTraceExportReceipt(receipt client.AgentTraceExportReceipt) bool {
	if receipt.Sent < 1 || receipt.Sent > 256 {
		return false
	}
	if receipt.State == "unknown" {
		return receipt.Accepted == nil && receipt.Rejected == nil
	}
	if !slices.Contains([]string{"exported", "partial", "warning"}, receipt.State) || receipt.Accepted == nil || receipt.Rejected == nil {
		return false
	}
	if *receipt.Accepted < 0 || *receipt.Rejected < 0 || *receipt.Accepted > receipt.Sent || *receipt.Rejected != receipt.Sent-*receipt.Accepted {
		return false
	}
	return (receipt.State == "partial") == (*receipt.Rejected > 0)
}
