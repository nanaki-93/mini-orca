package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestChatSessionEndpointsKeepMessagesBoundToOneFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"version":"v1","declaration":"func Run() { println(\"draft\") }","explanation":"Updated Run."}`}}}})
	}))
	defer server.Close()
	handler, identity := newChatSessionTestHandler(t, server.URL, 0)

	openBody := marshalChatBody(t, ChatSessionRequest{ProjectID: identity.projectID, ProjectRevision: identity.revision, BaseFileHash: identity.hash, OpenPath: "sample.go", Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: "Run"})
	opened := httptest.NewRecorder()
	handler.OpenSession(opened, httptest.NewRequest(http.MethodPost, "/api/projects/current/chat/sessions", bytes.NewReader(openBody)))
	if opened.Code != http.StatusCreated {
		t.Fatalf("open status = %d: %s", opened.Code, opened.Body.String())
	}
	var session app.ChatSession
	if err := json.NewDecoder(opened.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}

	message := httptest.NewRequest(http.MethodPost, "/api/projects/current/chat/sessions/"+session.ID+"/messages", bytes.NewReader([]byte(`{"message":"Improve Run."}`)))
	message.SetPathValue("sessionID", session.ID)
	generated := httptest.NewRecorder()
	handler.SendSessionMessage(generated, message)
	if generated.Code != http.StatusOK {
		t.Fatalf("message status = %d: %s", generated.Code, generated.Body.String())
	}
	var proposal app.ChatDraftProposal
	if err := json.NewDecoder(generated.Body).Decode(&proposal); err != nil {
		t.Fatal(err)
	}
	if proposal.Draft.TargetPath != "sample.go" || proposal.Draft.TargetSymbol != "Run" || proposal.Draft.Declaration == "" || proposal.AssistantMessage.Content != "Updated Run." {
		t.Fatalf("proposal = %+v", proposal)
	}

	retarget := httptest.NewRequest(http.MethodPost, "/api/projects/current/chat/sessions/"+session.ID+"/messages", bytes.NewReader([]byte(`{"message":"Change another file.","file_path":"other.go"}`)))
	retarget.SetPathValue("sessionID", session.ID)
	retargetResponse := httptest.NewRecorder()
	handler.SendSessionMessage(retargetResponse, retarget)
	if retargetResponse.Code != http.StatusBadRequest {
		t.Fatalf("retarget response = %d: %s", retargetResponse.Code, retargetResponse.Body.String())
	}
	assertStructuredError(t, retargetResponse)

}

func TestChatSessionEndpointReportsCancellationAndStaleSession(t *testing.T) {
	started := make(chan struct{}, 1)
	providerCanceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		started <- struct{}{}
		<-r.Context().Done()
		providerCanceled <- struct{}{}
	}))
	defer server.Close()
	handler, identity := newChatSessionTestHandler(t, server.URL, 0)
	session := openChatSessionThroughHandler(t, handler, identity)
	request := httptest.NewRequest(http.MethodPost, "/api/projects/current/chat/sessions/"+session.ID+"/messages", bytes.NewReader([]byte(`{"message":"Change Run."}`)))
	request.SetPathValue("sessionID", session.ID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.SendSessionMessage(response, request)
		close(done)
	}()
	waitForChatTestSignal(t, started, "chat provider request")
	cancel()
	waitForChatTestSignal(t, done, "canceled handler response")
	waitForChatTestSignal(t, providerCanceled, "provider cancellation")
	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("canceled status = %d: %s", response.Code, response.Body.String())
	}
}

func TestChatSessionEndpointPinsTaskSpecsAndRejectsUnpreparedRepairs(t *testing.T) {
	handler, identity := newChatSessionTestHandler(t, "http://127.0.0.1:1", 0)
	task := &project.BugTaskSpec{SchemaVersion: project.BugTaskSpecSchemaVersion, TargetPath: "sample.go", TargetSymbol: "Run", TargetSignature: "ignored", AcceptanceCriteria: []string{"Change only Run."}}
	response := httptest.NewRecorder()
	handler.OpenSession(response, httptest.NewRequest(http.MethodPost, "/api/projects/current/chat/sessions", bytes.NewReader(marshalChatBody(t, ChatSessionRequest{ProjectID: identity.projectID, ProjectRevision: identity.revision, BaseFileHash: identity.hash, OpenPath: "sample.go", Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: "Run", TaskSpec: task}))))
	if response.Code != http.StatusCreated {
		t.Fatalf("task session = %d: %s", response.Code, response.Body.String())
	}
	var session app.ChatSession
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.TaskSpec == nil || session.TaskSpec.TargetPath != "sample.go" || session.TaskSpec.TargetSymbol != "Run" {
		t.Fatalf("pinned task spec = %+v", session.TaskSpec)
	}
	repair := httptest.NewRequest(http.MethodPost, "/api/projects/current/chat/sessions/"+session.ID+"/messages", bytes.NewBufferString(`{"message":"Use checks.","repair":true}`))
	repair.SetPathValue("sessionID", session.ID)
	failed := httptest.NewRecorder()
	handler.SendSessionMessage(failed, repair)
	if failed.Code != http.StatusBadRequest {
		t.Fatalf("unprepared repair = %d: %s", failed.Code, failed.Body.String())
	}
	assertStructuredError(t, failed)
}

func TestChatSessionEndpointRejectsInvalidEditMode(t *testing.T) {
	handler, identity := newChatSessionTestHandler(t, "http://127.0.0.1:1", 0)
	response := httptest.NewRecorder()
	handler.OpenSession(response, httptest.NewRequest(http.MethodPost, "/api/projects/current/chat/sessions", bytes.NewReader(marshalChatBody(t, ChatSessionRequest{
		ProjectID: identity.projectID, ProjectRevision: identity.revision, BaseFileHash: identity.hash,
		OpenPath: "sample.go", Mode: "rename_symbol", TargetSymbol: "Run",
	}))))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode status = %d: %s", response.Code, response.Body.String())
	}
	assertStructuredError(t, response)
}

type chatSessionIdentity struct {
	projectID string
	revision  string
	hash      string
}

func newChatSessionTestHandler(t *testing.T, baseURL string, generationSeconds int) (*ChatHandler, chatSessionIdentity) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package sample\n\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{}); err != nil {
		t.Fatal(err)
	}
	analysis, err := manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := project.GetFileInfo(root, "sample.go")
	if err != nil {
		t.Fatal(err)
	}
	cfg := scopedHandlerConfig(baseURL)
	cfg.Timeouts = config.TimeoutConfig{GenerationSeconds: generationSeconds}
	cfg.Retry = config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1}
	service, err := app.New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	return NewChatHandler(service), chatSessionIdentity{projectID: analysis.ProjectID, revision: analysis.ProjectRevision, hash: file.ContentHash}
}

func openChatSessionThroughHandler(t *testing.T, handler *ChatHandler, identity chatSessionIdentity) app.ChatSession {
	t.Helper()
	response := httptest.NewRecorder()
	handler.OpenSession(response, httptest.NewRequest(http.MethodPost, "/api/projects/current/chat/sessions", bytes.NewReader(marshalChatBody(t, ChatSessionRequest{ProjectID: identity.projectID, ProjectRevision: identity.revision, BaseFileHash: identity.hash, OpenPath: "sample.go", Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: "Run"}))))
	if response.Code != http.StatusCreated {
		t.Fatalf("open status = %d: %s", response.Code, response.Body.String())
	}
	var session app.ChatSession
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	return session
}

func marshalChatBody(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func waitForChatTestSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}
