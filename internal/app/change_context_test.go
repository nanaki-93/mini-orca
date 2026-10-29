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

func TestChangeProposalRejectionNamesThePath(t *testing.T) {
	session := &ChangeSession{
		Targets: []ChangeTarget{{Path: "cmd/master_test.go", Exists: true, Content: "package cmd\n"}},
		Plan:    &ChangePlan{ReadTargets: []ChangeTarget{{Path: "cmd/master.go", Exists: true, Content: "package cmd\n"}}},
	}
	for _, test := range []struct {
		name, path, content, want string
	}{
		{"new file outside scope", "cmd/util_test.go", "package cmd\n", `proposal wrote "cmd/util_test.go", outside the task's files (a new file must be in the task's scope)`},
		{"read-only context", "./cmd/master.go", "package cmd\n", `proposal wrote "./cmd/master.go", which the task only reads; only the task's files can change`},
		{"escaping path", "../cmd/master_test.go", "package cmd\n", `proposal wrote "../cmd/master_test.go", outside the task's files`},
		{"deletion", "cmd/master_test.go", "", "proposal emptied cmd/master_test.go; deleting files is not supported"},
		{"binary content", "cmd/master_test.go", "package cmd\x00\n", "proposal for cmd/master_test.go is not UTF-8 text"},
		{"control characters", "cmd/\nevil.go", "package cmd\n", `proposal wrote "cmd/\nevil.go"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := buildChangeEdits(session, []ChangeEditInput{{Path: test.path, Content: test.content}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
	long := strings.Repeat("é", 200) + ".go"
	_, err := buildChangeEdits(session, []ChangeEditInput{{Path: long, Content: "package cmd\n"}})
	if err == nil || len(err.Error()) > 220 || !strings.Contains(err.Error(), "…") {
		t.Fatalf("long path was not bounded: %v", err)
	}
}

func TestChangeProposalAcceptsCleanedCapturedPaths(t *testing.T) {
	session := &ChangeSession{Targets: []ChangeTarget{{Path: "cmd/util_test.go"}}}
	edits, err := buildChangeEdits(session, []ChangeEditInput{{Path: "./cmd/util_test.go", Content: "package cmd\n"}})
	if err != nil || len(edits) != 1 || edits[0].Path != "cmd/util_test.go" {
		t.Fatalf("cleaned captured path was not accepted as the target: %+v, %v", edits, err)
	}
	_, err = buildChangeEdits(session, []ChangeEditInput{{Path: "cmd/util_test.go", Content: "package cmd\n"}, {Path: "cmd//util_test.go", Content: "package cmd\n\n"}})
	if err == nil || err.Error() != "proposal repeated cmd/util_test.go" {
		t.Fatalf("two spellings of one target were accepted: %v", err)
	}
}

func TestChangePromptListsTheWritablePaths(t *testing.T) {
	session := &ChangeSession{Kind: "fix", Targets: []ChangeTarget{
		{Path: "cmd/master_test.go", Exists: true, Content: "package cmd\n"},
		{Path: "cmd/util_test.go"},
	}}
	messages, err := changeStageMessages(session, "Move the helpers into util_test.go", "", false)
	if err != nil {
		t.Fatal(err)
	}
	want := "Write only these exact paths: cmd/master_test.go, cmd/util_test.go (new file). Any other path, including a new helper file, rejects the whole proposal"
	if !strings.Contains(messages[0].Content, want) {
		t.Fatalf("system prompt does not list the writable paths:\n%s", messages[0].Content)
	}
}
