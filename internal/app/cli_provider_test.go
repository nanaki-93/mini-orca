package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestCLIProvidersAnalyzeAndProposeDraftsWithScopeConsent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CLI providers require process-group cleanup")
	}
	for _, provider := range []config.ModelProvider{config.AgyProvider, config.PiProvider} {
		t.Run(string(provider), func(t *testing.T) {
			fixture, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			original, err := os.ReadFile(filepath.Join(root, "main.go"))
			if err != nil {
				t.Fatal(err)
			}
			cfg := config.Default()
			cfg.Retry = config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1}
			calls := filepath.Join(t.TempDir(), "calls")
			cfg.ModelScopes = config.ModelScopesConfig{
				Analyze:  cliAppProfile(t, provider, "analyze-model", `{"purpose":"Explains the fixture.","architecture":"One package.","components":[],"entry_points":[],"flows":[],"risks":[],"next_steps":[],"engineering_insight":null}`, calls),
				Bug:      cliAppProfile(t, provider, "bug-model", strings.TrimSuffix(validSemanticAnalysis, "}")+`,"engineering_insight":null}`, calls),
				Function: cliAppProfile(t, provider, "function-model", `{"version":"v1","declaration":"func Run() { fmt.Println(\"updated\") }","explanation":"Updates the message."}`, calls),
			}
			service, err := New(cfg, fixture.manager)
			if err != nil {
				t.Fatal(err)
			}
			for _, scope := range []config.ModelScope{config.AnalyzeModelScope, config.BugModelScope, config.FunctionModelScope} {
				if service.RequireRemoteConfirmation(scope, false) == nil {
					t.Fatalf("%s was not gated", scope)
				}
			}
			preview, err := service.AnalysisContextManifest("main.go")
			if err != nil || preview.ProviderOrigin != "cli://"+string(provider) || !preview.RemoteProvider {
				t.Fatalf("context preview = %+v, %v", preview, err)
			}
			if _, err := os.Stat(calls); !os.IsNotExist(err) {
				t.Fatal("preview or catalog launched a process")
			}
			if _, err := service.AnalyzeFile(context.Background(), "main.go", false, false); err == nil {
				t.Fatal("analysis dispatched without consent")
			}
			if _, err := os.Stat(calls); !os.IsNotExist(err) {
				t.Fatal("unconfirmed analysis launched a process")
			}
			analysis, err := service.AnalyzeProject(context.Background(), root)
			if err != nil || analysis.Report.Status != project.ProjectAnalysisStatusFresh {
				t.Fatalf("project analysis = %+v, %v", analysis, err)
			}
			if err := service.manager.Set(root, analysis); err != nil {
				t.Fatal(err)
			}
			report, err := service.AnalyzeFile(context.Background(), "main.go", false, true)
			if err != nil || report.Status != project.AnalysisStatusFresh || report.ProviderOrigin != "cli://"+string(provider) {
				t.Fatalf("file analysis = %+v, %v", report, err)
			}
			session := openFixtureChatSession(t, service, project.DeclarationEditReplaceSymbol, "Run")
			request := ChatSessionMessageRequest{SessionID: session.ID, Message: "Update the message."}
			if _, err := service.SendChatSessionMessage(context.Background(), request); err == nil {
				t.Fatal("proposal dispatched without Function consent")
			}
			request.ConfirmRemoteProvider = true
			proposal, err := service.SendChatSessionMessage(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if proposal.Draft.ID == "" || !strings.Contains(proposal.Draft.Declaration, "updated") || proposal.Draft.EffectiveModel.ProviderOrigin != "cli://"+string(provider) {
				t.Fatalf("proposal = %+v", proposal)
			}
			current, err := os.ReadFile(filepath.Join(root, "main.go"))
			if err != nil || string(current) != string(original) {
				t.Fatal("CLI proposal changed source before Apply")
			}
			count, err := os.ReadFile(calls)
			if err != nil || string(count) != "xxx" {
				t.Fatalf("CLI dispatches = %q, %v", count, err)
			}
		})
	}
}

