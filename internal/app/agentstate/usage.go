package agentstate

import "slices"

// UsageAccounting is deliberately separate from enforced budget charges. A
// missing projection identifies legacy records whose measurements are unknown.
type UsageAccounting struct {
	Version   int             `json:"version"`
	Providers []ProviderUsage `json:"providers"`
}

// ModelKey hashes only the configured provider and model, never credentials or
// endpoints. Reported totals may include partial receipts from interrupted work.
type ProviderUsage struct {
	ModelKey             string `json:"model_key"`
	ModelCalls           int    `json:"model_calls"`
	ActiveCalls          int    `json:"active_calls"`
	FailedCalls          int    `json:"failed_calls"`
	UnknownUsageCalls    int    `json:"unknown_usage_calls"`
	ReportedCalls        int    `json:"reported_calls"`
	PartialUsageCalls    int    `json:"partial_usage_calls"`
	CacheKnownCalls      int    `json:"cache_known_calls"`
	InputTokens          int64  `json:"input_tokens"`
	OutputTokens         int64  `json:"output_tokens"`
	ReportedInputTokens  int64  `json:"reported_input_tokens"`
	ReportedOutputTokens int64  `json:"reported_output_tokens"`
	CachedInputTokens    int64  `json:"cached_input_tokens"`
}

func (u BudgetUsage) Clone() BudgetUsage {
	if u.Accounting != nil {
		copy := *u.Accounting
		copy.Providers = slices.Clone(copy.Providers)
		u.Accounting = &copy
	}
	return u
}

func validAccounting(u BudgetUsage) bool {
	a := u.Accounting
	if a == nil {
		return true
	}
	if a.Version != 1 || len(a.Providers) > 64 {
		return false
	}
	var calls, active, unknown int
	var input, output int64
	seen := map[string]bool{}
	for _, group := range a.Providers {
		if seen[group.ModelKey] || !validProviderUsage(group) {
			return false
		}
		seen[group.ModelKey] = true
		calls, active, unknown = calls+group.ModelCalls, active+group.ActiveCalls, unknown+group.UnknownUsageCalls
		input, output = input+group.InputTokens, output+group.OutputTokens
	}
	return calls == u.ModelCalls && active == u.ActiveCalls && unknown == u.UnknownUsageCalls && input == u.InputTokens && output == u.OutputTokens
}

func validProviderUsage(u ProviderUsage) bool {
	if !validHash(u.ModelKey) {
		return false
	}
	for _, value := range providerCounters(u) {
		if value < 0 || value > 1<<50 {
			return false
		}
	}
	if u.ActiveCalls < 0 || u.ActiveCalls > u.ModelCalls || u.ReportedCalls+u.UnknownUsageCalls+u.ActiveCalls > u.ModelCalls || u.FailedCalls > u.ModelCalls-u.ActiveCalls {
		return false
	}
	return u.PartialUsageCalls <= u.UnknownUsageCalls && u.CacheKnownCalls <= u.ReportedCalls+u.PartialUsageCalls && u.CachedInputTokens <= u.ReportedInputTokens && u.ReportedInputTokens <= u.InputTokens && u.ReportedOutputTokens <= u.OutputTokens
}

func providerCounters(u ProviderUsage) []int64 {
	return []int64{int64(u.ModelCalls), int64(u.FailedCalls), int64(u.UnknownUsageCalls), int64(u.ReportedCalls), int64(u.PartialUsageCalls), int64(u.CacheKnownCalls), u.InputTokens, u.OutputTokens, u.ReportedInputTokens, u.ReportedOutputTokens, u.CachedInputTokens}
}

func monotonicAccounting(previous, next BudgetUsage) bool {
	if previous.Accounting == nil {
		return next.Accounting == nil || previous.ModelCalls == 0
	}
	if next.Accounting == nil || previous.Accounting.Version != next.Accounting.Version {
		return false
	}
	for _, before := range previous.Accounting.Providers {
		index := slices.IndexFunc(next.Accounting.Providers, func(after ProviderUsage) bool { return after.ModelKey == before.ModelKey })
		if index < 0 || !monotonicProvider(before, next.Accounting.Providers[index]) {
			return false
		}
	}
	return true
}

func monotonicProvider(before, after ProviderUsage) bool {
	afterCounts := providerCounters(after)
	for i, count := range providerCounters(before) {
		if afterCounts[i] < count {
			return false
		}
	}
	return true
}
