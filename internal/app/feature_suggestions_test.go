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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const validFeatureResponse = `{"suggestions":[{"title":"Add cancellation-aware work","benefit":"Let callers stop work early.","evidence":"The project exposes a Run entry point.","paths":["main.go"],"effort":"small","acceptance_criteria":["Canceled work returns promptly."]}]}`

func TestFeatureGenerationGetsIndependentExtendedDeadline(t *testing.T) {
	server := changeProvider(t, func() string { return validFeatureResponse })
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	for _, timeout := range []time.Duration{time.Millisecond, 20 * time.Minute} {
		runtime := s.runtimes.analyze
		runtime.effective.Timeout = timeout.String()
		var remaining time.Duration
		report, err := s.generateFeatures(context.Background(), featureRequestFor(t, s, "Improve cancellation."), runtime, nil, featureGenerationAuthority{
			BeforeAttempt: func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("feature request has no bounded deadline")
				}
				remaining = time.Until(deadline)
				return nil
			},
			Publish: func(write func() error) error { return write() },
		})
		if err != nil || len(report.Suggestions) != 1 || remaining < max(10*time.Minute, timeout)-time.Second {
			t.Fatalf("feature request deadline=%s report=%+v err=%v", remaining, report, err)
		}
	}
}

func featureRequestFor(t *testing.T, service *Service, goals string) FeatureRequest {
	t.Helper()
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	report, err := service.Features(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return FeatureRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, ExpectedHash: report.Hash, Goals: goals}
}

func TestFeatureGenerationGoalsAndSavedStatus(t *testing.T) {
	var calls atomic.Int32
	server := changeProvider(t, func() string { calls.Add(1); return validFeatureResponse })
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	request := featureRequestFor(t, service, "Improve cancellation.")
	goals, err := service.SaveFeatureGoals(context.Background(), request)
	if err != nil || goals.Goals != request.Goals || calls.Load() != 0 {
		t.Fatalf("passive goals: %+v, %v", goals, err)
	}
	request.ExpectedHash = goals.Hash
	report, err := service.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: request})
	if err != nil || report.Status != "ready" || len(report.Suggestions) != 1 {
		t.Fatalf("generated features: %+v, %v", report, err)
	}
	request.ExpectedHash = report.Hash
	saved, err := service.UpdateFeatureStatus(context.Background(), report.Suggestions[0].ID, FeatureStatusRequest{FeatureRequest: request, Status: "saved"})
	if err != nil || saved.Suggestions[0].Status != "saved" {
		t.Fatalf("save feature: %+v, %v", saved, err)
	}
	request.ExpectedHash = saved.Hash
	refreshed, err := service.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: request})
	if err != nil || refreshed.Suggestions[0].Status != "saved" {
		t.Fatalf("regeneration lost triage: %+v, %v", refreshed, err)
	}
	passive, err := service.Features(context.Background())
	if err != nil || passive.Freshness != "current" || calls.Load() != 2 {
		t.Fatalf("feature reads dispatch: %+v, %v", passive, err)
	}
	request.ExpectedHash = passive.Hash
	request.Goals = "Different project direction."
	changed, err := service.SaveFeatureGoals(context.Background(), request)
	if err != nil || changed.Freshness != "stale" {
		t.Fatalf("changed goals reused evidence: %+v, %v", changed, err)
	}
}

func TestFeatureMalformedOutputPreservesPreviousSuggestions(t *testing.T) {
	var output atomic.Value
	output.Store(validFeatureResponse)
	server := changeProvider(t, func() string { return output.Load().(string) })
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	good, err := service.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: featureRequestFor(t, service, "Improve the project.")})
	if err != nil {
		t.Fatal(err)
	}
	duplicate := strings.Replace(validFeatureResponse, `}]}`, `},`+strings.TrimSuffix(strings.TrimPrefix(validFeatureResponse, `{"suggestions":[`), `]}`)+`]}`, 1)
	for _, invalid := range []string{`{}`, strings.Replace(validFeatureResponse, "main.go", "invented.go", 1), strings.Replace(validFeatureResponse, "small", "instant", 1), strings.Replace(validFeatureResponse, `"benefit":`, `"status":"verified","benefit":`, 1), duplicate} {
		output.Store(invalid)
		request := featureRequestFor(t, service, "Improve the project.")
		if _, err := service.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: request}); err == nil {
			t.Fatalf("accepted output %s", invalid)
		}
		retained, err := service.Features(context.Background())
		if err != nil || retained.Status != "failed" || retained.Failure != "Feature search failed. Try again." || featureIdeasJSON(t, retained) != featureIdeasJSON(t, good) {
			t.Fatalf("failure lost previous ideas: %+v, %v", retained, err)
		}
	}
}

