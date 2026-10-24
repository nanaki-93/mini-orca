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
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

func TestArtifactHandlerDisclosureExactControlsAndStrictBoundary(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	h := NewAgentLibraryHandler(fixture.service)
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	store, err := agentstate.Open(fixture.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveArtifact(t.Context(), agentstate.ArtifactWrite{Key: agentstate.ArtifactKey{Session: agentstate.Key{App: "mini-orca", User: "local", ID: "fixture"}, Name: "report.txt"}, Kind: "text", MIME: "text/plain", Data: []byte("PRIVATE_ARTIFACT_BODY"), Provenance: map[string]string{"project": index.ProjectID, "revision": index.ProjectRevision}})
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	w := httptest.NewRecorder()
	h.List(w, httptest.NewRequest("GET", "/?"+query.Encode(), nil))
	var list app.ArtifactList
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "PRIVATE_ARTIFACT_BODY") || json.Unmarshal(w.Body.Bytes(), &list) != nil || len(list.Artifacts) != 1 {
		t.Fatalf("metadata: %d %s", w.Code, w.Body.String())
	}
	record := list.Artifacts[0]
	r := httptest.NewRequest("GET", "/?"+query.Encode(), nil)
	r.SetPathValue("artifactID", record.ID)
	w = httptest.NewRecorder()
	h.Detail(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "PRIVATE_ARTIFACT_BODY") {
		t.Fatalf("detail: %d %s", w.Code, w.Body.String())
	}
	control := app.ArtifactControl{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Action: "export", Version: record.Version, Hash: record.Hash}
	mutate := func(request app.ArtifactControl) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/", bytes.NewReader(data))
		r.SetPathValue("artifactID", record.ID)
		w := httptest.NewRecorder()
		h.Control(w, r)
		return w
	}
	w = mutate(control)
	var receipt app.ArtifactReceipt
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &receipt) != nil || receipt.State != "exported" || receipt.Path == "" || receipt.ID != record.ID {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}
	control.Action, control.Hash = "delete", strings.Repeat("0", 64)
	if w := mutate(control); w.Code != 409 {
		t.Fatalf("stale selection: %d %s", w.Code, w.Body.String())
	}
	control.Hash = record.Hash
	if w := mutate(control); w.Code != 200 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.Detail(w, r)
	if w.Code != 404 {
		t.Fatalf("deleted detail: %d %s", w.Code, w.Body.String())
	}
	for _, extra := range []string{"&unknown=1", "&project_id=other", "&limit=101", "&limit=bad", "&cursor=bad"} {
		w := httptest.NewRecorder()
		h.List(w, httptest.NewRequest("GET", "/?"+query.Encode()+extra, nil))
		if w.Code != 400 {
			t.Fatalf("invalid query %s: %d %s", extra, w.Code, w.Body.String())
		}
	}
	for _, body := range []string{`{"path":"main.go"}`, `{} {}`, `{}`, `{"action":"overwrite"}`} {
		w := httptest.NewRecorder()
		h.Control(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("invalid control %s: %d", body, w.Code)
		}
	}
}

func TestArtifactHandlerPreservesProtectionAndStorageErrors(t *testing.T) {
	for _, test := range []struct {
		err  error
		code int
	}{{app.ErrArtifactProtected, http.StatusConflict}, {app.ErrArtifactConflict, http.StatusConflict}, {app.ErrArtifactNotFound, http.StatusNotFound}, {app.ErrArtifactUnavailable, http.StatusInternalServerError}} {
		w := httptest.NewRecorder()
		respondArtifact(w, nil, test.err)
		if w.Code != test.code || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("error %v: %d %s", test.err, w.Code, w.Body.String())
		}
	}
}
