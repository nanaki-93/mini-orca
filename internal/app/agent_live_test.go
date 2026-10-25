package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

func liveAppServer(t *testing.T, handle func(*websocket.Conn)) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	var sockets []*websocket.Conn
	var workers sync.WaitGroup
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workers.Add(1)
		defer workers.Done()
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		mu.Lock()
		sockets = append(sockets, conn)
		mu.Unlock()
		defer conn.Close()
		var setup map[string]any
		if err := conn.ReadJSON(&setup); err != nil {
			return
		}
		if err := conn.WriteJSON(map[string]any{"setupComplete": map[string]any{}}); err != nil {
			return
		}
		handle(conn)
	}))
	t.Cleanup(func() {
		server.Close()
		mu.Lock()
		for _, conn := range sockets {
			_ = conn.Close()
		}
		mu.Unlock()
		workers.Wait()
	})
	return server
}

func liveAppFixture(t *testing.T, url string) (*Service, LiveOpenRequest) {
	t.Helper()
	s, request := researchService(t, url)
	s.agentRuntime.LiveModel = "gemini-3.8-live"
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := s.ShutdownChangeWorkflows(ctx); err != nil {
			t.Error(err)
		}
	})
	view, err := s.LiveSessions(t.Context(), AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision})
	if err != nil {
		t.Fatal(err)
	}
	return s, LiveOpenRequest{LiveBinding: view.LiveBinding, ConfirmConnect: true, ConfirmRemoteProvider: true}
}

func liveControl(view *LiveSession, action string) LiveControl {
	return LiveControl{LiveBinding: liveRecordBinding(view.LiveRecord), ControlRevision: view.ControlRevision, Action: action, ConfirmConnect: true, ConfirmSourceSharing: true, ConfirmRemoteProvider: true}
}

func joinLiveConnection(t *testing.T, s *Service, id string) {
	t.Helper()
	owner := s.live.owner(s.manager.Root(), id)
	owner.mu.Lock()
	conn := owner.connection
	owner.mu.Unlock()
	if conn != nil {
		select {
		case <-conn.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("connection did not finish")
		}
	}
	owner.workers.Wait()
}

