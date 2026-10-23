package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const testToolSchema = `{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"],"additionalProperties":false}`

func fixtureInvestigation(t *testing.T, handler func(context.Context, json.RawMessage) (json.RawMessage, error), authorize func(context.Context) error) *Investigation {
	t.Helper()
	if authorize == nil {
		authorize = func(context.Context) error { return nil }
	}
	if handler == nil {
		handler = func(_ context.Context, args json.RawMessage) (json.RawMessage, error) { return args, nil }
	}
	result, err := NewInvestigation(strings.Repeat("a", 64), []ToolSpec{{Name: "lookup", Description: "read admitted evidence", InputSchema: json.RawMessage(testToolSchema), OutputSchema: json.RawMessage(testToolSchema), Run: handler}}, authorize)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestGuardedToolValidatesActualDispatchAndCountsOnce(t *testing.T) {
	ctx, budget := budgetContext(t, nil)
	inv := fixtureInvestigation(t, nil, nil)
	ctx = context.WithValue(ctx, investigationKey{}, inv)
	args := map[string]any{"id": json.Number("9007199254740993")}
	for range 3 {
		if err := checkToolGrant(ctx, inv.tools[0]); err != nil {
			t.Fatal(err)
		}
	}
	result, err := inv.tools[0].execute(ctx, args)
	if err != nil || result["id"] != args["id"] || budget.Snapshot().ToolCalls != 1 {
		t.Fatalf("result=%v error=%v usage=%+v", result, err, budget.Snapshot())
	}
	if _, err := inv.tools[0].execute(context.Background(), args); !errors.Is(err, ErrPolicyDenied) {
		t.Fatal("unguarded dispatch", err)
	}
	other := fixtureInvestigation(t, nil, nil)
	if err := checkToolGrant(ctx, other.tools[0]); !errors.Is(err, ErrPolicyDenied) {
		t.Fatal("replaced tool admitted", err)
	}
}

func TestGuardedToolRejectsSchemaViolationsAndChangedAuthority(t *testing.T) {
	for _, args := range []any{nil, map[string]any{}, map[string]any{"id": "text"}, map[string]any{"id": 1, "extra": 1}, map[string]any{"id": 1.5}} {
		ctx, budget := budgetContext(t, nil)
		inv := fixtureInvestigation(t, func(context.Context, json.RawMessage) (json.RawMessage, error) {
			t.Fatal("invalid input executed")
			return nil, nil
		}, nil)
		ctx = context.WithValue(ctx, investigationKey{}, inv)
		if _, err := inv.tools[0].execute(ctx, args); err == nil || budget.Snapshot().ToolCalls != 0 {
			t.Fatalf("invalid input accepted: %v %v", args, err)
		}
	}
	changed := errors.New("source changed")
	ctx, _ := budgetContext(t, nil)
	inv := fixtureInvestigation(t, nil, func(context.Context) error { return changed })
	ctx = context.WithValue(ctx, investigationKey{}, inv)
	if _, err := inv.tools[0].execute(ctx, map[string]any{"id": 1}); !errors.Is(err, changed) || !errors.Is(err, ErrPolicyDenied) {
		t.Fatal(err)
	}
}

func TestGuardedToolRejectsInvalidOutputAndRetrievalOverrun(t *testing.T) {
	for _, output := range []string{`{"id":"bad"}`, `{"id":1,"id":2}`, `{"id":1} {}`, strings.Repeat(" ", maxToolPayload) + `{"id":1}`} {
		ctx, _ := budgetContext(t, nil)
		inv := fixtureInvestigation(t, func(context.Context, json.RawMessage) (json.RawMessage, error) { return json.RawMessage(output), nil }, nil)
		ctx = context.WithValue(ctx, investigationKey{}, inv)
		if _, err := inv.tools[0].execute(ctx, map[string]any{"id": 1}); err == nil {
			t.Fatal("invalid output published")
		}
	}
	ctx, _ := budgetContext(t, nil)
	inv := fixtureInvestigation(t, nil, nil)
	inv.outputBytes = 128 << 10
	ctx = context.WithValue(ctx, investigationKey{}, inv)
	if _, err := inv.tools[0].execute(ctx, map[string]any{"id": 1}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatal(err)
	}
}
