package agentruntime

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func TestLiveSetupCancellationJoinsBlockedProvider(t *testing.T) {
	entered, closed := make(chan struct{}), make(chan struct{})
	client := liveProvider(t, func(conn *websocket.Conn, _ *http.Request) {
		defer close(closed)
		_, _, _ = conn.ReadMessage()
		close(entered)
		_, _, _ = conn.ReadMessage()
	})
	ctx, budget := budgetContext(t, nil)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := OpenLive(ctx, client, liveOptions()); result <- err }()
	<-entered
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("setup ignored cancellation")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("setup socket leaked")
	}
	if usage := budget.Snapshot(); usage.ActiveCalls != 0 || usage.UnknownUsageCalls != 1 {
		t.Fatalf("usage: %+v", usage)
	}
}

func TestLiveEndpointRequiresExactExplicitSafeNativeCredentials(t *testing.T) {
	base := config.ModelProfile{Provider: config.GeminiProvider, Model: "gemini-3.8-live", APIBaseURL: "https://example.com/proxy", APIKey: "key"}
	u, err := liveEndpoint(base)
	if err != nil || u.String() != "wss://example.com/proxy"+liveSocketPath {
		t.Fatalf("endpoint: %v %v", u, err)
	}
	for _, address := range []string{"http://example.com", "https://user:secret@example.com", "https://example.com?key=x", "https://example.com/#x", "https://example.com/%2e%2e/", "https://example.com/../", "ws://localhost:123"} {
		copy := base
		copy.APIBaseURL = address
		if _, err := liveEndpoint(copy); err == nil {
			t.Fatalf("accepted %s", address)
		}
	}
	for _, key := range []string{"", " key", "key\r\nheader", "auth_tokens/secret", strings.Repeat("x", 8193)} {
		copy := base
		copy.APIKey = key
		if _, err := liveEndpoint(copy); err == nil {
			t.Fatal("accepted unsupported credential")
		}
	}
}

func TestLiveHandshakeDoesNotFollowRedirectsOrExposeResponseText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://example.com/PRIVATE")
		w.WriteHeader(307)
		_, _ = w.Write([]byte("PRIVATE_PROVIDER_BODY"))
	}))
	defer server.Close()
	client := NewProviderClient(config.ModelProfile{Provider: config.GeminiProvider, Model: "gemini-3.8-live", APIBaseURL: server.URL, APIKey: "PRIVATE_KEY"})
	ctx, _ := budgetContext(t, nil)
	_, err := OpenLive(ctx, client, liveOptions())
	if !errors.Is(err, ErrLiveTransport) || strings.Contains(err.Error(), "PRIVATE") {
		t.Fatalf("unsafe redirect: %v", err)
	}
}

func TestLiveWireIncludesControlFramesInTotalByteBounds(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	bounded := &liveWireConn{Conn: left, input: 2, output: 2}
	bounded.ready.Store(true)
	bounded.reading.Store(liveHeaderBytes + 2)
	bounded.writing.Store(liveHeaderBytes + 2)
	if _, err := bounded.Read(make([]byte, 1)); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatal(err)
	}
	if _, err := bounded.Write([]byte{1}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatal(err)
	}
}
