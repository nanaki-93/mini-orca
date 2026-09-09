package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Live evaluation returned parameter and field names as declaration explanations.
// Omit that optional section without losing a valid parent summary.
func TestSemanticAnalysisOmitsNonDeclarationExplanations(t *testing.T) {
	target := project.IndexFile{Path: "main.go", Symbols: []project.SymbolInfo{{Name: "Service.Charge"}}}
	for _, name := range []string{"ctx", "orders", "gateway"} {
		output := `{"purpose":"Delegates work.","symbol_explanations":{"Service.Charge":"Delegates work.","` + name + `":"Referenced value."}}`
		parsed, err := parseSemanticAnalysis(output, target, "")
		if err != nil || parsed.Purpose != "Delegates work." || len(parsed.SymbolExplanations) != 0 {
			t.Fatalf("invalid optional section %q was not isolated: %+v, %v", name, parsed, err)
		}
	}
	for _, name := range []string{"Service.Charge", "Charge"} {
		output := `{"purpose":"Delegates work.","symbol_explanations":{"` + name + `":"Delegates work."}}`
		if _, err := parseSemanticAnalysis(output, target, ""); err != nil {
			t.Fatalf("rejected known declaration %q: %v", name, err)
		}
	}
	for _, output := range []string{
		`{"purpose":"Delegates work.","symbol_explanations":{"ctx":"Parameter."},"risks":[{"severity":"invented","summary":"Invalid risk."}]}`,
		`{"purpose":"","symbol_explanations":{"ctx":"Parameter."}}`,
		`{"purpose":"Delegates work.","symbol_explanations":{"ctx":"Parameter."},"unexpected":true}`,
	} {
		if _, err := parseSemanticAnalysis(output, target, ""); err == nil {
			t.Fatal("optional explanation omission accepted an invalid parent")
		}
	}
}

func TestAnalyzeFileCachesStructuredOneFileSummary(t *testing.T) {
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		var gotSchema, wantSchema any
		if request.ResponseFormat == nil || request.ResponseFormat.Type != "json_schema" || request.ResponseFormat.JSONSchema == nil || !request.ResponseFormat.JSONSchema.Strict || request.ResponseFormat.JSONSchema.Name != fileAnalysisResponseSchemaName || json.Unmarshal(request.ResponseFormat.JSONSchema.Schema, &gotSchema) != nil || json.Unmarshal([]byte(fileAnalysisResponseSchemaDocument), &wantSchema) != nil || !reflect.DeepEqual(gotSchema, wantSchema) {
			t.Fatalf("file analysis response format = %+v", request.ResponseFormat)
		}
		prompt = request.Messages[0].Content
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "fixture-model", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Runs the selected command.","responsibilities":["dispatches work"],"dependencies":["fmt"],"side_effects":["writes stdout"],"risks":[{"severity":"High","summary":"No input validation."}],"suggestions":[{"title":"Validate input","summary":"Reject blank names.","target_symbol":"Run","action":"fix"}],"symbol_explanations":{"Run":"Dispatches the command."}}`}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	result, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != project.AnalysisStatusFresh || result.Purpose == "" || len(result.Symbols) != 1 || result.SymbolExplanations["Run"] == "" || result.Model != "fixture-model" || len(result.Risks) != 1 || result.Risks[0].Severity != "high" {
		t.Fatalf("analysis = %+v", result)
	}
	if strings.Contains(prompt, "helper secret") || !strings.Contains(prompt, "func Run") || !strings.Contains(prompt, "TARGET_SOURCE (the only source content supplied)") || !strings.Contains(prompt, "target_path copied exactly") || !strings.Contains(prompt, "target_symbol copied exactly") || !strings.Contains(prompt, "target_signature copied exactly") || !strings.Contains(prompt, "Do not use a symbol field") || !strings.Contains(prompt, "Ground selected-file behavior and engineering insights in TARGET_SOURCE") || !strings.Contains(prompt, "Default to tests that characterize current behavior") {
		t.Fatalf("semantic prompt was not one-file scoped: %s", prompt)
	}
	if _, err := os.Stat(filepath.Join(root, ".mini-orca", "file-analysis")); err != nil {
		t.Fatalf("analysis cache not written: %v", err)
	}
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	if file := findAppIndexFile(index, "main.go"); file == nil || file.AnalysisStatus != project.AnalysisStatusFresh {
		t.Fatalf("index analysis status = %+v, want fresh", file)
	}
	second, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil || second.Status != project.AnalysisStatusFresh {
		t.Fatalf("cached analysis = %+v, %v", second, err)
	}
}

