package handlers

import (
	"encoding/json"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
