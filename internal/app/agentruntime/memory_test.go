package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/adk/v2/memory"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func memoryResult() agentstate.MemoryMatch {
	return agentstate.MemoryMatch{Record: agentstate.MemoryRecord{ID: "retained", Version: 2, Kind: "decision", State: "accepted", Title: "Scoped decision", Text: "Use a bounded queue.", ProjectRevision: "old-revision", UpdatedAt: time.Now()}, Stale: true}
}

func memoryAccess(t *testing.T, validate func(context.Context, []agentstate.MemoryVersion) error) *MemoryAccess {
	t.Helper()
	return &MemoryAccess{Query: "bounded queue", Search: func(_ context.Context, query string) ([]agentstate.MemoryMatch, error) {
		if query != "bounded queue" {
			t.Errorf("query=%q", query)
		}
		return []agentstate.MemoryMatch{memoryResult()}, nil
	}, Validate: validate}
}

func TestMemoryCompletionLabelsAdvisoryContextAndSharesBudget(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Messages []llm.ChatMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		text := request.Messages[0].Content
		for _, wanted := range []string{"untrusted advisory", `"stale":true`, "Use a bounded queue.", "Original request"} {
			if !strings.Contains(text, wanted) {
				t.Errorf("missing %q in %s", wanted, text)
			}
		}
		json.NewEncoder(w).Encode(fixtureResponse("answer"))
	}))
	defer server.Close()
	ctx, budget := budgetContext(t, nil)
	ctx, err := WithMemory(ctx, memoryAccess(t, func(_ context.Context, versions []agentstate.MemoryVersion) error {
		if len(versions) != 1 || versions[0] != (agentstate.MemoryVersion{ID: "retained", Version: 2}) {
			t.Errorf("versions=%+v", versions)
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	client := NewProviderClient(config.ModelProfile{Provider: config.OpenAIProvider, APIBaseURL: server.URL, Model: "fixture", MaxTokens: 100, ContextMaxTokens: 1000})
	request := Request{Messages: []llm.ChatMessage{{Role: "user", Content: "Original request"}}}
	result, err := Complete(ctx, client, request)
	usage := budget.Snapshot()
	if err != nil || result.Content != "answer" || calls.Load() != 1 || usage.ToolCalls != 1 || usage.ModelCalls != 1 || usage.InputTokens != 31 {
		t.Fatalf("completion=%+v calls=%d usage=%+v err=%v", result, calls.Load(), usage, err)
	}
	if request.Messages[0].Content != "Original request" {
		t.Fatal("mutated caller's context")
	}
}

func TestMemoryDispatchRechecksForgottenVersion(t *testing.T) {
	var calls, validations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		json.NewEncoder(w).Encode(fixtureResponse("unexpected"))
	}))
	defer server.Close()
	ctx, _ := budgetContext(t, nil)
	ctx, err := WithMemory(ctx, memoryAccess(t, func(context.Context, []agentstate.MemoryVersion) error {
		if validations.Add(1) > 1 {
			return agentstate.ErrMemoryConflict
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	client := NewProviderClient(config.ModelProfile{Provider: config.OpenAIProvider, APIBaseURL: server.URL, Model: "fixture", MaxTokens: 100})
	_, err = Complete(ctx, client, Request{Messages: []llm.ChatMessage{{Role: "user", Content: "request"}}})
	if !errors.Is(err, ErrPolicyDenied) || !errors.Is(err, agentstate.ErrMemoryConflict) || calls.Load() != 0 {
		t.Fatalf("forgotten record dispatched: calls=%d err=%v", calls.Load(), err)
	}
}

func TestMemoryRejectsIngestionForeignQueriesAndInvalidResults(t *testing.T) {
	service := &curatedMemoryService{access: memoryAccess(t, func(context.Context, []agentstate.MemoryVersion) error { return nil }), selected: map[string]int64{}}
	if err := service.AddSessionToMemory(t.Context(), nil); !errors.Is(err, ErrPolicyDenied) {
		t.Fatal(err)
	}
	for _, request := range []*memory.SearchRequest{nil, {AppName: "other", UserID: "local"}, {AppName: "mini-orca", UserID: "other"}, {AppName: "mini-orca", UserID: "local", Query: strings.Repeat("x", 513)}} {
		if _, err := service.SearchMemory(t.Context(), request); !errors.Is(err, ErrPolicyDenied) {
			t.Fatal(err)
		}
	}
	for _, invalid := range []string{"candidate", "oversized", "too_many"} {
		t.Run(invalid, func(t *testing.T) {
			service.access.Search = func(context.Context, string) ([]agentstate.MemoryMatch, error) {
				r := memoryResult()
				switch invalid {
				case "candidate":
					r.Record.State = "candidate"
				case "oversized":
					r.Record.Text = strings.Repeat("x", 2049)
				case "too_many":
					return make([]agentstate.MemoryMatch, 9), nil
				}
				return []agentstate.MemoryMatch{r}, nil
			}
			if _, err := service.SearchMemory(t.Context(), &memory.SearchRequest{AppName: "mini-orca", UserID: "local"}); !errors.Is(err, ErrPolicyDenied) {
				t.Fatal(err)
			}
		})
	}
}

func TestMemoryContextLimitFailsBeforeProvider(t *testing.T) {
	ctx, err := WithMemory(t.Context(), memoryAccess(t, func(context.Context, []agentstate.MemoryVersion) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	client := completionFixture(func(context.Context, []llm.ChatMessage, *llm.JSONSchema) (*llm.ChatResponse, error) {
		t.Fatal("provider called")
		return nil, nil
	})
	_, err = Complete(ctx, client, Request{Messages: []llm.ChatMessage{{Role: "user", Content: "request"}}, ContextMaxTokens: 2})
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatal(err)
	}
	if query := boundedMemoryQuery(strings.Repeat("界", 600)); len(query) > 512 || strings.ContainsRune(query, '\uFFFD') {
		t.Fatal("invalid UTF-8 query")
	}
}

func TestMemoryForgottenAfterToolTurnBlocksNextModelCall(t *testing.T) {
	var calls atomic.Int32
	var forgotten atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writeLookupCall(w, "lookup-1")
	}))
	defer server.Close()
	ctx, budget := budgetContext(t, nil)
	ctx, err := WithMemory(ctx, memoryAccess(t, func(context.Context, []agentstate.MemoryVersion) error {
		if forgotten.Load() {
			return agentstate.ErrMemoryConflict
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	investigation := fixtureInvestigation(t, func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		forgotten.Store(true)
		return args, nil
	}, nil)
	client := NewProviderClient(config.ModelProfile{Provider: config.OpenAIProvider, APIBaseURL: server.URL, Model: "fixture", MaxTokens: 100})
	_, err = Complete(ctx, client, Request{Messages: []llm.ChatMessage{{Role: "user", Content: "investigate"}}, Investigation: investigation})
	if !errors.Is(err, ErrPolicyDenied) || calls.Load() != 1 || budget.Snapshot().ToolCalls != 2 {
		t.Fatalf("later tool turn used forgotten memory: calls=%d usage=%+v err=%v", calls.Load(), budget.Snapshot(), err)
	}
}

func TestMemoryCanceledValidationKeepsParentRunActive(t *testing.T) {
	parent, _ := budgetContext(t, nil)
	child, cancel := context.WithCancel(parent)
	defer cancel()
	child, err := WithMemory(child, memoryAccess(t, func(ctx context.Context, _ []agentstate.MemoryVersion) error {
		cancel()
		return context.Cause(ctx)
	}))
	if err != nil {
		t.Fatal(err)
	}
	client := completionFixture(func(context.Context, []llm.ChatMessage, *llm.JSONSchema) (*llm.ChatResponse, error) {
		t.Fatal("canceled memory dispatched a provider")
		return nil, nil
	})
	_, err = Complete(child, client, Request{Messages: []llm.ChatMessage{{Role: "user", Content: "request"}}})
	if !errors.Is(err, context.Canceled) || context.Cause(parent) != nil {
		t.Fatalf("child cancellation poisoned parent: result=%v parent=%v", err, context.Cause(parent))
	}
}
