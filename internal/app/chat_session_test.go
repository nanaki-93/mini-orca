package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestChatSessionCreatesDeclarationDraftsWithRevisionLineage(t *testing.T) {
	var prompts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		prompts = append(prompts, request.Messages[0].Content)
		output := `{"version":"v1","declaration":"func Run() { println(\"first\") }","explanation":"First proposal."}`
		if len(prompts) == 2 {
			output = `{"version":"v1","declaration":"func Run() { println(\"revised\") }","explanation":"Revised proposal."}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": output}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	session := openFixtureChatSession(t, service, project.DeclarationEditReplaceSymbol, "Run")

	first, err := service.SendChatSessionMessage(context.Background(), ChatSessionMessageRequest{SessionID: session.ID, Message: "Make it clearer."})
	if err != nil {
		t.Fatal(err)
	}
	if first.Draft.TargetPath != "main.go" || first.Draft.TargetSymbol != "Run" || first.Draft.ParentDraftID != "" || first.Draft.State != DraftGenerated {
		t.Fatalf("first proposal draft = %+v", first.Draft)
	}
	second, err := service.SendChatSessionMessage(context.Background(), ChatSessionMessageRequest{SessionID: session.ID, ParentDraftID: first.Draft.ID, Message: "Revise this proposal to say revised."})
	if err != nil {
		t.Fatal(err)
	}
	if second.Draft.ParentDraftID != first.Draft.ID || second.Draft.Declaration == first.Draft.Declaration {
		t.Fatalf("revision lineage = %+v", second.Draft)
	}
	stored, err := service.ChatSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.LatestDraftID != second.Draft.ID || len(stored.Messages) != 4 || stored.Messages[1].Content != "First proposal." {
		t.Fatalf("stored session = %+v", stored)
	}
	if len(prompts) != 2 || !strings.Contains(prompts[0], "Return exactly one complete Go function") || strings.Contains(prompts[0], "candidate_content") || !strings.Contains(prompts[1], "Current declaration proposal") || !strings.Contains(prompts[1], first.Draft.Declaration) {
		t.Fatalf("declaration prompts = %q", prompts)
	}
	content, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "first") || strings.Contains(string(content), "revised") {
		t.Fatalf("chat proposal changed source: %s", content)
	}
}

func TestChatSessionRejectsInvalidTargetsAndStaleState(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	if _, err := openChatSession(t, service, project.DeclarationEditReplaceSymbol, "Missing"); err == nil {
		t.Fatal("replace session accepted a missing symbol")
	}
	if _, err := openChatSession(t, service, project.DeclarationEditCreateSymbol, "Run"); err == nil {
		t.Fatal("create session accepted an existing symbol")
	}
	if _, err := openChatSession(t, service, project.DeclarationEditCreateSymbol, "not-valid"); err == nil {
		t.Fatal("create session accepted an invalid symbol")
	}
	session := openFixtureChatSession(t, service, project.DeclarationEditReplaceSymbol, "Run")
	if _, err := service.SendChatSessionMessage(context.Background(), ChatSessionMessageRequest{SessionID: session.ID, Message: "change", ParentDraftID: "different"}); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("retargeted revision error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SendChatSessionMessage(context.Background(), ChatSessionMessageRequest{SessionID: session.ID, Message: "change"}); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("stale session message error = %v", err)
	}
	stale, err := service.ChatSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stale.State != "stale" {
		t.Fatalf("session state = %q, want stale", stale.State)
	}
}

func TestParseDeclarationDraftResponseContract(t *testing.T) {
	valid := `{"version":"v1","declaration":"func Run() {}","imports":["fmt"],"explanation":"Adds output."}`
	response, err := ParseDeclarationDraftResponse(valid)
	if err != nil || response.Declaration != "func Run() {}" || response.Explanation == "" {
		t.Fatalf("response = %+v, err = %v", response, err)
	}
	for _, output := range []string{
		"```json\n" + valid + "\n```",
		`{"version":"v2","declaration":"func Run() {}","explanation":"x"}`,
		`{"version":"v1","declaration":"func Run() {}","explanation":"x","target_path":"other.go"}`,
		`{"version":"v1","explanation":"x"}`,
	} {
		if _, err := ParseDeclarationDraftResponse(output); err == nil {
			t.Fatalf("accepted invalid response %q", output)
		}
	}
}

func TestChatSessionCancellationStopsProviderRequest(t *testing.T) {
	started := make(chan struct{}, 1)
	canceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		started <- struct{}{}
		<-r.Context().Done()
		canceled <- struct{}{}
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	session := openFixtureChatSession(t, service, project.DeclarationEditReplaceSymbol, "Run")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.SendChatSessionMessage(ctx, ChatSessionMessageRequest{SessionID: session.ID, Message: "change"})
		done <- err
	}()
	waitForTestSignal(t, started, "chat provider request")
	cancel()
	if err := waitForTestError(t, done, "canceled chat generation"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	waitForTestSignal(t, canceled, "chat provider cancellation")
}

func openFixtureChatSession(t *testing.T, service *Service, mode project.DeclarationEditMode, target string) *ChatSession {
	t.Helper()
	session, err := openChatSession(t, service, mode, target)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func openChatSession(t *testing.T, service *Service, mode project.DeclarationEditMode, target string) (*ChatSession, error) {
	t.Helper()
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	return service.OpenChatSession(ChatSessionCreateRequest{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: file.ContentHash, OpenPath: "main.go", Mode: mode, TargetSymbol: target})
}