func TestFileAnalysisResponseSchemaRejectsInvalidOptionalObjects(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	document, err := jsonschema.UnmarshalJSON(strings.NewReader(fileAnalysisResponseSchemaDocument))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.AddResource("file-analysis-schema.json", document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("file-analysis-schema.json")
	if err != nil {
		t.Fatal(err)
	}
	valid := `{"purpose":"Summarizes one file.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[{"severity":"low","summary":"Conditional concern.","task_spec":{"schema_version":"1","target_path":"main.go","target_symbol":"Run","target_signature":"func Run()","acceptance_criteria":["Keep behavior."],"non_goals":[]},"engineering_insight":null}],"suggestions":[{"title":"Clarify behavior","summary":"Keep the call explicit.","action":"Use \\\"quoted\\\" text.","engineering_insight":{"mechanism":"Run calls one helper.","why_it_matters_here":"Run is the selected entry point.","tradeoff_or_failure_mode":"Changing call order can alter behavior.","transferable_lesson":"Exercise Run and verify call order."}}],"symbol_explanations":{"Run":"Runs the selected operation."}}`
	invalid := map[string]string{
		"scalar insight":        strings.Replace(valid, `"engineering_insight":null`, `"engineering_insight":"advice"`, 1),
		"partial insight":       strings.Replace(valid, `"mechanism":"Run calls one helper.","why_it_matters_here":"Run is the selected entry point.","tradeoff_or_failure_mode":"Changing call order can alter behavior.","transferable_lesson":"Exercise Run and verify call order."`, `"mechanism":"partial"`, 1),
		"wrong nested type":     strings.Replace(valid, `"action":"Use \\\"quoted\\\" text."`, `"action":false`, 1),
		"extra nested property": strings.Replace(valid, `"transferable_lesson":"Exercise Run and verify call order."`, `"transferable_lesson":"Exercise Run and verify call order.","extra":"no"`, 1),
		"empty explanation":     strings.Replace(valid, `"Run":"Runs the selected operation."`, `"Run":""`, 1),
	}
	if instance, err := jsonschema.UnmarshalJSON(strings.NewReader(valid)); err != nil || schema.Validate(instance) != nil {
		t.Fatalf("valid file-analysis schema instance rejected: %v", err)
	}
	for name, value := range invalid {
		t.Run(name, func(t *testing.T) {
			instance, err := jsonschema.UnmarshalJSON(strings.NewReader(value))
			if err != nil || schema.Validate(instance) == nil {
				t.Fatalf("invalid schema instance accepted: decode=%v", err)
			}
		})
	}
}

func TestFileAnalysisResponseSchemaBoundsInsightFieldsAtEveryLocation(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	document, err := jsonschema.UnmarshalJSON(strings.NewReader(fileAnalysisResponseSchemaDocument))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.AddResource("file-analysis-schema.json", document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("file-analysis-schema.json")
	if err != nil {
		t.Fatal(err)
	}

	insight := func(length int) string {
		field := strings.Repeat("界", length)
		return `{"mechanism":"` + field + `","why_it_matters_here":"` + field + `","tradeoff_or_failure_mode":"` + field + `","transferable_lesson":"` + field + `"}`
	}
	response := func(topLevel, risk, suggestion string) string {
		return `{"purpose":"Summarizes one file.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[{"severity":"low","summary":"Conditional concern.","engineering_insight":` + risk + `}],"suggestions":[{"title":"Clarify behavior","summary":"Keep the call explicit.","engineering_insight":` + suggestion + `}],"symbol_explanations":{},"engineering_insight":` + topLevel + `}`
	}

	atLimit := insight(fileAnalysisInsightMaxChars)
	overLimit := insight(fileAnalysisInsightMaxChars + 1)
	for _, test := range []struct {
		name     string
		response string
		valid    bool
	}{
		{name: "exact Unicode boundary", response: response(atLimit, atLimit, atLimit), valid: true},
		{name: "top level exceeds boundary", response: response(overLimit, atLimit, atLimit)},
		{name: "risk exceeds boundary", response: response(atLimit, overLimit, atLimit)},
		{name: "suggestion exceeds boundary", response: response(atLimit, atLimit, overLimit)},
	} {
		t.Run(test.name, func(t *testing.T) {
			instance, err := jsonschema.UnmarshalJSON(strings.NewReader(test.response))
			if err != nil {
				t.Fatal(err)
			}
			if gotValid := schema.Validate(instance) == nil; gotValid != test.valid {
				t.Fatalf("schema validation = %t, want %t", gotValid, test.valid)
			}
		})
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(fileAnalysisResponseSchemaDocument), &raw); err != nil {
		t.Fatal(err)
	}
	defs := raw["$defs"].(map[string]any)
	insightText := defs["insightText"].(map[string]any)
	ceiling := int(insightText["maxLength"].(float64))
	if ceiling != fileAnalysisInsightMaxChars || ceiling*fileAnalysisInsightFieldCount != project.MaxEngineeringInsightRunes {
		t.Fatalf("insight schema ceiling = %d; aggregate = %d", ceiling, project.MaxEngineeringInsightRunes)
	}
	properties := defs["insight"].(map[string]any)["properties"].(map[string]any)
	for _, field := range []string{"mechanism", "why_it_matters_here", "tradeoff_or_failure_mode", "transferable_lesson"} {
		if reference := properties[field].(map[string]any)["$ref"]; reference != "#/$defs/insightText" {
			t.Fatalf("%s schema reference = %q", field, reference)
		}
	}
}

func TestFileAnalysisOverLimitInsightsPreserveParents(t *testing.T) {
	field := strings.Repeat("界", fileAnalysisInsightMaxChars+1)
	overAggregateLimit := `{"mechanism":"` + field + `","why_it_matters_here":"` + field + `","tradeoff_or_failure_mode":"` + field + `","transferable_lesson":"` + field + `"}`
	output := `{"purpose":"Summarizes one file.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[{"severity":"low","summary":"Conditional concern.","engineering_insight":` + overAggregateLimit + `}],"suggestions":[{"title":"Clarify behavior","summary":"Keep the call explicit.","engineering_insight":` + overAggregateLimit + `}],"symbol_explanations":{},"engineering_insight":` + overAggregateLimit + `}`

	parsed, err := parseSemanticAnalysis(output, project.IndexFile{Path: "main.go"}, "package main")
	if err != nil || parsed.Purpose != "Summarizes one file." || len(parsed.Risks) != 1 || parsed.Risks[0].Summary != "Conditional concern." || len(parsed.Suggestions) != 1 || parsed.Suggestions[0].Title != "Clarify behavior" {
		t.Fatalf("over-limit insight changed parent data: %+v, %v", parsed, err)
	}
	if parsed.EngineeringInsight != nil || parsed.Risks[0].EngineeringInsight != nil || parsed.Suggestions[0].EngineeringInsight != nil {
		t.Fatalf("over-limit insights were retained: %+v", parsed)
	}
}

func TestAnalyzeFileDoesNotRetryUnsupportedStructuredFormat(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	service.runtimes.bug.effective.MaxRetries = 3
	result, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil || result.Status != project.AnalysisStatusFailed || requests != 1 {
		t.Fatalf("unsupported structured format result=%+v err=%v requests=%d", result, err, requests)
	}
}

func TestParseSemanticAnalysisRejectsInvalidJSONEscape(t *testing.T) {
	output := `{"purpose":"Summary.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[{"title":"Suggestion","summary":"Summary","action":"bad\q"}],"symbol_explanations":{}}`
	if _, err := parseSemanticAnalysis(output, project.IndexFile{Path: "main.go"}, "package main\n"); err == nil {
		t.Fatal("invalid JSON escape was accepted")
	}
}

func TestCachedFileAnalysisMarksV11PromptResultsStale(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	prepared, err := service.prepareFileAnalysis("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := EngineeringInsightPromptVersion(); got != "file-analysis-v12" {
		t.Fatalf("file analysis prompt version = %q, want file-analysis-v12", got)
	}
	legacy := project.FileAnalysis{
		SchemaVersion: "1", ProjectID: prepared.input.ProjectID, ProjectRevision: prepared.input.ProjectRevision,
		Path: prepared.input.Path, ContentHash: prepared.input.ContentHash, Language: prepared.input.Language,
		Status: project.AnalysisStatusFresh, Model: prepared.input.Model, ConfiguredModel: prepared.input.Model,
		Profile: prepared.input.Profile, Scope: prepared.input.Scope, ProviderOrigin: prepared.input.ProviderOrigin,
		ReasoningEffort: prepared.input.ReasoningEffort, PromptVersion: "file-analysis-v11", ContextPolicyVersion: prepared.input.ContextPolicyVersion,
	}
	if err := prepared.cache.Store(legacy); err != nil {
		t.Fatal(err)
	}
	cached, err := service.CachedFileAnalysis("main.go")
	if err != nil || cached.Status != project.AnalysisStatusStale {
		t.Fatalf("v11 cache = %+v, %v; want stale", cached, err)
	}
}

func TestAnalyzeFileRequiresRemoteConfirmationBeforeSendingPrompt(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
	}))
	defer server.Close()

	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	service.runtimes.bug.effective.RemoteProvider = true
	if _, err := service.AnalyzeFile(context.Background(), "main.go", false, false); err == nil || !strings.Contains(err.Error(), "explicit confirmation") {
		t.Fatalf("declined remote confirmation error = %v", err)
	}
	if requests != 0 {
		t.Fatalf("declined confirmation sent %d provider requests", requests)
	}
	if _, err := service.AnalyzeFile(context.Background(), "main.go", false, true); err != nil {
		t.Fatalf("confirmed analysis error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("confirmed request count = %d, want 1", requests)
	}
}

