package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func TestChangePlanAndFileReviewHTTPGuards(t *testing.T) {
	h := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	index, _ := h.manager.Index()
	plan := app.ChangePlanInput{Problem: "Document behavior", Approach: "Add a concise guide"}
	created := workflowResponse(t, h.Open, "POST", "/changes", "", app.ChangeCreateRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: "feature", Title: "Guide", Paths: []string{"README.md"}, AcceptanceCriteria: []string{"Explain Run"}, Plan: &plan})
	var session app.ChangeSession
	if created.Code != http.StatusOK || json.Unmarshal(created.Body.Bytes(), &session) != nil || session.Plan == nil {
		t.Fatalf("open plan: %d %s", created.Code, created.Body)
	}
	identity := app.ChangeIdentity{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Revision: session.Revision, Hash: session.Hash}
	plan.Approach = "Describe Run with an example"
	updated := workflowResponse(t, h.Plan, "POST", "/changes/id/plan", session.ID, app.ChangePlanRequest{ChangeIdentity: identity, Plan: plan, Paths: []string{"README.md"}, AcceptanceCriteria: session.AcceptanceCriteria})
	if updated.Code != http.StatusOK || json.Unmarshal(updated.Body.Bytes(), &session) != nil || session.Plan.Approach != plan.Approach || session.Plan.ApprovedHash != "" {
		t.Fatalf("update: %d %s", updated.Code, updated.Body)
	}
	stale := workflowResponse(t, h.Plan, "POST", "/changes/id/plan", session.ID, app.ChangePlanRequest{ChangeIdentity: identity, Plan: plan, Paths: []string{"README.md"}})
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale update: %d %s", stale.Code, stale.Body)
	}
	identity.Revision = session.Revision
	proposal := workflowResponse(t, h.Message, "POST", "/changes/id/messages", session.ID, app.ChangeMessageRequest{ChangeIdentity: identity, Changes: []app.ChangeEditInput{{Path: "README.md", Content: "# Run\nA bounded operation.\n", Criteria: []int{1}}}})
	if proposal.Code != http.StatusOK || json.Unmarshal(proposal.Body.Bytes(), &session) != nil {
		t.Fatalf("proposal: %d %s", proposal.Code, proposal.Body)
	}
	identity.Revision, identity.Hash = session.Revision, session.Hash
	for _, path := range []string{"main.go", "README.md"} {
		result := workflowResponse(t, h.ReviewFile, "POST", "/changes/id/review-file", session.ID, app.ChangeFileReviewRequest{ChangeIdentity: identity, Path: path})
		if path == "main.go" {
			if result.Code != http.StatusBadRequest {
				t.Fatalf("uncaptured file: %d", result.Code)
			}
			continue
		}
		if result.Code != http.StatusOK || json.Unmarshal(result.Body.Bytes(), &session) != nil || len(session.ReviewedFiles) != 1 || session.FileReviewHash != identity.Hash {
			t.Fatalf("file review: %d %s", result.Code, result.Body)
		}
	}
	for _, handler := range []http.HandlerFunc{h.Plan, h.ReviewFile} {
		request := httptest.NewRequest("POST", "/changes/id", strings.NewReader(`{"unexpected":"value"}`))
		request.SetPathValue("sessionID", session.ID)
		response := httptest.NewRecorder()
		handler(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("unknown field accepted: %d", response.Code)
		}
	}
}

func TestPlanInvestigationHTTPValidatesBoundScopeAndStrictBody(t *testing.T) {
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		content := `{"version":"v1","approach":"Add tests for Run","evidence":[],"assumptions":["Preserve the public API"],"suggested_scope":["main_test.go"],"acceptance_criteria":["Boundary cases are covered"]}`
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: content}}}})
	}))
	defer provider.Close()
	h := newChangeHandlerFixture(t, provider.URL)
	index, err := h.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	request := app.PlanInvestigationRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Paths: []string{"main.go"}, Plan: app.ChangePlanInput{Problem: "Missing coverage", Approach: "Inspect Run"}}
	for _, file := range index.Files {
		if file.Path == "main.go" {
			request.Sources = []app.PlanInvestigationSource{{Path: file.Path, Hash: file.ContentHash, Exists: true}}
		}
	}
	response := workflowResponse(t, h.InvestigatePlan, "POST", "/plans/investigate", "", request)
	var result app.PlanInvestigation
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.InputHash == "" || result.ProjectID != request.ProjectID || result.Approach == "" || calls != 1 {
		t.Fatalf("investigate: %d %s", response.Code, response.Body)
	}
	request.ProjectRevision = "stale"
	response = workflowResponse(t, h.InvestigatePlan, "POST", "/plans/investigate", "", request)
	if response.Code != http.StatusConflict || calls != 1 {
		t.Fatalf("stale plan dispatched: %d %s", response.Code, response.Body)
	}
	for _, body := range []string{`{"unexpected":true}`, `{} {}`, `{"plan":{"citations":[{"unexpected":true}]}}`} {
		response = httptest.NewRecorder()
		h.InvestigatePlan(response, httptest.NewRequest("POST", "/plans/investigate", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest || calls != 1 {
			t.Fatalf("invalid body dispatched: %d", response.Code)
		}
	}
	history, err := h.service.ChangeHistory(t.Context())
	if err != nil || len(history) != 0 {
		t.Fatalf("investigation created a change: %v %v", history, err)
	}
}
