package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func TestChangeWorkflowHTTPAdmissionProgressAndCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{}`}}}})
	}))
	defer provider.Close()
	defer close(release)
	h := newChangeHandlerFixture(t, provider.URL)
	defer h.service.CancelChangeWorkflows()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.service.ShutdownChangeWorkflows(ctx); err != nil {
			t.Error(err)
		}
	})
	index, _ := h.manager.Index()
	session, err := h.service.OpenChangeSession(context.Background(), app.ChangeCreateRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: "security", Title: "Fix unsafe input", Paths: []string{"main.go", "main_test.go"}})
	if err != nil {
		t.Fatal(err)
	}
	request := app.ChangeWorkflowRequest{ChangeIdentity: app.ChangeIdentity{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Revision: session.Revision, Hash: session.Hash}, Message: "Fix and test.", Models: app.ChangeWorkflowModels{Create: "function", Test: "bug", Review: "analyze"}}
	for _, handler := range []http.HandlerFunc{h.StartWorkflow, h.CancelWorkflow} {
		for _, body := range []string{`{"unknown":true}`, `{}`, `{} {}`} {
			r := httptest.NewRequest("POST", "/changes/id/workflow", strings.NewReader(body))
			r.SetPathValue("sessionID", session.ID)
			w := httptest.NewRecorder()
			handler(w, r)
			if w.Code < 400 {
				t.Fatalf("accepted invalid body: %s", body)
			}
		}
	}
	denied := workflowResponse(t, h.StartWorkflow, "POST", "/changes/id/workflow", session.ID, request)
	if denied.Code != 400 {
		t.Fatalf("missing intent/trust: %d", denied.Code)
	}
	request.ConfirmSecurity = true
	if _, err := h.service.TrustProjectExecution(index.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	started := workflowResponse(t, h.StartWorkflow, "POST", "/changes/id/workflow", session.ID, request)
	if started.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", started.Code, started.Body.String())
	}
	var active app.ChangeSession
	if err := json.Unmarshal(started.Body.Bytes(), &active); err != nil {
		t.Fatal(err)
	}
	<-entered
	query := "/changes/id?project_id=" + index.ProjectID + "&project_revision=" + index.ProjectRevision
	if progress := workflowResponse(t, h.Get, "GET", query, session.ID, nil); progress.Code != 200 || !strings.Contains(progress.Body.String(), `"status":"running"`) {
		t.Fatalf("progress: %d %s", progress.Code, progress.Body.String())
	}
	control := app.ChangeWorkflowControl{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, WorkflowID: "old-run"}
	if rejected := workflowResponse(t, h.CancelWorkflow, "POST", "/changes/id/workflow/cancel", session.ID, control); rejected.Code != 409 {
		t.Fatalf("stale control: %d", rejected.Code)
	}
	control.WorkflowID = active.Workflow.ID
	if canceled := workflowResponse(t, h.CancelWorkflow, "POST", "/changes/id/workflow/cancel", session.ID, control); canceled.Code != 200 {
		t.Fatalf("cancel: %d %s", canceled.Code, canceled.Body.String())
	}
}
