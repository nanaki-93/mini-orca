package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestDraftEndpointsAreRevisionAndHashGuarded(t *testing.T) {
	root := t.TempDir()
	writeProjectHandlerFixture(t, root, "main.go", "package main\n\nfunc Run() {}\n")
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(scopedHandlerConfig("http://127.0.0.1:1"), manager)
	if err != nil {
		t.Fatal(err)
	}
	index, err := manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	file, err := manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(app.DraftCreateRequest{ID: "draft-api", ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "func Run() { println(\"draft\") }"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewDraftHandler(service, manager)

	validate := draftHandlerRequest(http.MethodPost, "/api/projects/current/drafts/draft-api/validate", `{"project_revision":"`+index.ProjectRevision+`","expected_revision":1}`, draft.ID)
	validatedResponse := httptest.NewRecorder()
	handler.ValidateDraft(validatedResponse, validate)
	if validatedResponse.Code != http.StatusOK {
		t.Fatalf("validate = %d: %s", validatedResponse.Code, validatedResponse.Body.String())
	}
	var validated app.Draft
	if err := json.NewDecoder(validatedResponse.Body).Decode(&validated); err != nil {
		t.Fatal(err)
	}
	if validated.CompositionHash == "" || validated.State != app.DraftValid {
		t.Fatalf("validated draft = %+v", validated)
	}

	checks := draftHandlerRequest(http.MethodPost, "/api/projects/current/drafts/draft-api/checks", `{"project_revision":"`+index.ProjectRevision+`","expected_revision":1,"expected_hash":"`+validated.Hash+`"}`, draft.ID)
	checksResponse := httptest.NewRecorder()
	handler.CheckDraft(checksResponse, checks)
	if checksResponse.Code != http.StatusOK {
		t.Fatalf("checks = %d: %s", checksResponse.Code, checksResponse.Body.String())
	}
	var report app.DraftCheckReport
	if err := json.NewDecoder(checksResponse.Body).Decode(&report); err != nil {
		t.Fatal(err)
	}
	if report.DraftID != draft.ID || report.DraftHash != validated.Hash || report.CompositionHash != validated.CompositionHash {
		t.Fatalf("check report = %+v", report)
	}

	staleUpdate := draftHandlerRequest(http.MethodPatch, "/api/projects/current/drafts/draft-api", `{"project_revision":"`+index.ProjectRevision+`","expected_revision":0,"declaration":"func Run() {}"}`, draft.ID)
	staleResponse := httptest.NewRecorder()
	handler.UpdateDraft(staleResponse, staleUpdate)
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale update = %d: %s", staleResponse.Code, staleResponse.Body.String())
	}
}

func draftHandlerRequest(method, path, body, draftID string) *http.Request {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.SetPathValue("draftID", draftID)
	return request
}