func TestAnalyzeFileAcceptsUnqualifiedMethodExplanations(t *testing.T) {
	output := `{"purpose":"Explains the service.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{"FileSystemService":"The service implementation.","PrintFiles":"Prints files.","ListFiles":"Lists files."}}`
	parsed, err := parseSemanticAnalysis(output, project.IndexFile{Path: "service.go", Language: "Go", Symbols: []project.SymbolInfo{
		{Name: "FileSystemService"},
		{Name: "FileSystemService.PrintFiles"},
		{Name: "FileSystemService.ListFiles"},
	}}, "package service")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.SymbolExplanations["FileSystemService.PrintFiles"] != "Prints files." || parsed.SymbolExplanations["FileSystemService.ListFiles"] != "Lists files." {
		t.Fatalf("normalized symbol explanations = %+v", parsed.SymbolExplanations)
	}
	if _, exists := parsed.SymbolExplanations["PrintFiles"]; exists {
		t.Fatalf("unqualified symbol explanation was not normalized: %+v", parsed.SymbolExplanations)
	}
}

func TestAnalyzeFileValidatesAndCachesOneExactBugTaskWithoutAnotherModelCall(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Explains the file.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[{"severity":"high","summary":"Run ignores an error.","task_spec":{"schema_version":"1","target_path":"main.go","target_symbol":"Run","target_signature":"hallucinated","acceptance_criteria":["Return the formatting error to the caller."],"non_goals":["Do not edit helper files."],"go_test_candidate":{"name":"TestRunReturnsError","content":"package main\n\nimport \"testing\"\n\nfunc TestRunReturnsError(t *testing.T) {}\n"}}}],"suggestions":[],"symbol_explanations":{}}`}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)

	first, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil {
		t.Fatal(err)
	}
	task := first.Risks[0].TaskSpec
	if task == nil || task.TargetPath != "main.go" || task.TargetSymbol != "Run" || task.TargetSignature != "func()" || task.GoTestCandidate == nil {
		t.Fatalf("validated task = %+v", task)
	}
	content, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(content) != "package main\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"run\") }\n" {
		t.Fatalf("analysis changed imported source: %q, %v", content, err)
	}
	second, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil || second.Risks[0].TaskSpec == nil || calls != 1 {
		t.Fatalf("cached task = %+v, %v; calls = %d", second, err, calls)
	}
}

func TestSemanticBugTaskDropsUntrustedTargetsAndInvalidOptionalTests(t *testing.T) {
	target := project.IndexFile{Path: "main.go", Language: "Go", Symbols: []project.SymbolInfo{{Name: "Run", Signature: "func Run()", Confidence: "exact", AtomicTarget: true}}}
	valid := func(task string) string {
		return `{"purpose":"Explains.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[{"severity":"high","summary":"Risk.","task_spec":` + task + `}],"suggestions":[],"symbol_explanations":{}}`
	}
	spec := func(path, symbol string) string {
		return `{"schema_version":"1","target_path":"` + path + `","target_symbol":"` + symbol + `","acceptance_criteria":["Handle the error."],"non_goals":[]}`
	}
	for _, output := range []string{
		valid(spec("other.go", "Run")),
		valid(spec("main.go", "Unknown")),
		valid(`{"schema_version":"1","target_path":"main.go","target_symbol":"Run","acceptance_criteria":["` + strings.Repeat("x", project.MaxBugTaskItemBytes+1) + `"],"non_goals":[]}`),
		valid(`{"schema_version":"1","symbol":"Run","acceptance_criteria":["Handle the error."],"non_goals":[]}`),
	} {
		parsed, err := parseSemanticAnalysis(output, target, "package main\nfunc Run() {}")
		if err != nil || parsed.Risks[0].TaskSpec != nil {
			t.Fatalf("untrusted task was retained: %+v, %v", parsed, err)
		}
	}
	approximate := target
	approximate.Symbols = []project.SymbolInfo{{Name: "Run", Signature: "func Run()", Confidence: "approximate", AtomicTarget: true}}
	parsed, err := parseSemanticAnalysis(valid(spec("main.go", "Run")), approximate, "package main\nfunc Run() {}")
	if err != nil || parsed.Risks[0].TaskSpec != nil {
		t.Fatalf("approximate target was retained: %+v, %v", parsed, err)
	}
	nonAtomic := target
	nonAtomic.Symbols = append([]project.SymbolInfo(nil), target.Symbols...)
	nonAtomic.Symbols[0].AtomicTarget = false
	parsed, err = parseSemanticAnalysis(valid(spec("main.go", "Run")), nonAtomic, "package main\nfunc Run() {}")
	if err != nil || parsed.Risks[0].TaskSpec != nil {
		t.Fatalf("non-atomic target was retained: %+v, %v", parsed, err)
	}
	duplicate := target
	duplicate.Symbols = append(duplicate.Symbols, duplicate.Symbols[0])
	parsed, err = parseSemanticAnalysis(valid(spec("main.go", "Run")), duplicate, "package main\nfunc Run() {}")
	if err != nil || parsed.Risks[0].TaskSpec != nil {
		t.Fatalf("duplicate target was retained: %+v, %v", parsed, err)
	}

	for _, candidate := range []string{
		`{"name":"TestRun","content":"package other\nfunc TestRun() {}"}`,
		`{"name":"TestRun","content":"package main\nfunc TestRun( {"}`,
		`{"name":"NotATest","content":"package main\nfunc NotATest() {}"}`,
	} {
		output := valid(`{"schema_version":"1","target_path":"main.go","target_symbol":"Run","acceptance_criteria":["Handle the error."],"non_goals":[],"go_test_candidate":` + candidate + `}`)
		parsed, err := parseSemanticAnalysis(output, target, "package main\nfunc Run() {}")
		if err != nil || parsed.Risks[0].TaskSpec == nil || parsed.Risks[0].TaskSpec.GoTestCandidate != nil {
			t.Fatalf("invalid optional test = %+v, %v", parsed.Risks[0].TaskSpec, err)
		}
	}
}

