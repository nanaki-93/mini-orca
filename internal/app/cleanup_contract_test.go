package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestPreviewOperationsNeverMutateSelectedSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Messages) == 0 {
			t.Fatal("model request had no messages")
		}
		content := request.Messages[0].Content
		response := `{"purpose":"Explains the selected file.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{"Run":"Runs the command."}}`
		if strings.Contains(content, "Return exactly one complete Go function") {
			response = `{"version":"v1","declaration":"func Run() { println(\"draft\") }","explanation":"Produces a reviewable declaration."}`
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: response}}}})
	}))
	defer server.Close()

	service, root := newSemanticAnalysisService(t, server.URL, 0)
	sourcePath := filepath.Join(root, "main.go")
	original, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	assertSourceUnchanged := func() {
		t.Helper()
		current, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		if string(current) != string(original) {
			t.Fatalf("preview operation modified source:\nwant %q\n got %q", original, current)
		}
	}

	if _, err := service.AnalyzeFile(context.Background(), "main.go", false, false); err != nil {
		t.Fatal(err)
	}
	assertSourceUnchanged()

	session := openFixtureChatSession(t, service, project.DeclarationEditReplaceSymbol, "Run")
	proposal, err := service.SendChatSessionMessage(context.Background(), ChatSessionMessageRequest{SessionID: session.ID, Message: "Improve Run."})
	if err != nil {
		t.Fatal(err)
	}
	assertSourceUnchanged()

	validated, err := service.ValidateDraft(proposal.Draft.ID, proposal.Draft.Revision)
	if err != nil || validated.State != DraftValid {
		t.Fatalf("validate draft = %+v, %v", validated, err)
	}
	assertSourceUnchanged()

	checks, err := service.CheckDraft(context.Background(), DraftCheckRequest{ID: validated.ID, ExpectedRevision: validated.Revision, ExpectedHash: validated.Hash})
	if err != nil || !checks.Applicable {
		t.Fatalf("check draft = %+v, %v", checks, err)
	}
	assertSourceUnchanged()
}