func TestFeatureFreshnessEmptyAndRemoteConsent(t *testing.T) {
	server := changeProvider(t, func() string { return `{"suggestions":[]}` })
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	report, err := service.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: featureRequestFor(t, service, "Keep the project small.")})
	if err != nil || report.Status != "ready" || len(report.Suggestions) != 0 {
		t.Fatalf("empty success: %+v, %v", report, err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("New project guidance."), 0644); err != nil {
		t.Fatal(err)
	}
	stale, err := service.Features(context.Background())
	if err != nil || stale.Freshness != "stale" {
		t.Fatalf("instruction change not stale: %+v, %v", stale, err)
	}
	remote, _ := newSemanticAnalysisService(t, "https://provider.invalid", 0)
	if _, err := remote.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: featureRequestFor(t, remote, "Improve.")}); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("remote consent: %v", err)
	}
}

type featureTestProvider struct {
	output  atomic.Value
	prompt  atomic.Value
	calls   atomic.Int32
	block   atomic.Bool
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

// newFeatureTestProvider records the user prompt and can hold a response until
// it is released or the request is canceled.
func newFeatureTestProvider(t *testing.T, output string) (*featureTestProvider, string) {
	t.Helper()
	provider := &featureTestProvider{started: make(chan struct{}, 1), release: make(chan struct{})}
	provider.output.Store(output)
	provider.prompt.Store("")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) < 2 {
			http.Error(w, "invalid fixture request", http.StatusBadRequest)
			return
		}
		provider.calls.Add(1)
		provider.prompt.Store(request.Messages[1].Content)
		if provider.block.Load() {
			select {
			case provider.started <- struct{}{}:
			default:
			}
			select {
			case <-provider.release:
			case <-r.Context().Done():
				return
			}
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: provider.output.Load().(string)}}}})
	}))
	t.Cleanup(server.Close)
	t.Cleanup(provider.unblock)
	return provider, server.URL
}

func (provider *featureTestProvider) unblock() { provider.once.Do(func() { close(provider.release) }) }

func featureOutput(titles ...string) string {
	items := make([]map[string]any, 0, len(titles))
	for _, title := range titles {
		items = append(items, map[string]any{"title": title, "benefit": "Benefit of " + title + ".", "evidence": "The project exposes a Run entry point.",
			"paths": []string{"main.go"}, "effort": "small", "acceptance_criteria": []string{"The capability works."}})
	}
	data, err := json.Marshal(map[string]any{"suggestions": items})
	if err != nil {
		panic(err)
	}
	return string(data)
}

func generateFeaturesFor(t *testing.T, s *Service, goals string) (*FeatureReport, error) {
	t.Helper()
	return s.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: featureRequestFor(t, s, goals)})
}