func TestAnalyzeAllRetriesFailedFiles(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		callCount++
		call := callCount
		mu.Unlock()
		output := validSemanticAnalysis
		if call == 1 {
			output = "not-json"
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: output}}}})
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	failed, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil || failed.Status != project.AnalysisStatusFailed {
		t.Fatalf("initial analysis = %+v, %v", failed, err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1, MaxRetries: 0}, false); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, service, analysisAllStateCompleted)
	if len(job.Files) != 1 || job.Files[0].Path != "main.go" || job.Files[0].Status != analysisAllFileCompleted {
		t.Fatalf("retry job = %+v", job)
	}
	result, err := service.CachedFileAnalysis("main.go")
	if err != nil || result.Status != project.AnalysisStatusFresh {
		t.Fatalf("retried analysis = %+v, %v", result, err)
	}
	mu.Lock()
	gotCalls := callCount
	mu.Unlock()
	if gotCalls != 2 {
		t.Fatalf("model call count = %d, want initial failure plus retry", gotCalls)
	}
}

func TestAnalyzeFileStoresActionableMalformedAndEmptyFailures(t *testing.T) {
	for _, output := range []string{"not-json", ""} {
		t.Run("output", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: output}}}})
			}))
			defer server.Close()
			service, _ := newSemanticAnalysisService(t, server.URL, 0)
			result, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
			if err != nil || result.Status != project.AnalysisStatusFailed || result.Failure == "" {
				t.Fatalf("malformed result = %+v, %v", result, err)
			}
			index, err := service.manager.Index()
			if err != nil {
				t.Fatal(err)
			}
			if file := findAppIndexFile(index, "main.go"); file == nil || file.AnalysisStatus != project.AnalysisStatusFailed {
				t.Fatalf("index analysis status = %+v, want failed", file)
			}
		})
	}
}

func TestAnalyzeFileHonorsTimeout(t *testing.T) {
	handlerCanceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
		handlerCanceled <- struct{}{}
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 1)
	_, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("analysis error = %v, want deadline exceeded", err)
	}
	waitForTestSignal(t, handlerCanceled, "analysis provider cancellation")
}

func TestAnalyzeAllDoesNotRetryTimedOutFile(t *testing.T) {
	providerCanceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
		providerCanceled <- struct{}{}
	}))
	defer server.Close()

	service, _ := newSemanticAnalysisService(t, server.URL, 1)
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1, MaxRetries: 3}, false); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, service, analysisAllStateCompleted)
	if len(job.Files) != 1 || job.Files[0].Status != analysisAllFileFailed || job.Files[0].Attempts != 1 || job.Files[0].Error != analysisAllTimeoutError {
		t.Fatalf("timeout job = %+v", job)
	}
	waitForTestSignal(t, providerCanceled, "timed-out analysis provider cancellation")
}

