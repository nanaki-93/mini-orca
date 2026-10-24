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
)

func TestResearchHandlerConsentReportIsolationAndPassiveReads(t *testing.T) {
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.WriteString(w, `{"modelVersion":"gemini-2.5-flash","candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"External advice."}]}}]}`)
	}))
	defer remote.Close()
	fixture := newChangeHandlerFixture(t, remote.URL)
	cfg := scopedHandlerConfig(remote.URL)
	cfg.Integrations.GoogleSearch = true
	cfg.ModelScopes.Function.Provider = config.GeminiProvider
	cfg.ModelScopes.Function.APIKey = "explicit-key"
	cfg.ModelScopes.Function.Model = "gemini-2.5-flash"
	s, err := app.New(cfg, fixture.manager)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAgentResearchHandler(s)
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	w := httptest.NewRecorder()
	h.Overview(w, httptest.NewRequest("GET", "/?"+query.Encode(), nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"available"`) || calls.Load() != 0 {
		t.Fatalf("passive overview: %d %s", w.Code, w.Body.String())
	}
	request := app.ResearchRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Question: "Research this question.", ConfirmRemoteProvider: true, ConfirmSourceSharing: true}
	send := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(request)
		w := httptest.NewRecorder()
		h.Run(w, httptest.NewRequest("POST", "/", bytes.NewReader(body)))
		return w
	}
	if w := send(); w.Code != 400 || calls.Load() != 0 {
		t.Fatalf("missing search consent: %d %s", w.Code, w.Body.String())
	}
	request.ConfirmSearch = true
	w = send()
	var report app.ResearchReport
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &report) != nil || report.State != "ungrounded" || strings.Contains(w.Body.String(), "External advice") {
		t.Fatalf("report metadata: %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest("GET", "/?"+query.Encode(), nil)
	r.SetPathValue("reportID", report.ID)
	w = httptest.NewRecorder()
	h.Report(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "External advice.") || w.Header().Get("Content-Type") != "text/html; charset=utf-8" || w.Header().Get("Content-Security-Policy") != app.ResearchReportCSP || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("isolated report: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	r.SetPathValue("reportID", strings.Repeat("a", 32))
	w = httptest.NewRecorder()
	h.Report(w, r)
	if w.Code != 404 {
		t.Fatalf("missing report: %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{`{"unknown":true}`, `{} {}`, `{}`} {
		w := httptest.NewRecorder()
		h.Run(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code < 400 {
			t.Fatalf("invalid research accepted: %s", body)
		}
	}
	for _, handler := range []http.HandlerFunc{h.Overview, h.Report} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest("GET", "/?"+query.Encode()+"&unknown=1", nil))
		if w.Code != 400 {
			t.Fatalf("invalid query accepted: %d", w.Code)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("passive read dispatched: %d", calls.Load())
	}
}
