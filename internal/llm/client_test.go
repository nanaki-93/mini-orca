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

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func TestChatRejectsMalformedNonSuccessAndOversizedProviderResponses(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "malformed JSON", status: http.StatusOK, body: "{", want: "decode chat response"},
		{name: "non-success", status: http.StatusServiceUnavailable, body: `{"error":{"message":"provider detail must not be returned"}}`, want: "provider returned status 503"},
		{name: "oversized", status: http.StatusOK, body: strings.Repeat("x", maxProviderResponseBytes+1), want: "response exceeds"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()

			_, err := NewClient(testProfile(server.URL, "test-key")).Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}})
			if err == nil || !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "provider detail") || strings.Contains(err.Error(), "test-key") {
				t.Fatalf("error = %v, want %q without provider body or API key", err, test.want)
			}
		})
	}
}

func TestChatPropagatesCancellationAndRejectsMissingContent(t *testing.T) {
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
		_, err := NewClient(testProfile(server.URL, "")).Chat(ctx, []ChatMessage{{Role: "user", Content: "hello"}})
		done <- err
	}()
	waitForTestSignal(t, started, "provider request")
	cancel()
	if err := waitForTestError(t, done, "canceled chat request"); err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	_, err := NewClient(testProfile(server.URL, "")).Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}})
	if err == nil || !strings.Contains(err.Error(), "chat response has no content") {
		t.Fatalf("missing content error = %v", err)
	}
}

func TestClientPreservesConfiguredCompatibilityPrefixAndRequestContract(t *testing.T) {
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
				if request.Model != "compatible-model" || request.Temperature != 0.2 || request.MaxTokens != 1234 || request.ReasoningEffort != "" || len(request.Messages) != 1 {
					t.Fatalf("request = %+v", request)
				}
				_ = json.NewEncoder(w).Encode(ChatResponse{Model: "compatible-model", Choices: []ChatChoice{{Message: ChatMessage{Role: "assistant", Content: "ok"}}}})
			}))
			defer server.Close()

			profile := testProfile(server.URL+test.basePath, "test-key")
			profile.Model, profile.Temperature, profile.MaxTokens = "compatible-model", 0.2, 1234
			if _, err := NewClient(profile).Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}); err != nil {
				t.Fatalf("Chat() error = %v", err)
			}
		})
	}
}

func TestClientForwardsConfiguredReasoningEffort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.ReasoningEffort != "high" {
			t.Fatalf("reasoning effort = %q, want high", request.ReasoningEffort)
		}
		_ = json.NewEncoder(w).Encode(ChatResponse{Model: "reasoning-model", Choices: []ChatChoice{{Message: ChatMessage{Role: "assistant", Content: "ok"}}}})
	}))
	defer server.Close()

	profile := testProfile(server.URL+"/v1", "")
	profile.Model, profile.ReasoningEffort = "reasoning-model", "high"
	if _, err := NewClient(profile).Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
}

func testProfile(apiBaseURL, apiKey string) config.ModelProfile {
	return config.ModelProfile{APIBaseURL: apiBaseURL, APIKey: apiKey, Model: "fixture", Temperature: 0.2, MaxTokens: 1234}
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
