package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func costFixture() (agentstate.BudgetUsage, config.AgentPriceTable) {
	in, out, cached := 2.0, 4.0, 0.5
	table := config.AgentPriceTable{Version: "fixture-v1", Currency: "USD", Rates: []config.AgentModelPrice{{Provider: "openai", Model: "fixture", InputPerMillion: &in, OutputPerMillion: &out, CachedPerMillion: &cached}}}
	group := agentstate.ProviderUsage{ModelKey: agentruntime.ProviderUsageKey("openai", "fixture"), ModelCalls: 1, ReportedCalls: 1, CacheKnownCalls: 1, InputTokens: 1000, OutputTokens: 500, ReportedInputTokens: 1000, ReportedOutputTokens: 500, CachedInputTokens: 200}
	return agentstate.BudgetUsage{Accounting: &agentstate.UsageAccounting{Version: 1, Providers: []agentstate.ProviderUsage{group}}, ModelCalls: 1, InputTokens: 1000, OutputTokens: 500}, table
}

func TestAgentCostRequiresCompleteUsageRatesAndCacheKnowledge(t *testing.T) {
	usage, table := costFixture()
	cost := agentCost(usage, table)
	if cost.Status != "estimated" || cost.Total == nil || math.Abs(*cost.Total-0.0037) > 1e-12 {
		t.Fatalf("cost=%+v", cost)
	}
	for _, unknown := range []string{"price_table_unavailable", "legacy_usage", "model_rate_unavailable", "usage_incomplete", "cache_split_unknown"} {
		usage, table := costFixture()
		switch unknown {
		case "price_table_unavailable":
			table = config.AgentPriceTable{}
		case "legacy_usage":
			usage.Accounting = nil
		case "model_rate_unavailable":
			table.Rates[0].Model = "different"
		case "usage_incomplete":
			usage.Accounting.Providers[0].UnknownUsageCalls = 1
		case "cache_split_unknown":
			usage.Accounting.Providers[0].CacheKnownCalls = 0
			usage.Accounting.Providers[0].CachedInputTokens = 0
		}
		cost := agentCost(usage, table)
		if cost.Total != nil || cost.Status != "unknown" || !slices.Contains(cost.Reasons, unknown) {
			t.Fatalf("unknown became priced: %+v", cost)
		}
		if unknown == "cache_split_unknown" && cost.KnownSubtotal != 0.002 {
			t.Fatal("unmeasured cache split was priced", cost)
		}
	}
	usage, table = costFixture()
	usage.Accounting.Providers[0].CacheKnownCalls = 0
	usage.Accounting.Providers[0].CachedInputTokens = 0
	*table.Rates[0].CachedPerMillion = *table.Rates[0].InputPerMillion
	if cost := agentCost(usage, table); cost.Total == nil || *cost.Total != 0.004 {
		t.Fatal("equal cache price requires no split", cost)
	}
}

func TestAgentTraceProjectionIsPassiveScopedAndHonestAboutInterruption(t *testing.T) {
	s, store, query := agentHistoryFixture(t)
	run := seedAgentRun(t, s, store, "trace-run", "", time.Now().UTC())
	legacy, err := s.AgentRunDetail(t.Context(), query, run.ID, false)
	if err != nil || legacy.Trace == nil || !legacy.Trace.Legacy || !legacy.Trace.Incomplete || legacy.Cost.Total != nil {
		t.Fatal("legacy projection invented measurements", err)
	}
	span := agentstate.TraceSpan{ID: "0123456789abcdef", Kind: "provider", Status: "running", Started: time.Now().UTC(), RunKey: agentruntime.TraceIdentity(run.ID), OperationKey: strings.Repeat("a", 64)}
	if _, err := store.BeginSpan(t.Context(), run.BudgetID, span); err != nil {
		t.Fatal(err)
	}
	first, err := s.AgentRunDetail(t.Context(), query, run.ID, false)
	if err != nil || first.Text != nil || first.Trace.Legacy || !first.Trace.Incomplete || first.Trace.Spans[0].Status != "interrupted" {
		t.Fatal("unfinished trace became success", err)
	}
	second, err := s.AgentRunDetail(t.Context(), query, run.ID, false)
	if err != nil || first.Trace.Snapshot != second.Trace.Snapshot {
		t.Fatal("passive snapshot changed", err)
	}
	data, err := json.Marshal(first)
	if err != nil || strings.Contains(string(data), "PRIVATE_") {
		t.Fatal("source escaped into projection", err)
	}
	if err := store.FinishSpan(t.Context(), run.BudgetID, span.ID, "failed", time.Now().UTC(), 1000); err != nil {
		t.Fatal(err)
	}
	third, err := s.AgentRunDetail(t.Context(), query, run.ID, false)
	if err != nil || third.Trace.Snapshot == first.Trace.Snapshot || third.Trace.Spans[0].Status != "failed" {
		t.Fatal("trace change lost snapshot identity", err)
	}
	query.ProjectRevision = "foreign"
	if _, err := s.AgentRunDetail(t.Context(), query, run.ID, false); err == nil {
		t.Fatal("foreign trace scope accepted")
	}
}

