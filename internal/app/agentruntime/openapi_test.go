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

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

const apiFixture = `{"openapi":"3.0.3","info":{"title":"Fixture","version":"1"},"servers":[{"url":"https://unadmitted.invalid"}],"paths":{"/items/{id}":{"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"get":{"operationId":"lookup","summary":"Read an item","parameters":[{"name":"version","in":"query","schema":{"type":"integer"}}],"responses":{"200":{"description":"Item","content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"],"additionalProperties":false}}}}}},"post":{"operationId":"update","requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"],"additionalProperties":false}}}},"responses":{"204":{"description":"Updated"}}}}}}`

func apiInventoryFixture(t *testing.T, mutation bool) APIInventory {
	t.Helper()
	name, effect := "lookup", "read_external"
	if mutation {
		name, effect = "update", "mutate_external"
	}
	inventory, err := DiscoverAPI([]byte(apiFixture), []config.IntegrationTool{{Name: name, Effect: effect}})
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func TestOpenAPINativeReadAndProposalOnlyMutation(t *testing.T) {
	for _, mutation := range []bool{false, true} {
		t.Run(fmt.Sprint(mutation), func(t *testing.T) {
			inventory := apiInventoryFixture(t, mutation)
			var calls, proposals, turns atomic.Int32
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "GET" || r.URL.Path != "/v1/items/a b" || r.URL.Query().Get("version") != "9007199254740993" || r.Header.Get("Authorization") != "Bearer fixture-secret" {
					t.Errorf("changed API request: %s %s", r.Method, r.URL.String())
				}
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"id":7}`)
			}))
			defer remote.Close()
			connection := APIConnection{Endpoint: remote.URL + "/v1", BearerToken: "fixture-secret", Timeout: time.Second}
			authorize := func(ctx context.Context) error { return context.Cause(ctx) }
			tools := APITools(connection, "items", inventory, authorize, func(_ context.Context, action APIAction, _ json.RawMessage) (string, error) {
				proposals.Add(1)
				if action.Method != "POST" || string(action.Body) != `{"id":9007199254740993}` {
					t.Errorf("proposal lost exact payload: %+v", action)
				}
				return "candidate", nil
			})
			investigation, err := NewInvestigation(strings.Repeat("a", 64), tools, authorize)
			if err != nil {
				t.Fatal(err)
			}
			model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if strings.Contains(string(body), "fixture-secret") {
					t.Error("credential reached model")
				}
				if turns.Add(1) == 1 {
					args := `{"path":{"id":"a b"},"query":{"version":9007199254740993}}`
					if mutation {
						args = `{"path":{"id":"a b"},"body":{"id":9007199254740993}}`
					}
					skillToolResponse(w, false, tools[0].Name, args, 1)
					return
				}
				if mutation && !strings.Contains(string(body), "approval_required") {
					t.Error("proposal approval state missing")
				}
				json.NewEncoder(w).Encode(fixtureResponse("complete"))
			}))
			defer model.Close()
			client := NewProviderClient(config.ModelProfile{Provider: config.OpenAIProvider, APIBaseURL: model.URL, Model: "fixture", MaxTokens: 128})
			ctx, budget := budgetContext(t, nil)
			result, err := Complete(ctx, client, Request{Messages: []llm.ChatMessage{{Role: "user", Content: "Inspect the admitted API"}}, Investigation: investigation})
			if err != nil || result.Content != "complete" || budget.Snapshot().ToolCalls != 1 {
				t.Fatalf("native tool: %+v %v %+v", result, err, budget.Snapshot())
			}
			if mutation && (calls.Load() != 0 || proposals.Load() != 1) || !mutation && calls.Load() != 1 {
				t.Fatalf("calls=%d proposals=%d", calls.Load(), proposals.Load())
			}
		})
	}
}

func TestOpenAPIRejectsUnsupportedContractsAndUnsafeArguments(t *testing.T) {
	for name, document := range map[string]string{
		"version":       strings.Replace(apiFixture, "3.0.3", "3.1.0", 1),
		"reference":     strings.Replace(apiFixture, `"type":"string"`, `"$ref":"https://unadmitted.invalid/schema"`, 1),
		"serialization": strings.Replace(apiFixture, `"in":"query"`, `"in":"query","style":"deepObject"`, 1),
		"header":        strings.Replace(apiFixture, `"in":"query"`, `"in":"header"`, 1),
		"callback":      strings.Replace(apiFixture, `"operationId":"lookup"`, `"operationId":"lookup","callbacks":{}`, 1),
		"path":          strings.Replace(apiFixture, `"/items/{id}"`, `"/../items/{id}"`, 1),
		"duplicate":     strings.Replace(apiFixture, `"openapi":"3.0.3"`, `"openapi":"3.0.3","openapi":"3.0.3"`, 1),
		"schema type":   strings.Replace(apiFixture, `"type":"string"`, `"type":{}`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DiscoverAPI([]byte(document), []config.IntegrationTool{{Name: "lookup", Effect: "read_external"}}); err == nil {
				t.Fatal("unsupported contract accepted")
			}
		})
	}
	operation := apiInventoryFixture(t, false).Operations[0]
	for _, args := range []string{`{}`, `{"path":{"id":".."}}`, `{"path":{"id":"a/b"}}`, `{"path":{"id":"%2e%2e"}}`, `{"path":{"id":"a"},"query":{"secret":"x"}}`, `{"path":{"id":"a"},"body":{}}`} {
		if _, err := operation.Prepare("https://api.example", json.RawMessage(args)); err == nil {
			t.Fatalf("unsafe arguments accepted: %s", args)
		}
	}
	if _, err := DiscoverAPI([]byte(apiFixture), []config.IntegrationTool{{Name: "update", Effect: "read_external"}}); err == nil {
		t.Fatal("mutation classified as read")
	}
}

func TestOpenAPIWriteExchangeNeverRetriesAndRetainsAttemptState(t *testing.T) {
	operation := apiInventoryFixture(t, true).Operations[0]
	arguments := json.RawMessage(`{"path":{"id":"42"},"body":{"id":7}}`)
	for _, mode := range []string{"ok", "eof", "redirect", "oversize", "wrong_hash", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			var calls, redirected atomic.Int32
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
			defer other.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				io.Copy(io.Discard, r.Body)
				switch mode {
				case "eof":
					c, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					c.Close()
				case "redirect":
					http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
				case "oversize":
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, strings.Repeat("x", 20<<10))
				default:
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			defer server.Close()
			connection := APIConnection{Endpoint: server.URL, Timeout: time.Second}
			action, err := operation.Prepare(connection.Endpoint, arguments)
			if err != nil {
				t.Fatal(err)
			}
			expected := action.Fingerprint()
			if mode == "wrong_hash" {
				expected = strings.Repeat("0", 64)
			}
			authorize := func(ctx context.Context) error {
				if mode == "revoked" {
					return ErrPolicyDenied
				}
				return context.Cause(ctx)
			}
			response, err := ExecuteAPI(t.Context(), connection, operation, arguments, expected, authorize)
			if mode == "ok" {
				if err != nil || response.Status != 204 || !response.Attempted {
					t.Fatalf("completed exchange: %+v %v", response, err)
				}
			} else if err == nil {
				t.Fatal("failed exchange reported success")
			}
			want := int32(1)
			if mode == "wrong_hash" || mode == "revoked" {
				want = 0
			}
			if calls.Load() != want || redirected.Load() != 0 || response.Attempted != (want == 1) {
				t.Fatalf("calls=%d redirects=%d result=%+v", calls.Load(), redirected.Load(), response)
			}
		})
	}
}

func TestOpenAPICancellationJoinsOwnedExchange(t *testing.T) {
	started, closed := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(closed)
	}))
	defer server.Close()
	operation := apiInventoryFixture(t, true).Operations[0]
	arguments := json.RawMessage(`{"path":{"id":"42"},"body":{"id":7}}`)
	action, err := operation.Prepare(server.URL, arguments)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := ExecuteAPI(ctx, APIConnection{Endpoint: server.URL, Timeout: time.Second}, operation, arguments, action.Fingerprint(), func(ctx context.Context) error { return context.Cause(ctx) })
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; err == nil {
		t.Fatal("canceled exchange succeeded")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("remote request remained open")
	}
	if !errors.Is(context.Cause(ctx), context.Canceled) {
		t.Fatal("cancellation lost")
	}
}
