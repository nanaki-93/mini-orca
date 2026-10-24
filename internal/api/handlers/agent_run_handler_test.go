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
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
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

func TestAgentDetailTraceAndCostContractRemainsScopedAndSourceFree(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	store, err := agentstate.Open(fixture.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	key := agentstate.Key{App: "mini-orca", User: "local", ID: "trace-run"}
	if _, err := store.CreateSession(t.Context(), key, agentstate.State{"private": json.RawMessage(`"PRIVATE_PROMPT"`)}); err != nil {
		t.Fatal(err)
	}
	runtime, err := config.ResolveAgentRuntime(config.AgentRuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Checkpoint(t.Context(), agentstate.BudgetRecord{ID: "budget", Binding: agentstate.Binding{Project: index.ProjectID, Revision: index.ProjectRevision, Policy: "policy", Configuration: "config"}, Limits: runtime.Limits, Sequence: 1}, nil); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := store.BeginRun(t.Context(), agentstate.Run{ID: key.ID, Session: key, BudgetID: "budget", RequestHash: strings.Repeat("a", 64), SchemaHash: strings.Repeat("b", 64), SchemaVersion: "completion-v1", Status: "running", Started: now, Updated: now}, nil); err != nil {
		t.Fatal(err)
	}
	span := agentstate.TraceSpan{ID: "0123456789abcdef", Kind: "provider", Status: "running", Started: now}
	if _, err := store.BeginSpan(t.Context(), "budget", span); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishSpan(t.Context(), "budget", span.ID, "failed", now.Add(time.Millisecond), 1e6); err != nil {
		t.Fatal(err)
	}
	h := NewAgentRunHandler(fixture.service)
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/?"+query.Encode(), nil)
	r.SetPathValue("runID", key.ID)
	h.Detail(w, r)
	var detail client.AgentRunDetail
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || detail.Trace == nil || detail.Trace.Legacy || len(detail.Trace.Spans) != 1 || detail.Trace.Spans[0].Status != "failed" || detail.Cost.Status != "unknown" || detail.Cost.Total != nil || detail.Text != nil || strings.Contains(w.Body.String(), "PRIVATE_") {
		t.Fatalf("unsafe trace contract: %d %s", w.Code, w.Body.String())
	}
	query.Set("project_revision", "foreign")
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/?"+query.Encode(), nil)
	r.SetPathValue("runID", key.ID)
	h.Detail(w, r)
	if w.Code != http.StatusConflict || strings.Contains(w.Body.String(), span.ID) {
		t.Fatal("foreign trace disclosed", w.Code)
	}
}
