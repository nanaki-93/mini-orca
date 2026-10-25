package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func liveHandlerProvider(t *testing.T, calls *atomic.Int32, fail *atomic.Bool) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	var sockets []*websocket.Conn
	var workers sync.WaitGroup
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			http.Error(w, "PRIVATE_PROVIDER_FAILURE", http.StatusServiceUnavailable)
			return
		}
		workers.Add(1)
		defer workers.Done()
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		mu.Lock()
		sockets = append(sockets, conn)
		mu.Unlock()
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		if err := conn.WriteJSON(map[string]any{"setupComplete": map[string]any{}}); err != nil {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
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

func liveHandlerFixture(t *testing.T, endpoint string) (*AgentLiveHandler, string) {
	t.Helper()
	fixture := newChangeHandlerFixture(t, endpoint)
	cfg := scopedHandlerConfig(endpoint)
	cfg.ModelScopes.Function.Provider, cfg.ModelScopes.Function.APIKey, cfg.ModelScopes.Function.Model = config.GeminiProvider, "explicit-key", "gemini-2.5-flash"
	cfg.AgentRuntime.LiveModel = "gemini-3.8-live"
	s, err := app.New(cfg, fixture.manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := s.ShutdownChangeWorkflows(ctx); err != nil {
			t.Error(err)
		}
	})
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	return NewAgentLiveHandler(s), "/?" + query.Encode()
}

func liveHandlerCall(t *testing.T, handler http.HandlerFunc, id string, value any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(data))
	r.SetPathValue("liveID", id)
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func TestLiveHandlerTypedControlsPassiveMetadataAndProviderFailure(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	provider := liveHandlerProvider(t, &calls, &fail)
	h, query := liveHandlerFixture(t, provider.URL)
	w := httptest.NewRecorder()
	h.Overview(w, httptest.NewRequest(http.MethodGet, query, nil))
	var overview client.LiveOverview
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &overview) != nil || overview.State != "available" || overview.Model != "gemini-3.8-live" || overview.MaxConnections != 2 || calls.Load() != 0 {
		t.Fatalf("passive overview: %d %s", w.Code, w.Body.String())
	}
	request := client.LiveOpenRequest{LiveBinding: overview.LiveBinding, ConfirmRemoteProvider: true}
	if w := liveHandlerCall(t, h.Open, "", request); w.Code != http.StatusBadRequest || calls.Load() != 0 {
		t.Fatalf("missing connection confirmation: %d %s", w.Code, w.Body.String())
	}
	request.ConfirmConnect = true
	w = liveHandlerCall(t, h.Open, "", request)
	var session client.LiveSession
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &session) != nil || !session.Owned || session.State != "connected" || session.LiveBinding != overview.LiveBinding || calls.Load() != 1 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("typed open: %d %s", w.Code, w.Body.String())
	}
	control := client.LiveControl{LiveBinding: session.LiveBinding, ControlRevision: session.ControlRevision, Action: "send", Text: "PRIVATE_LIVE_INPUT", ConfirmRemoteProvider: true}
	if w := liveHandlerCall(t, h.Control, session.ID, control); w.Code != http.StatusBadRequest {
		t.Fatalf("sharing confirmation: %d %s", w.Code, w.Body.String())
	}
	control.ConfirmSourceSharing = true
	w = liveHandlerCall(t, h.Control, session.ID, control)
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &session) != nil || session.Turns != 1 || strings.Contains(w.Body.String(), "PRIVATE_") {
		t.Fatalf("typed send: %d %s", w.Code, w.Body.String())
	}
	if w := liveHandlerCall(t, h.Control, session.ID, control); w.Code != http.StatusConflict {
		t.Fatalf("stale control: %d", w.Code)
	}
	control.Action, control.Text, control.ControlRevision = "close", "", session.ControlRevision
	w = liveHandlerCall(t, h.Control, session.ID, control)
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &session) != nil || session.Owned || session.State != "closed" || session.Usage.ActiveCalls != 0 || len(session.OutputArtifacts) != 1 {
		t.Fatalf("joined close: %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest(http.MethodGet, query, nil)
	r.SetPathValue("liveID", session.ID)
	w = httptest.NewRecorder()
	h.Detail(w, r)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "PRIVATE_") || calls.Load() != 1 {
		t.Fatalf("passive detail: %d %s", w.Code, w.Body.String())
	}
	r.SetPathValue("liveID", "missing")
	w = httptest.NewRecorder()
	h.Detail(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing detail: %d %s", w.Code, w.Body.String())
	}
	fail.Store(true)
	w = liveHandlerCall(t, h.Open, "", request)
	if w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "PRIVATE_PROVIDER_FAILURE") || calls.Load() != 2 {
		t.Fatalf("provider failure: %d %s", w.Code, w.Body.String())
	}
}

func TestLiveHandlerStrictBoundary(t *testing.T) {
	h, query := liveHandlerFixture(t, "http://127.0.0.1:1")
	for _, handler := range []http.HandlerFunc{h.Open, h.Control} {
		for _, body := range []string{`{"url":"https://example.com/private"}`, `{"path":"/etc/passwd"}`, `{} {}`, `{}`, `{"resume_handle":"private"}`} {
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
			if w.Code < 400 {
				t.Fatalf("invalid request accepted: %s", body)
			}
		}
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"model":"`+strings.Repeat("x", 1<<20)+`"}`)))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("body limit: %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.Overview(w, httptest.NewRequest(http.MethodGet, query+"&unknown=true", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown query: %d", w.Code)
	}
}
