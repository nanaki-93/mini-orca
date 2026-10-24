package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func memoryScopeFixture(t *testing.T, s *Service, request MemoryMutation) memoryScope {
	t.Helper()
	store, err := memoryStore(s.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	policy, err := project.NewContextPolicy(s.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	return memoryScope{root: s.manager.Root(), projectID: request.ProjectID, revision: request.ProjectRevision, policy: policy.Version(), sources: request.Draft.Sources, store: store, authorize: func(context.Context) error { return nil }}
}

func TestMemoryRetrievalScopeExpirySourceAndForget(t *testing.T) {
	s, root := newSemanticAnalysisServiceWithHelper(t, "http://127.0.0.1:1", 0)
	request := memoryDraftFixture(t, s, "main.go")
	record := retainMemoryFixture(t, s, request)
	scope := memoryScopeFixture(t, s, request)
	for _, kind := range []string{"current", "historical", "foreign_path", "declaration", "hash", "policy"} {
		t.Run(kind, func(t *testing.T) {
			candidate := scope
			candidate.sources = append([]agentstate.MemorySource(nil), scope.sources...)
			switch kind {
			case "historical":
				candidate.revision = "another-revision"
			case "foreign_path":
				candidate.sources[0].Path = "helper.go"
			case "declaration":
				candidate.sources[0].StartLine, candidate.sources[0].EndLine = 5, 5
			case "hash":
				candidate.sources[0].Hash = "sha256:" + strings.Repeat("a", 64)
			case "policy":
				candidate.policy = "different-policy"
			}
			matches, err := candidate.search(t.Context(), "semantic")
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if kind == "current" || kind == "historical" {
				want = 1
			}
			if len(matches) != want {
				t.Fatalf("%s: matches=%+v", kind, matches)
			}
			if want == 1 && matches[0].Stale != (kind == "historical") {
				t.Fatal("lost historical label")
			}
		})
	}
	versions := []agentstate.MemoryVersion{{ID: record.ID, Version: record.Version}}
	if err := scope.validate(t.Context(), versions); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MutateMemory(t.Context(), record.ID, MemoryMutation{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Action: "forget", Version: record.Version}); err != nil {
		t.Fatal(err)
	}
	if err := scope.validate(t.Context(), versions); !errors.Is(err, ErrMemoryConflict) {
		t.Fatalf("forgotten record retained dispatch authority: %v", err)
	}
	retainMemoryFixture(t, s, request)
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Replaced() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	matches, err := scope.search(t.Context(), "semantic")
	if err != nil || len(matches) != 0 {
		t.Fatalf("changed source retrieved: %+v %v", matches, err)
	}
}

func TestMemoryRetrievalRevalidatesPolicyAndProjectAuthority(t *testing.T) {
	s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	request := memoryDraftFixture(t, s, "main.go")
	record := retainMemoryFixture(t, s, request)
	scope := memoryScopeFixture(t, s, request)
	if err := os.WriteFile(filepath.Join(root, ".mini-orcaignore"), []byte("main.go\n"), 0600); err != nil {
		t.Fatal(err)
	}
	matches, err := scope.search(t.Context(), "semantic")
	if err != nil || len(matches) != 0 {
		t.Fatalf("excluded source retrieved: %+v %v", matches, err)
	}
	if err := scope.validate(t.Context(), []agentstate.MemoryVersion{{ID: record.ID, Version: record.Version}}); !errors.Is(err, ErrMemoryConflict) {
		t.Fatalf("policy change ignored: %v", err)
	}
	scope.authorize = func(context.Context) error { return project.ErrRevisionConflict }
	if _, err := scope.search(t.Context(), "semantic"); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatal(err)
	}
}

func TestMemorySemanticWorkflowUsesOnlyAcceptedSelectedFileAdvice(t *testing.T) {
	var prompts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		prompts = append(prompts, request.Messages[0].Content)
		json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Runs the selected command."}`}}}})
	}))
	defer server.Close()
	s, _ := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	request := memoryDraftFixture(t, s, "main.go")
	candidate, err := s.MutateMemory(t.Context(), "", request)
	if err != nil {
		t.Fatal(err)
	}
	other := memoryDraftFixture(t, s, "helper.go")
	other.Draft.Text = "OTHER_FILE_PRIVATE_ADVICE"
	retainMemoryFixture(t, s, other)
	if _, err := s.AnalyzeFile(t.Context(), "main.go", true, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(prompts[0], "PRIVATE_ADVICE") {
		t.Fatal("candidate or foreign-file memory leaked")
	}
	_, err = s.MutateMemory(t.Context(), candidate.ID, MemoryMutation{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Action: "accept", Version: candidate.Version})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AnalyzeFile(t.Context(), "main.go", true, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompts[1], "RETAINED_PRIVATE_ADVICE") || strings.Contains(prompts[1], "OTHER_FILE_PRIVATE_ADVICE") {
		t.Fatal("selected-file memory scope not preserved")
	}
	count := len(prompts)
	if _, err := s.AnalyzeFile(t.Context(), "main.go", false, false); err != nil || len(prompts) != count {
		t.Fatalf("passive cache dispatched: %v", err)
	}
}

