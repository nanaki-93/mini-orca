package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

func TestMemoryHandlerCurationAndStrictBoundary(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	h := NewAgentMemoryHandler(fixture.service)
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	file, err := fixture.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	request := map[string]any{"project_id": index.ProjectID, "project_revision": index.ProjectRevision, "action": "create", "version": 0, "draft": map[string]any{"kind": "decision", "title": "Retained choice", "text": "private memory body", "sources": []any{map[string]any{"path": "main.go", "hash": file.ContentHash}}}}
	mutate := func(id string, value any) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		r.SetPathValue("memoryID", id)
		w := httptest.NewRecorder()
		h.Mutate(w, r)
		return w
	}
	w := mutate("", request)
	var memory app.AgentMemory
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &memory) != nil || memory.State != "candidate" || memory.SourceStatus != "current" {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	control := map[string]any{"project_id": index.ProjectID, "project_revision": index.ProjectRevision, "action": "accept", "version": memory.Version}
	w = mutate(memory.ID, control)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &memory) != nil || memory.State != "accepted" {
		t.Fatalf("accept: %d %s", w.Code, w.Body.String())
	}
	if stale := mutate(memory.ID, control); stale.Code != 409 {
		t.Fatalf("stale acceptance: %d %s", stale.Code, stale.Body.String())
	}
	w = httptest.NewRecorder()
	h.List(w, httptest.NewRequest("GET", "/?"+query.Encode(), nil))
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "private memory body") {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest("GET", "/?"+query.Encode(), nil)
	r.SetPathValue("memoryID", memory.ID)
	w = httptest.NewRecorder()
	h.Detail(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "private memory body") {
		t.Fatalf("detail: %d %s", w.Code, w.Body.String())
	}
	control["action"], control["version"] = "forget", memory.Version
	if w := mutate(memory.ID, control); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "null" {
		t.Fatalf("forget: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.Detail(w, r)
	if w.Code != 404 {
		t.Fatalf("forgotten detail: %d %s", w.Code, w.Body.String())
	}
	for _, extra := range []string{"&unknown=1", "&project_id=other"} {
		w := httptest.NewRecorder()
		h.List(w, httptest.NewRequest("GET", "/?"+query.Encode()+extra, nil))
		if w.Code != 400 {
			t.Fatalf("invalid query: %d", w.Code)
		}
	}
	for _, body := range []string{`{"unknown":true}`, `{} {}`, `{}`} {
		w := httptest.NewRecorder()
		h.Mutate(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code < 400 {
			t.Fatalf("invalid body accepted: %s", body)
		}
	}
	respond := httptest.NewRecorder()
	respondMemory(respond, nil, app.ErrMemoryUnavailable)
	if respond.Code != http.StatusInternalServerError {
		t.Fatal("storage failure was not a server error")
	}
}