func TestAnalyzeAllProcessesEligibleFilesSequentially(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	handlerErrors := make(chan error, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, err := analyzeAllRequestPath(r)
		if err != nil {
			handlerErrors <- err
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		paths = append(paths, path)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "second.go"), []byte("package main\nfunc Second() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 2}, false); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, service, analysisAllStateCompleted)
	if len(job.Files) != 2 || job.Files[0].Status != analysisAllFileCompleted || job.Files[1].Status != analysisAllFileCompleted {
		t.Fatalf("completed job = %+v", job)
	}
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"main.go", "second.go"} {
		if file := findAppIndexFile(index, path); file == nil || file.AnalysisStatus != project.AnalysisStatusFresh {
			t.Fatalf("index analysis status for %s = %+v, want fresh", path, file)
		}
	}
	mu.Lock()
	got := append([]string(nil), paths...)
	mu.Unlock()
	if want := []string{"main.go", "second.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("request order = %v, want %v", got, want)
	}
	assertNoTestHandlerError(t, handlerErrors)
}

func TestAnalyzeAllSkipsPlainTextAndMarkdownFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "NOTICE"), []byte("Notice\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index, err := service.Reindex()
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := service.selectAnalyzeAllCandidates(analysis, index, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Path != "main.go" {
		t.Fatalf("analysis candidates = %+v", candidates)
	}
}

func TestStartAnalyzeAllCompletesEmptyEligibleSelection(t *testing.T) {
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requested <- struct{}{}
	}))
	defer server.Close()

	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}

	job, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != analysisAllStateRunning || len(job.Files) != 0 || job.MaxFiles != defaultAnalyzeAllFileLimit || job.MaxRetries != defaultAnalyzeAllRetries {
		t.Fatalf("empty selection job = %+v", job)
	}
	waitForAnalyzeAll(t, service, analysisAllStateCompleted)
	select {
	case <-requested:
		t.Fatal("analyze-all requested a provider for an empty eligible selection")
	default:
	}
}

func TestStartAnalyzeAllReturnsActiveJobWithoutStartingAnotherWorker(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseProvider := func() { releaseOnce.Do(func() { close(release) }) }
	responded := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		<-release
		responded <- struct{}{}
	}))
	defer server.Close()
	defer releaseProvider()

	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	first, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "first analyze-all request")
	second, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	if second.ProjectID != first.ProjectID || second.ProjectRevision != first.ProjectRevision || !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("concurrent start returned another job: first=%+v second=%+v", first, second)
	}
	if _, err := service.CancelAnalyzeAll(); err != nil {
		t.Fatal(err)
	}
	releaseProvider()
	waitForTestSignal(t, responded, "active analyze-all response")
}

func TestStartAnalyzeAllReturnsPausedJobWithoutStartingAnotherWorker(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseProvider := func() { releaseOnce.Do(func() { close(release) }) }
	responded := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		<-release
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
		responded <- struct{}{}
	}))
	defer server.Close()
	defer releaseProvider()

	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	first, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "first analyze-all request")
	if _, err := service.PauseAnalyzeAll(); err != nil {
		t.Fatal(err)
	}
	second, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != analysisAllStatePaused || second.ProjectID != first.ProjectID || second.ProjectRevision != first.ProjectRevision || !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("concurrent paused start returned another job: first=%+v second=%+v", first, second)
	}
	releaseProvider()
	waitForTestSignal(t, responded, "paused analyze-all response")
	paused := waitForAnalyzeAllFile(t, service, analysisAllStatePaused, analysisAllFileCompleted)
	if paused.Files[0].Status != analysisAllFileCompleted {
		t.Fatalf("paused job did not retain its completed file: %+v", paused)
	}
}

func TestStartAnalyzeAllRollsBackControllerWhenPersistenceFails(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseProvider := func() { releaseOnce.Do(func() { close(release) }) }
	responded := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		<-release
		responded <- struct{}{}
	}))
	defer server.Close()
	defer releaseProvider()

	service, root := newSemanticAnalysisService(t, server.URL, 0)
	sessionPath := filepath.Join(root, ".mini-orca", "sessions")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false); err == nil {
		t.Fatal("analyze-all started despite persistence failure")
	}
	if err := os.Remove(sessionPath); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false); err != nil {
		t.Fatalf("analyze-all did not recover after persistence failure: %v", err)
	}
	waitForTestSignal(t, started, "analyze-all request after persistence recovery")
	if _, err := service.CancelAnalyzeAll(); err != nil {
		t.Fatal(err)
	}
	releaseProvider()
	waitForTestSignal(t, responded, "recovered analyze-all response")
}

func TestAnalyzeAllActivationRejectsPerformanceStartedAfterSelection(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseProvider := func() { releaseOnce.Do(func() { close(release) }) }
	responded := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		<-release
		responded <- struct{}{}
	}))
	defer server.Close()
	defer releaseProvider()

	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	input, err := service.prepareAnalyzeAllStart(AnalyzeAllOptions{MaxFiles: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	files, err := service.selectAnalyzeAllCandidates(input.analysis, input.index, input.options.MaxFiles)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 1, QueueID: preview.QueueID, PolicyFingerprint: preview.PolicyFingerprint}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "performance review request")
	if _, err := service.activateAnalyzeAll(input, files); err == nil {
		t.Fatal("analyze-all activated after performance started")
	}
	if active, err := service.AnalyzeAllJob(); err != nil || active != nil {
		t.Fatalf("analyze-all job after rejected activation = %+v, %v", active, err)
	}
	if _, err := service.CancelPerformanceJob(); err != nil {
		t.Fatal(err)
	}
	releaseProvider()
	waitForTestSignal(t, responded, "canceled performance response")
}