func TestAgentTraceProjectionBoundsHistoryWithoutChangingAccounting(t *testing.T) {
	s, _, _ := agentHistoryFixture(t)
	run := agentstate.RunSnapshot{Run: agentstate.Run{BudgetID: "budget"}, Usage: agentstate.BudgetUsage{ModelCalls: 300}}
	record := agentstate.TraceRecord{Version: 1, Dropped: 9}
	for i := range 300 {
		record.Spans = append(record.Spans, agentstate.TraceSpan{ID: fmt.Sprintf("%016x", i+1), Kind: "provider", Status: "failed", Started: time.Unix(int64(i), 0), Ended: time.Unix(int64(i)+1, 0), ElapsedNanos: 1e9})
	}
	history := agentstate.History{Traces: map[string]agentstate.TraceRecord{"budget": record}}
	result, err := s.agentTraceSummary(history, run)
	if err != nil || len(result.Spans) != 256 || !result.Truncated || result.Omitted != 53 || run.Usage.ModelCalls != 300 || len(history.Traces["budget"].Spans) != 300 {
		t.Fatal("bounded view altered history", err)
	}
}

func TestAgentTraceDoesNotInventInterruptedRetryOwnership(t *testing.T) {
	span := agentstate.TraceSpan{Kind: "retry", Status: "running"}
	if got := unfinishedTraceStatus(span, nil, false); got != "unfinished" {
		t.Fatal(got)
	}
	if got := unfinishedTraceStatus(span, nil, true); got != "running" {
		t.Fatal(got)
	}
	span.RunKey = "owned"
	if got := unfinishedTraceStatus(span, map[string]bool{"owned": true}, true); got != "running" {
		t.Fatal(got)
	}
	if got := unfinishedTraceStatus(span, nil, false); got != "interrupted" {
		t.Fatal(got)
	}
}

func TestAgentReportingSettingsPreserveLegacyExecutionIdentity(t *testing.T) {
	s, _, _ := agentHistoryFixture(t)
	before, err := s.agentConfiguration(s.agentRuntime)
	if err != nil {
		t.Fatal(err)
	}
	_, prices := costFixture()
	changed := s.agentRuntime
	changed.Telemetry.Prices = prices
	after, err := s.agentConfiguration(changed)
	if err != nil || after != before {
		t.Fatal("pricing invalidated execution preferences", err)
	}
	legacy := struct {
		Profiles [3]config.ModelProfile
		Runtime  struct {
			Roles   config.AgentRoles
			Limits  config.AgentLimits
			Context config.AgentContext
			Skills  []string
		}
	}{Profiles: [3]config.ModelProfile{s.runtimes.analyze.profile, s.runtimes.bug.profile, s.runtimes.function.profile}}
	legacy.Runtime.Roles, legacy.Runtime.Limits = s.agentRuntime.Roles, s.agentRuntime.Limits
	legacy.Runtime.Context, legacy.Runtime.Skills = s.agentRuntime.Context, s.agentRuntime.Skills
	data, err := json.Marshal(legacy)
	if err != nil || agentruntime.TraceIdentity(string(data)) != after {
		t.Fatal("empty telemetry changed legacy identity", err)
	}
}

func TestAgentTraceOverviewCancellationPreservesFactualInventory(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	s, _, _ := agentHistoryFixture(t)
	configured, err := New(scopedTestConfig(server.URL), s.manager)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type outcome struct {
		analysis *project.Analysis
		err      error
	}
	finished := make(chan outcome, 1)
	go func() {
		analysis, err := configured.AnalyzeProject(ctx, s.manager.Root())
		finished <- outcome{analysis, err}
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not start")
	}
	cancel()
	select {
	case result := <-finished:
		if result.err != nil || result.analysis == nil || result.analysis.AIStatus != project.ProjectAnalysisStatusFailed || result.analysis.FileCount == 0 {
			t.Fatalf("inventory lost: %+v %v", result.analysis, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled trace cleanup did not finish")
	}
}
