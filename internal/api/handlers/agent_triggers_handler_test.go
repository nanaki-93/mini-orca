package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func triggerHandlerFixture(t *testing.T, endpoint string) (*AgentTriggersHandler, string) {
	t.Helper()
	f := newChangeHandlerFixture(t, endpoint)
	s, err := app.New(scopedHandlerConfig(endpoint), f.manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.ShutdownChangeWorkflows(ctx); err != nil {
			t.Error(err)
		}
	})
	index, err := f.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	return NewAgentTriggersHandler(s), "/?" + query.Encode()
}

func callTriggerControl(t *testing.T, h *AgentTriggersHandler, request client.TriggerControl) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.Control(w, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(data)))
	return w
}

func TestAgentTriggersHandlerTypedSeparateControlsAndPassiveMetadata(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	t.Cleanup(provider.Close)
	h, query := triggerHandlerFixture(t, provider.URL)
	w := httptest.NewRecorder()
	h.Get(w, httptest.NewRequest(http.MethodGet, query, nil))
	var v client.AgentTriggers
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &v) != nil || v.State != "disabled" || v.Owned || calls.Load() != 0 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("passive read: %d %s", w.Code, w.Body.String())
	}
	request := client.TriggerControl{TriggerBinding: v.TriggerBinding, ControlRevision: v.Record.ControlRevision, Action: "watch", Options: &client.TriggerOptions{Mode: "changes", Paths: []string{"main.go"}}}
	if w = callTriggerControl(t, h, request); w.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed watch: %d %s", w.Code, w.Body.String())
	}
	request.Confirm = true
	w = callTriggerControl(t, h, request)
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &v) != nil || v.State != "watch_only" || !v.Owned || v.Record.Grant != nil {
		t.Fatalf("watch: %d %s", w.Code, w.Body.String())
	}
	if w = callTriggerControl(t, h, request); w.Code != http.StatusConflict {
		t.Fatalf("stale control: %d", w.Code)
	}
	request = client.TriggerControl{TriggerBinding: v.TriggerBinding, ControlRevision: v.Record.ControlRevision, Action: "grant", Confirm: true, ScopeHash: v.ScopeHash, GrantSeconds: 60, ConfirmRemoteProvider: true}
	if w = callTriggerControl(t, h, request); w.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed source sharing: %d", w.Code)
	}
	request.ConfirmSourceSharing = true
	w = callTriggerControl(t, h, request)
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &v) != nil || v.State != "granted" || v.Record.Grant == nil || calls.Load() != 0 {
		t.Fatalf("grant: %d %s", w.Code, w.Body.String())
	}
	for _, action := range []string{"revoke", "disable"} {
		request = client.TriggerControl{TriggerBinding: v.TriggerBinding, ControlRevision: v.Record.ControlRevision, Action: action, Confirm: true}
		request.ProjectRevision = "stale-revision"
		w = callTriggerControl(t, h, request)
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &v) != nil || v.Record.Grant != nil {
			t.Fatalf("%s recovery: %d %s", action, w.Code, w.Body.String())
		}
	}
	if v.State != "disabled" || v.Owned || calls.Load() != 0 {
		t.Fatal("control unexpectedly called provider or retained owner")
	}
}

func TestAgentTriggersHandlerStrictBoundary(t *testing.T) {
	h, query := triggerHandlerFixture(t, "http://127.0.0.1:1")
	for _, body := range []string{`{"url":"https://example.com"}`, `{} {}`, `{}`, `{"action":"apply","confirm":true}`} {
		w := httptest.NewRecorder()
		h.Control(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid body: %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.Control(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"model":"`+strings.Repeat("x", 1<<20)+`"}`)))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("body limit: %d", w.Code)
	}
	for _, suffix := range []string{"&unknown=true", "&worktree_path=/tmp/task", "&worktree_path="} {
		w = httptest.NewRecorder()
		h.Get(w, httptest.NewRequest(http.MethodGet, query+suffix, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid scope: %d", w.Code)
		}
	}
}
