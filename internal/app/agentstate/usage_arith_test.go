package agentstate

import (
	"strings"
	"testing"
)

func arithUsage(calls int, input, output int64, key string) BudgetUsage {
	return BudgetUsage{ModelCalls: calls, InputTokens: input, OutputTokens: output, Accounting: &UsageAccounting{Version: 1, Providers: []ProviderUsage{{ModelKey: key, ModelCalls: calls, ReportedCalls: calls, InputTokens: input, OutputTokens: output, ReportedInputTokens: input, ReportedOutputTokens: output}}}}
}

func TestUsageAddsAndSubtractsPerProvider(t *testing.T) {
	keyA, keyB := strings.Repeat("a", 64), strings.Repeat("b", 64)
	a, b := arithUsage(2, 100, 10, keyA), arithUsage(1, 50, 5, keyB)
	sum := BudgetUsage{}.Plus(a).Plus(b)
	if sum.ModelCalls != 3 || sum.InputTokens != 150 || len(sum.Accounting.Providers) != 2 || !validAccounting(sum) {
		t.Fatalf("sum: %+v", sum)
	}
	if delta := sum.Minus(a); delta.ModelCalls != 1 || delta.OutputTokens != 5 || len(delta.Accounting.Providers) != 1 || delta.Accounting.Providers[0].ModelKey != keyB || !validAccounting(delta) {
		t.Fatalf("delta keeps only the new provider: %+v", delta)
	}
	if a.Accounting.Providers[0].ModelCalls != 2 {
		t.Fatal("arithmetic mutated its operand")
	}
	legacy := BudgetUsage{ModelCalls: 1, InputTokens: 10}
	if mixed := a.Plus(legacy); mixed.Accounting != nil || mixed.ModelCalls != 3 {
		t.Fatalf("legacy usage kept a provider split: %+v", mixed)
	}
}
