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

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func planInvestigationRequest(t *testing.T, s *Service) PlanInvestigationRequest {
	t.Helper()
	d := declarationRequest(t, s)
	return PlanInvestigationRequest{ProjectID: d.ProjectID, ProjectRevision: d.ProjectRevision, Paths: []string{"main.go"}, Sources: []PlanInvestigationSource{{Path: "main.go", Hash: d.BaseFileHash, Exists: true}}, Plan: ChangePlanInput{Problem: "Improve Run", Approach: "Inspect the existing behavior"}, AcceptanceCriteria: []string{"Preserve the public API"}}
}

func planInvestigationFixture(hash string) planInvestigationOutput {
	return planInvestigationOutput{Version: "v1", Approach: "Keep Run's signature and add focused regression coverage.", Evidence: []PlanEvidence{{Path: "main.go", Hash: hash, StartLine: 1, EndLine: 1, Quote: "package main", Observation: "The target belongs to the main package."}}, Assumptions: []string{"Existing public behavior should remain compatible."}, SuggestedScope: []string{"main_test.go"}, AcceptanceCriteria: []string{"Run preserves its public signature", "Regression tests cover the observed behavior"}}
}

func TestPlanInvestigationUsesToolsWithoutCreatingOrApprovingChanges(t *testing.T) {
	calls := 0
	hash := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"response_format"`) || strings.Contains(string(body), "set_model_response") {
			t.Error("wrong structured tool protocol")
		}
		if strings.Contains(string(body), "helper secret") {
			t.Error("outside source reached provider")
		}
		if calls == 1 {
			repositoryToolCall(w, "read_snippet", `{"path":"main.go","start_line":1,"end_line":1}`)
			return
		}
		content, _ := json.Marshal(planInvestigationFixture(hash))
		json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: string(content)}}}})
	}))
	defer server.Close()
	s, root := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	request := planInvestigationRequest(t, s)
	hash = request.Sources[0].Hash
	before, _ := os.ReadFile(filepath.Join(root, "main.go"))
	result, err := s.InvestigatePlan(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "main.go"))
	history, err := s.ChangeHistory(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || result.InputHash == "" || len(result.Evidence) != 1 || len(history) != 0 || len(s.changeAuthority) != 0 || string(before) != string(after) {
		t.Fatalf("unexpected investigation: %+v calls=%d history=%v", result, calls, history)
	}
}

func TestPlanInvestigationRejectsConsentStaleAndUnexpectedScopeBeforeDispatch(t *testing.T) {
	s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	request := planInvestigationRequest(t, s)
	s.runtimes.function.effective.RemoteProvider = true
	if _, err := s.InvestigatePlan(t.Context(), request); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatal(err)
	}
	s.runtimes.function.effective.RemoteProvider = false
	for _, alter := range []func(*PlanInvestigationRequest){
		func(r *PlanInvestigationRequest) { r.ProjectRevision = "stale" },
		func(r *PlanInvestigationRequest) { r.Sources = nil },
		func(r *PlanInvestigationRequest) { r.Paths = []string{"other.go"} },
		func(r *PlanInvestigationRequest) { r.Plan.ReadPaths = []string{"main.go"} },
	} {
		copy := request
		alter(&copy)
		if _, err := s.preparePlanInvestigation(t.Context(), copy); err == nil {
			t.Fatal("invalid plan admission")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.preparePlanInvestigation(t.Context(), request); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatal(err)
	}
}

func TestPlanInvestigationRejectsFabricatedCitationsAndIncompleteOutput(t *testing.T) {
	s, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	request := planInvestigationRequest(t, s)
	p, err := s.preparePlanInvestigation(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	for _, alter := range []func(*planInvestigationOutput){
		func(r *planInvestigationOutput) { r.Evidence[0].Hash = "changed" },
		func(r *planInvestigationOutput) { r.Evidence[0].Path = "other.go" },
		func(r *planInvestigationOutput) { r.Evidence[0].Quote = "fabricated source" },
		func(r *planInvestigationOutput) { r.Evidence[0].EndLine = 100000 },
		func(r *planInvestigationOutput) { r.AcceptanceCriteria = nil },
		func(r *planInvestigationOutput) { r.Assumptions = nil },
		func(r *planInvestigationOutput) { r.SuggestedScope = []string{"../outside.go"} },
	} {
		response := planInvestigationFixture(request.Sources[0].Hash)
		alter(&response)
		data, _ := json.Marshal(response)
		if _, err := parsePlanInvestigation(string(data), p); err == nil {
			t.Fatalf("invalid evidence accepted: %s", data)
		}
	}
}

func TestPlanInvestigationRejectsNewFileAppearingDuringProviderWork(t *testing.T) {
	root, hash := "", ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package main\n"), 0600); err != nil {
			t.Error(err)
		}
		data, _ := json.Marshal(planInvestigationFixture(hash))
		json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: string(data)}}}})
	}))
	defer server.Close()
	s, dir := newSemanticAnalysisService(t, server.URL, 0)
	root = dir
	request := planInvestigationRequest(t, s)
	hash = request.Sources[0].Hash
	request.Paths = append(request.Paths, "new.go")
	request.Sources = append(request.Sources, PlanInvestigationSource{Path: "new.go"})
	if result, err := s.InvestigatePlan(t.Context(), request); !errors.Is(err, project.ErrRevisionConflict) || result != nil {
		t.Fatalf("late scope accepted: %+v %v", result, err)
	}
}

func TestPlanInvestigationCancellationPreservesDraft(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	request := planInvestigationRequest(t, s)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, err := s.InvestigatePlan(ctx, request); done <- err }()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if request.Plan.Approach != "Inspect the existing behavior" || len(s.changeAuthority) != 0 {
		t.Fatal("cancellation mutated plan")
	}
}

func TestPlanInvestigationCompletionOnlyProfilesReceiveBoundedCapturedSource(t *testing.T) {
	s, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	request := planInvestigationRequest(t, s)
	p, err := s.preparePlanInvestigation(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	runtime := s.runtimes.function
	runtime.profile.Provider = config.PiProvider
	input, err := s.planInvestigationInput(t.Context(), p, runtime)
	if err != nil || input.Investigation != nil || input.Schema == nil || len(input.Messages) != 1 || !strings.Contains(input.Messages[0].Content, "package main") {
		t.Fatalf("completion fallback lost source or added tools: %+v %v", input, err)
	}
	runtime.effective.ContextMaxTokens = 1
	if _, err := s.planInvestigationInput(t.Context(), p, runtime); err == nil {
		t.Fatal("completion fallback ignored configured context limit")
	}
}

func TestPlanInvestigationFreezesInputAndRejectsTrailingEmptyLineCitation(t *testing.T) {
	s, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	request := planInvestigationRequest(t, s)
	request.Plan.Assumptions = []string{"Keep the API"}
	p, err := s.preparePlanInvestigation(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Sources[0].Hash = "changed"
	request.Paths[0] = "other.go"
	request.Plan.Assumptions[0] = "different assumption"
	if p.request.Paths[0] != "main.go" || p.request.Sources[0].Hash == "changed" || p.request.Plan.Assumptions[0] != "Keep the API" {
		t.Fatal("caller mutated admitted snapshot")
	}
	source := "package main\n"
	citation := PlanEvidence{Path: "main.go", Hash: contentHash([]byte(source)), StartLine: 1, EndLine: 2, Quote: "package main", Observation: "Package"}
	if err := validatePlanCitation(citation, source); err == nil {
		t.Fatal("citation included a nonexistent trailing line")
	}
}
