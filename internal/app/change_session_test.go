package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func changeIdentity(session *ChangeSession) ChangeIdentity {
	return ChangeIdentity{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Revision: session.Revision, Hash: session.Hash}
}

func TestChangeHistoryLimitCountsOnlyConversationsAndRejectsCorruption(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	session := openChangeFixture(t, service, "main.go")
	initialID := session.ID
	for i := 0; i < 200; i++ {
		session.ID = fmt.Sprintf("change-%032x", i)
		if err := writeChangeSession(root, session); err != nil {
			t.Fatal(err)
		}
	}
	// Remove the initial random entry; keep exactly 200 conversations plus
	// metadata owned by other workflows.
	if err := os.Remove(filepath.Join(root, ".mini-orca/changes", initialID+".json")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"features.json", "last-mutation.json"} {
		if err := os.WriteFile(filepath.Join(root, ".mini-orca/changes", name), []byte("other metadata"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	history, err := service.ChangeHistory(context.Background())
	if err != nil || len(history) != 200 {
		t.Fatalf("metadata changed conversation limit: %d, %v", len(history), err)
	}
	if _, err := service.OpenChangeSession(context.Background(), ChangeCreateRequest{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Kind: "feature", Title: "Above limit", Paths: []string{"main.go"}}); err == nil {
		t.Fatal("conversation limit was not enforced")
	}
	session.Changes = []ChangeEdit{{Path: "main.go", Content: "package main\n", Hash: contentHash([]byte("package main\n"))}}
	session.Hash = changeProposalHash(session.Changes)
	session.Changes = append(session.Changes, session.Changes[0])
	if err := writeChangeSession(root, session); err == nil {
		t.Fatal("duplicate persisted changes accepted")
	}
}

func openChangeFixture(t *testing.T, service *Service, paths ...string) *ChangeSession {
	t.Helper()
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.OpenChangeSession(context.Background(), ChangeCreateRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: "feature", Title: "Implement a bounded task", Paths: paths, AcceptanceCriteria: []string{"Preserve existing behavior."}})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func changeProvider(t *testing.T, response func() string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: response()}}}})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestChangeGenerationHistoryAndRevision(t *testing.T) {
	var calls atomic.Int32
	server := changeProvider(t, func() string {
		calls.Add(1)
		return `{"explanation":"Add the helper.","changes":[{"path":"new.go","content":"package main\nfunc Helper() int {return 1}\n"}]}`
	})
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Preserve cancellation."), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	session := openChangeFixture(t, service, "main.go", "new.go")
	first, err := service.SendChangeMessage(context.Background(), session.ID, ChangeMessageRequest{ChangeIdentity: changeIdentity(session), Message: "Add a helper."})
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || len(first.Changes) != 1 || !strings.Contains(first.Changes[0].Content, "return 1") || len(first.ContextManifest.Included) != 3 {
		t.Fatalf("proposal: %+v", first)
	}
	if _, err := os.Stat(filepath.Join(root, "new.go")); !os.IsNotExist(err) {
		t.Fatal("generation wrote source")
	}
	if _, err := service.SendChangeMessage(context.Background(), session.ID, ChangeMessageRequest{ChangeIdentity: changeIdentity(session), Message: "Stale revision."}); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("accepted old revision: %v", err)
	}
	first.Checks = []DraftCheck{{Name: "parse", Required: true, State: CheckPassed}}
	first.ReviewedHash = first.Hash
	if err := writeChangeSession(root, first); err != nil {
		t.Fatal(err)
	}
	resumed, err := service.ResumeChange(context.Background(), session.ID)
	if err != nil || len(resumed.Messages) != 2 || len(resumed.Checks) != 0 || resumed.ReviewedHash != "" || resumed.Freshness != "current" {
		t.Fatalf("resume: %+v, %v", resumed, err)
	}
	history, err := service.ChangeHistory(context.Background())
	if err != nil || len(history) != 1 || calls.Load() != 1 {
		t.Fatalf("passive history: %+v, %v", history, err)
	}
	info, err := os.Stat(filepath.Join(root, ".mini-orca/changes", session.ID+".json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("history permissions: %v, %v", info, err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stale, err := service.ResumeChange(context.Background(), session.ID)
	if err != nil || stale.Freshness != "stale" || len(stale.Changes) != 1 {
		t.Fatalf("stale history lost proposal: %+v, %v", stale, err)
	}
}

func TestChangeRejectsMalformedAndRetargetedResponses(t *testing.T) {
	for _, output := range []string{`{}`, `{"explanation":"x","changes":[{"path":"other.go","content":"package main"}]}`, `{"explanation":"x","changes":[{"path":"new.go","content":"package main","extra":true}]}`, `{"explanation":"x","changes":[]} trailing`, `{"explanation":"x","changes":[{"path":"new.go","content":""}]}`} {
		t.Run(output, func(t *testing.T) {
			server := changeProvider(t, func() string { return output })
			service, _ := newSemanticAnalysisService(t, server.URL, 0)
			session := openChangeFixture(t, service, "new.go")
			if _, err := service.SendChangeMessage(context.Background(), session.ID, ChangeMessageRequest{ChangeIdentity: changeIdentity(session), Message: "Implement."}); err == nil {
				t.Fatal("accepted invalid model output")
			}
			stored, err := service.ChangeSession(context.Background(), session.ID)
			if err != nil || stored.Revision != 0 || len(stored.Messages) != 0 {
				t.Fatalf("published failed output: %+v, %v", stored, err)
			}
		})
	}
}

func TestChangeLateGenerationAndCancellationDoNotPublish(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server := changeProvider(t, func() string {
		close(entered)
		<-release
		return `{"explanation":"x","changes":[{"path":"new.go","content":"package main\n"}]}`
	})
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	session := openChangeFixture(t, service, "new.go")
	result := make(chan error, 1)
	go func() {
		_, err := service.SendChangeMessage(context.Background(), session.ID, ChangeMessageRequest{ChangeIdentity: changeIdentity(session), Message: "Implement."})
		result <- err
	}()
	<-entered
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Changed during generation."), 0644); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-result; !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("late publication: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.OpenChangeSession(ctx, ChangeCreateRequest{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Kind: "feature", Title: "Canceled", Paths: []string{"other.go"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestChangeManualInstructionsAndCorruptHistory(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	index, _ := service.manager.Index()
	proposal, err := service.ProposeInstructions(context.Background(), InstructionProposalRequest{ChangeCreateRequest: ChangeCreateRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: "instructions", Title: "Project instructions", Paths: []string{"AGENTS.md"}}, Content: "Use focused tests.\n"})
	if err != nil || proposal.Revision != 1 || len(proposal.Changes) != 1 {
		t.Fatalf("instruction proposal: %+v, %v", proposal, err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("manual proposal wrote instructions")
	}
	if err := os.WriteFile(filepath.Join(root, ".mini-orca/changes", proposal.ID+".json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeHistory(context.Background()); err == nil {
		t.Fatal("corrupt history became empty success")
	}
	if _, err := service.ChangeSession(context.Background(), "../escape"); err == nil {
		t.Fatal("accepted history traversal")
	}
}

func TestChangeScopeAndRemoteConsent(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "https://provider.invalid", 0)
	session := openChangeFixture(t, service, "new.go")
	if _, err := service.SendChangeMessage(context.Background(), session.ID, ChangeMessageRequest{ChangeIdentity: changeIdentity(session), Message: "Implement."}); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("missing remote consent: %v", err)
	}
	for _, paths := range [][]string{{"main.go", "main.go"}, {"../escape.go"}, {"config.yaml"}, {".mini-orca/state.md"}} {
		if _, err := service.OpenChangeSession(context.Background(), ChangeCreateRequest{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Kind: "feature", Title: "Invalid", Paths: paths}); err == nil {
			t.Fatalf("accepted scope: %v", paths)
		}
	}
}
