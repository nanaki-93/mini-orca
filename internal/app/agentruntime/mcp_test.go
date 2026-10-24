package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

type mcpFixture struct {
	server  *httptest.Server
	calls   atomic.Int32
	lists   atomic.Int32
	changed atomic.Bool
	mode    string
}

func newMCPFixture(t *testing.T, mode string) *mcpFixture {
	t.Helper()
	f := &mcpFixture{mode: mode}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-token" {
			t.Error("missing endpoint-bound credential")
		}
		if r.Method != "POST" {
			t.Errorf("unexpected MCP method %s", r.Method)
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if len(request.ID) == 0 {
			w.WriteHeader(202)
			return
		}
		result := any(map[string]any{})
		switch request.Method {
		case "ping":
			// Native ADK probes a failed connection before attempting its retry.
		case "initialize":
			if strings.Contains(string(request.Params), `"roots"`) || strings.Contains(string(request.Params), `"sampling"`) {
				t.Error("ambient capability advertised")
			}
			result = map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "fixture", "version": "1"}}
		case "tools/list":
			f.lists.Add(1)
			description := "Read fixture docs"
			if f.changed.Load() {
				description = "Changed contract"
			}
			result = map[string]any{"tools": []any{map[string]any{"name": "lookup", "description": description, "inputSchema": json.RawMessage(testToolSchema), "outputSchema": json.RawMessage(testToolSchema)}}}
			if mode == "pages" {
				result = map[string]any{"tools": []any{}, "nextCursor": "again"}
			}
		case "tools/call":
			f.calls.Add(1)
			result = map[string]any{"structuredContent": map[string]any{"id": 7}, "content": []any{}}
			switch mode {
			case "precise":
				if !strings.Contains(string(request.Params), `"id":9007199254740993`) {
					t.Errorf("outbound integer lost precision: %s", request.Params)
				}
			case "large_integer":
				result = json.RawMessage(`{"structuredContent":{"id":9007199254740993},"content":[]}`)
			case "media":
				result = map[string]any{"content": []any{map[string]any{"type": "image", "data": "AA==", "mimeType": "image/png"}}}
			case "oversize":
				result = map[string]any{"content": []any{map[string]any{"type": "text", "text": strings.Repeat("x", 17000)}}}
			case "error":
				result = map[string]any{"isError": true, "content": []any{map[string]any{"type": "text", "text": "PRIVATE_SERVER_ERROR"}}}
			case "eof":
				c, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				c.Close()
				return
			}
		default:
			t.Errorf("unexpected protocol method %s", request.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *mcpFixture) connection() MCPConnection {
	return MCPConnection{Endpoint: f.server.URL, BearerToken: "private-token", Timeout: 2 * time.Second}
}

func TestMCPNativeToolLoopAndSharedBudget(t *testing.T) {
	for _, google := range []bool{false, true} {
		t.Run(fmt.Sprint(google), func(t *testing.T) {
			mode, arguments := "precise", `{"id":9007199254740993}`
			if google {
				mode, arguments = "", `{"id":7}`
			}
			f := newMCPFixture(t, mode)
			authorize := func(ctx context.Context) error { return context.Cause(ctx) }
			inventory, err := DiscoverMCP(t.Context(), f.connection(), "docs", []string{"lookup"}, authorize)
			if err != nil || f.calls.Load() != 0 {
				t.Fatalf("discovery: %+v %v", inventory, err)
			}
			investigation, err := NewInvestigation(strings.Repeat("a", 64), MCPTools(f.connection(), "docs", inventory, authorize), authorize)
			if err != nil {
				t.Fatal(err)
			}
			var turns atomic.Int32
			handler := func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if strings.Contains(string(body), "private-token") {
					t.Error("credential reached model")
				}
				if turns.Add(1) == 1 {
					skillToolResponse(w, google, inventory.Tools[0].Name, arguments, 1)
					return
				}
				if !strings.Contains(string(body), "output") {
					t.Error("native tool result missing")
				}
				if google {
					io.WriteString(w, googleTextResponse)
				} else {
					json.NewEncoder(w).Encode(fixtureResponse("complete"))
				}
			}
			var client *ProviderClient
			if google {
				client = googleFixture(t, handler)
			} else {
				server := httptest.NewServer(http.HandlerFunc(handler))
				defer server.Close()
				client = NewProviderClient(config.ModelProfile{Provider: config.OpenAIProvider, APIBaseURL: server.URL, Model: "fixture", MaxTokens: 128})
			}
			ctx, budget := budgetContext(t, nil)
			result, err := Complete(ctx, client, Request{Messages: []llm.ChatMessage{{Role: "user", Content: "Inspect admitted source"}}, Investigation: investigation})
			if err != nil || result.Content != "complete" || f.calls.Load() != 1 || f.lists.Load() != 2 || budget.Snapshot().ToolCalls != 1 {
				t.Fatalf("result=%+v err=%v calls=%d lists=%d budget=%+v", result, err, f.calls.Load(), f.lists.Load(), budget.Snapshot())
			}
		})
	}
}

