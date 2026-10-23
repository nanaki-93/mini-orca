package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

func agentHandlerFixture(t *testing.T) (*AgentRunHandler, string) {
	t.Helper()
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	return NewAgentRunHandler(fixture.service), query.Encode()
}

func TestAgentHistoryHandlerGuardsRoutesAndEmptySnapshot(t *testing.T) {
	h, query := agentHandlerFixture(t)
	for _, handler := range []http.HandlerFunc{h.List, h.Events} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest("GET", "/?"+query, nil))
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("history read: %d %s", w.Code, w.Body.String())
		}
	}
	for _, extra := range []string{"&project_id=foreign", "&unknown=true", "&limit=101", "&cursor=broken"} {
		w := httptest.NewRecorder()
		h.List(w, httptest.NewRequest("GET", "/?"+query+extra, nil))
		if w.Code != 400 {
			t.Fatalf("invalid query %s: %d %s", extra, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/?"+query, nil)
	r.SetPathValue("runID", "missing")
	h.Detail(w, r)
	if w.Code != 404 {
		t.Fatalf("missing invocation: %d %s", w.Code, w.Body.String())
	}
	for _, handler := range []http.HandlerFunc{h.Control, h.Input} {
		for _, body := range []string{`{"unknown":true}`, `{} {}`, `{}`} {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/", strings.NewReader(body))
			r.SetPathValue("runID", "missing")
			handler(w, r)
			if w.Code < 400 {
				t.Fatalf("invalid control accepted: %s", body)
			}
		}
	}
	w = httptest.NewRecorder()
	h.Events(w, httptest.NewRequest("GET", "/?"+query, nil))
	var batch app.AgentEventBatch
	if err := json.Unmarshal(w.Body.Bytes(), &batch); err != nil || !batch.Reset || batch.Snapshot == nil || len(batch.Events) != 0 {
		t.Fatalf("empty snapshot: %+v %v", batch, err)
	}
}

func TestAgentHistoryStreamCancellationAndCursorConflict(t *testing.T) {
	h, query := agentHandlerFixture(t)
	returned := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.Events(w, r); close(returned) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/?"+query, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "text/event-stream")
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	if !scanner.Scan() || scanner.Text() != "event: agent_progress" {
		t.Fatal("missing initial stream frame", scanner.Err())
	}
	cancel()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("stream retained subscriber after cancellation")
	}
	r := httptest.NewRequest("GET", "/?"+query+"&cursor=epoch:1", nil)
	r.Header.Set("Last-Event-ID", "epoch:2")
	w := httptest.NewRecorder()
	h.Events(w, r)
	if w.Code != 400 {
		t.Fatal("conflicting replay cursors accepted")
	}
}

type failedAgentWriter struct {
	deadline time.Time
	header   http.Header
}

func (w *failedAgentWriter) Header() http.Header                    { return w.header }
func (*failedAgentWriter) WriteHeader(int)                          {}
func (*failedAgentWriter) Write([]byte) (int, error)                { return 0, errors.New("client disconnected") }
func (w *failedAgentWriter) SetWriteDeadline(value time.Time) error { w.deadline = value; return nil }
func (*failedAgentWriter) FlushError() error                        { return nil }

func TestAgentStreamBoundsSlowWritesAndFrames(t *testing.T) {
	w := &failedAgentWriter{header: http.Header{}}
	controller := http.NewResponseController(w)
	if err := writeAgentBatch(w, controller, &app.AgentEventBatch{}); err == nil || time.Until(w.deadline) > 11*time.Second || w.deadline.IsZero() {
		t.Fatal("failed write was ignored or unbounded", err)
	}
	if err := writeAgentBatch(w, controller, &app.AgentEventBatch{Cursor: strings.Repeat("x", 1<<20)}); err == nil {
		t.Fatal("oversized frame accepted")
	}
}
