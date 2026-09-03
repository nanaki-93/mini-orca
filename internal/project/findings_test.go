package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingIDAndValidationKeepSourcesDistinct(t *testing.T) {
	base := UnifiedFinding{Source: FindingSourceAI, Confidence: FindingConfidenceSuggested, Severity: "medium", Title: "Suggestion", Message: "Check input", Location: FindingLocation{Path: "main.go", StartLine: 4, EndLine: 4}}
	for _, test := range []struct {
		name  string
		value UnifiedFinding
		valid bool
	}{
		{name: "AI suggested", value: base, valid: true},
		{name: "tool reported", value: UnifiedFinding{Source: FindingSourceVet, Confidence: FindingConfidenceToolReported, Severity: "high", Title: "vet", Message: "bad format"}, valid: true},
		{name: "AI cannot claim a tool", value: UnifiedFinding{Source: FindingSourceAI, Confidence: FindingConfidenceToolReported, Severity: "low", Title: "bad", Message: "bad"}, valid: false},
		{name: "tool cannot claim suggestion", value: UnifiedFinding{Source: FindingSourceTest, Confidence: FindingConfidenceSuggested, Severity: "low", Title: "bad", Message: "bad"}, valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := normalizeFinding(test.value, FindingInput{ProjectID: "project", ProjectRevision: "revision"})
			err := validateFinding(value)
			if (err == nil) != test.valid {
				t.Fatalf("validateFinding(%+v) error = %v, want valid %t", value, err, test.valid)
			}
		})
	}
	if FindingID(base) != FindingID(base) {
		t.Fatal("finding ID is not deterministic")
	}
}

func TestFindingStoreReconcilesTriageSanitizationAndFreshness(t *testing.T) {
	root := t.TempDir()
	store, err := NewFindingStore(root)
	if err != nil {
		t.Fatal(err)
	}
	input := FindingInput{ProjectID: "project", ProjectRevision: "revision-one", FileHashes: map[string]string{"main.go": "hash-one"}}
	reported := []UnifiedFinding{{Source: FindingSourceAI, Confidence: FindingConfidenceSuggested, Severity: "medium", Title: "Risk", Message: "password: should-not-persist", Evidence: "api_key: should-not-persist", FileHash: "hash-one", Location: FindingLocation{Path: "main.go", Symbol: "Run"}, OriginatingAnalysis: "file", TaskSpec: &BugTaskSpec{SchemaVersion: BugTaskSpecSchemaVersion, TargetPath: "main.go", TargetSymbol: "Run", TargetSignature: "func()", AcceptanceCriteria: []string{"password: should-not-persist"}, NonGoals: []string{}}}}
	stored, err := store.Reconcile(input, reported)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Status != FindingStatusOpen || stored[0].Freshness != FindingFreshnessFresh || strings.Contains(stored[0].Message+stored[0].Evidence+stored[0].TaskSpec.AcceptanceCriteria[0], "should-not-persist") {
		t.Fatalf("stored findings = %+v", stored)
	}
	if err := store.SetStatus(stored[0].ID, FindingStatusDismissed); err != nil {
		t.Fatal(err)
	}
	rerun, err := store.Reconcile(input, reported)
	if err != nil || rerun[0].Status != FindingStatusDismissed {
		t.Fatalf("triage after unchanged rerun = %+v, %v", rerun, err)
	}
	stale, err := store.Load(FindingInput{ProjectID: "project", ProjectRevision: "revision-two", FileHashes: map[string]string{"main.go": "hash-two"}})
	if err != nil || stale[0].Freshness != FindingFreshnessStale {
		t.Fatalf("changed input findings = %+v, %v", stale, err)
	}
	retired, err := store.Reconcile(input, nil)
	if err != nil || retired[0].Status != FindingStatusRetired || retired[0].Freshness != FindingFreshnessStale {
		t.Fatalf("retired findings = %+v, %v", retired, err)
	}
}

func TestFindingStoreRecoversCorruptStorageAndAdaptsFreshAIRisks(t *testing.T) {
	root := t.TempDir()
	store, err := NewFindingStore(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, findingStorePath)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(FindingInput{ProjectID: "project", ProjectRevision: "revision"})
	if err != nil || len(loaded) != 0 {
		t.Fatalf("recovered findings = %+v, %v", loaded, err)
	}
	corrupt, err := filepath.Glob(path + ".corrupt-*")
	if err != nil || len(corrupt) != 1 {
		t.Fatalf("corrupt recovery = %v, %v", corrupt, err)
	}
	projectFindings := SuggestedFindingsForProject(ProjectAnalysisReport{Status: ProjectAnalysisStatusFresh, Risks: []ProjectAnalysisRisk{{Severity: "low", Summary: "Add tests"}}})
	task := &BugTaskSpec{SchemaVersion: BugTaskSpecSchemaVersion, TargetPath: "main.go", TargetSymbol: "Run", TargetSignature: "func Run()", AcceptanceCriteria: []string{"Return errors."}}
	fileFindings := SuggestedFindingsForFile(FileAnalysis{Status: AnalysisStatusFresh, Path: "main.go", ContentHash: "hash", Symbols: []SymbolInfo{{Name: "Run", StartLine: 4, EndLine: 6}}, Risks: []Finding{{Severity: "high", Summary: "Check errors", TaskSpec: task}}})
	if projectFindings[0].Confidence != FindingConfidenceSuggested || projectFindings[0].Source != FindingSourceAI || fileFindings[0].Location.Path != "main.go" || fileFindings[0].FileHash != "hash" || fileFindings[0].TaskSpec == nil || fileFindings[0].Location.Symbol != "Run" {
		t.Fatalf("adapted findings = %+v %+v", projectFindings, fileFindings)
	}
}