func TestMCPRejectsChangedSchemasUnsupportedResultsAndRetries(t *testing.T) {
	for _, mode := range []string{"changed", "media", "oversize", "error", "eof", "large_integer"} {
		t.Run(mode, func(t *testing.T) {
			f := newMCPFixture(t, mode)
			authorize := func(ctx context.Context) error { return context.Cause(ctx) }
			inventory, err := DiscoverMCP(t.Context(), f.connection(), "docs", []string{"lookup"}, authorize)
			if err != nil {
				t.Fatal(err)
			}
			f.changed.Store(mode == "changed")
			investigation, err := NewInvestigation(strings.Repeat("b", 64), MCPTools(f.connection(), "docs", inventory, authorize), authorize)
			if err != nil {
				t.Fatal(err)
			}
			var turns atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				turns.Add(1)
				skillToolResponse(w, false, inventory.Tools[0].Name, `{"id":7}`, 1)
			}))
			defer server.Close()
			client := NewProviderClient(config.ModelProfile{Provider: config.OpenAIProvider, APIBaseURL: server.URL, Model: "fixture", MaxTokens: 128})
			ctx, _ := budgetContext(t, nil)
			_, err = Complete(ctx, client, Request{Messages: []llm.ChatMessage{{Role: "user", Content: "Inspect"}}, Investigation: investigation})
			if err == nil || !errors.Is(err, ErrPolicyDenied) || strings.Contains(err.Error(), "PRIVATE_SERVER_ERROR") || turns.Load() != 1 {
				t.Fatalf("err=%v turns=%d", err, turns.Load())
			}
			want := int32(1)
			if mode == "changed" {
				want = 0
			}
			if f.calls.Load() != want {
				t.Fatalf("outbound calls=%d want=%d", f.calls.Load(), want)
			}
		})
	}
}

func TestMCPDiscoveryLimitsAndProtocolRetryGuard(t *testing.T) {
	f := newMCPFixture(t, "pages")
	authorize := func(ctx context.Context) error { return context.Cause(ctx) }
	if _, err := DiscoverMCP(t.Context(), f.connection(), "docs", []string{"lookup"}, authorize); err == nil || f.lists.Load() != 4 {
		t.Fatalf("pages=%d err=%v", f.lists.Load(), err)
	}
	g := &mcpProtocolGuard{tools: map[string]*mcp.Tool{}, callName: "lookup", authorize: authorize}
	var sent int
	guarded := g.middleware(func(context.Context, string, mcp.Request) (mcp.Result, error) { sent++; return nil, io.EOF })
	request := &mcp.ClientRequest[*mcp.CallToolParams]{Params: &mcp.CallToolParams{Name: "lookup", Arguments: map[string]any{"id": 7}}}
	for range 2 {
		if _, err := guarded(t.Context(), "tools/call", request); err == nil {
			t.Fatal("failed call reported success")
		}
	}
	if sent != 1 {
		t.Fatalf("native retry dispatched %d times", sent)
	}
	for _, schema := range []string{`{"type":"object","$ref":"https://elsewhere/schema"}`, `{"type":"object","properties":{"id":{"$ref":"#/$defs/id"}},"$defs":{"id":{"type":"integer"}}}`} {
		if _, err := mcpSchema(json.RawMessage(schema)); err == nil {
			t.Fatal("unsupported reference accepted")
		}
	}
	var rounded any
	if err := json.Unmarshal([]byte(`{"type":"object","properties":{"id":{"type":"integer","maximum":9007199254740993}}}`), &rounded); err != nil {
		t.Fatal(err)
	}
	if _, err := mcpSchema(rounded); err == nil {
		t.Fatal("potentially rounded schema bound accepted")
	}
	if !mcpSafeNumbers(map[string]any{"values": []any{float64(7), 1.25, "9007199254740993"}}) {
		t.Fatal("safe numbers or string identifier rejected")
	}
}

func TestMCPHTTPRejectsRedirectsAndCancellation(t *testing.T) {
	var destination atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destination.Add(1) }))
	defer other.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	authorize := func(ctx context.Context) error { return context.Cause(ctx) }
	_, err := DiscoverMCP(t.Context(), MCPConnection{Endpoint: redirect.URL, BearerToken: "secret", Timeout: time.Second}, "docs", []string{"lookup"}, authorize)
	if err == nil || destination.Load() != 0 {
		t.Fatalf("redirect followed: %v %d", err, destination.Load())
	}
	started := make(chan struct{})
	closed := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
			close(closed)
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := DiscoverMCP(ctx, MCPConnection{Endpoint: server.URL, Timeout: time.Second}, "docs", []string{"lookup"}, authorize)
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled discovery succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("discovery did not stop")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("HTTP request was not closed")
	}
}
