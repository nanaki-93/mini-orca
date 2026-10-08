package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type featureResponseWriter struct {
	http.ResponseWriter
	deadlines []time.Time
}

func (w *featureResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return http.NewResponseController(w.ResponseWriter).SetWriteDeadline(deadline)
}

func TestFeatureHTTPGenerationOutlivesServerWriteTimeout(t *testing.T) {
	for _, test := range []struct {
		name     string
		output   string
		status   int
		deadline time.Duration
	}{
		{name: "success", output: `{"suggestions":[]}`, status: http.StatusOK},
		{name: "invalid response", output: `{}`, status: http.StatusBadRequest},
		{name: "workflow deadline", output: `{"suggestions":[]}`, status: http.StatusGatewayTimeout, deadline: 50 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			const writeTimeout = 20 * time.Millisecond
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Hold the model response past the HTTP server's ordinary cutoff.
				select {
				case <-time.After(5 * writeTimeout):
				case <-r.Context().Done():
					return
				}
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: test.output}}}})
			}))
			defer provider.Close()
			fixture := newChangeHandlerFixture(t, provider.URL)
			handler := NewFeatureHandler(fixture.service, fixture.manager)
			deadlines := make(chan []time.Time, 1)
			daemon := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.deadline > 0 {
					ctx, cancel := context.WithTimeout(r.Context(), test.deadline)
					defer cancel()
					r = r.WithContext(ctx)
				}
				tracked := &featureResponseWriter{ResponseWriter: w}
				handler.Generate(tracked, r)
				deadlines <- tracked.deadlines
			}))
			daemon.Config.WriteTimeout = writeTimeout
			daemon.Start()
			defer daemon.Close()
			index, err := fixture.manager.Index()
			if err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(app.FeatureGenerateRequest{FeatureRequest: app.FeatureRequest{
				ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, ExpectedHash: "empty", Goals: "Improve cancellation.",
			}})
			if err != nil {
				t.Fatal(err)
			}
			client := daemon.Client()
			client.Timeout = 2 * time.Second
			response, err := client.Post(daemon.URL+"/api/projects/current/features/generate", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("feature response was cut off before the workflow completed: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.status {
				t.Fatalf("feature status = %d, want %d", response.StatusCode, test.status)
			}
			var result map[string]any
			if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
				t.Fatalf("incomplete feature response: %v", err)
			}
			if test.status == http.StatusOK && result["status"] != "ready" {
				t.Fatalf("feature result = %+v", result)
			}
			if test.status != http.StatusOK && result["user_message"] == nil {
				t.Fatalf("structured generation failure was lost: %+v", result)
			}
			applied := <-deadlines
			if len(applied) != 2 || !applied[0].IsZero() || applied[1].IsZero() {
				t.Fatalf("generation did not suspend then restore the response write deadline: %v", applied)
			}
		})
	}
}