func TestAnalyzeAllActivationRejectsProjectChangedAfterSelection(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	input, err := service.prepareAnalyzeAllStart(AnalyzeAllOptions{MaxFiles: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	files, err := service.selectAnalyzeAllCandidates(input.analysis, input.index, input.options.MaxFiles)
	if err != nil {
		t.Fatal(err)
	}

	newRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(newRoot, "next.go"), []byte("package next\nfunc Next() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := service.ActivateProject(newRoot, &project.Analysis{Name: "next", Path: newRoot}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.activateAnalyzeAll(input, files); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("activation after project change = %v, want revision conflict", err)
	}
	if active, err := service.AnalyzeAllJob(); err != nil || active != nil {
		t.Fatalf("new project analyze-all job = %+v, %v", active, err)
	}
	for _, projectRoot := range []string{root, newRoot} {
		if _, err := os.Stat(filepath.Join(projectRoot, ".mini-orca", "sessions", "analyze-all.json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("analyze-all record in %s = %v, want absent", projectRoot, err)
		}
	}
}

func TestAnalyzeAllActivationCannotCrossProjectReplacement(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	input, err := service.prepareAnalyzeAllStart(AnalyzeAllOptions{MaxFiles: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	files, err := service.selectAnalyzeAllCandidates(input.analysis, input.index, input.options.MaxFiles)
	if err != nil {
		t.Fatal(err)
	}
	newRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(newRoot, "next.go"), []byte("package next\nfunc Next() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	transitionReady := make(chan struct{})
	allowTransition := make(chan struct{})
	var allowOnce sync.Once
	defer allowOnce.Do(func() { close(allowTransition) })
	transitionDone := make(chan error, 1)
	go func() {
		transitionDone <- service.replaceActiveProject(func() error {
			close(transitionReady)
			<-allowTransition
			return service.manager.Set(newRoot, &project.Analysis{Name: "next", Path: newRoot})
		})
	}()
	waitForTestSignal(t, transitionReady, "project replacement after invalidation")

	activationDone := make(chan error, 1)
	go func() {
		_, err := service.activateAnalyzeAll(input, files)
		activationDone <- err
	}()
	allowOnce.Do(func() { close(allowTransition) })
	if err := <-transitionDone; err != nil {
		t.Fatal(err)
	}
	if err := <-activationDone; !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("activation after replacement transition = %v, want revision conflict", err)
	}
	if active, err := service.AnalyzeAllJob(); err != nil || active != nil {
		t.Fatalf("new project analyze-all job = %+v, %v", active, err)
	}
	for _, projectRoot := range []string{root, newRoot} {
		if _, err := os.Stat(filepath.Join(projectRoot, ".mini-orca", "sessions", "analyze-all.json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("analyze-all record in %s = %v, want absent", projectRoot, err)
		}
	}
}

func TestAnalyzeAllStopsRetryingAfterRetryBudget(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: "not-json"}}}})
	}))
	defer server.Close()

	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1, MaxRetries: 1}, false); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, service, analysisAllStateCompleted)
	if len(job.Files) != 1 || job.Files[0].Status != analysisAllFileFailed || job.Files[0].Attempts != 2 || job.Files[0].Error != "analysis failed" {
		t.Fatalf("exhausted retry job = %+v", job)
	}
	mu.Lock()
	gotCalls := calls
	mu.Unlock()
	if gotCalls != 2 {
		t.Fatalf("analysis call count after retry budget = %d, want 2", gotCalls)
	}
}

func TestAnalyzeAllCancelRetainsCompletedEntries(t *testing.T) {
	firstDone := make(chan struct{}, 1)
	secondStarted := make(chan struct{}, 1)
	secondCanceled := make(chan struct{}, 1)
	handlerErrors := make(chan error, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, err := analyzeAllRequestPath(r)
		if err != nil {
			handlerErrors <- err
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if path == "main.go" {
			_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
			firstDone <- struct{}{}
			return
		}
		if path != "second.go" {
			handlerErrors <- fmt.Errorf("analysis target path = %q, want second.go", path)
			http.Error(w, "unexpected analysis target", http.StatusBadRequest)
			return
		}
		secondStarted <- struct{}{}
		<-r.Context().Done()
		secondCanceled <- struct{}{}
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "second.go"), []byte("package main\nfunc Second() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 2}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, firstDone, "first analysis completion")
	waitForTestSignal(t, secondStarted, "second analysis request")
	if _, err := service.CancelAnalyzeAll(); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, service, analysisAllStateCanceled)
	waitForTestSignal(t, secondCanceled, "second analysis cancellation")
	if job.Files[0].Status != analysisAllFileCompleted || job.Files[1].Status == analysisAllFileCompleted {
		t.Fatalf("canceled job should preserve only the completed cache entry: %+v", job)
	}
	analysis, err := service.CachedFileAnalysis("main.go")
	if err != nil || analysis.Status != project.AnalysisStatusFresh {
		t.Fatalf("completed cache entry = %+v, %v", analysis, err)
	}
	assertNoTestHandlerError(t, handlerErrors)
}

func TestAnalyzeAllPersistsPausedJobAndResumesAfterServiceRestart(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, err := analyzeAllRequestPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if path == "main.go" {
			started <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
	}))
	defer server.Close()
	defer releaseOnce.Do(func() { close(release) })
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "second.go"), []byte("package main\nfunc Second() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 2}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "first analysis request")
	if _, err := service.PauseAnalyzeAll(); err != nil {
		t.Fatal(err)
	}
	releaseOnce.Do(func() { close(release) })
	waitForAnalyzeAll(t, service, analysisAllStatePaused)

	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	cfg := scopedTestConfig(server.URL)
	cfg.Retry = config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1}
	restarted, err := New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.ResumeAnalyzeAll(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, restarted, analysisAllStateCompleted)
	if len(job.Files) != 2 || job.Files[1].Status != analysisAllFileCompleted {
		t.Fatalf("resumed job = %+v", job)
	}
}

