package tui

import (
	"encoding/json"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func writeStreamBatch(t *testing.T, w http.ResponseWriter, batch client.AgentEventBatch) {
	t.Helper()
	data, err := json.Marshal(batch)
	if err != nil {
		t.Error(err)
		return
	}
	_, _ = fmt.Fprintf(w, "event: agent_progress\nid: %s\ndata: %s\n\n", batch.Cursor, data)
	w.(http.Flusher).Flush()
}

func TestExecutionStreamCancelsOnProjectSwitchAndRefreshesOnlyDisclosedText(t *testing.T) {
	s, detail := executionState()
	opened, privateRead, closed := make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/events") {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			opened <- struct{}{}
			<-r.Context().Done()
			closed <- struct{}{}
			return
		}
		if r.URL.Path != "/api/projects/current/agent-runs/model-call" || r.URL.Query().Get("include_text") != "true" {
			t.Errorf("unexpected private request: %s", r.URL)
		}
		response := detail
		response.Run = detail.Invocations[1]
		response.Text = &client.AgentPrivateText{Available: true, State: "partial_model_output", Text: "part one"}
		_ = json.NewEncoder(w).Encode(response)
		privateRead <- struct{}{}
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	request := *s.agentProgressRequest()
	s.AgentDisclosure = &agentDisclosure{Request: request, Invocation: "model-call", Status: "current", Detail: &detail}
	m := NewBubbleModel(t.Context(), s, cl, 80, 24)
	defer m.stopAgentStream()
	var cmds []tea.Cmd
	m.syncAgentStream(&cmds)
	select {
	case <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not open")
	}
	select {
	case <-privateRead:
	case <-time.After(5 * time.Second):
		t.Fatal("explicit private output did not refresh")
	}
	stream := m.agentStream
	update := <-stream.updates
	s.DetailScroll = 8
	m.applyAgentStreamUpdate(update)
	if s.AgentDisclosure.Detail.Text.Text != "part one" || s.DetailScroll != 8 {
		t.Fatal("private update lost text or moved scroll")
	}
	s.Project.ProjectRevision = "replacement"
	m.syncAgentStream(&cmds)
	if m.agentStream != nil || s.AgentDisclosure != nil {
		t.Fatal("scope change retained private subscription")
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("scope change did not cancel HTTP stream")
	}
	select {
	case <-stream.done:
	default:
		t.Fatal("scope change did not join worker")
	}
}

func TestExecutionStreamReconnectsWithCursorAndStops(t *testing.T) {
	s, detail := executionState()
	request := *s.agentProgressRequest()
	var connections, reads atomic.Int32
	reconnected := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/events") {
			reads.Add(1)
			_ = json.NewEncoder(w).Encode(detail)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if connections.Add(1) == 1 {
			writeStreamBatch(t, w, client.AgentEventBatch{ProjectID: "checkout", ProjectRevision: "rev", Epoch: "epoch", Cursor: "epoch:1", Reset: true, Snapshot: &client.AgentRunList{ProjectID: "checkout", ProjectRevision: "rev", Runs: []AgentRun{detail.Run}}})
			return
		}
		reconnected <- r.Header.Get("Last-Event-ID")
		for _, sequence := range []uint64{1, 2} {
			event := detail.Events[0]
			event.Sequence = sequence
			writeStreamBatch(t, w, client.AgentEventBatch{ProjectID: "checkout", ProjectRevision: "rev", Epoch: "epoch", Cursor: fmt.Sprintf("epoch:%d", sequence), Events: []AgentRunEvent{event}})
		}
		<-r.Context().Done()
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	stream := startAgentRunStream(t.Context(), cl, request, "")
	defer stream.stop()
	progress := 0
	for progress < 2 {
		select {
		case update := <-stream.updates:
			if update.progress != nil {
				progress++
			}
		case <-time.After(5 * time.Second):
			t.Fatal("stream did not reconnect")
		}
	}
	if cursor := <-reconnected; cursor != "epoch:1" {
		t.Fatalf("cursor=%q", cursor)
	}
	if reads.Load() != 2 {
		t.Fatalf("duplicate event triggered refresh: %d", reads.Load())
	}
	stream.stop()
	select {
	case <-stream.done:
	default:
		t.Fatal("stream did not join on stop")
	}
}

func TestExecutionStreamScopeReplacementDiscardsOldUpdates(t *testing.T) {
	s, detail := executionState()
	m := NewBubbleModel(t.Context(), s, nil, 80, 24)
	old := &agentRunStream{request: *s.agentProgressRequest()}
	current := &agentRunStream{request: old.request, done: make(chan struct{}), updates: make(chan agentStreamUpdate, 1)}
	m.agentStream = current
	s.DetailScroll = 11
	s.AgentStageID = "review"
	foreign := detail
	foreign.Events = []AgentRunEvent{{Sequence: 999}}
	if cmd := m.applyAgentStreamUpdate(agentStreamUpdate{stream: old, progress: &agentProgressView{Request: old.request, Detail: &foreign, Status: "current"}}); cmd != nil {
		t.Fatal("old stream scheduled continuation")
	}
	if progressSequence(s.AgentProgress.Detail) == 999 || s.DetailScroll != 11 || s.AgentStageID != "review" {
		t.Fatal("old stream changed current view")
	}
}
