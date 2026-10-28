package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func traceExportService(t *testing.T, endpoint string) (*Service, *agentstate.Store, agentstate.Run, AgentTraceExportRequest) {
	t.Helper()
	s, store, query := agentHistoryFixture(t)
	s.conf().agentRuntime.Telemetry.Export = config.AgentTelemetryExport{Endpoint: endpoint}
	run := seedAgentRun(t, s, store, "export-run", "", time.Now().UTC())
	span := agentstate.TraceSpan{ID: "0123456789abcdef", Kind: "provider", Status: "running", Started: time.Now().UTC(), RunKey: agentruntime.TraceIdentity(run.ID)}
	if _, err := store.BeginSpan(t.Context(), run.BudgetID, span); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishSpan(t.Context(), run.BudgetID, span.ID, "completed", span.Started.Add(time.Millisecond), 1e6); err != nil {
		t.Fatal(err)
	}
	span.ID, span.Kind = "abcdef0123456789", "tool"
	if _, err := store.BeginSpan(t.Context(), run.BudgetID, span); err != nil {
		t.Fatal(err)
	}
	detail, err := s.AgentRunDetail(t.Context(), query, run.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	return s, store, run, AgentTraceExportRequest{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Snapshot: detail.Trace.Snapshot, DestinationFingerprint: detail.Trace.Export.Fingerprint, Confirm: true}
}

func TestAgentTraceExportRequiresExactConfirmationAndRemainsSourceFree(t *testing.T) {
	var calls atomic.Int32
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if strings.Contains(string(data), "PRIVATE_") || strings.Contains(string(data), `"spanId":"abcdef0123456789"`) {
			t.Error("private or unfinished content exported")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"partialSuccess":{"rejectedSpans":"1","errorMessage":"PRIVATE_DIAGNOSTIC"}}`)
	}))
	defer collector.Close()
	s, _, run, request := traceExportService(t, collector.URL)
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	detail, err := s.AgentRunDetail(t.Context(), query, run.ID, false)
	if err != nil || !detail.Trace.Export.Enabled || detail.Trace.Export.Finished != 1 || detail.Trace.Export.Unfinished != 1 || !detail.Trace.Incomplete || calls.Load() != 0 {
		t.Fatal("passive preview lost state", err)
	}
	for _, invalid := range []string{"confirmation", "project", "revision", "snapshot", "destination", "run"} {
		changed, id := request, run.ID
		switch invalid {
		case "confirmation":
			changed.Confirm = false
		case "project":
			changed.ProjectID = "foreign"
		case "revision":
			changed.ProjectRevision = "foreign"
		case "snapshot":
			changed.Snapshot = strings.Repeat("f", 64)
		case "destination":
			changed.DestinationFingerprint = strings.Repeat("f", 64)
		case "run":
			id = "missing"
		}
		if _, err := s.ExportAgentTrace(t.Context(), id, changed); err == nil {
			t.Fatal("invalid export admitted", invalid)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.ExportAgentTrace(ctx, run.ID, request); err == nil {
		t.Fatal("canceled admission accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("unconfirmed or stale export sent")
	}
	receipt, err := s.ExportAgentTrace(t.Context(), run.ID, request)
	if err != nil || receipt.State != "partial" || receipt.Accepted == nil || *receipt.Accepted != 0 || *receipt.Rejected != 1 || receipt.Sent != 1 || receipt.Snapshot != request.Snapshot || receipt.DestinationFingerprint != request.DestinationFingerprint || calls.Load() != 1 {
		t.Fatal(receipt, err)
	}
	data, _ := json.Marshal(receipt)
	if strings.Contains(string(data), "PRIVATE_") {
		t.Fatal("collector error exposed")
	}
}

func TestAgentTraceExportInvalidatesChangedContentAndDestinationIndependently(t *testing.T) {
	var calls atomic.Int32
	collector := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer collector.Close()
	s, store, run, request := traceExportService(t, collector.URL)
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	_, prices := costFixture()
	s.conf().agentRuntime.Telemetry.Prices = prices
	s.conf().agentRuntime.Telemetry.Export.Endpoint = collector.URL + "/replacement"
	detail, err := s.AgentRunDetail(t.Context(), query, run.ID, false)
	if err != nil || detail.Trace.Snapshot != request.Snapshot || detail.Trace.Export.Fingerprint == request.DestinationFingerprint {
		t.Fatal("content and destination identities mixed", err)
	}
	if _, err := s.ExportAgentTrace(t.Context(), run.ID, request); err == nil {
		t.Fatal("old destination approval reused")
	}
	s.conf().agentRuntime.Telemetry.Export.Endpoint = collector.URL
	if err := store.FinishSpan(t.Context(), run.BudgetID, "abcdef0123456789", "failed", time.Now().UTC(), 123); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExportAgentTrace(t.Context(), run.ID, request); err == nil {
		t.Fatal("changed snapshot exported")
	}
	if calls.Load() != 0 {
		t.Fatal("stale approval dispatched")
	}
}

func TestAgentTraceExportPreviewCannotExportLegacyOrUnfinishedOnlyHistory(t *testing.T) {
	for _, trace := range []*AgentTraceSummary{{Legacy: true}, {Spans: []agentstate.TraceSpan{{Status: "interrupted"}}}} {
		preview := agentTraceExportPreview(trace, config.AgentTelemetryExport{Endpoint: "https://collector.example/v1/traces"})
		if preview.Enabled || preview.Finished != 0 || preview.Reason != "no_finished_spans" {
			t.Fatal(preview)
		}
	}
	preview := agentTraceExportPreview(&AgentTraceSummary{}, config.AgentTelemetryExport{})
	if preview.Enabled || preview.Fingerprint != "" || preview.Reason != "not_configured" {
		t.Fatal(preview)
	}
}