func TestFeatureHTTPGenerationAndTriage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"suggestions":[{"title":"Add cancellation","benefit":"Stop work early.","evidence":"Run is an entry point.","paths":["main.go"],"effort":"small","acceptance_criteria":["Canceled calls stop."]}]}`}}}})
	}))
	defer server.Close()
	fixture := newChangeHandlerFixture(t, server.URL)
	handler := NewFeatureHandler(fixture.service, fixture.manager)
	index, _ := fixture.manager.Index()
	request := app.FeatureRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, ExpectedHash: "empty", Goals: "Improve cancellation."}
	generated := workflowResponse(t, handler.Generate, "POST", "/features/generate", "", request)
	if generated.Code != 200 {
		t.Fatalf("features: %d %s", generated.Code, generated.Body.String())
	}
	var report app.FeatureReport
	if err := json.Unmarshal(generated.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	request.ExpectedHash = report.Hash
	saved := workflowResponse(t, handler.Status, "PATCH", "/features/id", report.Suggestions[0].ID, app.FeatureStatusRequest{FeatureRequest: request, Status: "saved"})
	if saved.Code != 200 {
		t.Fatal(saved.Body.String())
	}
	rejected := workflowResponse(t, handler.Status, "PATCH", "/features/id", report.Suggestions[0].ID, app.FeatureStatusRequest{FeatureRequest: request, Status: "verified"})
	if rejected.Code != 400 {
		t.Fatal("advisory idea accepted verified status")
	}
	assertStructuredError(t, rejected)
}

func TestFeatureHTTPCapacity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"suggestions":[{"title":"Add cancellation","benefit":"Stop work early.","evidence":"Run is an entry point.","paths":["main.go"],"effort":"small","acceptance_criteria":["Canceled calls stop."]}]}`}}}})
	}))
	defer server.Close()
	fixture := newChangeHandlerFixture(t, server.URL)
	handler := NewFeatureHandler(fixture.service, fixture.manager)

	index, _ := fixture.manager.Index()

	// mock huge file directly
	hugeStr := "A"
	for i := 0; i < 16; i++ {
		hugeStr += hugeStr
	}
	// hugeStr is 64KB long

	hugeReport := app.FeatureReport{
		SchemaVersion:   2,
		ProjectID:       index.ProjectID,
		ProjectRevision: index.ProjectRevision,
		Hash:            "huge_hash",
		Goals:           "Huge goals",
		Status:          "ready",
		Suggestions: []app.FeatureSuggestion{{
			ID:                 "huge_id",
			Title:              "Huge suggestion",
			Benefit:            hugeStr,
			Evidence:           "E",
			Paths:              []string{"main.go"},
			Effort:             "small",
			AcceptanceCriteria: []string{"A"},
		}},
	}
	data, _ := json.Marshal(hugeReport)

	os.MkdirAll(filepath.Join(fixture.manager.Root(), ".mini-orca", "changes"), 0755)
	os.WriteFile(filepath.Join(fixture.manager.Root(), ".mini-orca", "changes", "features.json"), data, 0644)

	sum := sha256.Sum256(data)
	realHash := "sha256:" + hex.EncodeToString(sum[:])
	request := app.FeatureGenerateRequest{FeatureRequest: app.FeatureRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, ExpectedHash: realHash, Goals: "New goals.", ConfirmRemoteProvider: true}, Profile: "analyze"}

	generated := workflowResponse(t, handler.Generate, "POST", "/features/generate", "", request)

	if generated.Code != 400 || !strings.Contains(generated.Body.String(), "bad_request") {
		t.Fatalf("expected capacity error (400, bad_request), got %d %s", generated.Code, generated.Body.String())
	}
}

func TestFeatureHTTPErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()
	fixture := newChangeHandlerFixture(t, server.URL)
	handler := NewFeatureHandler(fixture.service, fixture.manager)
	index, _ := fixture.manager.Index()

	getResp := workflowResponse(t, handler.Get, "GET", "/features", "", nil)
	var report app.FeatureReport
	json.Unmarshal(getResp.Body.Bytes(), &report)
	hash := report.Hash

	t.Run("invalid profile 400", func(t *testing.T) {
		req := app.FeatureGenerateRequest{FeatureRequest: app.FeatureRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, ExpectedHash: hash, Goals: "g"}, Profile: "invalid"}
		resp := workflowResponse(t, handler.Generate, "POST", "/features/generate", "", req)
		if resp.Code != 400 {
			t.Errorf("expected 400, got %d %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("unknown field on goals 400", func(t *testing.T) {
		body := `{"project_id":"` + index.ProjectID + `","project_revision":"` + index.ProjectRevision + `","expected_hash":"` + hash + `","goals":"g","unknown":"field"}`
		req := httptest.NewRequest("PATCH", "/features/goals", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.Goals(rr, req)
		if rr.Code != 400 {
			t.Errorf("expected 400, got %d %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("unknown field on status 400", func(t *testing.T) {
		body := `{"project_id":"` + index.ProjectID + `","project_revision":"` + index.ProjectRevision + `","expected_hash":"` + hash + `","status":"saved","unknown":"field"}`
		req := httptest.NewRequest("PATCH", "/features/id/123", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("featureID", "123")
		rr := httptest.NewRecorder()
		handler.Status(rr, req)
		if rr.Code != 400 {
			t.Errorf("expected 400, got %d %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("mismatched selection 409", func(t *testing.T) {
		req := app.FeatureGenerateRequest{FeatureRequest: app.FeatureRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, ExpectedHash: hash, Goals: "g"}, AnalysisSelectionID: "wrong_id"}
		resp := workflowResponse(t, handler.Generate, "POST", "/features/generate", "", req)
		if resp.Code != 409 {
			t.Errorf("expected 409, got %d %s", resp.Code, resp.Body.String())
		}
	})
}
