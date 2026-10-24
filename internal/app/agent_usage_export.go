package app

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func agentTraceExportPreview(trace *AgentTraceSummary, destination config.AgentTelemetryExport) *AgentTraceExportPreview {
	preview := &AgentTraceExportPreview{Destination: destination.Endpoint}
	for _, span := range trace.Spans {
		if exportableTraceSpan(span) {
			preview.Finished++
		} else {
			preview.Unfinished++
		}
	}
	if destination.Endpoint == "" {
		preview.Reason = "not_configured"
		return preview
	}
	// Only fixed string fields participate; credentials are resolved at dispatch.
	data, _ := json.Marshal([]string{"mini-orca.trace-export.v1", destination.Endpoint, destination.BearerEnv})
	preview.Fingerprint = agentruntime.TraceIdentity(string(data))
	preview.Enabled = !trace.Legacy && preview.Finished > 0
	if !preview.Enabled {
		preview.Reason = "no_finished_spans"
	}
	return preview
}

func exportableTraceSpan(span agentstate.TraceSpan) bool {
	return !span.Ended.IsZero() && slices.Contains([]string{"completed", "failed", "canceled", "timed_out"}, span.Status)
}

func (s *Service) traceForExport(ctx context.Context, id string, request AgentTraceExportRequest) (*AgentTraceSummary, error) {
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	history, _, err := s.readAgentHistory(ctx, query)
	if err != nil {
		return nil, err
	}
	run, err := findAgentRun(history, query.ProjectID, id)
	if err != nil {
		return nil, err
	}
	trace, err := s.agentTraceSummary(history, run)
	if err != nil {
		return nil, err
	}
	if trace.Snapshot != request.Snapshot || trace.Export.Fingerprint != request.DestinationFingerprint {
		return nil, fmt.Errorf("%w: trace snapshot or export destination changed; inspect and confirm again", project.ErrRevisionConflict)
	}
	if !trace.Export.Enabled {
		return nil, fmt.Errorf("trace export requires a configured destination and retained finished spans")
	}
	return trace, nil
}

func (s *Service) ExportAgentTrace(ctx context.Context, id string, request AgentTraceExportRequest) (*AgentTraceExportReceipt, error) {
	if !request.Confirm || request.Snapshot == "" || request.DestinationFingerprint == "" {
		return nil, fmt.Errorf("confirm the exact retained trace snapshot and configured destination before exporting")
	}
	root := s.manager.Root()
	trace, err := s.traceForExport(ctx, id, request)
	if err != nil {
		return nil, err
	}
	spans := make([]agentstate.TraceSpan, 0, trace.Export.Finished)
	for _, span := range trace.Spans {
		if exportableTraceSpan(span) {
			spans = append(spans, span)
		}
	}
	payload := agentruntime.TraceExport{TraceID: trace.TraceID, Snapshot: trace.Snapshot, Incomplete: trace.Incomplete, Truncated: trace.Truncated, Omitted: trace.Omitted, Unfinished: trace.Export.Unfinished, Spans: spans}
	destination := s.agentRuntime.Telemetry.Export
	receipt, err := agentruntime.ExportTrace(ctx, destination, payload, func(ctx context.Context) error {
		if _, err := s.traceForExport(ctx, id, request); err != nil {
			return err
		}
		return s.agentHistoryScope(AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}, root)
	})
	if err != nil {
		return nil, err
	}
	return &AgentTraceExportReceipt{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, RunID: id, Snapshot: trace.Snapshot, DestinationFingerprint: request.DestinationFingerprint, State: receipt.State, Sent: receipt.Sent, Accepted: receipt.Accepted, Rejected: receipt.Rejected, Reason: receipt.Reason}, nil
}
