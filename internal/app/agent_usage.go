package app

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func (s *Service) agentTraceSummary(history agentstate.History, run agentstate.RunSnapshot) (*AgentTraceSummary, error) {
	record, exists := history.Traces[run.BudgetID]
	result := &AgentTraceSummary{Version: 1, TraceID: agentruntime.TraceIdentity("mini-orca.trace.v1:" + run.BudgetID)[:32], Legacy: !exists, Incomplete: !exists, Omitted: record.Dropped, Spans: slices.Clone(record.Spans)}
	active := map[string]bool{}
	anyActive := false
	for _, candidate := range history.Runs {
		if candidate.BudgetID != run.BudgetID {
			continue
		}
		owned, _ := s.agentRuns.state(candidate.ID)
		active[agentruntime.TraceIdentity(candidate.ID)] = owned
		anyActive = anyActive || owned
	}
	for i := range result.Spans {
		span := &result.Spans[i]
		if span.Status == "running" {
			result.Incomplete = true
			span.Status = unfinishedTraceStatus(*span, active, anyActive)
		}
	}
	boundAgentTrace(result)
	// Only this source-free projection and its shared accounting enter the
	// snapshot identity. Reading it never loads private session events.
	data, err := json.Marshal(struct {
		Trace *AgentTraceSummary
		Usage agentstate.BudgetUsage
	}{result, run.Usage})
	if err != nil {
		return nil, fmt.Errorf("fingerprint source-free trace: %w", err)
	}
	result.Snapshot = agentruntime.TraceIdentity(string(data))
	return result, nil
}

func unfinishedTraceStatus(span agentstate.TraceSpan, active map[string]bool, anyActive bool) string {
	if span.RunKey == "" && !anyActive {
		// An attempt can be between invocations. Absence of a registered
		// invocation cannot prove that its enclosing retry has stopped.
		return "unfinished"
	}
	if span.RunKey != "" && !active[span.RunKey] {
		return "interrupted"
	}
	return "running"
}

func boundAgentTrace(result *AgentTraceSummary) {
	slices.SortFunc(result.Spans, func(a, b agentstate.TraceSpan) int {
		if order := a.Started.Compare(b.Started); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	if len(result.Spans) > 256 {
		result.Omitted += uint64(len(result.Spans) - 256)
		result.Spans = result.Spans[len(result.Spans)-256:]
	}
	result.Truncated = result.Omitted != 0
	if result.Spans == nil {
		result.Spans = []agentstate.TraceSpan{}
	}
}

func agentCost(usage agentstate.BudgetUsage, table config.AgentPriceTable) *AgentCostEstimate {
	result := &AgentCostEstimate{Status: "unknown", Currency: table.Currency, PriceVersion: table.Version, Reasons: []string{}}
	if table.Version == "" {
		result.Reasons = append(result.Reasons, "price_table_unavailable")
		return result
	}
	if usage.Accounting == nil {
		result.Reasons = append(result.Reasons, "legacy_usage")
		return result
	}
	prices := make(map[string]config.AgentModelPrice, len(table.Rates))
	for _, rate := range table.Rates {
		prices[agentruntime.ProviderUsageKey(rate.Provider, rate.Model)] = rate
	}
	for _, group := range usage.Accounting.Providers {
		price, known := prices[group.ModelKey]
		if !known {
			result.Reasons = append(result.Reasons, "model_rate_unavailable")
			continue
		}
		addProviderCost(result, group, price)
	}
	slices.Sort(result.Reasons)
	result.Reasons = slices.Compact(result.Reasons)
	if len(result.Reasons) == 0 {
		result.Status = "estimated"
		total := result.KnownSubtotal
		result.Total = &total
	}
	return result
}

func addProviderCost(result *AgentCostEstimate, group agentstate.ProviderUsage, price config.AgentModelPrice) {
	if price.InputPerMillion == nil || price.OutputPerMillion == nil || price.CachedPerMillion == nil {
		result.Reasons = append(result.Reasons, "model_rate_unavailable")
		return
	}
	result.KnownSubtotal += float64(group.ReportedOutputTokens) * *price.OutputPerMillion / 1e6
	result.KnownSubtotal += float64(group.CachedInputTokens) * *price.CachedPerMillion / 1e6
	cacheKnown := group.CacheKnownCalls == group.ReportedCalls+group.PartialUsageCalls
	if cacheKnown || *price.InputPerMillion == *price.CachedPerMillion {
		result.KnownSubtotal += float64(group.ReportedInputTokens-group.CachedInputTokens) * *price.InputPerMillion / 1e6
	} else {
		result.Reasons = append(result.Reasons, "cache_split_unknown")
	}
	if group.ActiveCalls > 0 || group.UnknownUsageCalls > 0 || group.ReportedCalls != group.ModelCalls {
		result.Reasons = append(result.Reasons, "usage_incomplete")
	}
}
