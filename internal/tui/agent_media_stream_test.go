package tui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func awaitMediaSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("media request did not reach its synchronization point")
	}
}

func TestMediaProgressStopsAndJoinsOnNavigationScopeAndQuit(t *testing.T) {
	for _, change := range []string{"navigate", "scope", "selection", "pending", "quit"} {
		t.Run(change, func(t *testing.T) {
			s := mediaTestState()
			s.selectMediaGroup("sessions")
			started, canceled := make(chan struct{}), make(chan struct{})
			var writes atomic.Int32
			cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					writes.Add(1)
					http.Error(w, "mutation forbidden", http.StatusBadRequest)
					return
				}
				close(started)
				<-r.Context().Done()
				close(canceled)
			})
			m := NewBubbleModel(t.Context(), s, cl, 80, 24)
			var cmds []tea.Cmd
			m.syncMediaStream(&cmds)
			if len(cmds) != 1 || m.mediaStream == nil {
				t.Fatal("progress owner not started")
			}
			stream := m.mediaStream
			t.Cleanup(m.stopMediaStream)
			awaitMediaSignal(t, started)
			switch change {
			case "navigate":
				s.Navigate(PageSummary)
			case "scope":
				s.Project.ProjectRevision = "replacement"
			case "selection":
				s.AgentMedia.Session = nil
			case "pending":
				s.AgentMedia.Pending = "explicit-operation"
			case "quit":
				s.Quitting = true
			}
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			awaitMediaSignal(t, canceled)
			awaitMediaSignal(t, stream.done)
			if m.mediaStream != nil || writes.Load() != 0 {
				t.Fatal("watcher leaked or closed the daemon session implicitly")
			}
			if change == "navigate" && (!s.AgentMedia.Session.Owned || !strings.Contains(s.mediaActiveNotice(), ":media")) {
				t.Fatal("leaving hid the explicit close recovery")
			}
			if m.applyMediaStreamUpdate(mediaStreamUpdate{stream: stream, value: &client.LiveSession{State: "foreign"}}) != nil {
				t.Fatal("late progress was accepted")
			}
		})
	}
}

func TestMediaProgressUsesPassiveMetadataAndRetainsAcknowledgedStateOnError(t *testing.T) {
	s := mediaTestState()
	s.selectMediaGroup("sessions")
	value := *s.AgentMedia.Session
	value.Version, value.CompletedTurns, value.Reported, value.ReportedInput = 2, 1, true, 4
	var count atomic.Int32
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/live-sessions/"+value.ID) {
			t.Error("non-passive progress request", r.Method, r.URL)
		}
		_ = json.NewEncoder(w).Encode(value)
	})
	m := NewBubbleModel(t.Context(), s, cl, 80, 24)
	var cmds []tea.Cmd
	m.syncMediaStream(&cmds)
	t.Cleanup(m.stopMediaStream)
	update := cmds[0]().(mediaStreamUpdate)
	if update.err != nil {
		t.Fatal(update.err)
	}
	if next := m.applyMediaStreamUpdate(update); next == nil || s.AgentMedia.Session.Version != 2 || s.AgentMedia.Session.CompletedTurns != 1 {
		t.Fatal("acknowledged progress lost")
	}
	m.applyMediaStreamUpdate(mediaStreamUpdate{stream: m.mediaStream, err: errors.New("connection to daemon unavailable")})
	m.syncMediaStream(&cmds)
	if s.AgentMedia.Failures["sessions"] == "" || s.AgentMedia.Session.CompletedTurns != 1 || m.mediaStream != nil || count.Load() != 1 {
		t.Fatal("progress failure retried or erased acknowledged state")
	}
	if len(s.PendingActions) != 0 || s.Confirmation != nil {
		t.Fatal("progress error reconnected")
	}
}

func TestMediaProgressRejectsIdentityAndRevisionRegression(t *testing.T) {
	for _, kind := range []string{"binding", "control", "version", "epoch"} {
		t.Run(kind, func(t *testing.T) {
			s := mediaTestState()
			s.AgentMedia.Session.Version, s.AgentMedia.Session.ControlRevision = 3, 2
			s.selectMediaGroup("sessions")
			request := s.mediaProgressRequest()
			stream := &mediaStream{request: *request}
			m := NewBubbleModel(t.Context(), s, nil, 80, 24)
			m.mediaStream = stream
			value := *s.AgentMedia.Session
			value.CompletedTurns = 99
			switch kind {
			case "binding":
				value.Configuration = strings.Repeat("d", 64)
			case "control":
				value.ControlRevision = 1
			case "version":
				value.Version = 2
			case "epoch":
				s.AgentMedia.Epoch = "replaced"
			}
			m.applyMediaStreamUpdate(mediaStreamUpdate{stream: stream, value: &value})
			if s.AgentMedia.Session.CompletedTurns != 0 {
				t.Fatal("stale or foreign progress accepted")
			}
		})
	}
}

func TestMediaCancelJoinsTransportAndDiscardsLateResult(t *testing.T) {
	s := mediaTestState()
	s.ExecuteCommand(":live-send retain on cancel")
	s.HandleKey(KeyEvent{Key: "Enter"})
	started, ended := make(chan struct{}), make(chan struct{})
	release := make(chan struct{})
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
			close(ended)
		case <-release:
		}
	})
	t.Cleanup(func() { close(release) })
	m, cmd := mediaPending(t, s, cl)
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	awaitMediaSignal(t, started)
	s.ExecuteCommand(":media-cancel")
	awaitMediaSignal(t, ended)
	select {
	case value := <-result:
		m.Update(value)
	case <-time.After(5 * time.Second):
		t.Fatal("canceled transport did not join")
	}
	if s.AgentMedia.Pending != "" || s.AgentMedia.LiveDraft != "retain on cancel" || s.AgentMedia.Recovery == "" || s.AgentMedia.Session.Turns != 0 {
		t.Fatal("cancel lost draft or published a late receipt")
	}
	if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "cancel-agent-media" {
		t.Fatal("cancel queued another provider action")
	}
	s.dispatchMediaAction(context.Background(), cl, s.PendingActions[0])
}
