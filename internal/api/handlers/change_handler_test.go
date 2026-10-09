package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestChangeHTTPSelectedProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"explanation":"Document the behavior.","changes":[{"path":"README.md","content":"# Behavior\nReturns the result.\n"}]}`}}}})
	}))
	defer server.Close()
	h := newChangeHandlerFixture(t, server.URL)
	index, _ := h.manager.Index()
	session, err := h.service.OpenChangeSession(context.Background(), app.ChangeCreateRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Title: "Document behavior", Kind: "fix", Paths: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	request := app.ChangeMessageRequest{ChangeIdentity: app.ChangeIdentity{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Revision: session.Revision, Hash: session.Hash}, Message: "Document behavior.", Profile: "unknown"}
	if w := workflowResponse(t, h.Message, "POST", "/changes/id/messages", session.ID, request); w.Code < 400 {
		t.Fatal("unknown model profile was accepted")
	}
	request.Profile = "analyze"
	w := workflowResponse(t, h.Message, "POST", "/changes/id/messages", session.ID, request)
	var proposal app.ChangeSession
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &proposal) != nil || proposal.ContextManifest.Scope != "analyze" {
		t.Fatalf("profile selection = %d %s", w.Code, w.Body)
	}
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
	var receipt app.ChangeMutationResult
	if err := json.Unmarshal(applied.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	old := workflowResponse(t, handler.Verify, "POST", "/changes/id/verify", session.ID, identity)
	if old.Code != 409 {
		t.Fatal("verification accepted the pre-Apply project revision")
	}
	identity.ProjectRevision = receipt.ProjectRevision
	verified := workflowResponse(t, handler.Verify, "POST", "/changes/id/verify", session.ID, identity)
	if verified.Code != 200 {
		t.Fatalf("text verification: %d %s", verified.Code, verified.Body.String())
	}
	var evidence app.ChangeVerification
	if err := json.Unmarshal(verified.Body.Bytes(), &evidence); err != nil || evidence.Status != "verified" {
		t.Fatalf("evidence: %+v, %v", evidence, err)
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

func TestInstructionPreviewIncludesScopedRecommendationsWithoutProviderOrWrites(t *testing.T) {
	handler := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	index, _ := handler.manager.Index()
	query := "?project_id=" + index.ProjectID + "&project_revision=" + index.ProjectRevision
	for _, path := range []string{"AGENTS.md", "docs/AGENTS.md"} {
		response := workflowResponse(t, handler.Instructions, "GET", "/instructions"+query+"&path="+path, "", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("preview: %d %s", response.Code, response.Body.String())
		}
		var preview InstructionPreview
		if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		foundGo := false
		foundGoTests := false
		for _, preset := range preview.Presets {
			if preset.ID == "go-tests" {
				foundGoTests = preset.Category == "Testing and validation"
			}
			if preset.ID == "go" {
				foundGo = true
				if preset.Reason == "" || len(preset.Evidence) != 1 || preset.Evidence[0] != "main.go" {
					t.Fatalf("missing recommendation evidence: %+v", preset)
				}
			}
		}
		if foundGo != (path == "AGENTS.md") || foundGoTests != foundGo || preview.ProjectRevision != index.ProjectRevision {
			t.Fatalf("incorrect scoped preview: %+v", preview)
		}
		if _, err := os.Stat(filepath.Join(handler.manager.Root(), path)); !os.IsNotExist(err) {
			t.Fatalf("preview wrote instructions: %v", err)
		}
	}
	for _, query := range []string{query + "&path=../AGENTS.md", "?path=AGENTS.md", strings.Replace(query, index.ProjectRevision, "stale", 1) + "&path=AGENTS.md"} {
		response := workflowResponse(t, handler.Instructions, "GET", "/instructions"+query, "", nil)
		if response.Code == http.StatusOK {
			t.Fatalf("unsafe or stale preview accepted: %s", query)
		}
	}
}
