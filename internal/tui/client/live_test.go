package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLiveClientActualControlsAndPassiveHistory(t *testing.T) {
	cl, scope, calls := mediaClientFixture(t)
	view, err := cl.ReadLiveSessions(t.Context(), "overview", scope)
	if err != nil || view.State != "available" || calls.Load() != 0 {
		t.Fatalf("overview: %+v %v", view, err)
	}
	open := LiveOpenRequest{LiveBinding: view.LiveBinding, ConfirmConnect: true, ConfirmRemoteProvider: true}
	session, err := cl.OpenLiveSession(t.Context(), "open", scope, open)
	if err != nil || !session.Owned || calls.Load() != 1 {
		t.Fatalf("open: %+v %v", session, err)
	}
	control := LiveControl{LiveBinding: session.LiveBinding, ControlRevision: session.ControlRevision, Action: "send", Text: "PRIVATE_LIVE_INPUT", ConfirmSourceSharing: true, ConfirmRemoteProvider: true}
	session, err = cl.ControlLiveSession(t.Context(), "send", scope, session.ID, control)
	if err != nil || session.Turns != 1 {
		t.Fatalf("send: %+v %v", session, err)
	}
	if _, err := cl.ControlLiveSession(t.Context(), "old", scope, session.ID, control); err == nil {
		t.Fatal("stale revision accepted")
	}
	control.Action, control.Text, control.ControlRevision = "close", "", session.ControlRevision
	session, err = cl.ControlLiveSession(t.Context(), "close", scope, session.ID, control)
	if err != nil || session.Owned || session.State != "closed" || session.Usage.UnknownUsageCalls != 1 {
		t.Fatalf("close: %+v %v", session, err)
	}
	detail, err := cl.LiveSessionDetail(t.Context(), "detail", scope, session.ID)
	if err != nil || len(detail.OutputArtifacts) != 1 || calls.Load() != 1 {
		t.Fatalf("detail: %+v %v", detail, err)
	}
	text, err := cl.ArtifactDetail(t.Context(), "transcript", scope, detail.OutputArtifacts[0].ID)
	if err != nil || !strings.Contains(text.Text, "PRIVATE_LIVE_INPUT") || calls.Load() != 1 {
		t.Fatalf("private transcript: %+v %v", text, err)
	}
}

func liveClientSessionFixture() (AgentSettingsScope, LiveSession) {
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}, WorktreePath: "/tmp/work tree"}
	now := time.Now().UTC()
	return scope, LiveSession{LiveBinding: LiveBinding{ProjectIdentity: scope.ProjectIdentity, Configuration: strings.Repeat("a", 64), Model: "gemini-3.8-live", ProviderOrigin: "https://provider.example"}, ID: strings.Repeat("a", 26), Version: 2, ControlRevision: 1, Policy: strings.Repeat("b", 64), State: "connected", Owned: true, Connections: 1, CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(time.Minute)}
}

func TestLiveClientRejectsCorruptAndForeignReceipts(t *testing.T) {
	for _, kind := range []string{"project", "session", "revision", "recovery", "owner", "usage", "lifetime", "binding"} {
		t.Run(kind, func(t *testing.T) {
			scope, item := liveClientSessionFixture()
			id := item.ID
			switch kind {
			case "project":
				item.ProjectID = "other"
			case "session":
				item.ID = strings.Repeat("c", 26)
			case "revision":
				item.ControlRevision = 3
			case "recovery":
				item.CanReconnect = true
			case "owner":
				item.Owned = false
			case "usage":
				item.Usage.InputTokens = -1
			case "lifetime":
				item.ExpiresAt = item.CreatedAt.Add(time.Hour)
			case "binding":
				item.Configuration = "missing"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("worktree_path") != scope.WorktreePath {
					t.Error("lost worktree selector")
				}
				_ = json.NewEncoder(w).Encode(item)
			}))
			defer server.Close()
			cl, _ := NewClient(server.URL)
			defer cl.Close()
			if _, err := cl.LiveSessionDetail(t.Context(), "bad", scope, id); err == nil {
				t.Fatalf("accepted %s", kind)
			}
		})
	}
}

func TestLiveClientCancellationClosesPassiveRead(t *testing.T) {
	scope, item := liveClientSessionFixture()
	entered, stopped := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(stopped) }))
	defer server.Close()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := cl.LiveSessionDetail(ctx, "cancel", scope, item.ID); done <- err }()
	<-entered
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled read succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("client did not stop")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("HTTP request outlived cancellation")
	}
}