func TestAnalyzeAllBecomesStaleWhenReindexChangesRevision(t *testing.T) {
	started := make(chan struct{}, 1)
	handlerCanceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		started <- struct{}{}
		<-r.Context().Done()
		handlerCanceled <- struct{}{}
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "analysis request")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() { println(\"changed\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, service, analysisAllStateStale)
	waitForTestSignal(t, handlerCanceled, "stale analysis cancellation")
	if job.Status != analysisAllStateStale {
		t.Fatalf("reindexed job = %+v", job)
	}
}

func TestAnalyzeAllBecomesStaleWhenProjectChanges(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseProvider := func() { releaseOnce.Do(func() { close(release) }) }
	responded := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		started <- struct{}{}
		<-release
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
		responded <- struct{}{}
	}))
	defer server.Close()
	defer releaseProvider()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "analysis request")
	newRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(newRoot, "next.go"), []byte("package next\nfunc Next() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := service.ActivateProject(newRoot, &project.Analysis{Name: "next", Path: newRoot}); err != nil {
		t.Fatal(err)
	}
	job, err := service.AnalyzeAllJob()
	if err != nil {
		t.Fatal(err)
	}
	if job == nil || job.Status != analysisAllStateStale {
		t.Fatalf("project-changed job = %+v", job)
	}
	persisted, err := loadAnalyzeAllJob(root)
	if err != nil || persisted == nil || persisted.Status != analysisAllStateStale {
		t.Fatalf("project-changed persisted job = %+v, %v", persisted, err)
	}
	releaseProvider()
	waitForTestSignal(t, responded, "project-changed analysis response")
}

func TestPersistAnalyzeAllJobKeepsNewerStaleState(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	running := &AnalyzeAllJob{
		ProjectID:       "project",
		ProjectRevision: "revision",
		Status:          analysisAllStateRunning,
		MaxFiles:        defaultAnalyzeAllFileLimit,
		MaxRetries:      defaultAnalyzeAllRetries,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		root:            root,
	}
	service.analysisAll.mu.Lock()
	service.analysisAll.job = running
	service.analysisAll.mu.Unlock()

	delayed := cloneAnalyzeAllJob(running)
	service.invalidateAnalyzeAll()
	if err := service.persistAnalyzeAllJob(delayed); err != nil {
		t.Fatal(err)
	}

	persisted, err := loadAnalyzeAllJob(root)
	if err != nil {
		t.Fatal(err)
	}
	if persisted == nil || persisted.Status != analysisAllStateStale {
		t.Fatalf("persisted job after delayed running snapshot = %+v", persisted)
	}
}

func TestAnalyzeAllWorkerDoesNotMutateReplacementJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
	}))
	defer server.Close()

	service, root := newSemanticAnalysisService(t, server.URL, 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	oldWorkerJob := &AnalyzeAllJob{
		ProjectID:       analysis.ProjectID,
		ProjectRevision: analysis.ProjectRevision,
		Status:          analysisAllStateRunning,
		MaxFiles:        defaultAnalyzeAllFileLimit,
		MaxRetries:      defaultAnalyzeAllRetries,
		Files:           []AnalyzeAllFileJob{{Path: "main.go", Status: analysisAllFileRunning, Attempts: 1}},
		CreatedAt:       now,
		UpdatedAt:       now,
		root:            root,
	}
	replacementJob := cloneAnalyzeAllJob(oldWorkerJob)
	replacementJob.Files[0].Status = analysisAllFilePending
	replacementJob.Files[0].Attempts = 0

	replacementWorker, replacementCancel := context.WithCancel(context.Background())
	service.analysisAll.mu.Lock()
	service.analysisAll.job = replacementJob
	service.analysisAll.cancel = replacementCancel
	service.analysisAll.mu.Unlock()

	service.recordAnalyzeAllResult(oldWorkerJob, "main.go", nil)
	service.runAnalyzeAll(context.Background(), oldWorkerJob, make(chan struct{}))

	service.analysisAll.mu.Lock()
	file := service.analysisAll.job.Files[0]
	cancel := service.analysisAll.cancel
	service.analysisAll.mu.Unlock()
	if file.Status != analysisAllFilePending || file.Attempts != 0 {
		t.Fatalf("replacement file after old worker result = %+v", file)
	}
	if cancel == nil {
		t.Fatal("old worker cleared the replacement worker cancellation")
	}

	go service.runAnalyzeAll(replacementWorker, replacementJob, make(chan struct{}))
	job := waitForAnalyzeAll(t, service, analysisAllStateCompleted)
	if len(job.Files) != 1 || job.Files[0].Status != analysisAllFileCompleted {
		t.Fatalf("replacement job did not run normally: %+v", job)
	}
}

func TestAnalyzeAllStartsReplacementWorkerBeforeOldWorkerReturns(t *testing.T) {
	firstStarted := make(chan struct{}, 1)
	secondStarted := make(chan struct{}, 1)
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	var releaseFirstOnce sync.Once
	var releaseSecondOnce sync.Once
	releaseOldWorker := func() { releaseFirstOnce.Do(func() { close(releaseFirst) }) }
	releaseReplacementWorker := func() { releaseSecondOnce.Do(func() { close(releaseSecond) }) }
	var requestsMu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsMu.Lock()
		requests++
		request := requests
		requestsMu.Unlock()
		if request == 1 {
			_, _ = io.Copy(io.Discard, r.Body)
			firstStarted <- struct{}{}
			<-releaseFirst
			_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
			return
		}
		secondStarted <- struct{}{}
		<-releaseSecond
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
	}))
	defer server.Close()
	defer releaseOldWorker()
	defer releaseReplacementWorker()

	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, firstStarted, "first analyze-all request")

	oldCanceled := make(chan struct{})
	var oldCancelOnce sync.Once
	service.analysisAll.mu.Lock()
	oldWorkerCancel := service.analysisAll.cancel
	oldWorkerDone := service.analysisAll.workerDone
	service.analysisAll.cancel = func() { oldCancelOnce.Do(func() { close(oldCanceled) }) }
	service.analysisAll.mu.Unlock()
	if oldWorkerCancel == nil || oldWorkerDone == nil {
		t.Fatal("first analyze-all worker was not installed")
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() { println(\"changed\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, oldCanceled, "old worker cancellation")

	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, secondStarted, "replacement analyze-all request")
	service.analysisAll.mu.Lock()
	replacement := service.analysisAll.job
	replacementFile := replacement.Files[0]
	replacementCancel := service.analysisAll.cancel
	service.analysisAll.mu.Unlock()
	if replacementFile.Status != analysisAllFileRunning || replacementCancel == nil {
		t.Fatalf("replacement worker was not active: job=%+v cancel=%t", replacement, replacementCancel != nil)
	}

	oldWorkerCancel()
	releaseOldWorker()
	waitForTestSignal(t, oldWorkerDone, "old analyze-all worker completion")
	service.analysisAll.mu.Lock()
	replacementFile = service.analysisAll.job.Files[0]
	replacementCancel = service.analysisAll.cancel
	service.analysisAll.mu.Unlock()
	if replacementFile.Status != analysisAllFileRunning || replacementCancel == nil {
		t.Fatalf("old worker changed replacement state: file=%+v cancel=%t", replacementFile, replacementCancel != nil)
	}

	releaseReplacementWorker()
	job := waitForAnalyzeAll(t, service, analysisAllStateCompleted)
	if len(job.Files) != 1 || job.Files[0].Status != analysisAllFileCompleted {
		t.Fatalf("replacement job did not complete: %+v", job)
	}
}

func TestStartAnalyzeAllRequiresRemoteProviderConfirmation(t *testing.T) {
	manager, err := project.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(scopedTestConfig("https://example.com/v1"), manager)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{}, false); err == nil {
		t.Fatal("analyze-all accepted an unconfirmed remote provider")
	}
}

