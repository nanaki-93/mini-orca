package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestChatRejectsMalformedNonOKAndOversizedProviderResponses(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "malformed JSON", status: http.StatusOK, body: "{", want: "failed to decode response"},
		{name: "non-OK", status: http.StatusServiceUnavailable, body: "provider detail must not be returned", want: "unexpected status code: 503"},
		{name: "oversized", status: http.StatusOK, body: strings.Repeat("x", maxProviderResponseBytes+1), want: "response exceeds"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()

			_, err := NewClient(server.URL, "test-key", "fixture", 0, 0).Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}})
			if err == nil || !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "provider detail") || strings.Contains(err.Error(), "test-key") {
				t.Fatalf("error = %v, want %q without provider body", err, test.want)
			}
		})
	}
}

func TestChatPropagatesCancellationAndLeavesEmptyChoicesForAgentValidation(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 2 {
			_ = json.NewEncoder(w).Encode(ChatResponse{Model: "fixture", Choices: []ChatChoice{}})
			return
		}
		started <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := NewClient(server.URL, "", "fixture", 0, 0).Chat(ctx, []ChatMessage{{Role: "user", Content: "hello"}})
		done <- err
	}()
	waitForTestSignal(t, started, "provider request")
	cancel()
	if err := waitForTestError(t, done, "canceled chat request"); err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	client := NewClient(server.URL, "", "fixture", 0, 0)
	response, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}})
	if err != nil || len(response.Choices) != 0 {
		t.Fatalf("empty choices response = %+v, %v", response, err)
	}
}

func TestAPIBaseClientPreservesCompatibilityPrefixAndRequestContract(t *testing.T) {
	for _, test := range []struct {
		name     string
		basePath string
	}{
		{name: "OpenAI API base", basePath: "/v1"},
		{name: "trailing slash", basePath: "/v1/"},
		{name: "Gemini compatibility API base", basePath: "/v1beta/openai"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if want := strings.TrimRight(test.basePath, "/") + "/chat/completions"; r.URL.Path != want {
					t.Fatalf("path = %q, want %q", r.URL.Path, want)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Fatalf("Authorization = %q", got)
				}
				var request ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if request.Model != "compatible-model" || request.Temperature != 0.2 || request.MaxTokens != 1234 || len(request.Messages) != 1 {
					t.Fatalf("request = %+v", request)
				}
				_ = json.NewEncoder(w).Encode(ChatResponse{Model: "compatible-model", Choices: []ChatChoice{{Message: ChatMessage{Role: "assistant", Content: "ok"}}}})
			}))
			defer server.Close()

			client := NewClientWithAPIBase(server.URL+test.basePath, "test-key", "compatible-model", 0.2, 1234)
			if _, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}); err != nil {
				t.Fatalf("Chat() error = %v", err)
			}
		})
	}
}

func TestNewClientRetainsLegacyHostPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(ChatResponse{Model: "legacy", Choices: []ChatChoice{{Message: ChatMessage{Role: "assistant", Content: "ok"}}}})
	}))
	defer server.Close()

	if _, err := NewClient(server.URL+"/", "", "legacy", 0, 0).Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
}

func TestListModelsUsesAPIBaseAndDoesNotGateChat(t *testing.T) {
	var chatRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1beta/openai/models":
			if got := r.Header.Get("Authorization"); got != "" {
				t.Fatalf("Authorization = %q, want empty", got)
			}
			w.WriteHeader(http.StatusNotFound)
		case "/v1beta/openai/chat/completions":
			if got := r.Header.Get("Authorization"); got != "" {
				t.Fatalf("Authorization = %q, want empty", got)
			}
			chatRequests.Add(1)
			_ = json.NewEncoder(w).Encode(ChatResponse{Model: "configured", Choices: []ChatChoice{{Message: ChatMessage{Role: "assistant", Content: "ok"}}}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClientWithAPIBase(server.URL+"/v1beta/openai", "", "configured", 0, 0)
	if _, err := client.ListModels(context.Background()); err == nil {
		t.Fatal("ListModels() error = nil")
	}
	if _, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if chatRequests.Load() != 1 {
		t.Fatalf("chat requests = %d, want 1", chatRequests.Load())
	}
}

func waitForTestSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForTestError(t *testing.T, done <-chan error, description string) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
		return nil
	}
}