func TestMemorySourceBuildersExcludeTruncatedInventoryAndStageZeroTests(t *testing.T) {
	manifest := project.ContextManifest{Included: []project.ContextFile{{Path: "inventory.go"}, {Path: "partial.go", Tokens: 20, Truncated: true}, {Path: "full.go", Tokens: 20}}}
	if sources := manifestMemorySources(manifest); len(sources) != 1 || sources[0].Path != "full.go" {
		t.Fatalf("inventoried or partial source admitted: %+v", sources)
	}
	session := &ChangeSession{Targets: []ChangeTarget{{Path: "main.go", Exists: true}, {Path: "main_test.go", Exists: true}, {Path: "new.go"}}}
	if sources := changeMemorySources(session, true); len(sources) != 1 || sources[0].Path != "main.go" {
		t.Fatalf("stage zero exposed tests: %+v", sources)
	}
}

func TestMemoryPlanInvestigationAdmitsWholeCapturedFiles(t *testing.T) {
	var prompt, hash string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		for _, message := range request.Messages {
			prompt += message.Content
		}
		content, err := json.Marshal(planInvestigationFixture(hash))
		if err != nil {
			t.Error(err)
		}
		json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: string(content)}}}})
	}))
	defer server.Close()
	s, _ := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	for _, path := range []string{"main.go", "helper.go"} {
		draft := memoryDraftFixture(t, s, path)
		draft.Draft.Title = "Run plan decision"
		if path == "helper.go" {
			draft.Draft.Text = "OUTSIDE_PLAN_PRIVATE_ADVICE"
		}
		retainMemoryFixture(t, s, draft)
	}
	request := planInvestigationRequest(t, s)
	hash = request.Sources[0].Hash
	if _, err := s.InvestigatePlan(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "RETAINED_PRIVATE_ADVICE") || strings.Contains(prompt, "OUTSIDE_PLAN_PRIVATE_ADVICE") {
		t.Fatal("whole-file plan memory did not preserve the captured read scope")
	}
}

func TestMemoryCanceledSearchIsNotPersistenceFailure(t *testing.T) {
	s, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	request := memoryDraftFixture(t, s, "main.go")
	retainMemoryFixture(t, s, request)
	scope := memoryScopeFixture(t, s, request)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	scope.authorize = func(context.Context) error { cancel(); return nil }
	_, err := scope.search(ctx, "semantic")
	if !errors.Is(err, context.Canceled) || errors.Is(err, agentstate.ErrPersistence) {
		t.Fatalf("cancellation classified as storage failure: %v", err)
	}
}
