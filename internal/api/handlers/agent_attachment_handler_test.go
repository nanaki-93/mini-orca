package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func TestAttachmentHandlerTypedImportAdviceAndPassiveLibraryDisclosure(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		if fail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, "PRIVATE_PROVIDER_FAILURE")
			return
		}
		io.WriteString(w, `{"modelVersion":"gemini-2.5-flash","candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"PRIVATE_MEDIA_ADVICE"}]}}]}`)
	}))
	defer provider.Close()
	fixture := newChangeHandlerFixture(t, provider.URL)
	cfg := scopedHandlerConfig(provider.URL)
	cfg.ModelScopes.Code.Provider, cfg.ModelScopes.Code.APIKey, cfg.ModelScopes.Code.Model = config.GeminiProvider, "explicit-key", "gemini-2.5-flash"
	s, err := app.New(cfg, fixture.manager)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAgentAttachmentHandler(s)
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	identity := client.ProjectIdentity{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}
	query := url.Values{"project_id": {identity.ProjectID}, "project_revision": {identity.ProjectRevision}}
	call := func(handler http.HandlerFunc, value any) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(data)))
		return w
	}
	upload := client.AttachmentImport{ProjectIdentity: identity, Name: "sample.pdf", MIME: "application/pdf", Data: []byte("%PDF-1.7\n1 0 obj\nPRIVATE_FILE\nendobj\n%%EOF\n"), ConfirmImport: true}
	w := call(h.Import, upload)
	var item client.AgentArtifact
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &item) != nil || item.ProjectIdentity != identity || item.Hash == "" || strings.Contains(w.Body.String(), "PRIVATE_FILE") || calls.Load() != 0 {
		t.Fatalf("typed import: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.Overview(w, httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil))
	var overview client.AttachmentOverview
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &overview) != nil || overview.State != "available" || len(overview.Attachments) != 1 || overview.ProjectIdentity != identity || w.Header().Get("Cache-Control") != "no-store" || calls.Load() != 0 {
		t.Fatalf("typed overview: %d %s", w.Code, w.Body.String())
	}
	request := client.AttachmentAdviceRequest{ProjectIdentity: identity, Configuration: overview.Configuration, SettingsVersion: overview.SettingsVersion, Model: overview.Model.Model, ProviderOrigin: overview.Model.ProviderOrigin, Question: "Explain the selected document.", Attachments: []client.AttachmentReference{{ID: item.ID, Version: item.Version, Hash: item.Hash}}, ConfirmRemoteProvider: true}
	if w := call(h.Advice, request); w.Code != http.StatusBadRequest || calls.Load() != 0 {
		t.Fatalf("sharing required: %d %s", w.Code, w.Body.String())
	}
	request.ConfirmSourceSharing = true
	w = call(h.Advice, request)
	var receipt client.AttachmentAdviceReceipt
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &receipt) != nil || receipt.State != "advisory" || receipt.Artifact.ProjectIdentity != identity || receipt.UsageKnown || strings.Contains(w.Body.String(), "PRIVATE_MEDIA_ADVICE") || calls.Load() != 1 {
		t.Fatalf("typed receipt: %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil)
	r.SetPathValue("artifactID", receipt.Artifact.ID)
	w = httptest.NewRecorder()
	NewAgentLibraryHandler(s).Detail(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "PRIVATE_MEDIA_ADVICE") || calls.Load() != 1 {
		t.Fatalf("explicit private answer: %d %s", w.Code, w.Body.String())
	}
	request.Attachments[0].Hash = strings.Repeat("a", 64)
	if w := call(h.Advice, request); w.Code != http.StatusConflict || calls.Load() != 1 {
		t.Fatalf("stale selection: %d %s", w.Code, w.Body.String())
	}
	request.Attachments[0].Hash = item.Hash
	fail.Store(true)
	if w := call(h.Advice, request); w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "PRIVATE_PROVIDER_FAILURE") || calls.Load() != 2 {
		t.Fatalf("provider failure: %d %s", w.Code, w.Body.String())
	}
}

func TestAttachmentHandlerStrictBoundaryAndBodyLimit(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	h := NewAgentAttachmentHandler(fixture.service)
	for _, handler := range []http.HandlerFunc{h.Import, h.Advice} {
		for _, body := range []string{`{"url":"https://private.example/media"}`, `{"path":"/etc/passwd"}`, `{} {}`, `{"data":"not base64"}`, `{}`} {
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
			if w.Code < 400 {
				t.Fatalf("invalid body accepted: %s", body)
			}
		}
	}
	w := httptest.NewRecorder()
	h.Import(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"`+strings.Repeat("x", 1<<20)+`"}`)))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("body limit changed: %d", w.Code)
	}
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	w = httptest.NewRecorder()
	h.Overview(w, httptest.NewRequest(http.MethodGet, "/?"+query.Encode()+"&unknown=true", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown query accepted: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.Overview(w, httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"state":"unsupported_profile"`) {
		t.Fatalf("unsupported profile hidden: %d %s", w.Code, w.Body.String())
	}
}