func TestNormalizeAnalyzeAllOptionsPreservesDefaultsAndLimits(t *testing.T) {
	tests := []struct {
		name  string
		input AnalyzeAllOptions
		want  AnalyzeAllOptions
	}{
		{name: "defaults", input: AnalyzeAllOptions{}, want: AnalyzeAllOptions{MaxFiles: defaultAnalyzeAllFileLimit, MaxRetries: defaultAnalyzeAllRetries}},
		{name: "negative values", input: AnalyzeAllOptions{MaxFiles: -1, MaxRetries: -1}, want: AnalyzeAllOptions{MaxFiles: defaultAnalyzeAllFileLimit, MaxRetries: defaultAnalyzeAllRetries}},
		{name: "maximum values", input: AnalyzeAllOptions{MaxFiles: maxAnalyzeAllFileLimit + 1, MaxRetries: maxAnalyzeAllRetries + 1}, want: AnalyzeAllOptions{MaxFiles: maxAnalyzeAllFileLimit, MaxRetries: maxAnalyzeAllRetries}},
		{name: "explicit values", input: AnalyzeAllOptions{MaxFiles: 4, MaxRetries: 2}, want: AnalyzeAllOptions{MaxFiles: 4, MaxRetries: 2}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeAnalyzeAllOptions(test.input); got != test.want {
				t.Fatalf("normalizeAnalyzeAllOptions(%+v) = %+v, want %+v", test.input, got, test.want)
			}
		})
	}
}

const validSemanticAnalysis = `{"purpose":"Explains the file.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{}}`

func waitForAnalyzeAll(t *testing.T, service *Service, want string) *AnalyzeAllJob {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := service.AnalyzeAllJob()
		if err != nil {
			t.Fatal(err)
		}
		if job != nil && job.Status == want {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("analyze-all job did not reach %q", want)
	return nil
}

func waitForAnalyzeAllFile(t *testing.T, service *Service, jobStatus, fileStatus string) *AnalyzeAllJob {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := service.AnalyzeAllJob()
		if err != nil {
			t.Fatal(err)
		}
		if job != nil && job.Status == jobStatus && len(job.Files) > 0 && job.Files[0].Status == fileStatus {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("analyze-all job did not reach %q with first file %q", jobStatus, fileStatus)
	return nil
}

func waitForTestSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForTestError(t *testing.T, done <-chan error, description string) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
		return nil
	}
}

func assertNoTestHandlerError(t *testing.T, handlerErrors <-chan error) {
	t.Helper()
	select {
	case err := <-handlerErrors:
		t.Fatal(err)
	default:
	}
}

func analyzeAllRequestPath(r *http.Request) (string, error) {
	var request llm.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return "", fmt.Errorf("decode LLM request: %w", err)
	}
	if len(request.Messages) != 1 {
		return "", fmt.Errorf("LLM messages = %d, want 1", len(request.Messages))
	}
	const marker = "TARGET_FACTS:\n{\"path\":\""
	prompt := request.Messages[0].Content
	start := strings.Index(prompt, marker)
	if start < 0 {
		return "", fmt.Errorf("target facts missing from semantic prompt")
	}
	path := prompt[start+len(marker):]
	var found bool
	path, _, found = strings.Cut(path, "\"")
	if !found || path == "" {
		return "", fmt.Errorf("target path missing from semantic prompt")
	}
	return path, nil
}

func findAppIndexFile(index *project.ProjectIndex, path string) *project.IndexFile {
	for i := range index.Files {
		if index.Files[i].Path == path {
			return &index.Files[i]
		}
	}
	return nil
}

func newSemanticAnalysisService(t *testing.T, baseURL string, analysisSeconds int) (*Service, string) {
	return newSemanticAnalysisServiceFixture(t, baseURL, analysisSeconds, false)
}

func newSemanticAnalysisServiceWithHelper(t *testing.T, baseURL string, analysisSeconds int) (*Service, string) {
	return newSemanticAnalysisServiceFixture(t, baseURL, analysisSeconds, true)
}

func newSemanticAnalysisServiceFixture(t *testing.T, baseURL string, analysisSeconds int, includeHelper bool) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"run\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if includeHelper {
		if err := os.WriteFile(filepath.Join(root, "helper.go"), []byte("package main\n\nfunc Helper() { println(\"helper secret\") }\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root, Summary: "A compact fixture project."}); err != nil {
		t.Fatal(err)
	}
	cfg := scopedTestConfig(baseURL)
	cfg.ModelScopes.Bug.Model = "analysis-model"
	cfg.Timeouts = config.TimeoutConfig{AnalysisSeconds: analysisSeconds}
	cfg.Retry = config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1}
	service, err := New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	return service, root
}
