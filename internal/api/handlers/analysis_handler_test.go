package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func newAnalysisHandlerFixture(t *testing.T) (*AnalysisHandler, *ProjectHandler, *project.Analysis, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		reply := `{"findings":[]}`
		if strings.HasPrefix(request.Messages[0].Content, "You summarize") {
			reply = `{"purpose":"Explains this file.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{}}`
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: reply}}}})
	}))
	t.Cleanup(server.Close)
	root := t.TempDir()
	writeProjectHandlerFixture(t, root, "main.go", "package main\nfunc Run() {}\n")
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(scopedHandlerConfig(server.URL), manager)
	if err != nil {
		t.Fatal(err)
	}
	analysis, _ := manager.Analysis()
	return NewAnalysisHandler(service), NewProjectHandler(manager, service), analysis, &calls
}

func analysisHandlerRequest(t *testing.T, handler http.HandlerFunc, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if value, ok := body.(string); ok {
		data = []byte(value)
	} else if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, target, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func TestAnalysisHandlerRetrySelectionRequiresMatchingAdmission(t *testing.T) {
	h, _, analysis, calls := newAnalysisHandlerFixture(t)
	request := app.AnalysisPreviewRequest{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Scope: "project", RetryStaleFailed: true, Limits: app.AnalysisRunLimits{BatchFiles: 100, BudgetSeconds: 30, MaxAttemptsPerStage: 2}}
	w := analysisHandlerRequest(t, h.Preview, "POST", "/analysis/preview", request)
	if w.Code != 200 || calls.Load() != 0 {
		t.Fatalf("retry preview=%d %s", w.Code, w.Body)
	}
	var preview app.AnalysisRunPreview
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.RetryStaleFailed || len(preview.Files) != 0 || preview.ExpectedModelRequests != 0 {
		t.Fatalf("missing file included: %+v", preview)
	}
	start := app.AnalysisRunStartRequest{Identity: preview.Identity, PreviewID: preview.PreviewID, Limits: preview.Limits}
	w = analysisHandlerRequest(t, h.Start, "POST", "/analysis/run", start)
	if w.Code != 409 {
		t.Fatalf("broader start=%d %s", w.Code, w.Body)
	}
	assertStructuredError(t, w)
	request.Refresh = true
	w = analysisHandlerRequest(t, h.Preview, "POST", "/analysis/preview", request)
	if w.Code != 400 {
		t.Fatalf("refresh retry=%d %s", w.Code, w.Body)
	}
	assertStructuredError(t, w)
	start.RetryStaleFailed, start.Refresh = true, true
	w = analysisHandlerRequest(t, h.Start, "POST", "/analysis/run", start)
	if w.Code != 400 {
		t.Fatalf("refresh start=%d %s", w.Code, w.Body)
	}
	assertStructuredError(t, w)
	start.Refresh = false
	w = analysisHandlerRequest(t, h.Start, "POST", "/analysis/run", start)
	if w.Code != 200 && w.Code != 202 {
		t.Fatalf("retry start=%d %s", w.Code, w.Body)
	}
	run := waitHandlerAnalysis(t, h, analysis, app.AnalysisRunUnavailable)
	if !run.Plan.RetryStaleFailed || run.Status != app.AnalysisRunUnavailable || calls.Load() != 0 {
		t.Fatalf("empty run=%+v calls=%d", run, calls.Load())
	}
}

func TestAnalysisHandlerPreflightStartControlsAndReadOnlySections(t *testing.T) {
	h, _, analysis, calls := newAnalysisHandlerFixture(t)
	request := app.AnalysisPreviewRequest{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Scope: "project", Limits: app.AnalysisRunLimits{BatchFiles: 100, BudgetSeconds: 30, MaxAttemptsPerStage: 2}}
	w := analysisHandlerRequest(t, h.Preview, "POST", "/analysis/preview", request)
	if w.Code != 200 || calls.Load() != 0 {
		t.Fatalf("preview=%d %s calls=%d", w.Code, w.Body, calls.Load())
	}
	var preview app.AnalysisRunPreview
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	start := app.AnalysisRunStartRequest{Identity: preview.Identity, PreviewID: preview.PreviewID, Limits: preview.Limits}
	w = analysisHandlerRequest(t, h.Start, "POST", "/analysis/run", start)
	if w.Code != 400 || calls.Load() != 0 {
		t.Fatalf("missing Security intent=%d %s calls=%d", w.Code, w.Body, calls.Load())
	}
	start.Confirmations.SecurityReview = true
	start.Identity.QueueID = "old"
	w = analysisHandlerRequest(t, h.Start, "POST", "/analysis/run", start)
	if w.Code != 409 || calls.Load() != 0 {
		t.Fatalf("changed queue=%d %s", w.Code, w.Body)
	}
	start.Identity = preview.Identity
	w = analysisHandlerRequest(t, h.Start, "POST", "/analysis/run", start)
	if w.Code != 202 {
		t.Fatalf("start=%d %s", w.Code, w.Body)
	}
	run := waitHandlerAnalysis(t, h, analysis, app.AnalysisRunCompletedEmpty)
	if run.Status != app.AnalysisRunCompletedEmpty || calls.Load() != 3 {
		t.Fatalf("run=%+v calls=%d", run, calls.Load())
	}
	guarded := analysisResultsQuery(run.Identity)
	for _, category := range []string{"bugs", "performance", "security"} {
		guarded.Set("category", category)
		guarded.Set("path", "main.go")
		w = analysisHandlerRequest(t, h.Results, "GET", "/analysis/results?"+guarded.Encode(), nil)
		if w.Code != 200 || calls.Load() != 3 {
			t.Fatalf("results=%d %s calls=%d", w.Code, w.Body, calls.Load())
		}
		var result app.AnalysisSectionResults
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		if result.Progress.FindingCount == nil || *result.Progress.FindingCount != 0 || result.Identity != run.Identity {
			t.Fatalf("result=%+v", result)
		}
	}
	for _, test := range []struct {
		name, value string
		code        int
	}{{"generation", "previous", 409}, {"provider_fingerprint", "other", 409}, {"queue_id", "other", 409}, {"project_revision", "other", 409}, {"path", "missing.go", 409}, {"path", "../main.go", 400}, {"category", "unknown", 400}, {"unexpected", "value", 400}} {
		t.Run(test.name+test.value, func(t *testing.T) {
			q := analysisResultsQuery(run.Identity)
			q.Set("category", "bugs")
			q.Set(test.name, test.value)
			w := analysisHandlerRequest(t, h.Results, "GET", "/analysis/results?"+q.Encode(), nil)
			if w.Code != test.code {
				t.Fatalf("query=%d %s", w.Code, w.Body)
			}
			assertStructuredError(t, w)
		})
	}
	identity := run.Identity
	identity.Generation = "previous"
	w = analysisHandlerRequest(t, h.Control, "POST", "/analysis/run/control", app.AnalysisRunControlRequest{Identity: identity, Action: app.AnalysisRunCancel})
	if w.Code != 409 || calls.Load() != 3 {
		t.Fatalf("late control=%d %s", w.Code, w.Body)
	}
	source, err := os.ReadFile(filepath.Join(analysis.Path, "main.go"))
	if err != nil || string(source) != "package main\nfunc Run() {}\n" {
		t.Fatalf("source changed: %s %v", source, err)
	}
}

func analysisResultsQuery(identity app.AnalysisRunIdentity) url.Values {
	return url.Values{"project_id": {identity.ProjectID}, "project_revision": {identity.ProjectRevision}, "policy_fingerprint": {identity.PolicyFingerprint}, "provider_fingerprint": {identity.ProviderFingerprint}, "queue_id": {identity.QueueID}, "id": {identity.ID}, "generation": {identity.Generation}}
}

func TestAnalysisHandlerRejectsMalformedBodiesAndAmbiguousGuards(t *testing.T) {
	h, _, analysis, calls := newAnalysisHandlerFixture(t)
	for _, body := range []string{`{}`, `{"scope":"file"}`, `{"unknown":"field"}`, `{} {}`, `{"limits":{"batch_files":501}}`, `{"scope":"project","path":"main.go"}`} {
		w := analysisHandlerRequest(t, h.Preview, "POST", "/analysis/preview", body)
		if w.Code != 400 {
			t.Fatalf("body %s => %d", body, w.Code)
		}
		assertStructuredError(t, w)
	}
	for _, query := range []string{"", "project_id=x", "project_id=x&project_id=y&project_revision=z", "project_id=x&project_revision=z&path=main.go", "project_id=%ZZ&project_revision=z"} {
		w := analysisHandlerRequest(t, h.Current, "GET", "/analysis/run?"+query, nil)
		if w.Code != 400 {
			t.Fatalf("query %s => %d", query, w.Code)
		}
	}
	q := url.Values{"project_id": {analysis.ProjectID}, "project_revision": {analysis.ProjectRevision}}
	if w := analysisHandlerRequest(t, h.Current, "GET", "/analysis/run?"+q.Encode(), nil); w.Code != 204 {
		t.Fatalf("absent=%d %s", w.Code, w.Body)
	}
	q.Set("project_id", "replacement")
	if w := analysisHandlerRequest(t, h.Current, "GET", "/analysis/run?"+q.Encode(), nil); w.Code != 409 {
		t.Fatalf("wrong project=%d %s", w.Code, w.Body)
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request dispatched")
	}
}

func waitHandlerAnalysis(t *testing.T, h *AnalysisHandler, analysis *project.Analysis, status app.AnalysisRunStatus) app.AnalysisRun {
	t.Helper()
	query := url.Values{"project_id": {analysis.ProjectID}, "project_revision": {analysis.ProjectRevision}}
	var run app.AnalysisRun
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		w := analysisHandlerRequest(t, h.Current, "GET", "/analysis/run?"+query.Encode(), nil)
		if w.Code != 200 {
			t.Fatalf("current=%d %s", w.Code, w.Body)
		}
		if err := json.Unmarshal(w.Body.Bytes(), &run); err != nil {
			t.Fatal(err)
		}
		if run.Status == status {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return run
}
