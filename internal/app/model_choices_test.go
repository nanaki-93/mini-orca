package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestAvailableModelsDeduplicatesJobsAndIncludesPiLocalModels(t *testing.T) {
	base, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	cfg := scopedTestConfig("http://127.0.0.1:1")
	cfg.ModelScopes.Bug = cfg.ModelScopes.Analyze
	cfg.ModelScopes.Function = config.ModelProfileConfig{Provider: config.PiProvider, CLIPath: "/private/test-pi", Model: "provider/default"}
	s, err := New(cfg, base.manager)
	if err != nil {
		t.Fatal(err)
	}
	s.discoverPiModels = func(_ context.Context, executable string) ([]llm.PiModel, error) {
		if executable != "/private/test-pi" {
			t.Fatalf("Pi executable = %q", executable)
		}
		return []llm.PiModel{
			{Provider: "lm-studio", ID: "qwen/local", Name: "Qwen Local", BaseURL: "http://127.0.0.1:1234/v1?api_key=private-token"},
			{Provider: "openai-codex", ID: "gpt-5", Name: "GPT-5", BaseURL: "https://provider.invalid/v1"},
			{Provider: "custom", ID: "model"},
		}, nil
	}
	catalog, err := s.AvailableModels(context.Background())
	if err != nil || len(catalog.Models) != 5 || catalog.Pi.Status != "ready" {
		t.Fatalf("catalog = %+v, %v", catalog, err)
	}
	if catalog.Defaults["analyze"] != catalog.Defaults["bug"] || catalog.Defaults["bug"] == catalog.Defaults["function"] {
		t.Fatalf("models still depend on their original job: %+v", catalog.Defaults)
	}
	locations := map[string]string{"pi:lm-studio/qwen/local": "local", "pi:openai-codex/gpt-5": "remote", "pi:custom/model": "unknown"}
	for _, choice := range catalog.Models {
		if choice.Model.Scope != "" || choice.Model.Profile != choice.ID {
			t.Fatalf("catalog retained a job: %+v", choice)
		}
		if choice.Source == "pi" && (choice.Location != locations[choice.ID] || choice.Model.ProviderOrigin != "cli://pi" || !choice.Model.RemoteProvider) {
			t.Fatalf("Pi location/consent metadata = %+v", choice)
		}
	}
	encoded, _ := json.Marshal(catalog)
	for _, secret := range []string{"private-token", "/private/test-pi", "api_key", "APIKey"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("catalog leaked %q", secret)
		}
	}
	s.discoverPiModels = func(context.Context, string) ([]llm.PiModel, error) { return nil, errors.New("private-token") }
	partial, err := s.AvailableModels(context.Background())
	if err != nil || len(partial.Models) != 2 || partial.Pi.Status != "unavailable" || strings.Contains(partial.Pi.Message, "private-token") {
		t.Fatalf("partial catalog = %+v, %v", partial, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.AvailableModels(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled catalog = %v", err)
	}
}

func TestCatalogModelsDispatchIndependentlyOfConfiguredJobsAndPersist(t *testing.T) {
	reply := func(stage AnalysisStage) string {
		if stage == AnalysisStageFeatures {
			return validFeatureResponse
		}
		return analysisReply(stage)
	}
	analyze, analyzeCalls := analysisResponseServer(t, reply)
	bug, bugCalls := analysisResponseServer(t, reply)
	function, functionCalls := analysisResponseServer(t, reply)
	base, root := newSemanticAnalysisService(t, analyze.URL, 0)
	cfg := scopedTestConfig(analyze.URL)
	cfg.ModelScopes.Bug.APIBaseURL = bug.URL
	cfg.ModelScopes.Function.APIBaseURL = function.URL
	s, err := New(cfg, base.manager)
	if err != nil {
		t.Fatal(err)
	}
	s.discoverPiModels = func(context.Context, string) ([]llm.PiModel, error) { return []llm.PiModel{}, nil }
	catalog, err := s.AvailableModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	models := &AnalysisModels{Code: catalog.Defaults["function"], Review: catalog.Defaults["bug"], Features: catalog.Defaults["analyze"]}
	preview := analysisModelsPreviewFor(t, s, models, AnalysisRunLimits{100, 30, 2})
	if analyzeCalls.Load()+bugCalls.Load()+functionCalls.Load() != 0 {
		t.Fatal("preview dispatched inference")
	}
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	run := completedAnalysisRun(t, s)
	if run.Status != AnalysisRunCompleted || functionCalls.Load() != 1 || bugCalls.Load() != 2 || analyzeCalls.Load() != 1 {
		t.Fatalf("dispatch = %s: %d/%d/%d", run.Status, functionCalls.Load(), bugCalls.Load(), analyzeCalls.Load())
	}
	restored, err := loadAnalysisRun(root)
	if err != nil || restored.Plan.Models == nil || *restored.Plan.Models != *models {
		t.Fatalf("saved model choices = %+v, %v", restored, err)
	}
	restarted, err := New(cfg, base.manager)
	if err != nil {
		t.Fatal(err)
	}
	cached := analysisModelsPreviewFor(t, restarted, models, preview.Limits)
	if cached.ExpectedModelRequests != 1 {
		t.Fatalf("restart lost selected-model caches: %+v", cached)
	}
	retargeted := analysisStartFor(cached)
	retargeted.Models.Code = "configured:" + strings.Repeat("0", 64)
	if _, err := restarted.StartAnalysisRun(context.Background(), retargeted); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("unknown model was admitted: %v", err)
	}
}

func TestPiCatalogChoicesRequireMembershipAndConsentBeforeDispatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CLI providers require process-group cleanup")
	}
	base, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	calls := filepath.Join(t.TempDir(), "calls")
	cfg := config.Default()
	cfg.ModelScopes.Function = cliAppProfile(t, config.PiProvider, "lm-studio/local-model", validFeatureResponse, calls)
	s, err := New(cfg, base.manager)
	if err != nil {
		t.Fatal(err)
	}
	available := []llm.PiModel{{Provider: "lm-studio", ID: "local-model", BaseURL: "http://127.0.0.1:1234/v1"}}
	s.discoverPiModels = func(context.Context, string) ([]llm.PiModel, error) { return available, nil }
	request := FeatureGenerateRequest{FeatureRequest: featureRequestFor(t, s, "Improve cancellation."), Profile: "pi:lm-studio/local-model"}
	if _, err := s.GenerateFeatures(context.Background(), request); err == nil {
		t.Fatal("Pi dispatched without consent")
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatal("unconfirmed selection launched inference")
	}
	request.ConfirmRemoteProvider = true
	report, err := s.GenerateFeatures(context.Background(), request)
	if err != nil || report.Status != "ready" || len(report.Suggestions) != 1 || report.Generations[0].Model == nil || report.Generations[0].Model.Model != "lm-studio/local-model" {
		t.Fatalf("Pi feature generation = %+v, %v", report, err)
	}
	excludeFeatureAnalysisFiles(t, s, []string{"main.go"})
	models := &AnalysisModels{Code: "bug", Review: "analyze", Features: request.Profile}
	preview := analysisModelsPreviewFor(t, s, models, AnalysisRunLimits{100, 30, 2})
	available = nil
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("removed Pi model admitted: %v", err)
	}
	if run, err := s.ReadAnalysisRun(context.Background(), preview.Identity.ProjectID, preview.Identity.ProjectRevision); err != nil || run != nil {
		t.Fatal("invalid selection created a run")
	}
	count, err := os.ReadFile(calls)
	if err != nil || string(count) != "x" {
		t.Fatalf("unexpected inference calls: %q, %v", count, err)
	}
}