func TestLiveProjectControlsReconnectRetainBudgetAndNeverReplayInput(t *testing.T) {
	var connects, inputs atomic.Int32
	server := liveAppServer(t, func(conn *websocket.Conn) {
		connects.Add(1)
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		inputs.Add(1)
		if !bytes.Contains(data, []byte("PRIVATE_USER_TEXT")) {
			t.Error("unexpected input")
		}
		_ = conn.WriteJSON(map[string]any{"serverContent": map[string]any{"outputTranscription": map[string]any{"text": "PRIVATE_LIVE_REPLY"}, "turnComplete": true}})
		_ = conn.WriteJSON(map[string]any{"goAway": map[string]any{"timeLeft": "1s"}})
	})
	s, open := liveAppFixture(t, server.URL)
	query := AgentRunQuery{ProjectID: open.ProjectID, ProjectRevision: open.ProjectRevision}
	before, err := os.ReadFile(filepath.Join(s.manager.Root(), "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.OpenLiveSession(t.Context(), open)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Owned || view.State != "connected" || connects.Load() != 1 || inputs.Load() != 0 {
		t.Fatalf("open: %+v", view)
	}
	control := liveControl(view, "send")
	control.Text = "PRIVATE_USER_TEXT"
	if _, err := s.ControlLiveSession(t.Context(), view.ID, control); err != nil {
		t.Fatal(err)
	}
	joinLiveConnection(t, s, view.ID)
	view, err = s.LiveSessionDetail(t.Context(), query, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.State != "go_away" || !view.CanReconnect || len(view.OutputArtifacts) != 1 || view.Usage.UnknownUsageCalls != 1 {
		t.Fatalf("retained: %+v", view)
	}
	metadata, _ := json.Marshal(view)
	if bytes.Contains(metadata, []byte("PRIVATE_")) {
		t.Fatal("transcript escaped receipt")
	}
	detail, err := s.ArtifactDetail(t.Context(), query, view.OutputArtifacts[0].ID)
	if err != nil || !bytes.Contains([]byte(detail.Text), []byte("PRIVATE_LIVE_REPLY")) {
		t.Fatalf("private transcript: %+v %v", detail, err)
	}
	expires, charge := view.ExpiresAt, view.Usage.InputTokens
	stale := liveControl(view, "reconnect")
	stale.ControlRevision--
	if _, err := s.ControlLiveSession(t.Context(), view.ID, stale); !errors.Is(err, agentstate.ErrLiveConflict) {
		t.Fatal(err)
	}
	view, err = s.ControlLiveSession(t.Context(), view.ID, liveControl(view, "reconnect"))
	if err != nil {
		t.Fatal(err)
	}
	if view.Connections != 2 || view.Usage.ModelCalls != 2 || view.Usage.InputTokens != charge || !view.ExpiresAt.Equal(expires) || inputs.Load() != 1 {
		t.Fatalf("reconnect reset or replayed: %+v inputs=%d", view, inputs.Load())
	}
	view, err = s.ControlLiveSession(t.Context(), view.ID, liveControl(view, "close"))
	if err != nil {
		t.Fatal(err)
	}
	if view.Owned || view.State != "closed" || view.Usage.ActiveCalls != 0 || view.Usage.UnknownUsageCalls != 2 {
		t.Fatalf("close: %+v", view)
	}
	after, err := os.ReadFile(filepath.Join(s.manager.Root(), "main.go"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("live advice changed source")
	}
}

func TestLiveOwnershipCancellationReindexSettingsShutdownAndRestart(t *testing.T) {
	for _, kind := range []string{"reindex", "settings", "shutdown", "restart"} {
		t.Run(kind, func(t *testing.T) {
			var connects atomic.Int32
			server := liveAppServer(t, func(conn *websocket.Conn) { connects.Add(1); _, _, _ = conn.ReadMessage() })
			s, open := liveAppFixture(t, server.URL)
			view, err := s.OpenLiveSession(t.Context(), open)
			if err != nil {
				t.Fatal(err)
			}
			owner := s.live.owner(s.manager.Root(), view.ID)
			switch kind {
			case "reindex":
				if _, err := s.Reindex(); err != nil {
					t.Fatal(err)
				}
			case "settings":
				if _, err := s.ControlAgentSettings(t.Context(), AgentSettingsControl{ProjectID: open.ProjectID, ProjectRevision: open.ProjectRevision, Configuration: open.Configuration, Version: open.SettingsVersion, Action: "reset", Confirm: true}); err != nil {
					t.Fatal(err)
				}
			case "shutdown":
				if err := s.ShutdownChangeWorkflows(t.Context()); err != nil {
					t.Fatal(err)
				}
			case "restart":
				// A new controller has no authority to adopt the stored active session.
				copy, err := New(s.config, s.manager)
				if err != nil {
					t.Fatal(err)
				}
				copy.runtimes, copy.agentRuntime = s.runtimes, s.agentRuntime
				history, err := copy.LiveSessions(t.Context(), AgentRunQuery{ProjectID: open.ProjectID, ProjectRevision: open.ProjectRevision})
				if err != nil || len(history.Sessions) != 1 || history.Sessions[0].State != "interrupted" || history.Sessions[0].CanReconnect {
					t.Fatalf("restart: %+v %v", history, err)
				}
				if _, err := copy.ControlLiveSession(t.Context(), view.ID, liveControl(view, "reconnect")); !errors.Is(err, agentstate.ErrLiveConflict) {
					t.Fatal(err)
				}
				owner.cancel(context.Canceled)
			}
			select {
			case <-owner.done:
			case <-time.After(5 * time.Second):
				t.Fatal("owner did not join")
			}
			if connects.Load() != 1 {
				t.Fatal("passive recovery connected")
			}
		})
	}
}

func TestLiveRejectsMissingConsentStaleBindingsAndUnsupportedMedia(t *testing.T) {
	var inputs atomic.Int32
	server := liveAppServer(t, func(conn *websocket.Conn) {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
			inputs.Add(1)
		}
	})
	s, open := liveAppFixture(t, server.URL)
	for _, kind := range []string{"confirm", "settings", "model", "origin", "revision", "disabled"} {
		request := open
		switch kind {
		case "confirm":
			request.ConfirmConnect = false
		case "settings":
			request.SettingsVersion++
		case "model":
			request.Model = "other"
		case "origin":
			request.ProviderOrigin = "https://other.example"
		case "revision":
			request.ProjectRevision = "old"
		case "disabled":
			s.agentRuntime.LiveModel = ""
		}
		if view, err := s.OpenLiveSession(t.Context(), request); err == nil || view != nil {
			t.Fatalf("accepted %s", kind)
		}
		s.agentRuntime.LiveModel = "gemini-3.8-live"
	}
	view, err := s.OpenLiveSession(t.Context(), open)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"sharing", "text", "revision", "hash", "pdf"} {
		control := liveControl(view, "send")
		control.Text = "input"
		switch kind {
		case "sharing":
			control.ConfirmSourceSharing = false
		case "text":
			control.Text = ""
		case "revision":
			control.ControlRevision++
		case "hash":
			control.Text = ""
			control.Attachment = &AttachmentReference{ID: "missing", Version: 1, Hash: "missing"}
		case "pdf":
			item, err := s.ImportAttachment(t.Context(), AttachmentImport{ProjectID: open.ProjectID, ProjectRevision: open.ProjectRevision, Name: "brief.pdf", MIME: "application/pdf", Data: []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n%%EOF"), ConfirmImport: true})
			if err != nil {
				t.Fatal(err)
			}
			control.Text = ""
			control.Attachment = &AttachmentReference{ID: item.ID, Version: item.Version, Hash: item.Hash}
		}
		if _, err := s.ControlLiveSession(t.Context(), view.ID, control); err == nil {
			t.Fatalf("accepted %s", kind)
		}
	}
	if inputs.Load() != 0 {
		t.Fatal("rejected input dispatched")
	}
}

func TestLiveRetentionFailureStopsBeforeInputAndKeepsFailureVisible(t *testing.T) {
	var inputs atomic.Int32
	server := liveAppServer(t, func(conn *websocket.Conn) {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			inputs.Add(1)
		}
	})
	s, open := liveAppFixture(t, server.URL)
	view, err := s.OpenLiveSession(t.Context(), open)
	if err != nil {
		t.Fatal(err)
	}
	owner := s.live.owner(s.manager.Root(), view.ID)
	paths, err := filepath.Glob(filepath.Join(s.manager.Root(), ".mini-orca", "agents", "*", "live-sessions.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("receipt location: %v %v", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(paths[0]); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(paths[0], 0700); err != nil {
		t.Fatal(err)
	}
	control := liveControl(view, "send")
	control.Text = "DO_NOT_DISPATCH"
	if _, err := s.ControlLiveSession(t.Context(), view.ID, control); !errors.Is(err, ErrArtifactUnavailable) {
		t.Fatalf("retention error: %v", err)
	}
	select {
	case <-owner.done:
	case <-time.After(5 * time.Second):
		t.Fatal("retention failure did not join owner")
	}
	if inputs.Load() != 0 {
		t.Fatal("input sent without durable admission")
	}
	if err := os.Remove(paths[0]); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[0], data, 0600); err != nil {
		t.Fatal(err)
	}
	view, err = s.LiveSessionDetail(t.Context(), AgentRunQuery{ProjectID: open.ProjectID, ProjectRevision: open.ProjectRevision}, view.ID)
	if err != nil || view.State != "failed" || view.Owned || !view.RetentionFailed || view.CanReconnect {
		t.Fatalf("failure disclosure: %+v %v", view, err)
	}
}

func TestLiveCanceledOpenJoinsControllerAndKeepsInterruptedReceipt(t *testing.T) {
	entered, closed := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		defer close(closed)
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		close(entered)
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	s, open := liveAppFixture(t, server.URL)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := s.OpenLiveSession(ctx, open); result <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("connection did not reach setup")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("open did not join")
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("socket outlived request")
	}
	history, err := s.LiveSessions(t.Context(), AgentRunQuery{ProjectID: open.ProjectID, ProjectRevision: open.ProjectRevision})
	if err != nil || len(history.Sessions) != 1 || history.Sessions[0].Owned || history.Sessions[0].State != "interrupted" || history.Sessions[0].Usage.ActiveCalls != 0 {
		t.Fatalf("canceled history: %+v %v", history, err)
	}
	owner := s.live.owner(s.manager.Root(), history.Sessions[0].ID)
	select {
	case <-owner.done:
	default:
		t.Fatal("controller outlived open cancellation")
	}
}

func TestLiveHistoryMarksChangedPolicyStaleAndAllowsExplicitClose(t *testing.T) {
	var inputs atomic.Int32
	server := liveAppServer(t, func(conn *websocket.Conn) {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			inputs.Add(1)
		}
	})
	s, open := liveAppFixture(t, server.URL)
	view, err := s.OpenLiveSession(t.Context(), open)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.manager.Root(), ".mini-orcaignore"), []byte("hidden.txt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	query := AgentRunQuery{ProjectID: open.ProjectID, ProjectRevision: open.ProjectRevision}
	history, err := s.LiveSessions(t.Context(), query)
	if err != nil || len(history.Sessions) != 1 || !history.Sessions[0].Stale {
		t.Fatalf("stale history: %+v %v", history, err)
	}
	view, err = s.LiveSessionDetail(t.Context(), query, view.ID)
	if err != nil || !view.Stale || view.CanReconnect {
		t.Fatalf("stale detail: %+v %v", view, err)
	}
	control := liveControl(view, "send")
	control.Text = "DO_NOT_SEND"
	if _, err := s.ControlLiveSession(t.Context(), view.ID, control); err == nil || inputs.Load() != 0 {
		t.Fatalf("changed policy dispatched: %v", err)
	}
	view, err = s.ControlLiveSession(t.Context(), view.ID, liveControl(view, "close"))
	if err != nil || view.Owned || view.State != "closed" || !view.Stale {
		t.Fatalf("stale close: %+v %v", view, err)
	}
}
