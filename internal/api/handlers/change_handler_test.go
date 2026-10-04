package handlers

import (
	"bytes"
	"encoding/json"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newChangeHandlerFixture(t *testing.T, baseURL string) *ChangeHandler {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(scopedHandlerConfig(baseURL), manager)
	if err != nil {
		t.Fatal(err)
	}
	return NewChangeHandler(service, manager)
}

func workflowResponse(t *testing.T, handler http.HandlerFunc, method, path, id string, value any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.SetPathValue("sessionID", id)
	request.SetPathValue("featureID", id)
	response := httptest.NewRecorder()
	handler(response, request)
	return response
}

func TestChangeHTTPInstructionProposalRequiresChecksReviewAndConfirmation(t *testing.T) {
	handler := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	index, _ := handler.manager.Index()
	request := app.InstructionProposalRequest{ChangeCreateRequest: app.ChangeCreateRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: "instructions", Title: "Instructions", Paths: []string{"AGENTS.md"}}, Content: "Use deterministic tests.\n"}
	response := workflowResponse(t, handler.ProposeInstructions, "POST", "/instructions/proposal", "", request)
	if response.Code != 200 {
		t.Fatalf("proposal: %d %s", response.Code, response.Body.String())
	}
	var session app.ChangeSession
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	identity := app.ChangeIdentity{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Revision: session.Revision, Hash: session.Hash}
	rejected := workflowResponse(t, handler.Apply, "POST", "/changes/id/apply", session.ID, app.ChangeApplyRequest{ChangeIdentity: identity, Confirm: true})
	if rejected.Code == 200 {
		t.Fatal("Apply bypassed review")
	}
	if _, err := os.Stat(filepath.Join(handler.manager.Root(), "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("proposal wrote source")
	}
	for _, step := range []http.HandlerFunc{handler.Checks, handler.Review} {
		response := workflowResponse(t, step, "POST", "/changes/id/action", session.ID, identity)
		if response.Code != 200 {
			t.Fatalf("preparation: %d %s", response.Code, response.Body.String())
		}
	}
	applied := workflowResponse(t, handler.Apply, "POST", "/changes/id/apply", session.ID, app.ChangeApplyRequest{ChangeIdentity: identity, Confirm: true})
	if applied.Code != 200 {
		t.Fatalf("Apply: %d %s", applied.Code, applied.Body.String())
	}
	source, _ := os.ReadFile(filepath.Join(handler.manager.Root(), "AGENTS.md"))
	if string(source) != request.Content {
		t.Fatal("approved instructions were not applied")
	}
}

func TestChangeHTTPReadGuardsAndStrictRequests(t *testing.T) {
	handler := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	index, _ := handler.manager.Index()
	query := "?project_id=" + index.ProjectID + "&project_revision=" + index.ProjectRevision
	for _, step := range []http.HandlerFunc{handler.List, handler.Recovery} {
		denied := workflowResponse(t, step, "GET", "/changes", "", nil)
		if denied.Code != 409 {
			t.Fatal("unguarded history read accepted")
		}
		response := workflowResponse(t, step, "GET", "/changes"+query, "", nil)
		if response.Code != 200 {
			t.Fatalf("passive read: %d %s", response.Code, response.Body.String())
		}
	}
	preview := workflowResponse(t, handler.Instructions, "GET", "/instructions"+query+"&path=AGENTS.md", "", nil)
	if preview.Code != 200 {
		t.Fatal(preview.Body.String())
	}
	bad := httptest.NewRecorder()
	handler.Open(bad, httptest.NewRequest("POST", "/changes", bytes.NewBufferString(`{"paths":["main.go"],"unexpected":true}`)))
	if bad.Code != 400 {
		t.Fatal("unknown field accepted")
	}
	assertStructuredError(t, bad)
	if response := workflowResponse(t, handler.ProposeInstructions, "POST", "/instructions/proposal", "", app.InstructionProposalRequest{ChangeCreateRequest: app.ChangeCreateRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: "instructions", Title: "Escape", Paths: []string{"../AGENTS.md"}}, Content: "Escape."}); response.Code != 400 {
		t.Fatal("unsafe instruction path accepted")
	}
}
