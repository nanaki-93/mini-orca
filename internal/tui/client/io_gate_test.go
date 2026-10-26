package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type recordingGate struct {
	mu     sync.Mutex
	events []string
	inIO   bool
}

func (g *recordingGate) BeginIO() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.events = append(g.events, "begin")
	g.inIO = true
}

func (g *recordingGate) EndIO() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.events = append(g.events, "end")
	g.inIO = false
}

func (g *recordingGate) isInIO() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.inIO
}

func TestRequestsReportBlockingIOToTheContextGate(t *testing.T) {
	gate := &recordingGate{}
	var duringRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		duringRequest = gate.isInIO()
		_, _ = w.Write([]byte(`{"version":"test"}`))
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	if _, err := cl.Request(WithIOGate(context.Background(), gate), "gated", http.MethodGet, "/status", ""); err != nil {
		t.Fatal(err)
	}
	if !duringRequest {
		t.Fatal("the gate must be open while the daemon handles the request")
	}
	if len(gate.events) != 2 || gate.events[0] != "begin" || gate.events[1] != "end" {
		t.Fatalf("gate events = %v, want one begin/end pair", gate.events)
	}

	// Requests without a gate and rejected requests never touch it.
	if _, err := cl.Request(context.Background(), "plain", http.MethodGet, "/status", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.Request(WithIOGate(context.Background(), gate), "bad", http.MethodDelete, "/status", ""); err == nil {
		t.Fatal("unsupported route must be rejected")
	}
	if len(gate.events) != 2 {
		t.Fatalf("gate touched outside daemon I/O: %v", gate.events)
	}
	EnterIO(context.Background())() // no gate: a no-op
}
