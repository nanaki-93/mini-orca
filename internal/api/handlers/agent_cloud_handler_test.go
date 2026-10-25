package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func cloudHandlerFixture(t *testing.T) (*AgentCloudHandler, client.CloudSelection, string) {
	t.Helper()
	f := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	cfg := scopedHandlerConfig("http://127.0.0.1:1")
	cfg.Integrations.Cloud = []config.CloudTarget{{ID: "archive", Service: "gcs_artifact", Project: "123456789012", Region: "us-central1", Resource: "archive-bucket", Principal: "archive-agent@example-project.iam.gserviceaccount.com", CredentialFileEnv: "CLOUD_FIXTURE_KEY", RetentionDays: 30}}
	t.Setenv("CLOUD_FIXTURE_KEY", filepath.Join(t.TempDir(), "missing-key"))
	s, err := app.New(cfg, f.manager)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := f.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	store, err := agentstate.Open(f.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveArtifact(t.Context(), agentstate.ArtifactWrite{Key: agentstate.ArtifactKey{Session: agentstate.Key{App: "mini-orca", User: "local", ID: "fixture"}, Name: "result.txt"}, Kind: "text", MIME: "text/plain", Data: []byte("PRIVATE_SOURCE"), Provenance: map[string]string{"project": idx.ProjectID, "revision": idx.ProjectRevision}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := s.ListArtifacts(t.Context(), app.AgentRunQuery{ProjectID: idx.ProjectID, ProjectRevision: idx.ProjectRevision})
	if err != nil {
		t.Fatal(err)
	}
	selection := client.CloudSelection{ProjectIdentity: client.ProjectIdentity{ProjectID: idx.ProjectID, ProjectRevision: idx.ProjectRevision}, TargetID: "archive", Kind: "artifact", SourceID: items.Artifacts[0].ID}
	return NewAgentCloudHandler(s), selection, "/?" + url.Values{"project_id": {idx.ProjectID}, "project_revision": {idx.ProjectRevision}}.Encode()
}
func cloudHandlerPost(t *testing.T, handler http.HandlerFunc, value any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(data)))
	return w
}

func TestCloudHandlerTypedPreviewAndUnavailableReceipt(t *testing.T) {
	h, selection, query := cloudHandlerFixture(t)
	w := httptest.NewRecorder()
	h.List(w, httptest.NewRequest(http.MethodGet, query, nil))
	var catalog client.CloudCatalog
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &catalog) != nil || len(catalog.Destinations) != 1 || len(catalog.Copies) != 0 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	w = cloudHandlerPost(t, h.Preview, selection)
	var preview client.CloudPreview
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &preview) != nil || preview.SourceID != selection.SourceID || strings.Contains(w.Body.String(), "PRIVATE_SOURCE") {
		t.Fatal(w.Code, w.Body.String())
	}
	request := client.CloudControl{ProjectIdentity: selection.ProjectIdentity, Action: "copy", ID: preview.ID, Preview: &preview, Confirm: true}
	if w = cloudHandlerPost(t, h.Control, request); w.Code != http.StatusBadRequest {
		t.Fatal("sharing consent bypassed", w.Code, w.Body.String())
	}
	request.ConfirmSourceSharing = true
	w = cloudHandlerPost(t, h.Control, request)
	var receipt client.CloudCopy
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &receipt) != nil || receipt.State != "unknown" || receipt.Hash != preview.Hash || receipt.SourceRevision != preview.SourceRevision {
		t.Fatal("missing credentials did not retain uncertainty", w.Code, w.Body.String())
	}
	if w = cloudHandlerPost(t, h.Control, request); w.Code != http.StatusConflict {
		t.Fatal("copy replay not rejected", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.List(w, httptest.NewRequest(http.MethodGet, query, nil))
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &catalog) != nil || len(catalog.Copies) != 1 {
		t.Fatal("receipt missing", w.Body.String())
	}
}

func TestCloudHandlerRejectsUnknownFieldsForeignScopeAndWorktrees(t *testing.T) {
	h, selection, query := cloudHandlerFixture(t)
	for _, handler := range []http.HandlerFunc{h.Preview, h.Control} {
		for _, body := range []string{`{"endpoint":"https://attacker.invalid"}`, `{} {}`, `{}`} {
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
			if w.Code < 400 {
				t.Fatal("invalid body accepted", body)
			}
		}
	}
	selection.ProjectRevision = "stale"
	if w := cloudHandlerPost(t, h.Preview, selection); w.Code != http.StatusConflict {
		t.Fatal("stale scope accepted", w.Code, w.Body.String())
	}
	for _, handler := range []http.HandlerFunc{h.List, h.Preview, h.Control} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodPost, query+"&worktree_path=", strings.NewReader(`{}`)))
		if w.Code != http.StatusBadRequest {
			t.Fatal("worktree scope accepted", w.Code)
		}
	}
}