func featureIdeasJSON(t *testing.T, report *FeatureReport) string {
	t.Helper()
	data, err := json.Marshal(report.Suggestions)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func featureHistoryPath(root string) string {
	return filepath.Join(root, ".mini-orca", "changes", "features.json")
}

func featureHistoryBytes(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(featureHistoryPath(root))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeFeatureHistoryBytes(t *testing.T, root string, data []byte) {
	t.Helper()
	if err := storage.WriteFile(featureHistoryPath(root), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureGenerationAddsNewIdeasAndSuppressesDuplicates(t *testing.T) {
	provider, url := newFeatureTestProvider(t, featureOutput("Add cancellation-aware work", "Export analysis reports"))
	s, _ := newSemanticAnalysisService(t, url, 0)
	first, err := generateFeaturesFor(t, s, "Improve.")
	if err != nil || len(first.Suggestions) != 2 || first.LastGeneration == nil || first.LastGeneration.AddedCount != 2 || first.LastGeneration.DuplicateCount != 0 {
		t.Fatalf("first generation=%+v err=%v", first, err)
	}
	firstGeneration := first.LastGeneration.GenerationID
	if len(first.Generations) != 1 || first.Generations[0].ID != firstGeneration || first.Generations[0].Model == nil || first.Generations[0].Model.Profile != "analyze" ||
		first.Generations[0].Model.Model != s.runtimes.analyze.effective.Model || first.Suggestions[0].GenerationID != firstGeneration {
		t.Fatalf("generation provenance=%+v", first.Generations)
	}
	request := featureRequestFor(t, s, "Improve.")
	saved, err := s.UpdateFeatureStatus(context.Background(), first.Suggestions[0].ID, FeatureStatusRequest{FeatureRequest: request, Status: "saved"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedHash = saved.Hash
	if _, err := s.UpdateFeatureStatus(context.Background(), first.Suggestions[1].ID, FeatureStatusRequest{FeatureRequest: request, Status: "dismissed"}); err != nil {
		t.Fatal(err)
	}
	// An exact repeat, a reworded repeat of the dismissed idea, one new idea and
	// a reworded repeat of that new idea within the same response.
	provider.output.Store(featureOutput("Add cancellation-aware work", "export   ANALYSIS reports!", "Schedule nightly analysis", "Schedule nightly analysis."))
	second, err := generateFeaturesFor(t, s, "Improve.")
	if err != nil || second.Status != "ready" || len(second.Suggestions) != 3 {
		t.Fatalf("second generation=%+v err=%v", second, err)
	}
	outcome := second.LastGeneration
	if outcome == nil || outcome.AddedCount != 1 || outcome.DuplicateCount != 3 || outcome.GenerationID == firstGeneration || len(second.Generations) != 2 {
		t.Fatalf("outcome=%+v generations=%+v", outcome, second.Generations)
	}
	kept, added := second.Suggestions[:2], second.Suggestions[2]
	if kept[0].ID != first.Suggestions[0].ID || kept[0].Status != "saved" || kept[1].ID != first.Suggestions[1].ID || kept[1].Status != "dismissed" ||
		kept[0].GenerationID != firstGeneration || kept[1].GenerationID != firstGeneration {
		t.Fatalf("existing ideas changed: %+v", kept)
	}
	if added.Title != "Schedule nightly analysis" || added.Status != "open" || added.GenerationID != outcome.GenerationID || added.Freshness != "current" {
		t.Fatalf("new idea=%+v", added)
	}
	prompt := provider.prompt.Load().(string)
	if !strings.Contains(prompt, "- dismissed: Export analysis reports\n- saved: Add cancellation-aware work\n") {
		t.Fatalf("existing ideas missing from prompt context: %s", prompt)
	}
	passive, err := s.Features(context.Background())
	if err != nil || featureIdeasJSON(t, passive) != featureIdeasJSON(t, second) || provider.calls.Load() != 2 {
		t.Fatalf("stored merge=%+v err=%v calls=%d", passive, err, provider.calls.Load())
	}
}

func TestFeatureGenerationDuplicateOnlyAndEmptyOutputKeepTheList(t *testing.T) {
	provider, url := newFeatureTestProvider(t, validFeatureResponse)
	s, root := newSemanticAnalysisService(t, url, 0)
	first, err := generateFeaturesFor(t, s, "Improve.")
	if err != nil {
		t.Fatal(err)
	}
	ideas := featureIdeasJSON(t, first)
	for _, test := range []struct {
		output     string
		duplicates int
	}{
		{featureOutput("ADD cancellation aware work"), 1},
		{`{"suggestions":[]}`, 0},
	} {
		provider.output.Store(test.output)
		report, err := generateFeaturesFor(t, s, "Improve.")
		if err != nil || report.Status != "ready" || report.LastGeneration.AddedCount != 0 || report.LastGeneration.DuplicateCount != test.duplicates || featureIdeasJSON(t, report) != ideas {
			t.Fatalf("no-new-ideas result=%+v err=%v", report, err)
		}
		// Only the generation that owns ideas and the latest outcome remain.
		if len(report.Generations) != 2 || report.Generations[0].ID != first.LastGeneration.GenerationID || report.Generations[1].ID != report.LastGeneration.GenerationID {
			t.Fatalf("generation records were not pruned: %+v", report.Generations)
		}
	}
	var stored map[string]any
	if err := json.Unmarshal(featureHistoryBytes(t, root), &stored); err != nil {
		t.Fatal(err)
	}
	if stored["schema_version"] != float64(featureHistorySchemaVersion) {
		t.Fatalf("history schema=%v", stored["schema_version"])
	}
	for _, idea := range stored["suggestions"].([]any) {
		if _, persisted := idea.(map[string]any)["freshness"]; persisted {
			t.Fatal("derived idea freshness was persisted")
		}
	}
}

func TestFeatureIdeaFreshnessFollowsItsOwnGeneration(t *testing.T) {
	provider, url := newFeatureTestProvider(t, validFeatureResponse)
	s, root := newSemanticAnalysisService(t, url, 0)
	first, err := generateFeaturesFor(t, s, "Improve cancellation.")
	if err != nil || first.Suggestions[0].Freshness != "current" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	changed, err := s.SaveFeatureGoals(context.Background(), featureRequestFor(t, s, "Ship reports."))
	if err != nil || changed.Freshness != "stale" || changed.Suggestions[0].Freshness != "stale" {
		t.Fatalf("changed goals kept ideas current: %+v, %v", changed, err)
	}
	provider.output.Store(featureOutput("Export analysis reports"))
	second, err := generateFeaturesFor(t, s, "Ship reports.")
	if err != nil || second.Freshness != "current" || second.Suggestions[0].Freshness != "stale" || second.Suggestions[1].Freshness != "current" {
		t.Fatalf("new generation revalidated old ideas: %+v, %v", second, err)
	}
	calls := provider.calls.Load()
	passive, err := s.Features(context.Background())
	if err != nil || featureIdeasJSON(t, passive) != featureIdeasJSON(t, second) || provider.calls.Load() != calls {
		t.Fatalf("passive freshness=%+v err=%v", passive, err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Changed guidance."), 0644); err != nil {
		t.Fatal(err)
	}
	stale, err := s.Features(context.Background())
	if err != nil || stale.Freshness != "stale" || stale.Suggestions[0].Freshness != "stale" || stale.Suggestions[1].Freshness != "stale" {
		t.Fatalf("source change kept ideas current: %+v, %v", stale, err)
	}
}

func legacyFeatureHistory(t *testing.T, s *Service, root, freshness string, suggestions []FeatureSuggestion) []byte {
	t.Helper()
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := benchmarkSourceFingerprint(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	report := FeatureReport{SchemaVersion: legacyFeatureSchemaVersion, ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Hash: contentHash([]byte("legacy fixture")),
		Goals: "Improve.", Status: "ready", Freshness: freshness, Suggestions: suggestions, WorkspaceHash: fingerprint, UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "generations")
	data, err = json.MarshalIndent(fields, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func legacyFeatureIdea(title, padding string) FeatureSuggestion {
	return FeatureSuggestion{ID: contentHash([]byte(title)), Title: title, Benefit: "Benefit of " + title + ".", Evidence: "Legacy evidence." + padding,
		Paths: []string{"main.go"}, Effort: "small", AcceptanceCriteria: []string{"The capability works."}, Status: "open"}
}

func TestFeatureHistoryReadsLegacyV1AndUpgradesOnWrite(t *testing.T) {
	provider, url := newFeatureTestProvider(t, validFeatureResponse)
	s, root := newSemanticAnalysisService(t, url, 0)
	ideas := []FeatureSuggestion{legacyFeatureIdea("Export analysis reports", ""), legacyFeatureIdea("Schedule nightly analysis", "")}
	writeFeatureHistoryBytes(t, root, legacyFeatureHistory(t, s, root, "current", ideas))
	legacy, err := s.Features(context.Background())
	if err != nil || legacy.SchemaVersion != legacyFeatureSchemaVersion || legacy.Hash != contentHash([]byte("legacy fixture")) || len(legacy.Generations) != 1 {
		t.Fatalf("legacy read=%+v err=%v", legacy, err)
	}
	generation := legacy.Generations[0]
	if generation.ID != legacyFeatureGenerationID || generation.Model != nil || generation.GoalsHash != contentHash([]byte("Improve.")) ||
		legacy.Suggestions[0].GenerationID != legacyFeatureGenerationID || legacy.Suggestions[1].Freshness != "current" {
		t.Fatalf("legacy provenance=%+v ideas=%+v", generation, legacy.Suggestions)
	}
	request := featureRequestFor(t, s, "Improve.")
	upgraded, err := s.UpdateFeatureStatus(context.Background(), ideas[0].ID, FeatureStatusRequest{FeatureRequest: request, Status: "dismissed"})
	if err != nil || upgraded.SchemaVersion != featureHistorySchemaVersion || upgraded.Suggestions[0].Status != "dismissed" || upgraded.Suggestions[1].Status != "open" {
		t.Fatalf("legacy triage=%+v err=%v", upgraded, err)
	}
	var stored struct {
		SchemaVersion int              `json:"schema_version"`
		Generations   []map[string]any `json:"generations"`
		Suggestions   []map[string]any `json:"suggestions"`
	}
	data := featureHistoryBytes(t, root)
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("upgraded history=%s err=%v", data, err)
	}
	if stored.SchemaVersion != featureHistorySchemaVersion || len(stored.Generations) != 1 || stored.Generations[0]["model"] != nil || stored.Suggestions[0]["generation_id"] != legacyFeatureGenerationID {
		t.Fatalf("upgraded history=%s", data)
	}

	// Legacy ideas whose goals were already stale never become current.
	writeFeatureHistoryBytes(t, root, legacyFeatureHistory(t, s, root, "stale", ideas))
	staleLegacy, err := s.Features(context.Background())
	if err != nil || staleLegacy.Generations[0].GoalsHash != "" || staleLegacy.Suggestions[0].Freshness != "stale" {
		t.Fatalf("stale legacy=%+v err=%v", staleLegacy, err)
	}
	generated, err := generateFeaturesFor(t, s, "Improve.")
	if err != nil || len(generated.Suggestions) != 3 || generated.Suggestions[0].ID != ideas[0].ID || generated.Suggestions[0].Freshness != "stale" ||
		generated.Suggestions[2].Freshness != "current" || generated.Freshness != "current" || provider.calls.Load() != 1 {
		t.Fatalf("generation over legacy history=%+v err=%v", generated, err)
	}
}

func TestFeatureHistoryRejectsCorruptReferencesWithoutDispatch(t *testing.T) {
	provider, url := newFeatureTestProvider(t, validFeatureResponse)
	s, root := newSemanticAnalysisService(t, url, 0)
	if _, err := generateFeaturesFor(t, s, "Improve."); err != nil {
		t.Fatal(err)
	}
	valid := featureHistoryBytes(t, root)
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown generation", func(report map[string]any) {
			report["suggestions"].([]any)[0].(map[string]any)["generation_id"] = "gen-missing"
		}},
		{"missing generation reference", func(report map[string]any) {
			delete(report["suggestions"].([]any)[0].(map[string]any), "generation_id")
		}},
		{"duplicate generation", func(report map[string]any) {
			report["generations"] = append(report["generations"].([]any), report["generations"].([]any)[0])
		}},
		{"duplicate idea", func(report map[string]any) {
			report["suggestions"] = append(report["suggestions"].([]any), report["suggestions"].([]any)[0])
		}},
		{"unknown last generation", func(report map[string]any) {
			report["last_generation"].(map[string]any)["generation_id"] = "gen-missing"
		}},
		{"impossible outcome", func(report map[string]any) { report["last_generation"].(map[string]any)["added_count"] = 6 }},
		{"missing generations", func(report map[string]any) { delete(report, "generations") }},
		{"legacy with provenance", func(report map[string]any) { report["schema_version"] = 1 }},
		{"future schema", func(report map[string]any) { report["schema_version"] = 3 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var report map[string]any
			if err := json.Unmarshal(valid, &report); err != nil {
				t.Fatal(err)
			}
			test.mutate(report)
			corrupt, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			writeFeatureHistoryBytes(t, root, corrupt)
			if _, err := s.Features(context.Background()); !errors.Is(err, errUnsupportedFeatureHistory) {
				t.Fatalf("corrupt history read=%v", err)
			}
			index, _ := s.manager.Index()
			request := FeatureRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, ExpectedHash: report["hash"].(string), Goals: "Improve."}
			if _, err := s.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: request}); !errors.Is(err, errUnsupportedFeatureHistory) || provider.calls.Load() != 1 {
				t.Fatalf("corrupt history generation=%v calls=%d", err, provider.calls.Load())
			}
			if string(featureHistoryBytes(t, root)) != string(corrupt) {
				t.Fatal("corrupt history was rewritten")
			}
		})
	}
}

// fillFeatureHistory writes a valid v2 report with only slack bytes left below
// the bound, measured with the longest possible update timestamp.
func fillFeatureHistory(t *testing.T, s *Service, root string, slack int) {
	t.Helper()
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := benchmarkSourceFingerprint(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	report := emptyFeatureReport(index)
	report.Status, report.WorkspaceHash = "ready", fingerprint
	report.Generations = []FeatureGeneration{{ID: "gen-fixture", GeneratedAt: time.Now().UTC(), ProjectRevision: index.ProjectRevision, WorkspaceHash: fingerprint, GoalsHash: contentHash(nil)}}
	size := func() int {
		measured := *report
		measured.Hash, measured.UpdatedAt = contentHash(nil), time.Date(2026, 12, 31, 23, 59, 59, 123456789, time.UTC)
		data, err := json.MarshalIndent(measured, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		return len(data)
	}
	for i := 0; size() <= maxFeatureHistoryBytes; i++ {
		idea := legacyFeatureIdea(fmt.Sprintf("Fixture idea %d", i), strings.Repeat("e", 1000))
		idea.GenerationID = "gen-fixture"
		report.Suggestions = append(report.Suggestions, idea)
	}
	report.Suggestions = report.Suggestions[:len(report.Suggestions)-1]
	for maxFeatureHistoryBytes-size() < slack {
		report.Suggestions = report.Suggestions[:len(report.Suggestions)-1]
	}
	last := &report.Suggestions[len(report.Suggestions)-1]
	last.Evidence += strings.Repeat("e", maxFeatureHistoryBytes-size()-slack)
	if err := writeFeatures(root, report); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureHistoryCapacityKeepsTheFileUnchanged(t *testing.T) {
	provider, url := newFeatureTestProvider(t, `{}`)
	s, root := newSemanticAnalysisService(t, url, 0)
	fillFeatureHistory(t, s, root, 16)
	before := featureHistoryBytes(t, root)
	// The failure status cannot fit either, so the generation error is reported.
	if _, err := generateFeaturesFor(t, s, ""); err == nil || errors.Is(err, errFeatureHistoryFull) || !strings.Contains(err.Error(), "invalid feature suggestions") {
		t.Fatalf("malformed output at capacity=%v", err)
	}
	if string(featureHistoryBytes(t, root)) != string(before) {
		t.Fatal("failed generation changed a full history")
	}
	provider.output.Store(featureOutput("A genuinely new capability"))
	if _, err := generateFeaturesFor(t, s, ""); !errors.Is(err, errFeatureHistoryFull) {
		t.Fatalf("capacity result=%v", err)
	}
	if string(featureHistoryBytes(t, root)) != string(before) || provider.calls.Load() != 2 {
		t.Fatal("history-full generation changed the saved report")
	}
	prompt := provider.prompt.Load().(string)
	block := prompt[strings.Index(prompt, "Existing ideas"):strings.Index(prompt, "Project context:")]
	if len(block) > maxExistingFeatureIdeasBytes || !strings.Contains(block, "\n- open: Fixture idea") {
		t.Fatalf("existing ideas block=%d bytes: %s", len(block), block)
	}
}

func TestFeatureLegacyUpgradeOverheadFailsTriageExplicitly(t *testing.T) {
	s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	var ideas []FeatureSuggestion
	padding := strings.Repeat("e", 1000)
	for len(legacyFeatureHistory(t, s, root, "current", ideas)) <= maxFeatureHistoryBytes {
		ideas = append(ideas, legacyFeatureIdea(fmt.Sprintf("Legacy idea %d", len(ideas)), padding))
	}
	ideas = ideas[:len(ideas)-1]
	for maxFeatureHistoryBytes-len(legacyFeatureHistory(t, s, root, "current", ideas)) < 8 {
		ideas = ideas[:len(ideas)-1]
	}
	remaining := maxFeatureHistoryBytes - len(legacyFeatureHistory(t, s, root, "current", ideas))
	ideas[len(ideas)-1].Evidence += strings.Repeat("e", remaining-8)
	legacy := legacyFeatureHistory(t, s, root, "current", ideas)
	writeFeatureHistoryBytes(t, root, legacy)
	request := featureRequestFor(t, s, "Improve.")
	if _, err := s.UpdateFeatureStatus(context.Background(), ideas[0].ID, FeatureStatusRequest{FeatureRequest: request, Status: "saved"}); !errors.Is(err, errFeatureHistoryFull) {
		t.Fatalf("legacy upgrade overhead=%v", err)
	}
	if _, err := s.SaveFeatureGoals(context.Background(), request); !errors.Is(err, errFeatureHistoryFull) {
		t.Fatalf("legacy goals upgrade overhead=%v", err)
	}
	report, err := s.Features(context.Background())
	if err != nil || string(featureHistoryBytes(t, root)) != string(legacy) || report.SchemaVersion != legacyFeatureSchemaVersion || report.Suggestions[0].Status != "open" {
		t.Fatalf("legacy history changed: %+v, %v", report, err)
	}
}

func TestFeatureGenerationCancellationAndTimeoutKeepTheFile(t *testing.T) {
	for _, test := range []struct {
		name string
		want error
	}{{"cancel", context.Canceled}, {"timeout", context.DeadlineExceeded}} {
		t.Run(test.name, func(t *testing.T) {
			provider, url := newFeatureTestProvider(t, validFeatureResponse)
			s, root := newSemanticAnalysisService(t, url, 0)
			if _, err := generateFeaturesFor(t, s, "Improve."); err != nil {
				t.Fatal(err)
			}
			before := featureHistoryBytes(t, root)
			provider.output.Store(featureOutput("Export analysis reports"))
			provider.block.Store(true)
			var ctx context.Context
			var cancel context.CancelFunc
			if test.want == context.Canceled {
				ctx, cancel = context.WithCancel(context.Background())
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), time.Second)
			}
			defer cancel()
			done := make(chan error, 1)
			request := featureRequestFor(t, s, "Improve.")
			go func() {
				_, err := s.GenerateFeatures(ctx, FeatureGenerateRequest{FeatureRequest: request})
				done <- err
			}()
			waitForTestSignal(t, provider.started, "blocked feature request")
			if test.want == context.Canceled {
				cancel()
			}
			if err := <-done; !errors.Is(err, test.want) {
				t.Fatalf("generation error=%v", err)
			}
			if string(featureHistoryBytes(t, root)) != string(before) {
				t.Fatal("interrupted generation changed the saved report")
			}
		})
	}
}

func TestFeatureGenerationRejectsConcurrentReportChange(t *testing.T) {
	provider, url := newFeatureTestProvider(t, validFeatureResponse)
	s, root := newSemanticAnalysisService(t, url, 0)
	first, err := generateFeaturesFor(t, s, "Improve.")
	if err != nil {
		t.Fatal(err)
	}
	provider.output.Store(featureOutput("Export analysis reports"))
	provider.block.Store(true)
	done := make(chan error, 1)
	request := featureRequestFor(t, s, "Improve.")
	go func() {
		_, err := s.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: request})
		done <- err
	}()
	waitForTestSignal(t, provider.started, "blocked feature request")
	if _, err := s.UpdateFeatureStatus(context.Background(), first.Suggestions[0].ID, FeatureStatusRequest{FeatureRequest: request, Status: "saved"}); err != nil {
		t.Fatal(err)
	}
	triaged := featureHistoryBytes(t, root)
	provider.unblock()
	if err := <-done; !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("late generation over newer triage=%v", err)
	}
	report, err := s.Features(context.Background())
	if err != nil || string(featureHistoryBytes(t, root)) != string(triaged) || len(report.Suggestions) != 1 || report.Suggestions[0].Status != "saved" {
		t.Fatalf("newer triage overwritten: %+v, %v", report, err)
	}
}

func TestFeatureGenerationPersistenceFailureKeepsTheFile(t *testing.T) {
	provider, url := newFeatureTestProvider(t, validFeatureResponse)
	s, root := newSemanticAnalysisService(t, url, 0)
	if _, err := generateFeaturesFor(t, s, "Improve."); err != nil {
		t.Fatal(err)
	}
	before := featureHistoryBytes(t, root)
	directory := filepath.Dir(featureHistoryPath(root))
	if err := os.Chmod(directory, 0500); err != nil {
		t.Skipf("cannot create read-only metadata fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0700) })
	if probe, err := os.CreateTemp(directory, "probe-*"); err == nil {
		_ = probe.Close()
		t.Skip("test process can write to a read-only directory")
	}
	provider.output.Store(featureOutput("Export analysis reports"))
	_, err := generateFeaturesFor(t, s, "Improve.")
	if err == nil || errors.Is(err, errFeatureHistoryFull) || !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("persistence failure=%v", err)
	}
	if string(featureHistoryBytes(t, root)) != string(before) {
		t.Fatal("failed persistence changed the saved report")
	}
}

func TestFeatureGenerationBudgetCountsExistingIdeas(t *testing.T) {
	provider, url := newFeatureTestProvider(t, featureOutput("Add cancellation-aware work", "Export analysis reports"))
	s, root := newSemanticAnalysisService(t, url, 0)
	report, err := generateFeaturesFor(t, s, "Improve.")
	if err != nil {
		t.Fatal(err)
	}
	text, _, _, err := featureContext(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := len(text) + len("Improve.")
	limit := (base + len(existingFeatureIdeas(nil)) + 3) / 4
	if base+len(existingFeatureIdeas(report.Suggestions)) <= limit*4 {
		t.Fatal("fixture ideas do not exceed the budget")
	}
	s.runtimes.analyze.effective.ContextMaxTokens = limit
	before := featureHistoryBytes(t, root)
	_, err = generateFeaturesFor(t, s, "Improve.")
	if err == nil || !strings.Contains(err.Error(), "existing ideas") || !strings.Contains(err.Error(), fmt.Sprintf("%d-token", limit)) ||
		!strings.Contains(err.Error(), s.runtimes.analyze.effective.Model) || provider.calls.Load() != 1 {
		t.Fatalf("budget error=%v calls=%d", err, provider.calls.Load())
	}
	if string(featureHistoryBytes(t, root)) != string(before) {
		t.Fatal("budget rejection changed the saved report")
	}
}

func TestFeatureTitleNormalizationAndMergeRules(t *testing.T) {
	for title, want := range map[string]string{
		"  Add  Cancellation-Aware Work! ": "add cancellation aware work",
		"Ünïcode ÑAME":                     "ünïcode ñame",
		"v2.0 export":                      "v2 0 export",
		"!!! ***":                          "",
	} {
		if got := normalizedFeatureTitle(title); got != want {
			t.Fatalf("normalizedFeatureTitle(%q)=%q want %q", title, got, want)
		}
	}
	existing := []FeatureSuggestion{{ID: "a", Title: "Export reports", Status: "dismissed", GenerationID: "gen-old"}}
	candidates := []FeatureSuggestion{{ID: "b", Title: "!!!", Status: "open"}, {ID: "c", Title: "???", Status: "open"}, {ID: "d", Title: "export-reports", Status: "open"}}
	merged, outcome := mergeFeatureSuggestions(existing, candidates, "gen-new")
	if len(merged) != 3 || merged[0].ID != "a" || merged[0].Status != "dismissed" || merged[0].GenerationID != "gen-old" || merged[1].ID != "b" || merged[2].ID != "c" ||
		merged[2].GenerationID != "gen-new" || outcome.AddedCount != 2 || outcome.DuplicateCount != 1 {
		t.Fatalf("merged=%+v outcome=%+v", merged, outcome)
	}
	if existing[0].Status != "dismissed" || len(existing) != 1 {
		t.Fatal("merge mutated existing ideas")
	}
}

func TestExistingFeatureIdeasBlockIsBoundedNewestFirst(t *testing.T) {
	ideas := make([]FeatureSuggestion, 200)
	for i := range ideas {
		ideas[i] = FeatureSuggestion{Title: fmt.Sprintf("Idea %03d %s\nsecond line", i, strings.Repeat("x", 80)), Status: "open"}
	}
	ideas[199].Status = "dismissed"
	block := existingFeatureIdeas(ideas)
	lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
	if len(block) > maxExistingFeatureIdeasBytes || !strings.HasPrefix(lines[1], "- dismissed: Idea 199 ") || !strings.HasPrefix(lines[2], "- open: Idea 198 ") {
		t.Fatalf("block=%d bytes, first lines=%q", len(block), lines[:3])
	}
	listed := len(lines) - 2
	if want := fmt.Sprintf("%d older ideas omitted.", len(ideas)-listed); lines[len(lines)-1] != want {
		t.Fatalf("omission line=%q want %q", lines[len(lines)-1], want)
	}
}

func TestFeatureGenerationWithProfileAndSelection(t *testing.T) {
	server := changeProvider(t, func() string {
		return `{"suggestions":[{"title":"T1","benefit":"B","evidence":"E","paths":["main.go"],"effort":"small","acceptance_criteria":["A"]}]}`
	})
	s, _ := newSemanticAnalysisService(t, server.URL, 0)

	index, _ := s.manager.Index()
	request := FeatureGenerateRequest{
		FeatureRequest: FeatureRequest{
			ProjectID:             index.ProjectID,
			ProjectRevision:       index.ProjectRevision,
			ExpectedHash:          "empty",
			Goals:                 "Goals",
			ConfirmRemoteProvider: true,
		},
		Profile: "analyze",
	}

	// 1. Profile routing to chosen provider
	report, err := s.GenerateFeatures(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(report.Suggestions))
	}
	if report.LastGeneration.GenerationID == "" {
		t.Fatal("missing generation ID")
	}

	// Check that the provider origin and model are captured
	var gen *FeatureGeneration
	for _, g := range report.Generations {
		if g.ID == report.LastGeneration.GenerationID {
			gen = &g
			break
		}
	}
	if gen == nil || gen.Model == nil || gen.Model.Profile != "analyze" {
		t.Fatal("expected profile model to be captured")
	}

	// 2. Unconfigured profile
	request2 := request
	request2.Profile = "invalid_profile"
	_, err = s.GenerateFeatures(context.Background(), request2)
	if err == nil || !strings.Contains(err.Error(), "invalid feature profile") {
		t.Fatalf("expected invalid profile error, got %v", err)
	}

	// 3. Selection exclusions applied to suggested paths
	server2 := changeProvider(t, func() string {
		return `{"suggestions":[{"title":"T1","benefit":"B","evidence":"E","paths":["excluded.go"],"effort":"small","acceptance_criteria":["A"]}]}`
	})
	s2, _ := newSemanticAnalysisService(t, server2.URL, 0)

	os.WriteFile(filepath.Join(s2.manager.Root(), "excluded.go"), []byte("package main\n"), 0644)
	s2.manager.Reindex()
	index2, _ := s2.manager.Index()

	emptySelection, _ := s2.ReadAnalysisSelection(context.Background(), index2.ProjectID, index2.ProjectRevision)
	selReq := AnalysisSelectionRequest{
		ProjectID:       index2.ProjectID,
		ProjectRevision: index2.ProjectRevision,
		SelectionID:     emptySelection.SelectionID,
		ExcludedPaths:   []string{"excluded.go"},
	}
	sel, err := s2.SaveAnalysisSelection(context.Background(), selReq)
	if err != nil {
		t.Fatal(err)
	}

	request3 := FeatureGenerateRequest{
		FeatureRequest: FeatureRequest{
			ProjectID:             index2.ProjectID,
			ProjectRevision:       index2.ProjectRevision,
			ExpectedHash:          "empty",
			Goals:                 "Goals",
			ConfirmRemoteProvider: true,
		},
		Profile:             "analyze",
		AnalysisSelectionID: sel.SelectionID,
	}

	_, err = s2.GenerateFeatures(context.Background(), request3)
	// The provider returns a path "excluded.go", which should be rejected
	if err == nil || !strings.Contains(err.Error(), "feature suggestion targets an excluded analysis file") {
		t.Fatalf("expected excluded path error, got %v", err)
	}

	// 4. Stale selection id gives 409
	request4 := request3
	request4.AnalysisSelectionID = "stale_id"
	_, err = s2.GenerateFeatures(context.Background(), request4)
	if err == nil || !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("expected ErrRevisionConflict for stale selection ID, got %v", err)
	}
}
