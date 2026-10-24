package tui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func TestMissionExecutionStreamRefreshesPassivelyReconnectsAndJoins(t *testing.T) {
	s := missionExecutionTestState(true)
	s.Mission.Execution.OwnerState = "active"
	request := *s.missionProgressRequest()
	f := missionHTTPFromState(s)
	f.execution.Execution.Revision++
	f.execution.Execution.Failure = "retained diagnostic"
	var reads, connections, writes atomic.Int32
	reconnected, closed := make(chan string, 1), make(chan struct{}, 1)
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes.Add(1)
			http.Error(w, "mutation", 400)
			return
		}
		if r.URL.Query().Get("worktree_path") != request.Scope.WorktreePath || r.URL.Query().Get("project_id") != request.Scope.ProjectID {
			t.Error("stream lost nested owner")
		}
		if strings.HasSuffix(r.URL.Path, "/events") {
			w.Header().Set("Content-Type", "text/event-stream")
			if connections.Add(1) == 1 {
				writeStreamBatch(t, w, client.AgentEventBatch{ProjectID: request.Scope.ProjectID, ProjectRevision: request.Scope.ProjectRevision, Epoch: "mission-epoch", Cursor: "mission-epoch:1", Reset: true, Snapshot: &client.AgentRunList{ProjectID: request.Scope.ProjectID, ProjectRevision: request.Scope.ProjectRevision, Runs: []AgentRun{}}})
				return
			}
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			reconnected <- r.Header.Get("Last-Event-ID")
			<-r.Context().Done()
			closed <- struct{}{}
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/execution") {
			t.Error("unexpected progress route", r.URL)
		}
		reads.Add(1)
		_ = json.NewEncoder(w).Encode(f.execution)
	})
	m := NewBubbleModel(t.Context(), s, cl, 80, 24)
	defer m.stopMissionExecutionStream()
	var commands []tea.Cmd
	m.syncMissionExecutionStream(&commands)
	stream := m.missionStream
	if stream == nil || len(commands) != 1 {
		t.Fatal("mission stream did not start")
	}
	var update missionExecutionUpdate
	select {
	case update = <-stream.updates:
	case <-time.After(5 * time.Second):
		t.Fatal("local event did not refresh execution")
	}
	s.DetailScroll, s.Mission.ChildID = 7, "use"
	s.Mission.Draft.Goal = "retained local edit"
	if command := m.applyMissionExecutionUpdate(update); command == nil {
		t.Fatal("stream did not continue")
	}
	if s.Mission.Execution.Execution.Revision != 2 || s.Mission.Execution.Execution.Failure != "retained diagnostic" || s.DetailScroll != 7 || s.Mission.ChildID != "use" || s.Mission.Draft.Goal != "retained local edit" {
		t.Fatal("progress replaced selection, edit or evidence")
	}
	select {
	case cursor := <-reconnected:
		if cursor != "mission-epoch:1" {
			t.Fatal("reconnect lost acknowledged cursor", cursor)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not reconnect")
	}
	s.Navigate(PageEditor)
	m.syncMissionExecutionStream(&commands)
	if m.missionStream != nil || writes.Load() != 0 || reads.Load() == 0 {
		t.Fatal("inspection dispatched or retained subscription")
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("stream HTTP request survived navigation")
	}
	select {
	case <-stream.done:
	default:
		t.Fatal("stream workers were not joined")
	}
}

func TestMissionExecutionStreamPreservesPendingConfirmationAndRejectsOldScope(t *testing.T) {
	s := missionExecutionTestState(false)
	s.Mission.Execution.OwnerState = "active"
	request := *s.missionProgressRequest()
	done := make(chan struct{})
	close(done)
	stream := &missionExecutionStream{request: request, done: done, cancel: func() {}}
	m := NewBubbleModel(t.Context(), s, nil, 80, 24)
	m.missionStream = stream
	value := *s.Mission.Execution
	value.Execution.Revision++
	foreign := &missionExecutionStream{request: request}
	if m.applyMissionExecutionUpdate(missionExecutionUpdate{stream: foreign, value: &value}) != nil || s.Mission.Execution.Execution.Revision != 1 {
		t.Fatal("replaced stream changed execution")
	}
	s.Mission.Pending = &missionAction{ID: "pending", Operation: "execution-control"}
	fingerprint := s.missionFingerprint()
	if m.applyMissionExecutionUpdate(missionExecutionUpdate{stream: stream, value: &value}) != nil || s.missionFingerprint() != fingerprint || s.Mission.Pending.ID != "pending" {
		t.Fatal("background progress invalidated its own mutation")
	}
	if m.missionStream != nil {
		t.Fatal("pending mutation did not stop inspection")
	}
}

func TestMissionExecutionStreamPollFailureStaysVisible(t *testing.T) {
	s := missionExecutionTestState(false)
	s.Mission.Execution.OwnerState = "waiting_for_owner"
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	})
	stream := startMissionExecutionStream(t.Context(), cl, *s.missionProgressRequest())
	defer stream.stop()
	select {
	case update := <-stream.updates:
		if update.err == nil || update.value != nil {
			t.Fatal("transport failure became valid execution")
		}
		m := NewBubbleModel(t.Context(), s, cl, 80, 24)
		m.missionStream = stream
		m.applyMissionExecutionUpdate(update)
		if s.Mission.ProgressFailure == "" || s.Mission.Execution.Execution.Revision != 1 {
			t.Fatal("failed progress replaced retained receipt")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("bounded fallback did not report unavailable progress")
	}
}
