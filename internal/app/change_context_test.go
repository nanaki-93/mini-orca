package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func TestGuidedChangeRequestsSolutionWithoutRepeatingCause(t *testing.T) {
	for _, kind := range []string{"fix", "performance", "security", "feature"} {
		t.Run(kind, func(t *testing.T) {
			requests := make(chan llm.ChatRequest, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request llm.ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode model request: %v", err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				requests <- request
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{
					Content: `{"explanation":"Document the return value. Runtime behavior still needs verification.","changes":[{"path":"README.md","content":"# Behavior\nReturns the result.\n"}]}`,
				}}}})
			}))
			t.Cleanup(server.Close)
			service, _ := newSemanticAnalysisService(t, server.URL, 0)
			index, err := service.manager.Index()
			if err != nil {
				t.Fatal(err)
			}
			session, err := service.OpenChangeSession(context.Background(), ChangeCreateRequest{
				ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision,
				Kind: kind, Title: "Document the return value", Paths: []string{"README.md"},
			})
			if err != nil {
				t.Fatal(err)
			}
			proposal, err := service.SendChangeMessage(context.Background(), session.ID, ChangeMessageRequest{
				ChangeIdentity: changeIdentity(session), Message: "Cause: the return value is undocumented. Document the behavior.",
			})
			if err != nil {
				t.Fatal(err)
			}
			request := <-requests
			input := request.Messages[len(request.Messages)-1].Content
			if !strings.Contains(input, "Cause: the return value is undocumented.") {
				t.Fatal("original finding evidence was removed from model context")
			}
			for _, guidance := range []string{"Describe only the correction", "Do not repeat the cause", "Include unresolved failures, limitations and uncertainty"} {
				if strings.Contains(request.Messages[0].Content, guidance) != (kind != "feature") {
					t.Fatalf("incorrect explanation guidance for %s: %s", kind, guidance)
				}
			}
			if got := proposal.Messages[len(proposal.Messages)-1].Content; got != "Document the return value. Runtime behavior still needs verification." {
				t.Fatalf("solution or limitation changed: %q", got)
			}
		})
	}
}