func TestAgyGeneratesFeaturesWithoutNativeToolCalls(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CLI providers require process-group cleanup")
	}
	for _, analysis := range []bool{false, true} {
		name, content := "direct suggestions", validFeatureResponse
		if analysis {
			name, content = "analysis stage", `{"suggestions":[]}`
		}
		t.Run(name, func(t *testing.T) {
			fixture, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			original, err := os.ReadFile(filepath.Join(root, "main.go"))
			if err != nil {
				t.Fatal(err)
			}
			cfg := config.Default()
			cfg.Retry = config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1}
			calls := filepath.Join(t.TempDir(), "calls")
			cfg.ModelScopes.Analyze = cliAppProfile(t, config.AgyProvider, "analyze-model", content, calls)
			service, err := New(cfg, fixture.manager)
			if err != nil {
				t.Fatal(err)
			}
			if analysis {
				excludeFeatureAnalysisFiles(t, service, []string{"main.go"})
				preview := featureAnalysisPreviewFor(t, service, AnalysisRunLimits{1, 30, 2}, nil)
				if _, err := service.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
					t.Fatal(err)
				}
				run := completedAnalysisRun(t, service)
				if run.Status != AnalysisRunCompletedEmpty || run.Features.Status != AnalysisStageCompletedEmpty || run.Features.Attempts != 1 {
					t.Fatalf("feature stage did not complete: status=%s features=%+v", run.Status, run.Features)
				}
			} else {
				request := featureRequestFor(t, service, "Improve cancellation.")
				request.ConfirmRemoteProvider = true
				report, err := service.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: request})
				if err != nil || report.Status != "ready" || len(report.Suggestions) != 1 {
					t.Fatalf("feature suggestions = %+v, %v", report, err)
				}
			}
			report, err := service.Features(context.Background())
			if err != nil || report.Status != "ready" || report.Failure != "" {
				t.Fatalf("saved feature report = %+v, %v", report, err)
			}
			current, err := os.ReadFile(filepath.Join(root, "main.go"))
			if err != nil || string(current) != string(original) {
				t.Fatal("feature generation changed project source")
			}
			count, err := os.ReadFile(calls)
			if err != nil || string(count) != "x" {
				t.Fatalf("CLI dispatches = %q, %v", count, err)
			}
		})
	}
}

func cliAppProfile(t *testing.T, provider config.ModelProvider, model, content, calls string) config.ModelProfileConfig {
	t.Helper()
	var output string
	if provider == config.PiProvider {
		message, err := json.Marshal(map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "model": model, "stopReason": "stop", "content": []map[string]string{{"type": "text", "text": content}}}})
		if err != nil {
			t.Fatal(err)
		}
		output = "{\"type\":\"agent_start\"}\n" +
			`{"type":"message_end","message":{"role":"system","content":"","sections":{"preamble":"Only explain the supplied source."}}}` + "\n" +
			string(message) + "\n{\"type\":\"agent_end\"}\n"
	} else {
		init, _ := json.Marshal(map[string]any{"event": "init", "init": map[string]any{"agent": "mini-orca", "model": model, "tools": []string{"view_file", "run_command", "finish"}}})
		result, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": content}})
		output = string(init) + "\n" + string(result) + "\n"
	}
	path := filepath.Join(t.TempDir(), "fake-agent")
	script := "#!/bin/sh\ncat >/dev/null\nprintf x >> '" + strings.ReplaceAll(calls, "'", "'\\''") + "'\n"
	if provider == config.AgyProvider {
		// Reproduce installations where native schema completion is blocked by
		// a tool hook. A final JSON response must not depend on finish being allowed.
		script += "for arg in \"$@\"; do\nif [ \"$arg\" = --json-schema ]; then\nexit 1\nfi\ndone\n"
	}
	script += "cat <<'MINI_ORCA_RESPONSE'\n" + output + "MINI_ORCA_RESPONSE\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return config.ModelProfileConfig{Provider: provider, CLIPath: path, Model: model}
}
