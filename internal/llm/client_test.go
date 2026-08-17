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

			_, err := NewClient(server.URL, "", "fixture", 0, 0).Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}})
			if err == nil || !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "provider detail") {
				t.Fatalf("error = %v, want %q without provider body", err, test.want)
			}
		})
	}
}

func TestChatPropagatesCancellationAndLeavesEmptyChoicesForAgentValidation(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 2 {
			_ = json.NewEncoder(w).Encode(ChatResponse{Model: "fixture", Choices: []ChatChoice{}})
			return
		}
		close(started)
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := NewClient(server.URL, "", "fixture", 0, 0).Chat(ctx, []ChatMessage{{Role: "user", Content: "hello"}})
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	<-canceled

	client := NewClient(server.URL, "", "fixture", 0, 0)
	response, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}})
	if err != nil || len(response.Choices) != 0 {
		t.Fatalf("empty choices response = %+v, %v", response, err)
	}
}
